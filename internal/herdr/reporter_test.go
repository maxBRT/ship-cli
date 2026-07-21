package herdr_test

import (
	"context"
	"errors"
	"testing"

	"github.com/maxBRT/ship-cli/internal/herdr"
)

func TestReporter_envAbsent_doesNotReport(t *testing.T) {
	// Outside Herdr, Ship must not invoke the multiplexer CLI — normal
	// terminals and CI stay unchanged even if herdr is missing.
	var calls int
	r := herdr.Reporter{
		LookupEnv: func(string) (string, bool) { return "", false },
		Exec: func(context.Context, ...string) error {
			calls++
			return nil
		},
	}
	r.Working(context.Background(), "Implement")
	r.Idle(context.Background())
	if calls != 0 {
		t.Errorf("herdr exec calls = %d, want 0 when HERDR_* env is absent", calls)
	}
}

func TestReporter_envPresent_reportsWorkingAndIdle(t *testing.T) {
	// Worked example from the Herdr socket contract / CLI wrapper.
	env := map[string]string{
		"HERDR_ENV":         "1",
		"HERDR_SOCKET_PATH": "/tmp/herdr.sock",
		"HERDR_PANE_ID":     "w1:p1",
	}
	var calls [][]string
	r := herdr.Reporter{
		LookupEnv: func(k string) (string, bool) {
			v, ok := env[k]
			return v, ok
		},
		Exec: func(_ context.Context, args ...string) error {
			cp := make([]string, len(args))
			copy(cp, args)
			calls = append(calls, cp)
			return nil
		},
	}

	r.Working(context.Background(), "Iteration 2 · Review · #50 Pi Agent adapter")
	r.Idle(context.Background())

	want := [][]string{
		{"pane", "report-agent", "w1:p1", "--source", "ship:run", "--agent", "ship", "--state", "working", "--message", "Iteration 2 · Review · #50 Pi Agent adapter"},
		{"pane", "report-agent", "w1:p1", "--source", "ship:run", "--agent", "ship", "--state", "idle"},
	}
	if len(calls) != len(want) {
		t.Fatalf("herdr exec calls = %d, want %d; got %v", len(calls), len(want), calls)
	}
	for i := range want {
		if !equalStrings(calls[i], want[i]) {
			t.Errorf("call %d = %v, want %v", i, calls[i], want[i])
		}
	}
}

func TestReporter_execError_doesNotPanic(t *testing.T) {
	// Missing herdr binary or socket failure must not surface into the Run.
	env := map[string]string{
		"HERDR_ENV":         "1",
		"HERDR_SOCKET_PATH": "/tmp/herdr.sock",
		"HERDR_PANE_ID":     "w1:p1",
	}
	r := herdr.Reporter{
		LookupEnv: func(k string) (string, bool) {
			v, ok := env[k]
			return v, ok
		},
		Exec: func(context.Context, ...string) error {
			return errors.New("herdr: executable file not found")
		},
	}
	r.Working(context.Background(), "Final")
	r.Idle(context.Background())
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
