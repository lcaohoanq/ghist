package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/lcaohoanq/ghist/internal/history"
)

func TestFilesFromFrozenHead(t *testing.T) {
	ctx := context.Background()
	dir := fixture(t)
	names := []string{".hidden", "-file", "sub/đổi [x]*\nfile.go", "space file"}
	for _, name := range names {
		write(t, dir, name, "content")
	}
	if err := os.Symlink("space file", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	names = append(names, "link")
	head := commit(t, dir, "files")
	// A gitlink is a submodule entry, not a selectable blob.
	gitCmd(t, dir, "update-index", "--add", "--cacheinfo", "160000,"+head+",module")
	gitCmd(t, dir, "commit", "-m", "submodule")
	repo, err := OpenDirectory(ctx, filepath.Join(dir, "sub"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "space file")); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "untracked", "new")
	write(t, dir, "staged", "new")
	gitCmd(t, dir, "add", "staged")
	files, err := repo.Files(ctx)
	sort.Strings(names)
	if err != nil || !reflect.DeepEqual(files, names) {
		t.Fatalf("files = %q, %v; want %q", files, err, names)
	}
	gitCmd(t, dir, "commit", "-m", "later")
	files, err = repo.Files(ctx)
	if err != nil || !reflect.DeepEqual(files, names) {
		t.Fatal("captured HEAD changed", files, err)
	}
	repo.Path = "space file"
	if _, err = repo.FileHistory(ctx); err != nil {
		t.Fatal("locally deleted file lost history", err)
	}
}

func TestDirectoryPickerErrorsAndEmptyTree(t *testing.T) {
	ctx := context.Background()
	if _, err := OpenDirectory(ctx, t.TempDir()); !errors.Is(err, history.ErrNotRepository) {
		t.Fatal(err)
	}
	dir := fixture(t)
	if _, err := OpenDirectory(ctx, dir); !errors.Is(err, history.ErrNoCommits) {
		t.Fatal(err)
	}
	gitCmd(t, dir, "commit", "--allow-empty", "-m", "empty")
	repo, err := OpenDirectory(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	files, err := repo.Files(ctx)
	if err != nil || len(files) != 0 {
		t.Fatal(files, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := repo.Files(canceled); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
