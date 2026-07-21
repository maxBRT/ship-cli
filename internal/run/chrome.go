package run

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/maxBRT/ship-cli/internal/theme"
)

// WriteRunHeader prints a one-time Lipgloss Run frame with Agent kind and branch.
// When color is false, marks and rules stay plain (no ANSI).
func WriteRunHeader(w io.Writer, agent, branch string, color bool) {
	sep := "────────────────────────────────────────────"
	if color {
		brand := lipgloss.NewStyle().Foreground(theme.Amber).Bold(true)
		dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
		fmt.Fprintln(w, brand.Render(theme.Brand))
		fmt.Fprintf(w, "  run · agent %s · branch %s\n", agent, branch)
		fmt.Fprintln(w, dim.Render(sep))
		return
	}
	fmt.Fprintln(w, theme.Brand)
	fmt.Fprintf(w, "  run · agent %s · branch %s\n", agent, branch)
	fmt.Fprintln(w, sep)
}

// FormatQueueRemaining builds the dim queue-remainder hint under the throbber.
func FormatQueueRemaining(numbers []int) string {
	if len(numbers) == 0 {
		return ""
	}
	parts := make([]string, len(numbers))
	for i, n := range numbers {
		parts[i] = fmt.Sprintf("#%d", n)
	}
	return "queue remaining · " + strings.Join(parts, " · ")
}
