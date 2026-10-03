package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
	"github.com/lcaohoanq/ghist/internal/history"
	"github.com/lcaohoanq/ghist/internal/tui"
)

const usage = `ghist — explore how a file evolved

Usage: ghist [--] [<file>]
       ghist --help

Explore committed history from HEAD, following file renames.
Without a file, fuzzy-find a file in HEAD using fzf (Enter selects; Esc cancels).
Esc from History returns to the picker when no file argument was given.
Paths are relative to your current directory; absolute paths also work.
Use -- before filenames beginning with a dash.

Keys: ↑/↓ or j/k select, Enter inspect, d diff, f file,
      p older, n newer, Esc back, q or Ctrl+C quit.
`

func parseArgs(args []string, out io.Writer) (string, error) {
	flags := flag.NewFlagSet("ghist", flag.ContinueOnError)
	flags.SetOutput(out)
	flags.Usage = func() { fmt.Fprint(out, usage) }
	if err := flags.Parse(args); err != nil {
		return "", err
	}
	if flags.NArg() > 1 {
		return "", errors.New("expected at most one file path; usage: ghist [--] [<file>]")
	}
	if flags.NArg() == 1 && flags.Arg(0) == "" {
		return "", errors.New("file path must not be empty")
	}
	return flags.Arg(0), nil
}
func run(args []string) error {
	path, err := parseArgs(args, os.Stdout)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		return errors.New("ghist requires an interactive terminal (stdin and stdout)")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for {
		repo, err := openTarget(ctx, path)
		if errors.Is(err, errPickerCancelled) {
			return nil
		}
		if err != nil {
			return err
		}
		model := tui.New(ctx, history.NewService(repo), repo.Path)
		if path == "" {
			model = model.WithPicker()
		}
		final, err := tea.NewProgram(model, tea.WithContext(ctx)).Run()
		if errors.Is(err, tea.ErrInterrupted) {
			return nil
		}
		if err != nil {
			return err
		}
		if result, ok := final.(tui.Model); !ok || !result.BackToPicker() {
			return nil
		}
	}
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ghist:", err)
		os.Exit(1)
	}
}
