package prompt_test

import (
	"strings"
	"testing"

	"github.com/maxBRT/ship-cli/internal/prompt"
)

func TestImplement_requiresCommitAndIncludesTicketContext(t *testing.T) {
	got := prompt.Implement(prompt.ImplementInput{
		Ticket: prompt.TicketInput{
			Number: 9,
			Title:  "Built-in prompts for Implement, Review, and Final",
			Body:   "## Acceptance criteria\n\n- ship verifies commits",
		},
		Branch: "ship/abc123",
	})

	assertContains(t, got,
		"#9",
		"Built-in prompts for Implement, Review, and Final",
		"## Acceptance criteria",
		"ship verifies commits",
		"ship/abc123",
	)
	assertContainsFold(t, got,
		"must commit",
	)
}

func TestReview_allowsCommitlessSinglePass(t *testing.T) {
	got := prompt.Review(prompt.ReviewInput{
		Ticket: prompt.TicketInput{
			Number: 9,
			Title:  "Built-in prompts for Implement, Review, and Final",
			Body:   "Review the Implement commits for this Ticket.",
		},
		Branch: "ship/abc123",
	})

	assertContains(t, got,
		"#9",
		"Built-in prompts for Implement, Review, and Final",
		"Review the Implement commits for this Ticket.",
		"ship/abc123",
	)
	assertContainsFold(t, got,
		"single pass",
		"may adjust",
		"without a new commit",
	)
}

func TestFinal_opensPRWithRiskQAAndLinkedTickets(t *testing.T) {
	got := prompt.Final(prompt.FinalInput{
		Branch: "ship/abc123",
		Tickets: []prompt.TicketInput{
			{Number: 9, Title: "Built-in prompts"},
			{Number: 8, Title: "Cursor Agent adapter"},
		},
		PartialProgress: false,
		MaxIterations:   10,
	})

	assertContains(t, got,
		"ship/abc123",
		"#9",
		"Built-in prompts",
		"#8",
		"Cursor Agent adapter",
	)
	assertContainsFold(t, got,
		"tests",
		"e2e",
		"open a pull request",
		"RISK",
		"QA",
		"opens a pull request",
	)
	assertNotContainsFold(t, got,
		"partial progress",
		"max iterations",
	)
}

func TestFinal_disclosesPartialProgressWhenFlagSet(t *testing.T) {
	got := prompt.Final(prompt.FinalInput{
		Branch: "ship/abc123",
		Tickets: []prompt.TicketInput{
			{Number: 9, Title: "Built-in prompts"},
		},
		PartialProgress: true,
		MaxIterations:   10,
	})

	assertContainsFold(t, got,
		"partial progress",
		"max iterations",
		"10",
	)
}

func assertContains(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q\nprompt:\n%s", want, got)
		}
	}
}

func assertContainsFold(t *testing.T, got string, wants ...string) {
	t.Helper()
	lower := strings.ToLower(got)
	for _, want := range wants {
		if !strings.Contains(lower, strings.ToLower(want)) {
			t.Errorf("prompt missing %q (case-insensitive)\nprompt:\n%s", want, got)
		}
	}
}

func assertNotContainsFold(t *testing.T, got string, wants ...string) {
	t.Helper()
	lower := strings.ToLower(got)
	for _, want := range wants {
		if strings.Contains(lower, strings.ToLower(want)) {
			t.Errorf("prompt unexpectedly contains %q\nprompt:\n%s", want, got)
		}
	}
}
