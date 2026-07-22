package throbber_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/maxBRT/ship-cli/internal/throbber"
)

func TestStatus_Label_joinsPresentFields(t *testing.T) {
	got := throbber.Status{Phase: "Review", Iteration: 2, Ticket: "#50 Pi Agent adapter"}.Label()
	want := "Iteration 2 · Review · #50 Pi Agent adapter"
	if got != want {
		t.Errorf("Label() = %q, want %q", got, want)
	}
}

func TestStatus_Label_omitsEmptyFields(t *testing.T) {
	got := throbber.Status{Phase: "Final"}.Label()
	if got != "Final" {
		t.Errorf("Label() = %q, want %q", got, "Final")
	}
}

func TestSilent_During_runsWorkAndReturnsItsError(t *testing.T) {
	want := errors.New("phase failed")
	called := false

	err := throbber.Silent{}.During(context.Background(), throbber.Status{Phase: "Implement"}, func(context.Context) error {
		called = true
		return want
	})

	if !called {
		t.Fatal("work was not called")
	}
	if !errors.Is(err, want) {
		t.Fatalf("During = %v, want %v", err, want)
	}
}

func TestSilent_During_returnsNilWhenWorkSucceeds(t *testing.T) {
	err := throbber.Silent{}.During(context.Background(), throbber.Status{Phase: "Review"}, func(context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("During = %v, want nil", err)
	}
}

func TestLine_During_nonTTY_showsIterationPhaseTicket(t *testing.T) {
	var buf bytes.Buffer
	err := throbber.Line{Out: &buf, Color: false}.During(
		context.Background(),
		throbber.Status{Phase: "Implement", Iteration: 2, Ticket: "#7 one"},
		func(context.Context) error {
			time.Sleep(5 * time.Millisecond)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("During: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Iteration 2") {
		t.Errorf("output missing iteration: %q", out)
	}
	if !strings.Contains(out, "Implement") {
		t.Errorf("output missing phase: %q", out)
	}
	if !strings.Contains(out, "#7 one") {
		t.Errorf("output missing ticket: %q", out)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("output missing success finish: %q", out)
	}
}

func TestLine_During_nonTTY_finalOmitsIterationAndTicket(t *testing.T) {
	var buf bytes.Buffer
	err := throbber.Line{Out: &buf, Color: false}.During(
		context.Background(),
		throbber.Status{Phase: "Final"},
		func(context.Context) error { return nil },
	)
	if err != nil {
		t.Fatalf("During: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Final") {
		t.Errorf("output missing Final: %q", out)
	}
	if strings.Contains(out, "Iteration") {
		t.Errorf("Final wait must not show Iteration: %q", out)
	}
	if strings.Contains(out, "#") {
		t.Errorf("Final wait must not show Ticket: %q", out)
	}
}

func TestLine_During_nonTTY_showsQueueRemainingHint(t *testing.T) {
	var buf bytes.Buffer
	err := throbber.Line{Out: &buf, Color: false}.During(
		context.Background(),
		throbber.Status{
			Phase:     "Implement",
			Iteration: 1,
			Ticket:    "#8 Add throbber",
			Remaining: "queue remaining · #7 · #9",
		},
		func(context.Context) error { return nil },
	)
	if err != nil {
		t.Fatalf("During: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "queue remaining") {
		t.Errorf("output missing queue remaining hint: %q", out)
	}
	if !strings.Contains(out, "#7") || !strings.Contains(out, "#9") {
		t.Errorf("output missing remaining tickets: %q", out)
	}
}

func TestLine_During_nonTTY_failureShowsNoCheckmark(t *testing.T) {
	var buf bytes.Buffer
	want := errors.New("boom")
	err := throbber.Line{Out: &buf, Color: false}.During(
		context.Background(),
		throbber.Status{Phase: "Review", Iteration: 1, Ticket: "#3 fix"},
		func(context.Context) error { return want },
	)
	if !errors.Is(err, want) {
		t.Fatalf("During = %v, want %v", err, want)
	}
	out := buf.String()
	if strings.Contains(out, "✓") {
		t.Errorf("failure finish must not show checkmark: %q", out)
	}
	if !strings.Contains(out, "failed") {
		t.Errorf("output missing failure finish: %q", out)
	}
}
