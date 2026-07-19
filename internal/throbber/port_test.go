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

func TestSilent_During_runsWorkAndReturnsItsError(t *testing.T) {
	want := errors.New("phase failed")
	called := false

	err := throbber.Silent{}.During(context.Background(), "Implement", func(context.Context) error {
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
	err := throbber.Silent{}.During(context.Background(), "Review", func(context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("During = %v, want nil", err)
	}
}

func TestTableau_During_nonTTY_successShowsWaitingAndDone(t *testing.T) {
	var buf bytes.Buffer
	err := throbber.Tableau{Out: &buf, Color: false}.During(
		context.Background(), "Implement",
		func(context.Context) error {
			time.Sleep(5 * time.Millisecond)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("During: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "waiting on Implement") {
		t.Errorf("output missing waiting line: %q", out)
	}
	if !strings.Contains(out, "Implement") || !strings.Contains(out, "✓") {
		t.Errorf("output missing success finish: %q", out)
	}
}

func TestTableau_During_nonTTY_failureShowsNoCheckmark(t *testing.T) {
	var buf bytes.Buffer
	want := errors.New("boom")
	err := throbber.Tableau{Out: &buf, Color: false}.During(
		context.Background(), "Review",
		func(context.Context) error { return want },
	)
	if !errors.Is(err, want) {
		t.Fatalf("During = %v, want %v", err, want)
	}
	out := buf.String()
	if !strings.Contains(out, "waiting on Review") {
		t.Errorf("output missing waiting line: %q", out)
	}
	if strings.Contains(out, "✓") {
		t.Errorf("failure finish must not show checkmark: %q", out)
	}
	if !strings.Contains(out, "failed") {
		t.Errorf("output missing failure finish: %q", out)
	}
}
