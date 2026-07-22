package run

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/maxBRT/ship-cli/internal/theme"
)

// Config holds the Run configuration parsed from .ship/config.yaml and flags.
type Config struct {
	Branch        string
	Agent         string
	Model         string
	MaxIterations int
	Timeout       time.Duration
}

// Agent kinds accepted by agent: / --agent.
const (
	AgentKindCursor = "cursor"
	AgentKindPi     = "pi"
	AgentKindCodex  = "codex"
	AgentKindClaude = "claude"
)

var knownAgentKinds = map[string]struct{}{
	AgentKindCursor: {},
	AgentKindPi:     {},
	AgentKindCodex:  {},
	AgentKindClaude: {},
}

// WriteUsage prints CLI help using Ship domain language.
func WriteUsage(w io.Writer) {
	heading := lipgloss.NewStyle().Foreground(theme.Amber).Bold(true)
	fmt.Fprintf(w, `%s - Turn your ticktes into a PR.

%s
  ship [flags]
  ship init
  ship update
  ship --version

%s
`, theme.Brand, heading.Render(theme.Brand+"  Usage"), heading.Render(theme.Brand+"  Flags"))
	writeFlagColumns(w)
	fmt.Fprintf(w, `
Commands:
  init     create .ship/config.yaml with defaults
  update   download the latest GitHub Release and replace this binary
`)
}

func writeFlagColumns(w io.Writer) {
	defaults := defaultConfig()
	fs := newFlagSet(&defaults)

	type row struct {
		name, meaning, def string
	}
	var rows []row
	nameWidth, meaningWidth := 0, 0
	fs.VisitAll(func(f *flag.Flag) {
		name := "-" + f.Name
		r := row{name: name, meaning: f.Usage, def: f.DefValue}
		rows = append(rows, r)
		if len(name) > nameWidth {
			nameWidth = len(name)
		}
		if len(f.Usage) > meaningWidth {
			meaningWidth = len(f.Usage)
		}
	})

	nameStyle := lipgloss.NewStyle().Width(nameWidth + 2)
	meaningStyle := lipgloss.NewStyle().Width(meaningWidth + 2).Foreground(lipgloss.Color("245"))
	defStyle := lipgloss.NewStyle().Foreground(theme.Green)
	for _, r := range rows {
		fmt.Fprint(w, "  ")
		fmt.Fprint(w, nameStyle.Render(r.name))
		fmt.Fprint(w, meaningStyle.Render(r.meaning))
		fmt.Fprintln(w, defStyle.Render(r.def))
	}
}

func defaultConfig() Config {
	return Config{
		Agent:         AgentKindCursor,
		MaxIterations: 10,
		Timeout:       20 * time.Minute,
	}
}

func newFlagSet(cfg *Config) *flag.FlagSet {
	fs := flag.NewFlagSet("ship", flag.ContinueOnError)
	fs.StringVar(&cfg.Branch, "branch", cfg.Branch, "git branch for the Run (empty means generate ship/<id>)")
	fs.StringVar(&cfg.Agent, "agent", cfg.Agent, "Agent kind for every Phase (cursor, pi, codex, claude)")
	fs.StringVar(&cfg.Model, "model", cfg.Model, "optional model for the Agent")
	fs.IntVar(&cfg.MaxIterations, "max-iterations", cfg.MaxIterations, "max Iterations (one Ticket each) before Final")
	fs.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "per-Phase timeout (Go duration)")
	return fs
}

// ParseConfig builds a Run Config from .ship/config.yaml in dir, then applies flags.
// YAML overrides built-in defaults; flags override YAML.
func ParseConfig(args []string, dir string) (Config, error) {
	cfg, err := LoadConfig(dir)
	if err != nil {
		return Config{}, err
	}

	fs := newFlagSet(&cfg)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	if err := validateAgentKind(cfg.Agent); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func validateAgentKind(kind string) error {
	if _, ok := knownAgentKinds[kind]; ok {
		return nil
	}
	if isLegacyBinaryAgent(kind) {
		return fmt.Errorf("agent %q is a legacy binary path; use agent: cursor (or pi, codex, claude)", kind)
	}
	return fmt.Errorf("unknown agent kind %q (want cursor, pi, codex, or claude)", kind)
}

func isLegacyBinaryAgent(kind string) bool {
	if kind == "agent" {
		return true
	}
	return strings.Contains(kind, "/") || strings.Contains(kind, `\`)
}
