package prompt

import (
	"fmt"
	"strings"
)

// ReviewInput is structured input for the Review phase prompt.
type ReviewInput struct {
	Ticket TicketInput
	Branch string
}

// Review returns the built-in Review phase prompt.
// Ship allows Review to succeed without a new commit.
func Review(in ReviewInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are in the Review phase of a Ship Run on branch %s.\n\n", in.Branch)
	fmt.Fprintf(&b, "Ticket #%d: %s\n\n", in.Ticket.Number, in.Ticket.Title)
	if in.Ticket.Body != "" {
		fmt.Fprintf(&b, "Ticket body:\n%s\n\n", in.Ticket.Body)
	}
	b.WriteString("Perform a single pass review of this Ticket's commits on the branch.\n")
	b.WriteString("You may adjust the code or commits if you find issues that need fixing in this same pass.\n")
	b.WriteString("Commitless success is allowed: if the Implement work is already correct, finish without a new commit. Ship does not require Review to produce commits.\n")
	return b.String()
}
