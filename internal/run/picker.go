package run

import (
	"context"
	"errors"
	"os"

	"github.com/maxBRT/ship-cli/internal/ticket"
)

// ErrNonInteractive is returned when the interactive picker cannot run because
// stdin/stderr are not a terminal. Ship refuses to half-stamp a queue.
var ErrNonInteractive = errors.New("interactive Ticket picker requires a terminal")

// ErrCanceled is returned when the user cancels queue confirmation.
var ErrCanceled = errors.New("queue confirmation canceled")

// Interactive is the production Queue: an interactive Ticket picker over
// ship-labeled candidates. Confirm, drop, and reorder happen before any Phase.
type Interactive struct {
	// IsTerminal reports whether an interactive session is available.
	// Optional; default requires stdin and stderr to be character devices.
	IsTerminal func() bool
	// Forms prompts for Queue confirmation on a TTY. Optional; default Forms (huh).
	Forms FormRunner
}

func (p Interactive) Confirm(_ context.Context, candidates []ticket.Ticket) ([]ticket.Ticket, error) {
	if !p.terminal() {
		return nil, ErrNonInteractive
	}
	return p.forms().ConfirmQueue(candidates)
}

func (p Interactive) forms() FormRunner {
	if p.Forms != nil {
		return p.Forms
	}
	return Forms{}
}

func (p Interactive) terminal() bool {
	if p.IsTerminal != nil {
		return p.IsTerminal()
	}
	return isTerminalFile(os.Stdin) && isTerminalFile(os.Stderr)
}

func isTerminalFile(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

var _ Queue = Interactive{}
var _ Queue = AllCandidates{}
