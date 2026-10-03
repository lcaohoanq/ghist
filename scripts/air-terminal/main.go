//go:build linux || darwin

// air-terminal gives an Air child the foreground terminal for interactive TUI
// development, then returns it to Air after the child exits or reloads.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/charmbracelet/x/term"
	"golang.org/x/sys/unix"
)

func run() error {
	path := os.Getenv("FILE")
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer tty.Close()
	fd := int(tty.Fd())
	foreground, err := unix.IoctlGetInt(fd, unix.TIOCGPGRP)
	if err != nil {
		return err
	}
	state, err := term.GetState(tty.Fd())
	if err != nil {
		return err
	}
	// Air starts its child in a background process group. Job-control signals
	// must be ignored while we claim and later restore the foreground group.
	signal.Ignore(syscall.SIGTTOU, syscall.SIGTTIN)
	signals := make(chan os.Signal, 4)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	if err = unix.IoctlSetPointerInt(fd, unix.TIOCSPGRP, syscall.Getpgrp()); err != nil {
		return err
	}
	defer func() {
		// Air signals the process tree, so a child may receive more than one
		// interrupt. Reset display modes even if it exits before Tea cleans up.
		_, _ = tty.WriteString("\x1b[0m\x1b[?1000l\x1b[?1002l\x1b[?1003l\x1b[?1006l\x1b[?2004l\x1b[?25h\x1b[?1049l")
		_ = term.Restore(tty.Fd(), state)
		_ = unix.IoctlSetPointerInt(fd, unix.TIOCSPGRP, foreground)
	}()
	var args []string
	if os.Getenv("FOLLOW") == "1" {
		args = append(args, "--follow")
	}
	if path != "" {
		args = append(args, "--", path)
	}
	// Resolve the development binary before changing the child's directory.
	bin, err := filepath.Abs("./tmp/ghist")
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = os.Getenv("PROJECT")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = tty, tty, tty
	if err = cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	for {
		select {
		case sig := <-signals:
			_ = cmd.Process.Signal(sig)
		case err := <-done:
			return err
		}
	}
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "air-terminal:", err)
		os.Exit(1)
	}
}
