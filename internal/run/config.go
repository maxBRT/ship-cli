package run

import (
	"flag"
	"fmt"
	"io"
	"time"
)

// Config holds the Run configuration parsed from .ship/config.yaml and flags.
type Config struct {
	Branch        string
	Feature       string
	Agent         string
	Model         string
	MaxIterations int
	Timeout       time.Duration
}

// WriteUsage prints CLI help using Ship domain language.
func WriteUsage(w io.Writer) {
	fmt.Fprintf(w, `ship - run a sequential Ticket Run in the current checkout.

A Run claims Ready for Agent Tickets, processes each through one Iteration
(Implement Phase then Review Phase), then a Final Phase.

Usage:
  ship [flags]
  ship init

Flags:
`)
	defaults := defaultConfig()
	fs := newFlagSet(&defaults)
	fs.SetOutput(w)
	fs.PrintDefaults()
	fmt.Fprintf(w, `
Config:
  .ship/config.yaml at the checkout root (defaults → YAML → flags).
  Run "ship init" to create one with filled defaults.
`)
}

func defaultConfig() Config {
	return Config{
		Agent:         "agent",
		MaxIterations: 10,
		Timeout:       10 * time.Minute,
	}
}

func newFlagSet(cfg *Config) *flag.FlagSet {
	fs := flag.NewFlagSet("ship", flag.ContinueOnError)
	fs.StringVar(&cfg.Branch, "branch", cfg.Branch, "git branch for the Run (empty means generate ship/<id>)")
	fs.StringVar(&cfg.Feature, "feature", cfg.Feature, "optional Ready for Agent filter label for Tickets")
	fs.StringVar(&cfg.Agent, "agent", cfg.Agent, "Agent binary used for every Phase")
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

	return cfg, nil
}
