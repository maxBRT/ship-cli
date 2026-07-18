package agent

import (
	"context"
	"time"
)

// Port is the Agent surface the Run orchestrator calls for a Phase.
// Implementations stay behind this seam so the Run loop stays agent-agnostic.
type Port interface {
	// RunPhase runs one fresh Agent invocation for a Phase and returns nil on
	// CLI success. Non-zero exit, timeout, or missing success signaling is an error.
	RunPhase(ctx context.Context, req PhaseRequest) error
}

// PhaseRequest is one fresh Agent invocation within a Run.
type PhaseRequest struct {
	Prompt    string
	Workspace string
	Model     string // optional; empty omits --model
	Timeout   time.Duration
}
