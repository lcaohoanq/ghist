package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+filepath.Join(dir, "absent-config"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func call(t *testing.T, req request, result any) *protocolError {
	t.Helper()
	in, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := serve(context.Background(), bytes.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	var res struct {
		ProtocolVersion int             `json:"protocolVersion"`
		Result          json.RawMessage `json:"result"`
		Error           *protocolError  `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("invalid JSON response %q: %v", out.String(), err)
	}
	if res.ProtocolVersion != 1 {
		t.Fatalf("protocol version = %d", res.ProtocolVersion)
	}
	if res.Error == nil && result != nil {
		if err := json.Unmarshal(res.Result, result); err != nil {
			t.Fatal(err)
		}
	}
	return res.Error
}

func TestHistoryAndBothSnapshotSides(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-b", "main")
	git(t, dir, "config", "user.name", "Test Author")
	git(t, dir, "config", "user.email", "test@example.com")
	git(t, dir, "config", "commit.gpgsign", "false")
	old, renamed := "old file.txt", "đổi [x] $file.txt"
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(subject string) {
		t.Helper()
		git(t, dir, "add", "-A")
		git(t, dir, "commit", "-m", subject)
	}
	write(old, "first\nsecond\nthird\n")
	commit("create")
	git(t, dir, "mv", old, renamed)
	commit("rename")
	write(renamed, "first\nchanged\nthird\n")
	commit("edit")
	git(t, dir, "rm", renamed)
	commit("delete")
	write("untracked", "keep me")
	before := git(t, dir, "status", "--porcelain=v1")
	var h fileHistory
	if err := call(t, request{ProtocolVersion: 1, Operation: "history", FilePath: filepath.Join(dir, renamed)}, &h); err != nil {
		t.Fatal(err)
	}
	if len(h.Versions) != 4 || h.Path != renamed || h.Head != git(t, dir, "rev-parse", "HEAD") {
		t.Fatalf("history: %+v", h)
	}
	if h.Versions[2].BeforePath != old || h.Versions[2].AfterPath != renamed {
		t.Fatal("rename paths were lost")
	}
	// Read the exact parent of each commit, not the previous item in the list.
	wantBefore := []string{"first\nchanged\nthird\n", "first\nsecond\nthird\n", "first\nsecond\nthird\n", ""}
	wantAfter := []string{"", "first\nchanged\nthird\n", "first\nsecond\nthird\n", "first\nsecond\nthird\n"}
	for i, v := range h.Versions {
		for _, side := range []struct{ revision, path, content string }{
			{v.Parent, v.BeforePath, wantBefore[i]},
			{v.Hash, v.AfterPath, wantAfter[i]},
		} {
			if side.path == "" || side.revision == "" {
				if side.content != "" {
					t.Fatal("nonempty content for an absent side")
				}
				continue
			}
			var s snapshot
			if err := call(t, request{ProtocolVersion: 1, Operation: "snapshot", Root: h.Root, Revision: side.revision, Path: side.path}, &s); err != nil {
				t.Fatal(err)
			}
			if s.Content != side.content || !s.Exists || s.Binary || s.Truncated {
				t.Fatalf("snapshot at %s:%s = %+v", side.revision, side.path, s)
			}
		}
	}
	if got := git(t, dir, "status", "--porcelain=v1"); got != before {
		t.Fatalf("working tree changed: %q -> %q", before, got)
	}
	if err := call(t, request{ProtocolVersion: 1, Operation: "history", FilePath: filepath.Join(dir, "untracked")}, nil); err == nil || err.Code != "no_history" {
		t.Fatalf("untracked error: %+v", err)
	}
	for _, tc := range []struct {
		name, content     string
		binary, truncated bool
	}{
		{"binary", "\x00\x01", true, false},
		{"large", strings.Repeat("a", (2<<20)+1), false, true},
	} {
		write(tc.name, tc.content)
		commit(tc.name)
		var s snapshot
		if err := call(t, request{ProtocolVersion: 1, Operation: "snapshot", Root: h.Root, Revision: git(t, dir, "rev-parse", "HEAD"), Path: tc.name}, &s); err != nil {
			t.Fatal(err)
		}
		if s.Binary != tc.binary || s.Truncated != tc.truncated {
			t.Fatalf("%s flags: %+v", tc.name, s)
		}
	}
}

func TestReviews(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper uses a POSIX shell script")
	}
	dir := t.TempDir()
	git(t, dir, "init", "-b", "main")
	git(t, dir, "config", "user.name", "Test Author")
	git(t, dir, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "file.txt")
	git(t, dir, "commit", "-m", "create")
	git(t, dir, "remote", "add", "origin", "https://github.com/acme/repo.git")
	hash := git(t, dir, "rev-parse", "HEAD")
	bin := filepath.Join(dir, "test-bin")
	if err := os.Mkdir(bin, 0755); err != nil {
		t.Fatal(err)
	}
	gh := filepath.Join(bin, "gh")
	response := `[{"number":12,"html_url":"https://github.com/acme/repo/pull/12"}]`
	if err := os.WriteFile(gh, []byte("#!/bin/sh\nprintf '%s\\n' '"+response+"'\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	var got reviews
	if err := call(t, request{ProtocolVersion: 1, Operation: "reviews", Root: dir, Revision: hash}, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Links) != 1 || got.Links[0].Provider != "github" || got.Links[0].Number != 12 || got.Links[0].URL != "https://github.com/acme/repo/pull/12" {
		t.Fatalf("reviews = %+v", got)
	}
}

func TestInvalidRequests(t *testing.T) {
	for _, input := range []string{
		`{`, `null`, `{}`, `{"protocolVersion":2,"operation":"history"}`,
		`{"protocolVersion":1,"operation":"unknown"}`,
		`{"protocolVersion":1,"operation":"history","filePath":"relative"}`,
		`{"protocolVersion":1,"operation":"history","unknown":true}`,
		`{"protocolVersion":1,"operation":"reviews","root":"/tmp","revision":"not-a-hash"}`,
		`{"protocolVersion":1,"operation":"history"} {}`,
	} {
		t.Run(input, func(t *testing.T) {
			var out bytes.Buffer
			if err := serve(context.Background(), strings.NewReader(input), &out); err != nil {
				t.Fatal(err)
			}
			var res response
			if err := json.Unmarshal(out.Bytes(), &res); err != nil || res.Error == nil || res.Result != nil {
				t.Fatalf("expected JSON error, got %s (%v)", out.String(), err)
			}
		})
	}
	for _, name := range []string{"", "../file", "a/../../file", "/absolute", "a/../file", ".", ".."} {
		err := call(t, request{ProtocolVersion: 1, Operation: "snapshot", Root: t.TempDir(), Revision: "--help", Path: name}, nil)
		if err == nil || !strings.Contains(err.Message, "repository-relative") {
			t.Fatalf("accepted path %q: %+v", name, err)
		}
	}
}

func TestOversizedRequest(t *testing.T) {
	var out bytes.Buffer
	if err := serve(context.Background(), strings.NewReader(strings.Repeat(" ", 1<<20)+"{}"), &out); err != nil {
		t.Fatal(err)
	}
	var res response
	if err := json.Unmarshal(out.Bytes(), &res); err != nil || res.Error == nil || !strings.Contains(res.Error.Message, "1 MiB") {
		t.Fatalf("expected size error, got %s (%v)", out.String(), err)
	}
}
