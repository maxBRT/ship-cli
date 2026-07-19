package prompt

import (
	"fmt"
	"strings"
)

// FinalInput is structured input for the Final phase prompt.
type FinalInput struct {
	Branch          string
	Tickets         []TicketInput // Done Tickets from this Run (linked in the PR)
	PartialProgress bool
	MaxIterations   int
}

// Final returns the built-in Final phase prompt.
// Ship verifies that Final opens a pull request.
func Final(in FinalInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are in the Final phase of a Ship Run on branch %s.\n\n", in.Branch)

	b.WriteString("Review the whole branch diff for this Run.\n")
	b.WriteString("Run the project's tests and e2e checks; fix any failures before opening the pull request.\n")
	b.WriteString("Open a pull request for this branch. The PR body must include RISK, QA notes, and linked Tickets.\n")
	b.WriteString("Ship verifies that Final opens a pull request; exiting without an opened PR is a failure.\n\n")

	if len(in.Tickets) > 0 {
		b.WriteString("Linked Tickets from this Run:\n")
		for _, t := range in.Tickets {
			fmt.Fprintf(&b, "- #%d: %s\n", t.Number, t.Title)
		}
		b.WriteString("\n")
	}

	if in.PartialProgress {
		fmt.Fprintf(&b,
			"This Run is Partial Progress: it stopped because it hit max iterations (%d) with Ready for Agent Tickets still remaining. Disclose that clearly in the PR body so reviewers know more work remains.\n",
			in.MaxIterations,
		)
	}

	return b.String()
}
