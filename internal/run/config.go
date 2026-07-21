package run

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
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
	fmt.Fprintf(w, `ship - run a sequential Ticket Run in the current checkout.

A Run confirms a ship queue from ship-labeled Tickets via an interactive
picker, processes each through one Iteration (Implement Phase then Review
Phase), then a Final Phase.

Usage:
  ship [flags]
  ship init
  ship update
  ship --version

Flags:
`)
	defaults := defaultConfig()
	fs := newFlagSet(&defaults)
	fs.SetOutput(w)
	fs.PrintDefaults()
	fmt.Fprintf(w, `
Commands:
  init     create .ship/config.yaml with defaults
  update   download the latest GitHub Release and replace this binary

Config:
  .ship/config.yaml at the checkout root (defaults → YAML → flags).
  Run "ship init" to create one with filled defaults.
`)
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
