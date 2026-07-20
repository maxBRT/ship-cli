package run

import (
	"context"

	"github.com/maxBRT/ship-cli/internal/ticket"
)

// Queue is the fakeable picker/queue port under the Run Orchestrator. It
// confirms which Ready for Agent candidates become this Run's ordered ship
// queue. Production uses Interactive; tests inject fakes. Nil Queue means
// AllCandidates (confirm every Ready for Agent Ticket in listed order).
type Queue interface {
	Confirm(ctx context.Context, candidates []ticket.Ticket) ([]ticket.Ticket, error)
}

// AllCandidates confirms the Ready for Agent list as-is. Useful as a test
// default and as the nil-Queue fallback; production Ships use Interactive.
type AllCandidates struct{}

func (AllCandidates) Confirm(_ context.Context, candidates []ticket.Ticket) ([]ticket.Ticket, error) {
	out := make([]ticket.Ticket, len(candidates))
	copy(out, candidates)
	return out, nil
}
