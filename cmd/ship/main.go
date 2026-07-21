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
	"github.com/maxBRT/ship-cli/internal/herdr"
	"github.com/maxBRT/ship-cli/internal/observe"
	"github.com/maxBRT/ship-cli/internal/run"
	"github.com/maxBRT/ship-cli/internal/throbber"
	"github.com/maxBRT/ship-cli/internal/ticket"
)

func main() {
	os.Exit(Main(os.Args[1:], os.Stdout, os.Stderr, "."))
}

// Main is the testable CLI entrypoint. It parses Run configuration and drives
// the Run orchestrator over the checkout in dir.
func Main(args []string, stdout, stderr io.Writer, dir string) int {
	if len(args) > 0 && args[0] == "init" {
		return runInit(stderr, dir)
	}
	if wantsHelp(args) {
		run.WriteUsage(stdout)
		return 0
	}

	created, err := run.InitConfig(dir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if created {
		fmt.Fprintln(stderr, "created .ship/config.yaml with defaults")
	}

	cfg, err := run.ParseConfig(args, dir)
	if errors.Is(err, flag.ErrHelp) {
		run.WriteUsage(stdout)
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	gh := &ticket.GitHub{}
	orchestrator := run.Orchestrator{
		Tickets:  gh,
		Queue:    run.Interactive{Out: stderr},
		Agent:    agent.Cursor{Bin: cfg.Agent},
		PRs:      gh,
		Repo:     gitops.Repo{Dir: dir},
		Config:   cfg,
		Throbber: throbber.Line{Out: stderr, Color: true},
		Observer: observe.New(dir, stderr),
		Herdr:    herdr.Reporter{},
		Stdout:   stdout,
	}
	if err := orchestrator.Run(context.Background()); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runInit(stderr io.Writer, dir string) int {
	created, err := run.InitConfig(dir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if !created {
		fmt.Fprintln(stderr, ".ship/config.yaml already exists")
		return 0
	}
	fmt.Fprintln(stderr, "created .ship/config.yaml with defaults")
	return 0
}

func wantsHelp(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "-help" || a == "--help" {
			return true
		}
	}
	return false
}
