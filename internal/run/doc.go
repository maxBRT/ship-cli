// Package run holds Run configuration and the Run orchestrator that drives the
// sequential Claim, Implement, Review, Done loop over Tickets, then Final with
// pull-request side-effect verification. Phase failure, missing side effects,
// and timeouts Abort: restore the Ticket, undo its commits, and stop the Run.
package run
