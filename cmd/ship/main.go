package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/maxBRT/ship-cli/internal/agent"
	"github.com/maxBRT/ship-cli/internal/gitops"
	"github.com/maxBRT/ship-cli/internal/run"
	"github.com/maxBRT/ship-cli/internal/throbber"
	"github.com/maxBRT/ship-cli/internal/ticket"
)

func main() {
	os.Exit(Main(os.Args[1:], os.Getenv, os.Stdout, os.Stderr, "."))
}

// Main is the testable CLI entrypoint. It parses Run configuration and drives
// the Run orchestrator over the checkout in dir.
func Main(args []string, getenv func(string) string, stdout, stderr io.Writer, dir string) int {
	cfg, err := run.ParseConfig(args, getenv)
	if errors.Is(err, flag.ErrHelp) {
		run.WriteUsage(stdout)
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	color := getenv("NO_COLOR") == ""
	gh := &ticket.GitHub{}
	orchestrator := run.Orchestrator{
		Tickets:  gh,
		Agent:    agent.Cursor{Bin: cfg.Agent},
		PRs:      gh,
		Repo:     gitops.Repo{Dir: dir},
		Config:   cfg,
		Throbber: throbber.Tableau{Out: stderr, Color: color},
		Stdout:   stdout,
	}
	if err := orchestrator.Run(context.Background()); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
