package prompt

import (
	"fmt"
	"strings"
)

// TicketInput is Ticket identity and body for phase prompts.
type TicketInput struct {
	Number int
	Title  string
	Body   string
}

// ImplementInput is structured input for the Implement phase prompt.
type ImplementInput struct {
	Ticket TicketInput
	Branch string
}

// Implement returns the built-in Implement phase prompt.
// Ship verifies that this phase produces commit(s).
func Implement(in ImplementInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are in the Implement phase of a Ship Run on branch %s.\n\n", in.Branch)
	fmt.Fprintf(&b, "Ticket #%d: %s\n\n", in.Ticket.Number, in.Ticket.Title)
	if in.Ticket.Body != "" {
		fmt.Fprintf(&b, "Ticket body:\n%s\n\n", in.Ticket.Body)
	}
	b.WriteString("Do the Ticket work described above.\n")
	b.WriteString("You must commit your changes before finishing. Ship verifies that Implement produces at least one commit; a commitless exit is a failure.\n")
	return b.String()
}
