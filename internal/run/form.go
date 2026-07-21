package run

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/maxBRT/ship-cli/internal/ticket"
)

// FormRunner is the operator prompt seam behind Init and Interactive Queue
// confirmation. Production uses Forms (huh); tests inject stubs. Queue and
// the orchestrator stay unaware of this seam.
type FormRunner interface {
	// PickAgent returns the Agent kind chosen for a new Shipfile.
	PickAgent() (string, error)
	// ConfirmQueue returns the ordered Ticket set for a Run, or ErrCanceled.
	ConfirmQueue(candidates []ticket.Ticket) ([]ticket.Ticket, error)
}

// Forms is the production FormRunner: huh Agent picker on Init and huh
// multi-select Queue confirmation (with Shift+↑/↓ reorder). Methods live in
// form_agent.go and form_queue.go.
type Forms struct{}

var _ FormRunner = Forms{}

// TypedLines is a FormRunner of typed-line prompts on In/Out.
// Kept for tests and non-huh callers; production TTY uses Forms.
type TypedLines struct {
	In  io.Reader
	Out io.Writer
}

func (t TypedLines) PickAgent() (string, error) {
	out := t.Out
	if out == nil {
		out = io.Discard
	}
	fmt.Fprintln(out, "Choose an Agent kind for this checkout:")
	fmt.Fprintln(out, "  cursor, pi, codex, claude")
	fmt.Fprint(out, "> ")

	in := t.In
	if in == nil {
		in = strings.NewReader("")
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read agent kind: %w", err)
	}
	kind := strings.TrimSpace(line)
	if err := validateAgentKind(kind); err != nil {
		return "", err
	}
	return kind, nil
}

func (t TypedLines) ConfirmQueue(candidates []ticket.Ticket) ([]ticket.Ticket, error) {
	in := t.In
	if in == nil {
		in = strings.NewReader("")
	}
	out := t.Out
	if out == nil {
		out = io.Discard
	}

	fmt.Fprintln(out, "ship Tickets - confirm the Run queue.")
	fmt.Fprintln(out)
	for i, tk := range candidates {
		fmt.Fprintf(out, "  %d. #%d %s\n", i+1, tk.Number, tk.Title)
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

var _ FormRunner = TypedLines{}
