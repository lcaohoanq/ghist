package git

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/lcaohoanq/ghist/internal/history"
)

func (r *Repository) FileHistory(ctx context.Context) (history.FileHistory, error) {
	result := history.FileHistory{Path: r.Path, Head: r.Head}
	err := r.StreamHistory(ctx, func(v history.FileVersion) error {
		result.Versions = append(result.Versions, v)
		return nil
	})
	return result, err
}

// StreamHistory obtains metadata and historical paths in one Git traversal.
// Paths come from each record, rather than a mutable path shared by branches.
func (r *Repository) StreamHistory(ctx context.Context, emit func(history.FileVersion) error) error {
	count := 0
	err := stream(ctx, r.Root, func(reader io.Reader) error {
		return parseHistory(reader, r.Path, func(v history.FileVersion) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			count++
			return emit(v)
		})
	}, "log", "--root", "--no-notes", "--follow", "-M", "--no-show-signature", "--diff-merges=first-parent",
		"--name-status", "--no-ext-diff", "--no-textconv", "--topo-order",
		"--format=%x00%H%x00%P%x00%an%x00%ae%x00%aI%x00%s", "-z", r.Head, "--", r.Path)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err == nil && count == 0 {
		return history.ErrNoHistory
	}
	return err
}

// NUL framing preserves tabs/newlines in paths. Metadata fields are consumed
// positionally, so a subject or filename resembling a hash is never a header.
func parseHistory(reader io.Reader, fallbackPath string, emit func(history.FileVersion) error) error {
	readerBuf := bufio.NewReader(reader)
	token := func() (string, error) {
		s, err := readerBuf.ReadString(0)
		if err != nil {
			if err == io.EOF && s != "" {
				return "", io.ErrUnexpectedEOF
			}
			return "", err
		}
		return strings.TrimSuffix(s, "\x00"), nil
	}
	var current *history.FileVersion
	flush := func() error {
		if current == nil {
			return nil
		}
		return emit(*current)
	}
	for {
		s, err := token()
		if err == io.EOF {
			return flush()
		}
		if err != nil {
			return err
		}
		if s == "" {
			continue
		}
		if objectID.MatchString(s) {
			if err := flush(); err != nil {
				return err
			}
			fields := make([]string, 5)
			for i := range fields {
				fields[i], err = token()
				if err != nil {
					return fmt.Errorf("invalid Git history metadata: %w", err)
				}
			}
			date, err := time.Parse(time.RFC3339, fields[3])
			if err != nil {
				return fmt.Errorf("invalid commit date: %w", err)
			}
			parents := strings.Fields(fields[0])
			current = &history.FileVersion{
				Commit: history.Commit{Hash: s, ShortHash: s[:7], Author: fields[1], Email: fields[2], Date: date, Subject: fields[4]},
				Merge:  len(parents) > 1, BeforePath: fallbackPath, AfterPath: fallbackPath, Change: "M",
			}
			if len(parents) > 0 {
				current.Parent = parents[0]
			}
			continue
		}
		if current == nil {
			return fmt.Errorf("Git history change without commit")
		}
		status := strings.TrimPrefix(s, "\n")
		if status == "" || !strings.ContainsRune("AMDRCTUXB", rune(status[0])) {
			return fmt.Errorf("invalid Git history status")
		}
		path, err := token()
		if err != nil {
			return fmt.Errorf("invalid Git history path: %w", err)
		}
		before, after := path, path
		switch status[0] {
		case 'A':
			before = ""
		case 'D':
			after = ""
		case 'R', 'C':
			after, err = token()
			if err != nil {
				return fmt.Errorf("invalid Git rename path: %w", err)
			}
		}
		current.BeforePath, current.AfterPath, current.Change = before, after, status
	}
}
