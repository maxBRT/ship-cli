package run_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/maxBRT/ship-cli/internal/run"
	"github.com/maxBRT/ship-cli/internal/ticket"
)

func TestInteractive_nonTTYFailsClearly(t *testing.T) {
	var out bytes.Buffer
	p := run.Interactive{
		In:  strings.NewReader(""),
		Out: &out,
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
	if out.Len() != 0 {
		t.Errorf("picker wrote %q before failing non-interactively", out.String())
	}
}

func TestInteractive_confirmAllKeepsListedOrder(t *testing.T) {
	var out bytes.Buffer
	p := run.Interactive{
		In:  strings.NewReader("\n"),
		Out: &out,
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
	var out bytes.Buffer
	p := run.Interactive{
		In:  strings.NewReader("3 1\n"),
		Out: &out,
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
	var out bytes.Buffer
	p := run.Interactive{
		In:  strings.NewReader("q\n"),
		Out: &out,
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
	var out bytes.Buffer
	p := run.Interactive{
		In:  strings.NewReader("none\n"),
		Out: &out,
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

func TestInteractive_showsLeftoverShipHint(t *testing.T) {
	var out bytes.Buffer
	p := run.Interactive{
		In:  strings.NewReader("\n"),
		Out: &out,
		IsTerminal: func() bool {
			return true
		},
	}
	candidates := []ticket.Ticket{
		{Number: 7, Title: "leftover", OnShip: true},
		{Number: 8, Title: "fresh", OnShip: false},
	}

	if _, err := p.Confirm(context.Background(), candidates); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "#7") || !strings.Contains(text, "leftover") {
		t.Errorf("picker output missing Ticket #7:\n%s", text)
	}
	if !strings.Contains(text, "[ship]") {
		t.Errorf("picker output missing leftover ship hint:\n%s", text)
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "#8") && strings.Contains(line, "[ship]") {
			t.Errorf("fresh Ticket line should not show [ship]: %q", line)
		}
	}
}

func ticketNumbers(ts []ticket.Ticket) []int {
	out := make([]int, len(ts))
	for i, t := range ts {
		out[i] = t.Number
	}
	return out
}
