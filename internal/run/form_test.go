package run_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/maxBRT/ship-cli/internal/run"
	"github.com/maxBRT/ship-cli/internal/ticket"
)

// stubForms returns canned FormRunner answers for Init and Queue tests.
type stubForms struct {
	agent     string
	agentErr  error
	queue     []ticket.Ticket
	queueErr  error
}

func (s stubForms) PickAgent() (string, error) {
	return s.agent, s.agentErr
}

func (s stubForms) ConfirmQueue([]ticket.Ticket) ([]ticket.Ticket, error) {
	return s.queue, s.queueErr
}

func TestInit_formRunnerPickWritesAgent(t *testing.T) {
	dir := t.TempDir()

	created, err := (run.Init{
		Forms: stubForms{agent: "claude"},
		IsTerminal: func() bool {
			return true
		},
	}).Config(dir)
	if err != nil {
		t.Fatalf("Init.Config: %v", err)
	}
	if !created {
		t.Fatal("Init.Config: want created=true")
	}

	cfg, err := run.LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Agent != "claude" {
		t.Errorf("Agent = %q, want claude", cfg.Agent)
	}
}

func TestInit_formRunnerErrorDoesNotWriteShipfile(t *testing.T) {
	dir := t.TempDir()

	created, err := (run.Init{
		Forms: stubForms{agentErr: run.ErrCanceled},
		IsTerminal: func() bool {
			return true
		},
	}).Config(dir)
	if err == nil {
		t.Fatal("Init.Config: want error when FormRunner fails")
	}
	if created {
		t.Fatal("Init.Config: want created=false on FormRunner error")
	}
	if _, statErr := os.Stat(shipConfigPath(dir)); !os.IsNotExist(statErr) {
		t.Fatalf("Shipfile should not exist after cancel; stat=%v", statErr)
	}
}

func TestInteractive_formRunnerConfirmReturnsSelection(t *testing.T) {
	candidates := []ticket.Ticket{
		{Number: 7, Title: "seven"},
		{Number: 8, Title: "eight"},
		{Number: 9, Title: "nine"},
	}
	p := run.Interactive{
		Forms: stubForms{queue: []ticket.Ticket{candidates[2], candidates[0]}},
		IsTerminal: func() bool {
			return true
		},
	}

	got, err := p.Confirm(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if len(got) != 2 || got[0].Number != 9 || got[1].Number != 7 {
		t.Fatalf("Confirm = %v, want [9 7]", ticketNumbers(got))
	}
}

func TestInteractive_formRunnerCancelReturnsErrCanceled(t *testing.T) {
	p := run.Interactive{
		Forms: stubForms{queueErr: run.ErrCanceled},
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

func TestInteractive_formRunnerEmptySelection(t *testing.T) {
	p := run.Interactive{
		Forms: stubForms{queue: []ticket.Ticket{}},
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

func TestInit_nonTTYIgnoresFormRunner(t *testing.T) {
	dir := t.TempDir()

	created, err := (run.Init{
		Forms: stubForms{agentErr: errors.New("FormRunner must not run off TTY")},
		IsTerminal: func() bool {
			return false
		},
	}).Config(dir)
	if err != nil {
		t.Fatalf("Init.Config: %v", err)
	}
	if !created {
		t.Fatal("Init.Config: want created=true")
	}
	cfg, err := run.LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Agent != "cursor" {
		t.Errorf("Agent = %q, want cursor", cfg.Agent)
	}
}

func TestInteractive_nonTTYIgnoresFormRunner(t *testing.T) {
	p := run.Interactive{
		Forms: stubForms{queue: []ticket.Ticket{{Number: 99}}},
		IsTerminal: func() bool {
			return false
		},
	}

	got, err := p.Confirm(context.Background(), []ticket.Ticket{{Number: 7}})
	if !errors.Is(err, run.ErrNonInteractive) {
		t.Fatalf("Confirm error = %v, want ErrNonInteractive", err)
	}
	if got != nil {
		t.Errorf("Confirm tickets = %v, want nil", got)
	}
}
