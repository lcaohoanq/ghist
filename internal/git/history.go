package git

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lcaohoanq/ghist/internal/history"
)

func (r *Repository) FileHistory(ctx context.Context) (history.FileHistory, error) {
	result := history.FileHistory{Path: r.Path, Head: r.Head}
	out, err := run(ctx, r.Root, "log", "--follow", "-M", "--no-show-signature", "--diff-merges=first-parent", "--no-patch", "--topo-order", "--format=%H%x00%P%x00%an%x00%ae%x00%aI%x00%s", "-z", r.Head, "--", r.Path)
	if err != nil {
		return result, err
	}
	if len(out) == 0 {
		return result, history.ErrNoHistory
	}
	fields := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	if len(fields)%6 != 0 {
		return result, fmt.Errorf("invalid Git history response")
	}
	path := r.Path
	for i := 0; i < len(fields); i += 6 {
		date, err := time.Parse(time.RFC3339, fields[i+4])
		if err != nil {
			return result, fmt.Errorf("invalid commit date: %w", err)
		}
		hash := fields[i]
		if len(hash) < 7 {
			return result, fmt.Errorf("invalid commit hash")
		}
		parents := strings.Fields(fields[i+1])
		v := history.FileVersion{Commit: history.Commit{Hash: hash, ShortHash: hash[:7], Author: fields[i+2], Email: fields[i+3], Date: date, Subject: fields[i+5]}, Merge: len(parents) > 1, BeforePath: path, AfterPath: path, Change: "M"}
		if len(parents) > 0 {
			v.Parent = parents[0]
		}
		args := []string{"diff-tree", "--no-commit-id", "--root", "-r", "-M", "--no-ext-diff", "--no-textconv", "--name-status", "-z"}
		if v.Parent != "" {
			args = append(args, v.Parent)
		}
		args = append(args, hash, "--")
		changes, err := run(ctx, r.Root, args...)
		if err != nil {
			return result, err
		}
		entries, err := parseChanges(changes)
		if err != nil {
			return result, err
		}
		for _, c := range entries {
			if c.after == path || (c.after == "" && c.before == path) {
				v.BeforePath, v.AfterPath, v.Change = c.before, c.after, c.status
				if c.before != "" {
					path = c.before
				}
				break
			}
		}
		result.Versions = append(result.Versions, v)
	}
	return result, nil
}

type change struct{ status, before, after string }

func parseChanges(data []byte) ([]change, error) {
	if len(data) == 0 {
		return nil, nil
	}
	tokens := strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
	var result []change
	for i := 0; i < len(tokens); {
		status := tokens[i]
		i++
		if status == "" || i >= len(tokens) {
			return nil, fmt.Errorf("invalid Git change response")
		}
		path := tokens[i]
		i++
		c := change{status: status, before: path, after: path}
		switch status[0] {
		case 'A':
			c.before = ""
		case 'D':
			c.after = ""
		case 'R', 'C':
			if i >= len(tokens) {
				return nil, fmt.Errorf("invalid Git rename response")
			}
			c.after = tokens[i]
			i++
		}
		result = append(result, c)
	}
	return result, nil
}
