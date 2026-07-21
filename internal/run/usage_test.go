package run_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/maxBRT/ship-cli/internal/run"
	"github.com/maxBRT/ship-cli/internal/theme"
)

func TestWriteUsage_sectionHeadingsIncludeShipBrand(t *testing.T) {
	var buf bytes.Buffer
	run.WriteUsage(&buf)
	out := buf.String()

	for _, section := range []string{"Usage", "Flags", "Config"} {
		found := false
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, section) && strings.Contains(line, theme.Brand) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("section %q heading should include %q brand framing; got:\n%s", section, theme.Brand, out)
		}
	}
}

func TestWriteUsage_flagsShowNameMeaningAndDefaultOnOneLine(t *testing.T) {
	var buf bytes.Buffer
	run.WriteUsage(&buf)
	out := buf.String()

	// Literals from today's flag set and defaults (independent of render layout).
	cases := []struct {
		name, meaning, def string
	}{
		{"-agent", "Agent kind for every Phase", "cursor"},
		{"-branch", "git branch for the Run", ""},
		{"-max-iterations", "max Iterations (one Ticket each) before Final", "10"},
		{"-model", "optional model for the Agent", ""},
		{"-timeout", "per-Phase timeout (Go duration)", "20m0s"},
	}
	for _, tc := range cases {
		found := false
		for _, line := range strings.Split(out, "\n") {
			if !strings.Contains(line, tc.name) || !strings.Contains(line, tc.meaning) {
				continue
			}
			if tc.def != "" && !strings.Contains(line, tc.def) {
				continue
			}
			found = true
			break
		}
		if !found {
			t.Errorf("flag %q should appear with meaning %q and default %q on one line; got:\n%s",
				tc.name, tc.meaning, tc.def, out)
		}
	}
}
