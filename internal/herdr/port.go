package herdr

import "context"

// Port is the multiplexer agent-state surface the Run orchestrator calls
// around each Phase. Optional; nil means no reporting.
//
// Reports are best-effort: implementations must not fail the Phase.
type Port interface {
	// Working announces that a Phase is in progress. Message may describe the
	// Phase / Ticket for sidebar display.
	Working(ctx context.Context, message string)
	// Idle announces that the Phase has ended (success, abort, or timeout).
	Idle(ctx context.Context)
}
