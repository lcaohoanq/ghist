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
	Truncated                 bool
}

type FileDiff struct {
	CommitHash, Path, Parent, Patch string
	Truncated                       bool
}

type Repository interface {
	FileHistory(context.Context) (FileHistory, error)
	Snapshot(context.Context, FileVersion) (FileSnapshot, error)
	Diff(context.Context, FileVersion) (FileDiff, error)
}

// HistoryStreamer emits immutable versions in Git order. A callback error stops
// the traversal, allowing consumers to cancel even while applying backpressure.
type HistoryStreamer interface {
	StreamHistory(context.Context, func(FileVersion) error) error
}

type Service struct {
	repo  Repository
	cache *contentCache
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, cache: newContentCache(64, 32<<20)}
}
func (s *Service) StreamHistory(ctx context.Context, emit func(FileVersion) error) error {
	if stream, ok := s.repo.(HistoryStreamer); ok {
		return stream.StreamHistory(ctx, emit)
	}
	h, err := s.repo.FileHistory(ctx)
	if err != nil {
		return err
	}
	for _, v := range h.Versions {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := emit(v); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) ExploreFile(ctx context.Context) (FileHistory, error) {
	return s.repo.FileHistory(ctx)
}
func (s *Service) GetSnapshot(ctx context.Context, v FileVersion) (FileSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return FileSnapshot{}, err
	}
	k := contentKey{kind: 's', hash: v.Commit.Hash, parent: v.Parent, before: v.BeforePath, after: v.AfterPath}
	if cached, ok := s.cache.get(k); ok {
		return cached.(FileSnapshot), nil
	}
	result, err := s.repo.Snapshot(ctx, v)
	if err == nil && ctx.Err() == nil {
		s.cache.put(k, result, len(result.Content)+len(result.Path)+len(result.CommitHash))
	}
	return result, err
}
func (s *Service) GetDiff(ctx context.Context, v FileVersion) (FileDiff, error) {
	if err := ctx.Err(); err != nil {
		return FileDiff{}, err
	}
	k := contentKey{kind: 'd', hash: v.Commit.Hash, parent: v.Parent, before: v.BeforePath, after: v.AfterPath}
	if cached, ok := s.cache.get(k); ok {
		return cached.(FileDiff), nil
	}
	result, err := s.repo.Diff(ctx, v)
	if err == nil && ctx.Err() == nil {
		s.cache.put(k, result, len(result.Patch)+len(result.Path)+len(result.CommitHash)+len(result.Parent))
	}
	return result, err
}

// Move clamps navigation to the available versions (newest first).
func (h FileHistory) Move(selected, delta int) int {
	if len(h.Versions) == 0 {
		return 0
	}
	return max(0, min(selected+delta, len(h.Versions)-1))
}

// Full content bypasses the repository's initial output cap and the LRU, so a
// large explicitly requested result is not retained after leaving the view.
func (s *Service) GetFullDiff(ctx context.Context, v FileVersion) (FileDiff, error) {
	if repo, ok := s.repo.(interface {
		FullDiff(context.Context, FileVersion) (FileDiff, error)
	}); ok {
		return repo.FullDiff(ctx, v)
	}
	return s.GetDiff(ctx, v)
}
func (s *Service) GetFullSnapshot(ctx context.Context, v FileVersion) (FileSnapshot, error) {
	if repo, ok := s.repo.(interface {
		FullSnapshot(context.Context, FileVersion) (FileSnapshot, error)
	}); ok {
		return repo.FullSnapshot(ctx, v)
	}
	return s.GetSnapshot(ctx, v)
}
