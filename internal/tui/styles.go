package tui

// ANSI styles used throughout the TUI. Keeping them in one place makes the
// palette easy to adjust without hunting for numeric escape codes.
const (
	styleBold       = "1"
	styleRed        = "31"
	styleYellow     = "33"
	styleCyan       = "36"
	styleMuted      = "90"
	styleBoldRed    = "1;31"
	styleBoldYellow = "1;33"
	styleBoldCyan   = "1;36"
	styleBoldMuted  = "1;90"
	styleSelected   = "1;7"

	// Subtle paired foreground/background colors keep diff content legible.
	styleDiffAdded   = "38;2;222;230;226;48;2;24;48;38"
	styleDiffRemoved = "38;2;235;225;227;48;2;52;30;36"
)
