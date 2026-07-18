package run

import (
	"flag"
	"fmt"
	"io"
	"strconv"
	"time"
)

// Config holds the Run configuration parsed from flags and environment.
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

Flags:
`)
	defaults := defaultConfig()
	fs := newFlagSet(&defaults)
	fs.SetOutput(w)
	fs.PrintDefaults()
	fmt.Fprintf(w, `
Environment (overridden by flags):
  SHIP_BRANCH           same as --branch
  SHIP_FEATURE          same as --feature
  SHIP_AGENT            same as --agent
  SHIP_MODEL            same as --model
  SHIP_MAX_ITERATIONS   same as --max-iterations
  SHIP_PHASE_TIMEOUT    same as --timeout
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
	fs.StringVar(&cfg.Branch, "branch", cfg.Branch, "git branch for the Run (empty means generate ship/<id> later)")
	fs.StringVar(&cfg.Feature, "feature", cfg.Feature, "optional Ready for Agent filter label for Tickets")
	fs.StringVar(&cfg.Agent, "agent", cfg.Agent, "Agent binary used for every Phase")
	fs.StringVar(&cfg.Model, "model", cfg.Model, "optional model for the Agent")
	fs.IntVar(&cfg.MaxIterations, "max-iterations", cfg.MaxIterations, "max Iterations (one Ticket each) before Final")
	fs.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "per-Phase timeout (Go duration)")
	return fs
}

// ParseConfig builds a Run Config from args and getenv.
// Environment overrides defaults; flags override environment.
func ParseConfig(args []string, getenv func(string) string) (Config, error) {
	cfg := defaultConfig()

	if v := getenv("SHIP_BRANCH"); v != "" {
		cfg.Branch = v
	}
	if v := getenv("SHIP_FEATURE"); v != "" {
		cfg.Feature = v
	}
	if v := getenv("SHIP_AGENT"); v != "" {
		cfg.Agent = v
	}
	if v := getenv("SHIP_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := getenv("SHIP_MAX_ITERATIONS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("SHIP_MAX_ITERATIONS: invalid integer %q", v)
		}
		cfg.MaxIterations = n
	}
	if v := getenv("SHIP_PHASE_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("SHIP_PHASE_TIMEOUT: invalid duration %q", v)
		}
		cfg.Timeout = d
	}

	fs := newFlagSet(&cfg)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
