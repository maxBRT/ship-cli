package run_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/maxBRT/ship-cli/internal/run"
	"github.com/maxBRT/ship-cli/internal/ticket"
)

func TestInteractive_nonTTYFailsClearly(t *testing.T) {
	p := run.Interactive{
		IsTerminal: func() bool {
			return false
		},
	}
	candidates := []ticket.Ticket{{Number: 7, Title: "seven"}}

	got, err := p.Confirm(context.Background(), candidates)
	if err == nil {
		t.Fatal("Confirm error = nil, want non-interactive failure")
	}
	if !errors.Is(err, run.ErrNonInteractive) {
		t.Errorf("Confirm error = %v, want ErrNonInteractive", err)
	}
	if !strings.Contains(err.Error(), "terminal") {
		t.Errorf("Confirm error = %v, want clear terminal guidance", err)
	}
	if got != nil {
		t.Errorf("Confirm tickets = %v, want nil", got)
	}
}

func TestInteractive_confirmAllKeepsListedOrder(t *testing.T) {
	p := run.Interactive{
		Forms: run.TypedLines{
			In:  strings.NewReader("\n"),
			Out: io.Discard,
		},
		IsTerminal: func() bool {
			return true
		},
	}
	candidates := []ticket.Ticket{
		{Number: 7, Title: "seven"},
		{Number: 8, Title: "eight"},
	}

	got, err := p.Confirm(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if len(got) != 2 || got[0].Number != 7 || got[1].Number != 8 {
		t.Fatalf("Confirm = %v, want [7 8] in listed order", ticketNumbers(got))
	}
}

func TestInteractive_dropAndReorderViaIndexList(t *testing.T) {
	p := run.Interactive{
		Forms: run.TypedLines{
			In:  strings.NewReader("3 1\n"),
			Out: io.Discard,
		},
		IsTerminal: func() bool {
			return true
		},
	}
	candidates := []ticket.Ticket{
		{Number: 7, Title: "seven"},
		{Number: 8, Title: "eight"},
		{Number: 9, Title: "nine"},
	}

	got, err := p.Confirm(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if len(got) != 2 || got[0].Number != 9 || got[1].Number != 7 {
		t.Fatalf("Confirm = %v, want [9 7] (drop #8, reorder)", ticketNumbers(got))
	}
}

func TestInteractive_cancelReturnsErrCanceled(t *testing.T) {
	p := run.Interactive{
		Forms: run.TypedLines{
			In:  strings.NewReader("q\n"),
			Out: io.Discard,
		},
		IsTerminal: func() bool {
			return true
		},
	}

	got, err := p.Confirm(context.Background(), []ticket.Ticket{{Number: 7}})
	if !errors.Is(err, run.ErrCanceled) {
		t.Fatalf("Confirm error = %v, want ErrCanceled", err)
	}
	if got != nil {
		t.Errorf("Confirm tickets = %v, want nil", got)
	}
}

func TestInteractive_noneConfirmsEmptySelection(t *testing.T) {
	p := run.Interactive{
		Forms: run.TypedLines{
			In:  strings.NewReader("none\n"),
			Out: io.Discard,
		},
		IsTerminal: func() bool {
			return true
		},
	}

	got, err := p.Confirm(context.Background(), []ticket.Ticket{{Number: 7}, {Number: 8}})
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Confirm = %v, want empty selection", ticketNumbers(got))
	}
}

func ticketNumbers(ts []ticket.Ticket) []int {
	out := make([]int, len(ts))
	for i, t := range ts {
		out[i] = t.Number
	}
	return out
}
