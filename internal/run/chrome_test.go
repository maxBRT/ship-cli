package run_test

import (
	"context"
	"strings"
	"testing"

	"github.com/maxBRT/ship-cli/internal/gitops"
	"github.com/maxBRT/ship-cli/internal/run"
	"github.com/maxBRT/ship-cli/internal/ticket"
)

func TestWriteRunHeader_includesAgentAndBranch(t *testing.T) {
	var b strings.Builder
	run.WriteRunHeader(&b, "cursor", "feature/ship", true)
	got := b.String()
	for _, want := range []string{"ship", "cursor", "feature/ship", "agent", "branch"} {
		if !strings.Contains(got, want) {
			t.Errorf("WriteRunHeader missing %q; got:\n%s", want, got)
		}
	}
}

func TestWriteRunHeader_plainWhenColorOff(t *testing.T) {
	var b strings.Builder
	run.WriteRunHeader(&b, "cursor", "feature/ship", false)
	got := b.String()
	if strings.Contains(got, "\033[") {
		t.Errorf("plain WriteRunHeader must not emit ANSI; got %q", got)
	}
	for _, want := range []string{"ship", "cursor", "feature/ship", "agent", "branch"} {
		if !strings.Contains(got, want) {
			t.Errorf("WriteRunHeader missing %q; got:\n%s", want, got)
		}
	}
}

func TestFormatQueueRemaining_listsTicketNumbers(t *testing.T) {
	got := run.FormatQueueRemaining([]int{7, 9})
	if !strings.Contains(got, "#7") || !strings.Contains(got, "#9") {
		t.Fatalf("FormatQueueRemaining = %q, want #7 and #9", got)
	}
	if !strings.Contains(strings.ToLower(got), "remaining") && !strings.Contains(got, "queue") {
		t.Fatalf("FormatQueueRemaining = %q, want queue remaining hint", got)
	}
}

func TestFormatQueueRemaining_emptyIsBlank(t *testing.T) {
	if got := run.FormatQueueRemaining(nil); got != "" {
		t.Fatalf("FormatQueueRemaining(nil) = %q, want empty", got)
	}
}

func TestRun_printsChromeHeaderOnce(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{{Number: 7, Title: "seven"}}}
	prs := &fakePRs{}
	ag := committingAgent(t, prs, "ship/run")
	var chrome strings.Builder

	r := run.Orchestrator{
		Tickets: tickets,
		Agent:   ag,
		PRs:     prs,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", Agent: "cursor", MaxIterations: 10},
		Header:  &chrome,
		Stdout:  &strings.Builder{},
	}
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := chrome.String()
	if c := strings.Count(got, "run · agent"); c != 1 {
		t.Fatalf("run header count = %d, want 1; got:\n%s", c, got)
	}
	if !strings.Contains(got, "cursor") || !strings.Contains(got, "ship/run") {
		t.Fatalf("header missing agent/branch; got:\n%s", got)
	}
}

func TestRun_passesQueueRemainingToThrobber(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{
		{Number: 8, Title: "eight"},
		{Number: 7, Title: "seven"},
		{Number: 9, Title: "nine"},
	}}
	prs := &fakePRs{}
	ag := committingAgent(t, prs, "ship/run")
	th := &recordingThrobber{}

	r := run.Orchestrator{
		Tickets:  tickets,
		Agent:    ag,
		PRs:      prs,
		Repo:     gitops.Repo{Dir: dir},
		Config:   run.Config{Branch: "ship/run", MaxIterations: 1},
		Throbber: th,
		Stdout:   &strings.Builder{},
	}
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(th.remainings) < 2 {
		t.Fatalf("throbber remainings = %v, want Implement+Review hints", th.remainings)
	}
	for _, got := range th.remainings[:2] {
		if !strings.Contains(got, "#7") || !strings.Contains(got, "#9") {
			t.Errorf("Remaining = %q, want #7 and #9", got)
		}
		if !strings.Contains(got, "queue remaining") {
			t.Errorf("Remaining = %q, want queue remaining label", got)
		}
	}
}
