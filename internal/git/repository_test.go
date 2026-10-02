package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcaohoanq/ghist/internal/history"
)

func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-b", "main")
	gitCmd(t, dir, "config", "user.name", "Test Author")
	gitCmd(t, dir, "config", "user.email", "test@example.com")
	gitCmd(t, dir, "config", "commit.gpgsign", "false")
	return dir
}
func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func write(t *testing.T, dir, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, path)), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
func commit(t *testing.T, dir, message string) string {
	t.Helper()
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", message)
	return gitCmd(t, dir, "rev-parse", "HEAD")
}
func explore(t *testing.T, path string) (*Repository, history.FileHistory) {
	t.Helper()
	r, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := r.FileHistory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return r, h
}
func TestRenameTimeline(t *testing.T) {
	dir := fixture(t)
	ctx := context.Background()
	old, newPath := "old file.go", "nested/đổi [x]*\nfile.go"
	write(t, dir, old, "one\ntwo\nthree\n")
	a := commit(t, dir, "create")
	write(t, dir, old, "one\ntwo edited\nthree\n")
	b := commit(t, dir, "modify")
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "mv", old, newPath)
	c := commit(t, dir, "rename")
	write(t, dir, newPath, "one\ntwo edited\nthree\nfour\n")
	d := commit(t, dir, "modify again")
	gitCmd(t, dir, "rm", newPath)
	e := commit(t, dir, "delete")
	r, h := explore(t, filepath.Join(dir, newPath))
	want := []string{e, d, c, b, a}
	if len(h.Versions) != len(want) {
		t.Fatalf("versions: %+v", h.Versions)
	}
	contents := []string{"", "one\ntwo edited\nthree\nfour\n", "one\ntwo edited\nthree\n", "one\ntwo edited\nthree\n", "one\ntwo\nthree\n"}
	for i, v := range h.Versions {
		if v.Commit.Hash != want[i] {
			t.Fatalf("version %d: %s", i, v.Commit.Hash)
		}
		snap, err := r.Snapshot(ctx, v)
		if err != nil {
			t.Fatal(err)
		}
		if snap.Content != contents[i] || snap.Exists != (i != 0) {
			t.Fatalf("snapshot %d: %+v", i, snap)
		}
		diff, err := r.Diff(ctx, v)
		if err != nil {
			t.Fatal(err)
		}
		if diff.Patch == "" {
			t.Fatalf("empty diff at %d", i)
		}
	}
	if h.Versions[2].BeforePath != old || h.Versions[2].AfterPath != newPath {
		t.Fatal("rename paths lost")
	}
	if h.Versions[4].Parent != "" || h.Versions[4].BeforePath != "" {
		t.Fatal("root commit should have no before side")
	}
	diff, _ := r.Diff(ctx, h.Versions[2])
	if !strings.Contains(diff.Patch, "rename from") {
		t.Fatal(diff.Patch)
	}
}
func TestErrorsAndContent(t *testing.T) {
	ctx := context.Background()
	dir := fixture(t)
	if _, err := Open(ctx, filepath.Join(dir, "missing")); !errors.Is(err, history.ErrNoCommits) {
		t.Fatal(err)
	}
	write(t, dir, "empty", "")
	write(t, dir, "binary", string([]byte{0, 1, 2}))
	write(t, dir, "-file", "text\n")
	commit(t, dir, "files")
	for _, name := range []string{"empty", "binary", "-file"} {
		r, h := explore(t, filepath.Join(dir, name))
		s, err := r.Snapshot(ctx, h.Versions[0])
		if err != nil {
			t.Fatal(err)
		}
		if !s.Exists || s.Binary != (name == "binary") {
			t.Fatalf("%s: %+v", name, s)
		}
		v := h.Versions[0]
		v.Commit.Hash = "--help"
		if _, err := r.Snapshot(ctx, v); err == nil {
			t.Fatal("accepted invalid hash")
		}
		v.Commit.Hash = strings.Repeat("f", 40)
		if _, err := r.Diff(ctx, v); err == nil {
			t.Fatal("accepted nonexistent commit")
		}
	}
	r, err := Open(ctx, filepath.Join(dir, "missing"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.FileHistory(ctx); !errors.Is(err, history.ErrNoHistory) {
		t.Fatal(err)
	}
	write(t, dir, "untracked", "x")
	r, err = Open(ctx, filepath.Join(dir, "untracked"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.FileHistory(ctx); !errors.Is(err, history.ErrNoHistory) {
		t.Fatal(err)
	}
	if _, err = Open(ctx, dir); err == nil {
		t.Fatal("accepted directory")
	}
	if _, err = Open(ctx, filepath.Join(t.TempDir(), "file")); !errors.Is(err, history.ErrNotRepository) {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = Open(cancelled, filepath.Join(dir, "empty")); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestMergeFirstParent(t *testing.T) {
	dir := fixture(t)
	write(t, dir, "file", "base\n")
	commit(t, dir, "base")
	gitCmd(t, dir, "checkout", "-b", "side")
	write(t, dir, "file", "side\n")
	commit(t, dir, "side")
	gitCmd(t, dir, "checkout", "main")
	write(t, dir, "file", "main\n")
	main := commit(t, dir, "main")
	cmd := exec.Command("git", "merge", "--no-ff", "side")
	cmd.Dir = dir
	if err := cmd.Run(); err == nil {
		t.Fatal("expected conflict")
	}
	write(t, dir, "file", "resolved\n")
	merge := commit(t, dir, "resolve")
	r, h := explore(t, filepath.Join(dir, "file"))
	v := h.Versions[0]
	if v.Commit.Hash != merge || !v.Merge || v.Parent != main {
		t.Fatalf("merge: %+v", v)
	}
	d, err := r.Diff(context.Background(), v)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.Patch, "-main") || !strings.Contains(d.Patch, "+resolved") {
		t.Fatal(d.Patch)
	}
}
func TestRelativePathAndFrozenHead(t *testing.T) {
	dir := fixture(t)
	write(t, dir, "sub/file", "first\n")
	commit(t, dir, "first")
	t.Chdir(filepath.Join(dir, "sub"))
	r, err := Open(context.Background(), "file")
	if err != nil {
		t.Fatal(err)
	}
	write(t, dir, "sub/file", "second\n")
	commit(t, dir, "second")
	h, err := r.FileHistory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if h.Path != "sub/file" || len(h.Versions) != 1 {
		t.Fatalf("%+v", h)
	}
}

func TestRenameAcrossMerge(t *testing.T) {
	dir := fixture(t)
	write(t, dir, "old", "a\nb\nc\nd\ne\n")
	commit(t, dir, "base")
	gitCmd(t, dir, "checkout", "-b", "side")
	gitCmd(t, dir, "mv", "old", "new")
	commit(t, dir, "rename")
	write(t, dir, "new", "side\nb\nc\nd\ne\n")
	commit(t, dir, "side edit")
	gitCmd(t, dir, "checkout", "main")
	write(t, dir, "old", "main\nb\nc\nd\ne\n")
	commit(t, dir, "main edit")
	cmd := exec.Command("git", "merge", "--no-ff", "side")
	cmd.Dir = dir
	_ = cmd.Run()
	write(t, dir, "new", "resolved\nb\nc\nd\ne\n")
	if err := os.Remove(filepath.Join(dir, "old")); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	commit(t, dir, "merge resolved")
	r, h := explore(t, filepath.Join(dir, "new"))
	for _, v := range h.Versions {
		s, err := r.Snapshot(context.Background(), v)
		if err != nil {
			t.Fatalf("%s (%s): %v", v.Commit.Subject, v.Path(), err)
		}
		if !s.Exists {
			t.Fatalf("unexpected deletion in %s", v.Commit.Subject)
		}
	}
}
