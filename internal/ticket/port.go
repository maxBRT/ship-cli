package ticket

import "context"

// Ticket is a unit of work a Run may Claim and process through an Iteration.
type Ticket struct {
	Number int
	Title  string
}

// Label names on the GitHub tracker for Ticket states Ship manages.
const (
	LabelReadyForAgent = "ready-for-agent"
	LabelInProgress    = "in-progress"
)

// Port is the tracker surface the Run orchestrator calls. Tests can fake it;
// the GitHub implementation shells to gh.
type Port interface {
	// EnsureLabels creates the Ready for Agent and In Progress labels when
	// they are missing from the tracker, so Claim and Abort can succeed.
	EnsureLabels(ctx context.Context) error

	// ListReady returns Tickets labeled Ready for Agent, optionally also
	// matching feature, ordered by native GitHub priority then oldest.
	ListReady(ctx context.Context, feature string) ([]Ticket, error)

	// Claim moves a Ticket from Ready for Agent to In Progress.
	Claim(ctx context.Context, t Ticket) error

	// Done marks a Ticket Done by closing its GitHub issue.
	Done(ctx context.Context, t Ticket) error

	// Abort restores a Ticket from In Progress to Ready for Agent.
	Abort(ctx context.Context, t Ticket) error
}
