// Package theme holds shared Lipgloss visual tokens for Ship chrome:
// amber wait, green success, red failure, and the ship brand prefix.
package theme

import "github.com/charmbracelet/lipgloss"

// Brand is the framing prefix for interactive frames and help headers.
const Brand = "ship"

// Palette matches throbber wait / success / fail RGB.
const (
	Amber = lipgloss.Color("#E6AF5A") // 230, 175, 90
	Green = lipgloss.Color("#78C88C") // 120, 200, 140
	Red   = lipgloss.Color("#DC5A5A") // 220, 90, 90
)
