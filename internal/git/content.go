package git

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/lcaohoanq/ghist/internal/history"
)

var objectID = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

func validVersion(v history.FileVersion) error {
	if !objectID.MatchString(v.Commit.Hash) || (v.Parent != "" && !objectID.MatchString(v.Parent)) {
		return fmt.Errorf("invalid commit hash")
	}
	return nil
}

func (r *Repository) Snapshot(ctx context.Context, v history.FileVersion) (history.FileSnapshot, error) {
	return r.snapshot(ctx, v, false)
}
func (r *Repository) FullSnapshot(ctx context.Context, v history.FileVersion) (history.FileSnapshot, error) {
	return r.snapshot(ctx, v, true)
}
func (r *Repository) snapshot(ctx context.Context, v history.FileVersion, full bool) (history.FileSnapshot, error) {
	result := history.FileSnapshot{CommitHash: v.Commit.Hash, Path: v.Path()}
	if err := validVersion(v); err != nil {
		return result, err
	}
	if v.AfterPath == "" {
		return result, nil
	}
	out, truncated, err := runContent(ctx, r.Root, full, "show", v.Commit.Hash+":"+v.AfterPath)
	result.Truncated = truncated
	// A byte cap may split a UTF-8 rune; do not mistake that for binary data.
	if truncated {
		for n := 0; n < utf8.UTFMax-1 && len(out) > 0 && !utf8.Valid(out); n++ {
			out = out[:len(out)-1]
		}
	}
	if err != nil {
		return result, fmt.Errorf("cannot read file at commit %s: %w", v.Commit.ShortHash, err)
	}
	result.Exists = true
	result.Binary = strings.ContainsRune(string(out), 0) || !utf8.Valid(out)
	if !result.Binary {
		result.Content = string(out)
	}
	return result, nil
}

func (r *Repository) Diff(ctx context.Context, v history.FileVersion) (history.FileDiff, error) {
	return r.diff(ctx, v, false)
}
func (r *Repository) FullDiff(ctx context.Context, v history.FileVersion) (history.FileDiff, error) {
	return r.diff(ctx, v, true)
}
func (r *Repository) diff(ctx context.Context, v history.FileVersion, full bool) (history.FileDiff, error) {
	result := history.FileDiff{CommitHash: v.Commit.Hash, Path: v.Path(), Parent: v.Parent}
	if err := validVersion(v); err != nil {
		return result, err
	}
	args := []string{"diff-tree", "--root", "--no-commit-id", "-r", "-p", "--no-ext-diff", "--no-textconv"}
	if r.FollowRenames {
		args = append(args, "-M")
	} else {
		args = append(args, "--no-renames")
	}
	if v.Parent != "" {
		args = append(args, v.Parent)
	}
	args = append(args, v.Commit.Hash, "--")
	if v.BeforePath != "" {
		args = append(args, v.BeforePath)
	}
	if v.AfterPath != "" && v.AfterPath != v.BeforePath {
		args = append(args, v.AfterPath)
	}
	if v.BeforePath == "" && v.AfterPath == "" {
		return result, fmt.Errorf("version has no file path")
	}
	out, truncated, err := runContent(ctx, r.Root, full, args...)
	result.Truncated = truncated
	if err != nil {
		return result, err
	}
	result.Patch = string(out)
	return result, nil
}
