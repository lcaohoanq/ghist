// Package git implements history.Repository using the Git executable.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lcaohoanq/ghist/internal/history"
)

type Repository struct{ Root, Path, Head string }

// commandError preserves Git diagnostics without displaying them in the TUI.
type commandError struct {
	operation, detail string
	cause             error
}

func (e *commandError) Error() string { return "Git " + e.operation + " failed" }
func (e *commandError) Unwrap() error { return e.cause }

func gitCommand(ctx context.Context, dir string, args ...string) *exec.Cmd {
	base := []string{"--no-pager", "--literal-pathspecs", "-c", "color.ui=false", "-c", "core.quotePath=false"}
	cmd := exec.CommandContext(ctx, "git", append(base, args...)...)
	cmd.Dir = dir
	// Ignore repository overrides inherited from a surrounding Git invocation.
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch key {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_NAMESPACE", "GIT_PREFIX":
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	cmd.Env = append(cmd.Env, "LC_ALL=C", "GIT_OPTIONAL_LOCKS=0")
	return cmd
}

func run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	var out bytes.Buffer
	err := stream(ctx, dir, func(r io.Reader) error { _, err := io.Copy(&out, r); return err }, args...)
	return out.Bytes(), err
}

// stream drains stdout before Wait, and always reaps Git, including on parser
// failure or cancellation while a consumer is applying backpressure.
func stream(ctx context.Context, dir string, consume func(io.Reader) error, args ...string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := gitCommand(ctx, dir, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	parseErr := consume(out)
	if parseErr != nil {
		cancel()
	}
	err = cmd.Wait()
	if parseErr != nil {
		return parseErr
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return &commandError{operation: args[0], detail: stderr.String(), cause: err}
	}
	return nil
}

func Open(ctx context.Context, path string) (*Repository, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("Git is required; install git and try again: %w", err)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if info, err := os.Lstat(absolute); err == nil && info.IsDir() {
		return nil, fmt.Errorf("expected a file, got a directory: %s", path)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	dir := filepath.Dir(absolute)
	for {
		_, err := os.Stat(dir)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, history.ErrNotRepository
		}
		dir = parent
	}
	repo, err := OpenDirectory(ctx, dir)
	if err != nil {
		return nil, err
	}
	root := repo.Root
	// Resolve symlinks in the existing parent, but preserve a tracked symlink file.
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	suffix, err := filepath.Rel(dir, absolute)
	if err != nil {
		return nil, err
	}
	absolute = filepath.Join(resolvedDir, suffix)
	relative, err := filepath.Rel(root, absolute)
	if err != nil {
		return nil, err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("file is outside repository: %s", path)
	}
	repo.Path = filepath.ToSlash(relative)
	return repo, nil
}

// OpenDirectory captures a repository's HEAD without selecting a file.
func OpenDirectory(ctx context.Context, dir string) (*Repository, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("Git is required; install git and try again: %w", err)
	}
	rootBytes, err := run(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: %w", history.ErrNotRepository, err)
	}
	root := strings.TrimSuffix(string(rootBytes), "\n")
	head, err := run(ctx, root, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: %w", history.ErrNoCommits, err)
	}
	return &Repository{Root: root, Head: strings.TrimSpace(string(head))}, nil
}
