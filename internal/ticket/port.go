package ticket

import "context"

// Ticket is a unit of work a Run may process through an Iteration once it is
// confirmed into the Run's ship queue.
type Ticket struct {
	Number int
	Title  string
}

// Label names on the GitHub tracker for Ticket states Ship manages.
const (
	LabelReadyForAgent = "ready-for-agent"
	LabelShip          = "ship"
)

// Port is the tracker surface the Run orchestrator calls. Tests can fake it;
// the GitHub implementation shells to gh.
type Port interface {
	// EnsureLabels creates the Ready for Agent and ship labels when they are
	// missing from the tracker, so queue stamping and Done can succeed.
	EnsureLabels(ctx context.Context) error

	// ListReady returns Tickets labeled ship in stable default order
	// (issue number / created date).
	ListReady(ctx context.Context) ([]Ticket, error)

	// Stamp adds the ship label to the confirmed queue without removing
	// Ready for Agent.
	Stamp(ctx context.Context, tickets []Ticket) error

	// Done removes ship and closes the Ticket's GitHub issue.
	Done(ctx context.Context, t Ticket) error
}
