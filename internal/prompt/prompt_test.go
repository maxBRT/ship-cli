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
		"at least one git commit",
		"must commit",
		"red before green",
		"pre-agreed seams",
		"implementation-coupled",
		"tautological",
		"horizontal slicing",
		"vertical slices",
	)
	assertNotContainsFold(t, got,
		"/tdd",
		"ship run",
		"implement phase",
	)
}

func TestReview_allowsCommitlessSinglePass(t *testing.T) {
	got := prompt.Review(prompt.ReviewInput{
		Ticket: prompt.TicketInput{
			Number: 9,
			Title:  "Built-in prompts for Implement, Review, and Final",
			Body:   "Acceptance: review recent commits for this issue.",
		},
		Branch: "ship/abc123",
	})

	assertContains(t, got,
		"#9",
		"Built-in prompts for Implement, Review, and Final",
		"Acceptance: review recent commits for this issue.",
		"ship/abc123",
	)
	assertContainsFold(t, got,
		"single pass",
		"without a new commit",
		"standards",
		"spec",
		"smell baseline",
	)
	assertNotContainsFold(t, got,
		"/code-review",
		"ship run",
		"review phase",
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
		"integration review",
		"tests",
		"e2e",
		"lint",
		"typecheck",
		"open a pull request",
		"RISK",
		"QA",
		"happy path",
		"out of scope",
		"commit",
		"opened pr",
	)
	assertNotContainsFold(t, got,
		"partial progress",
		"max iterations",
		"final phase",
		"ship run",
		"light final",
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
		"skip the deep",
		"green bar",
		"open",
		"pull request",
	)
	assertNotContainsFold(t, got,
		"branch-level integration review",
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
