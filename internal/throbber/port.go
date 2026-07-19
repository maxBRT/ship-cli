package throbber

import "context"

// Port is the Phase-wait UI surface the Run orchestrator calls.
type Port interface {
	// During runs work while showing wait UI labeled by phase
	// ("Implement", "Review", "Final"). It returns work's error unchanged.
	//
	// Invariants:
	//   - Terminal is restored before During returns (success or fail).
	//   - work error == nil → success chrome; else → failure chrome.
	//   - Non-TTY / Silent adapters must still be safe and cheap.
	//   - ctx cancel stops the wait UI; work's ctx is the same ctx.
	During(ctx context.Context, phase string, work func(context.Context) error) error
}

// Silent is a no-op Port for tests. Production non-TTY output is handled
// inside Tableau (plain waiting lines), not by Silent.
type Silent struct{}

var _ Port = Silent{}

func (Silent) During(ctx context.Context, _ string, work func(context.Context) error) error {
	return work(ctx)
}
