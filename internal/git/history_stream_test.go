package git

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lcaohoanq/ghist/internal/history"
)

func TestHistoryUsesOneGitProcess(t *testing.T) {
	dir := fixture(t)
	for i := 0; i < 8; i++ {
		write(t, dir, "file", fmt.Sprint(i))
		commit(t, dir, "change")
	}
	r, err := Open(context.Background(), filepath.Join(dir, "file"))
	if err != nil {
		t.Fatal(err)
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	shim := t.TempDir()
	trace := filepath.Join(shim, "trace")
	if err := os.WriteFile(filepath.Join(shim, "git"), []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$GHIST_TEST_TRACE\"\nexec \"$GHIST_TEST_GIT\" \"$@\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GHIST_TEST_GIT", realGit)
	t.Setenv("GHIST_TEST_TRACE", trace)
	t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
	h, err := r.FileHistory(context.Background())
	if err != nil || len(h.Versions) != 8 {
		t.Fatalf("%d versions: %v", len(h.Versions), err)
	}
	data, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "\n") != 1 {
		t.Fatalf("extra Git calls: %s", data)
	}
	ctx, cancel := context.WithCancel(context.Background())
	err = r.StreamHistory(ctx, func(history.FileVersion) error { cancel(); return ctx.Err() })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestParseHistoryStreamsAndPreservesPaths(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	got := make(chan history.FileVersion, 2)
	done := make(chan error, 1)
	go func() {
		done <- parseHistory(reader, "unused", func(v history.FileVersion) error { got <- v; return nil })
	}()
	hash := strings.Repeat("a", 40)
	old := "old\t\nname"
	next := "new\n\tname"
	first := "\x00" + hash + "\x00\x00Author\x00mail\x002026-01-01T00:00:00Z\x00subject\x00\nR100\x00" + old + "\x00" + next + "\x00\x00\x00" + hash + "\x00"
	go func() { _, _ = io.WriteString(writer, first) }()
	select {
	case v := <-got:
		if v.BeforePath != old || v.AfterPath != next {
			t.Fatalf("paths: %+v", v)
		}
	case <-time.After(time.Second):
		t.Fatal("waited for entire history")
	}
	writer.Close() // deliberately truncate the next commit's metadata
	if err := <-done; err == nil {
		t.Fatal("accepted truncated metadata")
	}
}

func TestRenameOnMergedBranchUsesRecordPaths(t *testing.T) {
	dir := fixture(t)
	write(t, dir, "old", "one\ntwo\nthree\nfour\nfive\n")
	commit(t, dir, "base")
	gitCmd(t, dir, "checkout", "-b", "side")
	gitCmd(t, dir, "mv", "old", "new")
	commit(t, dir, "rename")
	write(t, dir, "new", "one\ntwo\nthree\nfour\nside\n")
	commit(t, dir, "side edit")
	gitCmd(t, dir, "checkout", "main")
	write(t, dir, "unrelated", "main")
	commit(t, dir, "unrelated main")
	gitCmd(t, dir, "merge", "--no-ff", "side", "-m", "merge")
	r, h := explore(t, filepath.Join(dir, "new"))
	for _, v := range h.Versions {
		if v.BeforePath != "" && v.Parent != "" {
			gitCmd(t, dir, "cat-file", "-e", v.Parent+":"+v.BeforePath)
		}
		if _, err := r.Snapshot(context.Background(), v); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFastHistoryDoesNotFollowRenamesEvenWithGitConfig(t *testing.T) {
	dir := fixture(t)
	write(t, dir, "old", "one\ntwo\nthree\n")
	original := commit(t, dir, "create")
	gitCmd(t, dir, "mv", "old", "new")
	renamed := commit(t, dir, "rename")
	gitCmd(t, dir, "config", "log.follow", "true")
	gitCmd(t, dir, "config", "diff.renames", "true")
	r, err := Open(context.Background(), filepath.Join(dir, "new"))
	if err != nil {
		t.Fatal(err)
	}
	h, err := r.FileHistory(context.Background())
	if err != nil || len(h.Versions) != 1 {
		t.Fatalf("fast history: %+v %v", h, err)
	}
	if v := h.Versions[0]; v.Commit.Hash != renamed || v.Change != "A" || v.BeforePath != "" {
		t.Fatalf("fast rename treated as %+v", v)
	}
	d, err := r.Diff(context.Background(), h.Versions[0])
	if err != nil || !strings.Contains(d.Patch, "new file mode") {
		t.Fatalf("fast diff: %+v %v", d, err)
	}
	r.FollowRenames = true
	h, err = r.FileHistory(context.Background())
	if err != nil || len(h.Versions) != 2 || h.Versions[1].Commit.Hash != original || h.Versions[0].BeforePath != "old" {
		t.Fatalf("follow history: %+v %v", h, err)
	}
	d, err = r.Diff(context.Background(), h.Versions[0])
	if err != nil || !strings.Contains(d.Patch, "rename from old") {
		t.Fatalf("follow diff: %+v %v", d, err)
	}
}
