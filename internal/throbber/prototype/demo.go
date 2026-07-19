package prototype

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Demo is the `ship throbber` entrypoint — independent of the Run loop.
func Demo(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ship throbber", flag.ContinueOnError)
	fs.SetOutput(stderr)
	seconds := fs.Float64("seconds", 12, "how long to run before exiting")
	phase := fs.String("phase", "Implement", "phase label for the bottom HUD")
	noColor := fs.Bool("no-color", false, "disable truecolor (still animates)")
	forceColor := fs.Bool("color", false, "force color even if NO_COLOR is set")

	fs.Usage = func() {
		fmt.Fprintf(stderr, `ship throbber — PROTOTYPE full-screen rocket-ascent wait scene (throwaway)

Not wired into the Run loop. Owns the whole terminal until done.

Usage:
  ship throbber [flags]

Flags:
`)
		fs.PrintDefaults()
		fmt.Fprintf(stderr, `
Examples:
  go run ./cmd/ship throbber
  go run ./cmd/ship throbber --seconds 20 --phase Review
  go run ./cmd/ship throbber --color   # if NO_COLOR is set in your env
`)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}

	color := !*noColor && (os.Getenv("NO_COLOR") == "" || *forceColor)
	if *forceColor {
		color = true
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *seconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(*seconds*float64(time.Second)))
		defer cancel()
	}

	_ = stdout
	Run(ctx, stderr, Options{Phase: *phase, Color: color})
	return 0
}
