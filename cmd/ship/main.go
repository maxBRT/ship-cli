package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/maxBRT/ship-cli/internal/run"
)

func main() {
	os.Exit(Main(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

// Main is the testable CLI entrypoint. It parses Run configuration and exits
// without starting the Run (scaffold only).
func Main(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	_, err := run.ParseConfig(args, getenv)
	if errors.Is(err, flag.ErrHelp) {
		run.WriteUsage(stdout)
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
