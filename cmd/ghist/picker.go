package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	gitrepo "github.com/lcaohoanq/ghist/internal/git"
)

var errPickerCancelled = errors.New("file selection cancelled")

func openTarget(ctx context.Context, path string) (*gitrepo.Repository, error) {
	if path != "" {
		return gitrepo.Open(ctx, path)
	}
	repo, err := gitrepo.OpenDirectory(ctx, ".")
	if err != nil {
		return nil, err
	}
	paths, err := repo.Files(ctx)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, errors.New("no files in HEAD to select")
	}
	selected, err := pickFile(ctx, paths)
	if err != nil {
		return nil, err
	}
	repo.Path = selected
	return repo, nil
}

func pickFile(ctx context.Context, paths []string) (string, error) {
	binary, err := exec.LookPath("fzf")
	if err != nil {
		return "", errors.New("fzf is required for file selection; install fzf (https://github.com/junegunn/fzf#installation) or run ghist <file>")
	}
	cmd := exec.CommandContext(ctx, binary, "--read0", "--print0", "--no-multi", "--layout=reverse", "--prompt=ghist > ", "--header=Files in HEAD · Enter: history · Esc: cancel")
	// User fzf bindings and output transforms can replace accept/abort semantics.
	// Keep this embedded picker deterministic, without changing the user's config.
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch key {
		case "FZF_DEFAULT_OPTS", "FZF_DEFAULT_OPTS_FILE", "FZF_DEFAULT_COMMAND":
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	cmd.Stdin = strings.NewReader(strings.Join(paths, "\x00") + "\x00")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && (exit.ExitCode() == 130 || exit.ExitCode() == 1) {
			return "", errPickerCancelled
		}
		return "", fmt.Errorf("fzf failed: %w", err)
	}
	if len(out) == 0 {
		return "", errPickerCancelled
	}
	selected := string(bytes.TrimSuffix(out, []byte{0}))
	for _, path := range paths {
		if selected == path {
			return selected, nil
		}
	}
	return "", errors.New("fzf returned an invalid file selection")
}
