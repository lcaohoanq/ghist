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
	gitrepo "github.com/lcaohoanq/ghist/internal/git"
	"github.com/lcaohoanq/ghist/internal/history"
	"github.com/lcaohoanq/ghist/internal/tui"
)

const usage = `ghist — explore how a file evolved

Usage: ghist [--] <file>
       ghist --help

Explore committed history from HEAD, following file renames.
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
	if flags.NArg() != 1 {
		return "", errors.New("expected one file path; usage: ghist [--] <file>")
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
	repo, err := gitrepo.Open(ctx, path)
	if err != nil {
		return err
	}
	model := tui.New(ctx, history.NewService(repo), repo.Path)
	_, err = tea.NewProgram(model, tea.WithContext(ctx)).Run()
	if errors.Is(err, tea.ErrInterrupted) {
		return nil
	}
	return err
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ghist:", err)
		os.Exit(1)
	}
}
