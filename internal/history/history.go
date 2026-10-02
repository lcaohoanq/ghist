// Package history defines the UI-independent file history model and service.
package history

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNoHistory     = errors.New("file has no Git history")
	ErrNoCommits     = errors.New("repository has no commits")
	ErrNotRepository = errors.New("not inside a Git repository")
)

type Commit struct {
	Hash, ShortHash, Author, Email, Subject string
	Date                                    time.Time
}

// FileVersion carries paths on both sides of a change, including renames.
// Empty BeforePath/AfterPath means the file does not exist on that side.
type FileVersion struct {
	Commit                        Commit
	Parent                        string
	Merge                         bool
	BeforePath, AfterPath, Change string
}

func (v FileVersion) Path() string {
	if v.AfterPath != "" {
		return v.AfterPath
	}
	return v.BeforePath
}

type FileHistory struct {
	Path, Head string
	Versions   []FileVersion
}

type FileSnapshot struct {
	CommitHash, Path, Content string
	Exists, Binary            bool
}

type FileDiff struct {
	CommitHash, Path, Parent, Patch string
}

type Repository interface {
	FileHistory(context.Context) (FileHistory, error)
	Snapshot(context.Context, FileVersion) (FileSnapshot, error)
	Diff(context.Context, FileVersion) (FileDiff, error)
}

type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo: repo} }
func (s *Service) ExploreFile(ctx context.Context) (FileHistory, error) {
	return s.repo.FileHistory(ctx)
}
func (s *Service) GetSnapshot(ctx context.Context, v FileVersion) (FileSnapshot, error) {
	return s.repo.Snapshot(ctx, v)
}
func (s *Service) GetDiff(ctx context.Context, v FileVersion) (FileDiff, error) {
	return s.repo.Diff(ctx, v)
}

// Move clamps navigation to the available versions (newest first).
func (h FileHistory) Move(selected, delta int) int {
	if len(h.Versions) == 0 {
		return 0
	}
	return max(0, min(selected+delta, len(h.Versions)-1))
}
