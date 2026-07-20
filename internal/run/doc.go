// Package run holds Run configuration and the Run orchestrator that drives
// queue confirmation, ship stamp, Implement, Review, Done over Tickets, then
// Final with pull-request side-effect verification. Phase failure, missing
// side effects, and timeouts Abort: undo that Ticket's commits and stop the
// Run without In Progress label restore.
package run
