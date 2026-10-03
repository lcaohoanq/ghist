package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func fakeFZF(t *testing.T, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fzf"), []byte("#!/bin/sh\n"+body), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

func TestPickerPreservesPathsAndIsolatesOptions(t *testing.T) {
	fakeFZF(t, `[ -z "$FZF_DEFAULT_OPTS$FZF_DEFAULT_OPTS_FILE$FZF_DEFAULT_COMMAND" ] || exit 2
[ "$1" = --read0 ] && [ "$2" = --print0 ] || exit 2
exec /bin/cat
`)
	t.Setenv("FZF_DEFAULT_OPTS", "--print-query --multi")
	t.Setenv("FZF_DEFAULT_OPTS_FILE", "/nonexistent")
	t.Setenv("FZF_DEFAULT_COMMAND", "exit 2")
	for _, path := range []string{"space file", "-flag", "đổi [x]*\nfile.go", " trailing "} {
		got, err := pickFile(context.Background(), []string{path})
		if err != nil || got != path {
			t.Fatalf("%q => %q, %v", path, got, err)
		}
	}
}

func TestPickerCancellationAndFailures(t *testing.T) {
	for _, tc := range []struct {
		body   string
		cancel bool
	}{
		{"exit 130", true}, {"exit 1", true}, {"exit 0", true},
		{"exit 2", false}, {"printf 'unexpected\\000'", false},
	} {
		t.Run(tc.body, func(t *testing.T) {
			fakeFZF(t, tc.body)
			_, err := pickFile(context.Background(), []string{"file"})
			if err == nil || errors.Is(err, errPickerCancelled) != tc.cancel {
				t.Fatal(err)
			}
		})
	}
	t.Run("missing", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		_, err := pickFile(context.Background(), []string{"file"})
		if err == nil || !strings.Contains(err.Error(), "install fzf") {
			t.Fatal(err)
		}
	})
	t.Run("context", func(t *testing.T) {
		fakeFZF(t, "exec /bin/cat")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := pickFile(ctx, []string{"file"})
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	})
}
