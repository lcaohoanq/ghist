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
	result := history.FileSnapshot{CommitHash: v.Commit.Hash, Path: v.Path()}
	if err := validVersion(v); err != nil {
		return result, err
	}
	if v.AfterPath == "" {
		return result, nil
	}
	out, err := run(ctx, r.Root, "show", v.Commit.Hash+":"+v.AfterPath)
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
	result := history.FileDiff{CommitHash: v.Commit.Hash, Path: v.Path(), Parent: v.Parent}
	if err := validVersion(v); err != nil {
		return result, err
	}
	args := []string{"diff-tree", "--root", "--no-commit-id", "-r", "-p", "-M", "--no-ext-diff", "--no-textconv"}
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
	out, err := run(ctx, r.Root, args...)
	if err != nil {
		return result, err
	}
	result.Patch = string(out)
	return result, nil
}
