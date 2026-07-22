package theme_test

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/maxBRT/ship-cli/internal/theme"
)

func TestTheme_brandAndPalette(t *testing.T) {
	if theme.Brand != "ship" {
		t.Errorf("Brand = %q, want ship", theme.Brand)
	}
	// Independent source: throbber RGB (230,175,90), (120,200,140), (220,90,90).
	assertColor(t, "Amber", theme.Amber, "#E6AF5A")
	assertColor(t, "Green", theme.Green, "#78C88C")
	assertColor(t, "Red", theme.Red, "#DC5A5A")
}

func assertColor(t *testing.T, name string, got lipgloss.TerminalColor, wantHex string) {
	t.Helper()
	c, ok := got.(lipgloss.Color)
	if !ok {
		t.Fatalf("%s: got %T, want lipgloss.Color", name, got)
	}
	if string(c) != wantHex {
		t.Errorf("%s = %q, want %q", name, c, wantHex)
	}
}
