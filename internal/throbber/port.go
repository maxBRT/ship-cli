package throbber

import "context"

// Status labels a Phase wait: Iteration, Phase, and Ticket when applicable.
type Status struct {
	Phase     string // Implement, Review, or Final
	Iteration int    // 1-based; 0 omits Iteration (Final)
	Ticket    string // e.g. "#7 one"; empty omits Ticket (Final)
}

// Port is the Phase-wait UI surface the Run orchestrator calls.
type Port interface {
	// During runs work while showing wait UI for status. It returns work's
	// error unchanged.
	//
	// Invariants:
	//   - Terminal is restored before During returns (success or fail).
	//   - work error == nil → success chrome; else → failure chrome.
	//   - Non-TTY / Silent adapters must still be safe and cheap.
	//   - ctx cancel stops the wait UI; work's ctx is the same ctx.
	During(ctx context.Context, status Status, work func(context.Context) error) error
}

// Silent is a no-op Port for tests. Production non-TTY output is handled
// inside Line (plain waiting lines), not by Silent.
type Silent struct{}

var _ Port = Silent{}

func (Silent) During(ctx context.Context, _ Status, work func(context.Context) error) error {
	return work(ctx)
}
