// Package ticket holds the Ticket tracker port (GitHub via gh for MVP).
//
// Port is the agent-agnostic surface the Run orchestrator calls. GitHub is the
// production implementation; tests inject an Exec that fakes gh.
package ticket
