package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/maxBRT/ship-cli/internal/gitops"
	"github.com/maxBRT/ship-cli/internal/run"
	"github.com/maxBRT/ship-cli/internal/throbber/prototype"
)

func main() {
	os.Exit(Main(os.Args[1:], os.Getenv, os.Stdout, os.Stderr, "."))
}

// Main is the testable CLI entrypoint. It parses Run configuration and
// prepares the Run branch in dir (the current checkout). It does not start
// the full Run loop yet.
func Main(args []string, getenv func(string) string, stdout, stderr io.Writer, dir string) int {
	// PROTOTYPE: independent throbber demo — not part of the Run loop.
	if len(args) > 0 && args[0] == "throbber" {
		return prototype.Demo(args[1:], stdout, stderr)
	}

	cfg, err := run.ParseConfig(args, getenv)
	if errors.Is(err, flag.ErrHelp) {
		run.WriteUsage(stdout)
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if _, err := (gitops.Repo{Dir: dir}).EnsureBranch(cfg.Branch); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
