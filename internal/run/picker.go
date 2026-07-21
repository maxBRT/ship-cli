package run

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

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
	In  io.Reader // optional; default os.Stdin
	Out io.Writer // optional; default os.Stderr
	// IsTerminal reports whether an interactive session is available.
	// Optional; default requires stdin and stderr to be character devices.
	IsTerminal func() bool
}

func (p Interactive) Confirm(_ context.Context, candidates []ticket.Ticket) ([]ticket.Ticket, error) {
	if !p.terminal() {
		return nil, ErrNonInteractive
	}
	in := p.in()
	out := p.out()

	fmt.Fprintln(out, "ship Tickets - confirm the Run queue.")
	fmt.Fprintln(out)
	for i, t := range candidates {
		fmt.Fprintf(out, "  %d. #%d %s\n", i+1, t.Number, t.Title)
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Enter numbers in Run order (e.g. 2 1), empty line for all,")
	fmt.Fprintln(out, "'none' for empty selection, or 'q' to cancel:")
	fmt.Fprint(out, "> ")

	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("read picker input: %w", err)
	}
	line = strings.TrimSpace(line)

	switch {
	case line == "q" || line == "cancel":
		return nil, ErrCanceled
	case line == "none":
		return []ticket.Ticket{}, nil
	case line == "" || line == "all":
		outTickets := make([]ticket.Ticket, len(candidates))
		copy(outTickets, candidates)
		return outTickets, nil
	}

	fields := strings.Fields(line)
	selected := make([]ticket.Ticket, 0, len(fields))
	seen := make(map[int]struct{}, len(fields))
	for _, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 1 || n > len(candidates) {
			return nil, fmt.Errorf("invalid picker selection %q (want 1..%d)", f, len(candidates))
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		selected = append(selected, candidates[n-1])
	}
	return selected, nil
}

func (p Interactive) terminal() bool {
	if p.IsTerminal != nil {
		return p.IsTerminal()
	}
	return isTerminalFile(os.Stdin) && isTerminalFile(os.Stderr)
}

func (p Interactive) in() io.Reader {
	if p.In != nil {
		return p.In
	}
	return os.Stdin
}

func (p Interactive) out() io.Writer {
	if p.Out != nil {
		return p.Out
	}
	return os.Stderr
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
