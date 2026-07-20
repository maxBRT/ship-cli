package run_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/maxBRT/ship-cli/internal/agent"
	"github.com/maxBRT/ship-cli/internal/gitops"
	"github.com/maxBRT/ship-cli/internal/run"
	"github.com/maxBRT/ship-cli/internal/throbber"
	"github.com/maxBRT/ship-cli/internal/ticket"
)

func TestRun_wrapsEachPhaseInThrobberDuring(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{{Number: 1, Title: "one"}}}
	prs := &fakePRs{}
	ag := committingAgent(t, prs, "ship/run")
	th := &recordingThrobber{}
	var out strings.Builder

	r := run.Orchestrator{
		Tickets:  tickets,
		Agent:    ag,
		PRs:      prs,
		Repo:     gitops.Repo{Dir: dir},
		Config:   run.Config{Branch: "ship/run", MaxIterations: 10},
		Throbber: th,
		Stdout:   &out,
	}
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := []string{"Implement", "Review", "Final"}
	if got := th.phases; !slices.Equal(got, want) {
		t.Errorf("throbber phases = %v, want %v", got, want)
	}
	if th.workCalls != 3 {
		t.Errorf("throbber work calls = %d, want 3", th.workCalls)
	}
	if len(ag.reqs) != 3 {
		t.Fatalf("agent called %d times, want 3", len(ag.reqs))
	}
}

func TestRun_emptyQueueExitsWithoutBranchOrWork(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{}
	ag := &fakeAgent{}
	var out strings.Builder

	r := run.Orchestrator{
		Tickets: tickets,
		Agent:   ag,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{MaxIterations: 10},
		Stdout:  &out,
	}
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !tickets.ensured {
		t.Error("EnsureLabels was not called before listing Ready for Agent Tickets")
	}
	if !strings.Contains(out.String(), "No Ready for Agent Tickets") {
		t.Errorf("stdout = %q, want empty-queue message", out.String())
	}
	if b := currentBranch(t, dir); b != "main" {
		t.Errorf("branch = %q, want main (no branch prepared)", b)
	}
	if len(tickets.claimed) != 0 {
		t.Errorf("claimed = %v, want none", tickets.claimed)
	}
	if len(ag.reqs) != 0 {
		t.Errorf("agent called %d times, want 0", len(ag.reqs))
	}
}

func TestRun_ensureLabelsFailureStopsBeforeClaim(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{
		ready:     []ticket.Ticket{{Number: 1, Title: "one"}},
		ensureErr: errors.New("no permission to create labels"),
	}
	ag := &fakeAgent{}
	var out strings.Builder

	r := run.Orchestrator{
		Tickets: tickets,
		Agent:   ag,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", MaxIterations: 10},
		Stdout:  &out,
	}
	err := r.Run(context.Background())
	if err == nil {
		t.Fatal("Run error = nil, want ensure labels failure")
	}
	if !strings.Contains(err.Error(), "ensure tracker labels") {
		t.Fatalf("Run error = %v, want ensure tracker labels wrap", err)
	}
	if len(tickets.claimed) != 0 {
		t.Errorf("claimed = %v, want none when EnsureLabels fails", tickets.claimed)
	}
	if len(ag.reqs) != 0 {
		t.Errorf("agent called %d times, want 0", len(ag.reqs))
	}
	if b := currentBranch(t, dir); b != "main" {
		t.Errorf("branch = %q, want main (no branch prepared)", b)
	}
}

func TestRun_processesEachTicketThroughClaimImplementReviewDone(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{{Number: 7, Title: "seven"}, {Number: 8, Title: "eight"}}}
	prs := &fakePRs{}
	ag := committingAgent(t, prs, "ship/run")
	var out strings.Builder

	r := run.Orchestrator{
		Tickets: tickets,
		Agent:   ag,
		PRs:     prs,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", MaxIterations: 10, Model: "composer"},
		Stdout:  &out,
	}
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if b := currentBranch(t, dir); b != "ship/run" {
		t.Errorf("branch = %q, want ship/run", b)
	}
	if got := tickets.claimed; !equalInts(got, []int{7, 8}) {
		t.Errorf("claimed = %v, want [7 8]", got)
	}
	if got := tickets.doneList; !equalInts(got, []int{7, 8}) {
		t.Errorf("done = %v, want [7 8]", got)
	}
	// Two Phases (Implement, Review) per Ticket, then Final.
	if len(ag.reqs) != 5 {
		t.Fatalf("agent called %d times, want 5", len(ag.reqs))
	}
	// Each Phase gets the checkout as its workspace and the configured model.
	for i, req := range ag.reqs {
		if req.Workspace != dir {
			t.Errorf("req %d workspace = %q, want %q", i, req.Workspace, dir)
		}
		if req.Model != "composer" {
			t.Errorf("req %d model = %q, want composer", i, req.Model)
		}
	}
	// Implement Phase for Ticket 7 references its number and the branch.
	if !strings.Contains(ag.reqs[0].Prompt, "#7") || !strings.Contains(ag.reqs[0].Prompt, "ship/run") {
		t.Errorf("first prompt missing Ticket/branch context:\n%s", ag.reqs[0].Prompt)
	}
	if !strings.Contains(out.String(), "Queue drained") {
		t.Errorf("stdout = %q, want drain summary", out.String())
	}
}

func TestRun_processesFrozenReadyOrderDespiteMidRunReadyChanges(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{
		ready: []ticket.Ticket{{Number: 7, Title: "seven"}, {Number: 8, Title: "eight"}},
		afterClaim: func(f *fakeTickets, claimed ticket.Ticket) {
			if claimed.Number == 7 {
				// Mid-Run tracker rewrite: #99 jumps ahead and #8 disappears.
				// A re-list between Iterations would process #99 next; the
				// frozen snapshot must still walk #7 then #8.
				f.ready = []ticket.Ticket{{Number: 99, Title: "intruder"}}
			}
		},
	}
	prs := &fakePRs{}
	ag := committingAgent(t, prs, "ship/run")

	r := run.Orchestrator{
		Tickets: tickets,
		Agent:   ag,
		PRs:     prs,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", MaxIterations: 10},
		Stdout:  &strings.Builder{},
	}
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := tickets.claimed; !equalInts(got, []int{7, 8}) {
		t.Errorf("claimed = %v, want [7 8] (frozen Ready for Agent order)", got)
	}
	if got := tickets.doneList; !equalInts(got, []int{7, 8}) {
		t.Errorf("done = %v, want [7 8] (frozen Ready for Agent order)", got)
	}
	if slices.Contains(tickets.claimed, 99) || slices.Contains(tickets.doneList, 99) {
		t.Errorf("Run must ignore mid-Run Ready for Agent changes; claimed=%v done=%v", tickets.claimed, tickets.doneList)
	}
}

func TestRun_skipsFinalWhenNoSuccessfulIteration(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{{Number: 7, Title: "seven"}}}
	prs := &fakePRs{}
	ag := &fakeAgent{}
	var out strings.Builder

	r := run.Orchestrator{
		Tickets: tickets,
		Agent:   ag,
		PRs:     prs,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", MaxIterations: 0},
		Stdout:  &out,
	}
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(ag.reqs) != 0 {
		t.Fatalf("agent called %d times, want 0 (no Iteration, so no Final)", len(ag.reqs))
	}
	if len(tickets.doneList) != 0 {
		t.Errorf("done = %v, want none", tickets.doneList)
	}
	open, _ := prs.HasOpenPR(context.Background(), "ship/run")
	if open {
		t.Error("no PR should be recorded when Final never runs")
	}
}

func TestRun_finalPhaseRunsAfterQueueDrains(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{{Number: 7, Title: "seven"}}}
	prs := &fakePRs{}
	ag := committingAgent(t, prs, "ship/run")
	var out strings.Builder

	r := run.Orchestrator{
		Tickets: tickets,
		Agent:   ag,
		PRs:     prs,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", MaxIterations: 10},
		Stdout:  &out,
	}
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Implement, Review, then Final.
	if len(ag.reqs) != 3 {
		t.Fatalf("agent called %d times, want 3 (Implement, Review, Final)", len(ag.reqs))
	}
	finalPrompt := ag.reqs[2].Prompt
	if !strings.Contains(finalPrompt, "ship/run") {
		t.Errorf("Final prompt missing branch:\n%s", finalPrompt)
	}
	if !strings.Contains(finalPrompt, "#7") || !strings.Contains(finalPrompt, "seven") {
		t.Errorf("Final prompt missing Done Ticket:\n%s", finalPrompt)
	}
	if strings.Contains(strings.ToLower(finalPrompt), "partial progress") {
		t.Errorf("drain Final must not disclose Partial Progress:\n%s", finalPrompt)
	}
	if !strings.Contains(out.String(), "Queue drained") {
		t.Errorf("stdout = %q, want drain summary", out.String())
	}
}

func TestRun_finalWithoutOpenedPRFails(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{{Number: 7, Title: "seven"}}}
	// Agent succeeds every Phase but never opens a PR.
	ag := committingAgent(t, nil, "")
	prs := &fakePRs{}

	r := run.Orchestrator{
		Tickets: tickets,
		Agent:   ag,
		PRs:     prs,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", MaxIterations: 10},
		Stdout:  &strings.Builder{},
	}
	err := r.Run(context.Background())
	if err == nil {
		t.Fatal("Run: want error when Final opens no pull request")
	}
	if !strings.Contains(err.Error(), "Abort") {
		t.Errorf("error = %q, want Abort summary", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "pull request") {
		t.Errorf("error = %q, want mention of missing pull request", err)
	}
}

func TestRun_finalPhaseFailureAbortsRun(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{{Number: 7, Title: "seven"}}}
	prs := &fakePRs{}
	ag := &fakeAgent{handler: func(idx int, req agent.PhaseRequest) error {
		if isFinalPrompt(req.Prompt) {
			return errors.New("final agent boom")
		}
		if idx == 1 {
			writeAndCommit(t, req.Workspace, "impl.txt", "work\n", "implement")
		}
		return nil
	}}

	r := run.Orchestrator{
		Tickets: tickets,
		Agent:   ag,
		PRs:     prs,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", MaxIterations: 10},
		Stdout:  &strings.Builder{},
	}
	err := r.Run(context.Background())
	if err == nil {
		t.Fatal("Run: want error when Final Phase fails")
	}
	if !strings.Contains(err.Error(), "Abort") {
		t.Errorf("error = %q, want Abort summary", err)
	}
	if !strings.Contains(err.Error(), "final agent boom") {
		t.Errorf("error = %q, want underlying Final failure", err)
	}
	if !strings.Contains(err.Error(), "Final") {
		t.Errorf("error = %q, want Final Phase context", err)
	}
	// Ticket was already Done before Final; Abort does not reopen Done Tickets.
	if got := tickets.doneList; !equalInts(got, []int{7}) {
		t.Errorf("done = %v, want [7]", got)
	}
	if len(tickets.aborted) != 0 {
		t.Errorf("aborted = %v, want none (Final has no In Progress Ticket)", tickets.aborted)
	}
	open, _ := prs.HasOpenPR(context.Background(), "ship/run")
	if open {
		t.Error("no PR should be recorded when Final Agent fails")
	}
}

func TestRun_stopsAtMaxIterationsWithPartialProgress(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{{Number: 7}, {Number: 8}, {Number: 9}}}
	prs := &fakePRs{}
	ag := committingAgent(t, prs, "ship/run")
	var out strings.Builder

	r := run.Orchestrator{
		Tickets: tickets,
		Agent:   ag,
		PRs:     prs,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", MaxIterations: 2},
		Stdout:  &out,
	}
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := tickets.doneList; !equalInts(got, []int{7, 8}) {
		t.Errorf("done = %v, want [7 8] (stopped at max)", got)
	}
	if got := tickets.claimed; !equalInts(got, []int{7, 8}) {
		t.Errorf("claimed = %v, want [7 8]; Ticket 9 must stay claimable", got)
	}
	if !strings.Contains(out.String(), "max iterations") {
		t.Errorf("stdout = %q, want partial-progress message", out.String())
	}
	// Implement+Review for two Tickets, then Final with Partial Progress.
	if len(ag.reqs) != 5 {
		t.Fatalf("agent called %d times, want 5 (4 Iteration + Final)", len(ag.reqs))
	}
	finalPrompt := ag.reqs[4].Prompt
	if !strings.Contains(strings.ToLower(finalPrompt), "partial progress") {
		t.Errorf("Final prompt must disclose Partial Progress:\n%s", finalPrompt)
	}
	if !strings.Contains(finalPrompt, "2") {
		t.Errorf("Final prompt must include max iterations (2):\n%s", finalPrompt)
	}
}

func TestRun_implementWithoutCommitFailsAndDoesNotMarkDone(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{{Number: 7}}}
	// Agent that never commits: Implement produces no side effect.
	ag := &fakeAgent{handler: func(int, agent.PhaseRequest) error { return nil }}
	var out strings.Builder

	r := run.Orchestrator{
		Tickets: tickets,
		Agent:   ag,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", MaxIterations: 10},
		Stdout:  &out,
	}
	err := r.Run(context.Background())
	if err == nil {
		t.Fatal("Run: want error when Implement commits nothing")
	}
	if !strings.Contains(err.Error(), "no commit") {
		t.Errorf("error = %q, want mention of missing commit", err)
	}
	if got := tickets.claimed; !equalInts(got, []int{7}) {
		t.Errorf("claimed = %v, want [7]", got)
	}
	if len(tickets.doneList) != 0 {
		t.Errorf("done = %v, want none (Iteration failed)", tickets.doneList)
	}
	// Review Phase must not run after a failed Implement.
	if len(ag.reqs) != 1 {
		t.Errorf("agent called %d times, want 1 (Implement only)", len(ag.reqs))
	}
}

func TestRun_implementWithoutCommitAbortsTicket(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{{Number: 7}, {Number: 8}}}
	ag := &fakeAgent{handler: func(int, agent.PhaseRequest) error { return nil }}

	r := run.Orchestrator{
		Tickets: tickets,
		Agent:   ag,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", MaxIterations: 10},
		Stdout:  &strings.Builder{},
	}
	err := r.Run(context.Background())
	if err == nil {
		t.Fatal("Run: want error when Implement commits nothing")
	}
	if got := tickets.aborted; !equalInts(got, []int{7}) {
		t.Errorf("aborted = %v, want [7] (side-effect miss Aborts)", got)
	}
	if len(tickets.doneList) != 0 {
		t.Errorf("done = %v, want none", tickets.doneList)
	}
	if got := tickets.claimed; !equalInts(got, []int{7}) {
		t.Errorf("claimed = %v, want [7] only (no further Tickets)", got)
	}
}

func TestRun_phaseTimeoutAbortsTicket(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{{Number: 7}, {Number: 8}}}
	timeout := 50 * time.Millisecond
	ag := &fakeAgent{handler: func(_ int, req agent.PhaseRequest) error {
		if req.Timeout != timeout {
			return fmt.Errorf("PhaseRequest.Timeout = %v, want %v", req.Timeout, timeout)
		}
		return context.DeadlineExceeded
	}}

	r := run.Orchestrator{
		Tickets: tickets,
		Agent:   ag,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", MaxIterations: 10, Timeout: timeout},
		Stdout:  &strings.Builder{},
	}
	err := r.Run(context.Background())
	if err == nil {
		t.Fatal("Run: want error when Phase times out")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "deadline") {
		t.Errorf("error = %q, want timeout/deadline", err)
	}
	if got := tickets.aborted; !equalInts(got, []int{7}) {
		t.Errorf("aborted = %v, want [7] (timeout Aborts like other Phase failures)", got)
	}
	if len(tickets.doneList) != 0 {
		t.Errorf("done = %v, want none", tickets.doneList)
	}
	if got := tickets.claimed; !equalInts(got, []int{7}) {
		t.Errorf("claimed = %v, want [7] only", got)
	}
}

func TestRun_abortErrorSummarizesWhy(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{{Number: 7, Title: "seven"}}}
	ag := &fakeAgent{handler: func(int, agent.PhaseRequest) error {
		return errors.New("agent boom")
	}}

	r := run.Orchestrator{
		Tickets: tickets,
		Agent:   ag,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", MaxIterations: 10},
		Stdout:  &strings.Builder{},
	}
	err := r.Run(context.Background())
	if err == nil {
		t.Fatal("Run: want Abort error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "Abort") {
		t.Errorf("error = %q, want Abort summary", msg)
	}
	if !strings.Contains(msg, "agent boom") {
		t.Errorf("error = %q, want underlying reason", msg)
	}
	if !strings.Contains(msg, "#7") && !strings.Contains(msg, "Ticket") {
		t.Errorf("error = %q, want Ticket context", msg)
	}
}

func TestRun_reviewMayBeCommitless(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{{Number: 7}}}
	prs := &fakePRs{}
	// Commit only on the Implement Phase (call 1); Review (call 2) is commitless.
	ag := &fakeAgent{handler: func(idx int, req agent.PhaseRequest) error {
		if isFinalPrompt(req.Prompt) {
			prs.openPR("ship/run")
			return nil
		}
		if idx == 1 {
			writeAndCommit(t, req.Workspace, "impl.txt", "work\n", "implement")
		}
		return nil
	}}

	r := run.Orchestrator{
		Tickets: tickets,
		Agent:   ag,
		PRs:     prs,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", MaxIterations: 10},
		Stdout:  &strings.Builder{},
	}
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := tickets.doneList; !equalInts(got, []int{7}) {
		t.Errorf("done = %v, want [7]; commitless Review must still finish", got)
	}
}

func TestRun_phaseFailureStopsRun(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{{Number: 7}, {Number: 8}}}
	ag := &fakeAgent{handler: func(int, agent.PhaseRequest) error {
		return errors.New("agent boom")
	}}

	r := run.Orchestrator{
		Tickets: tickets,
		Agent:   ag,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", MaxIterations: 10},
		Stdout:  &strings.Builder{},
	}
	err := r.Run(context.Background())
	if err == nil {
		t.Fatal("Run: want error when a Phase fails")
	}
	if !strings.Contains(err.Error(), "agent boom") {
		t.Errorf("error = %q, want underlying agent failure", err)
	}
	if len(tickets.doneList) != 0 {
		t.Errorf("done = %v, want none", tickets.doneList)
	}
	// Only the first Ticket's Implement Phase ran; the Run stopped.
	if len(ag.reqs) != 1 {
		t.Errorf("agent called %d times, want 1", len(ag.reqs))
	}
}

func TestRun_phaseFailureAbortsTicketToReadyForAgent(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{{Number: 7, Title: "seven"}, {Number: 8, Title: "eight"}}}
	ag := &fakeAgent{handler: func(int, agent.PhaseRequest) error {
		return errors.New("agent boom")
	}}

	r := run.Orchestrator{
		Tickets: tickets,
		Agent:   ag,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", MaxIterations: 10},
		Stdout:  &strings.Builder{},
	}
	err := r.Run(context.Background())
	if err == nil {
		t.Fatal("Run: want error when a Phase fails")
	}
	if got := tickets.aborted; !equalInts(got, []int{7}) {
		t.Errorf("aborted = %v, want [7] (restored to Ready for Agent)", got)
	}
	if len(tickets.doneList) != 0 {
		t.Errorf("done = %v, want none", tickets.doneList)
	}
	// Abort must stop the Run: no Review, no Ticket 8, no Final.
	if len(ag.reqs) != 1 {
		t.Errorf("agent called %d times, want 1 (Implement only)", len(ag.reqs))
	}
	if got := tickets.claimed; !equalInts(got, []int{7}) {
		t.Errorf("claimed = %v, want [7] only", got)
	}
}

func TestRun_reviewFailureAbortsAndUndoesTicketCommits(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{{Number: 7, Title: "seven"}, {Number: 8, Title: "eight"}}}
	ag := &fakeAgent{handler: func(idx int, req agent.PhaseRequest) error {
		if idx == 1 {
			writeAndCommit(t, req.Workspace, "impl.txt", "work\n", "implement")
			return nil
		}
		return errors.New("review boom")
	}}

	r := run.Orchestrator{
		Tickets: tickets,
		Agent:   ag,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", MaxIterations: 10},
		Stdout:  &strings.Builder{},
	}

	before := headSHA(t, dir)
	err := r.Run(context.Background())
	if err == nil {
		t.Fatal("Run: want error when Review Phase fails")
	}
	if !strings.Contains(err.Error(), "review boom") {
		t.Errorf("error = %q, want underlying Review failure", err)
	}
	if got := tickets.aborted; !equalInts(got, []int{7}) {
		t.Errorf("aborted = %v, want [7]", got)
	}
	if len(tickets.doneList) != 0 {
		t.Errorf("done = %v, want none", tickets.doneList)
	}
	if head := headSHA(t, dir); head != before {
		t.Errorf("HEAD after Abort = %q, want restore point %q (Ticket commits undone)", head, before)
	}
	// No further Tickets or Final after Abort.
	if len(ag.reqs) != 2 {
		t.Errorf("agent called %d times, want 2 (Implement + Review)", len(ag.reqs))
	}
	if got := tickets.claimed; !equalInts(got, []int{7}) {
		t.Errorf("claimed = %v, want [7] only", got)
	}
}

// recordingThrobber records Phase labels and that work ran inside During.
type recordingThrobber struct {
	phases    []string
	workCalls int
}

func (r *recordingThrobber) During(ctx context.Context, status throbber.Status, work func(context.Context) error) error {
	r.phases = append(r.phases, status.Phase)
	r.workCalls++
	return work(ctx)
}

// fakeTickets is an in-memory Ticket port. Done removes a Ticket from the
// Ready for Agent queue so the in-memory set can change mid-Run like a tracker.
type fakeTickets struct {
	ready      []ticket.Ticket
	claimed    []int
	doneList   []int
	aborted    []int
	ensured    bool
	ensureErr  error
	afterClaim func(*fakeTickets, ticket.Ticket)
}

func (f *fakeTickets) EnsureLabels(context.Context) error {
	f.ensured = true
	return f.ensureErr
}

func (f *fakeTickets) ListReady(context.Context, string) ([]ticket.Ticket, error) {
	out := make([]ticket.Ticket, len(f.ready))
	copy(out, f.ready)
	return out, nil
}

func (f *fakeTickets) Claim(_ context.Context, t ticket.Ticket) error {
	f.claimed = append(f.claimed, t.Number)
	if f.afterClaim != nil {
		f.afterClaim(f, t)
	}
	return nil
}

func (f *fakeTickets) Done(_ context.Context, t ticket.Ticket) error {
	f.doneList = append(f.doneList, t.Number)
	kept := f.ready[:0]
	for _, r := range f.ready {
		if r.Number != t.Number {
			kept = append(kept, r)
		}
	}
	f.ready = kept
	return nil
}

func (f *fakeTickets) Abort(_ context.Context, t ticket.Ticket) error {
	f.aborted = append(f.aborted, t.Number)
	return nil
}

// fakePRs reports which branches have an open pull request. Final success
// requires HasOpenPR to return true for the Run branch after the Agent exits.
type fakePRs struct {
	open map[string]bool
}

func (f *fakePRs) HasOpenPR(_ context.Context, branch string) (bool, error) {
	if f.open == nil {
		return false, nil
	}
	return f.open[branch], nil
}

func (f *fakePRs) openPR(branch string) {
	if f.open == nil {
		f.open = map[string]bool{}
	}
	f.open[branch] = true
}

// fakeAgent records Phase requests and defers behavior to handler. The 1-based
// call index alternates Implement (odd) then Review (even) within a Run.
type fakeAgent struct {
	reqs    []agent.PhaseRequest
	handler func(callIdx int, req agent.PhaseRequest) error
}

func (f *fakeAgent) RunPhase(_ context.Context, req agent.PhaseRequest) error {
	f.reqs = append(f.reqs, req)
	if f.handler != nil {
		return f.handler(len(f.reqs), req)
	}
	return nil
}

// committingAgent commits on each Implement Phase so the side-effect check
// passes; Review Phases are commitless. When prs is non-nil, Final opens a
// pull request for branch (the side effect Ship verifies after Final).
func committingAgent(t *testing.T, prs *fakePRs, branch string) *fakeAgent {
	t.Helper()
	return &fakeAgent{handler: func(idx int, req agent.PhaseRequest) error {
		if isFinalPrompt(req.Prompt) {
			if prs != nil {
				prs.openPR(branch)
			}
			return nil
		}
		if idx%2 == 1 {
			writeAndCommit(t, req.Workspace, fmt.Sprintf("impl-%d.txt", idx), "work\n", "implement work")
		}
		return nil
	}}
}

func isFinalPrompt(prompt string) bool {
	return strings.Contains(prompt, "Exiting without an opened PR is a failure")
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func initTempRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "ship@example.com")
	runGit(t, dir, "config", "user.name", "Ship Test")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("init\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "README")
	runGit(t, dir, "commit", "-m", "init")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func currentBranch(t *testing.T, dir string) string {
	t.Helper()
	return runGit(t, dir, "branch", "--show-current")
}

func headSHA(t *testing.T, dir string) string {
	t.Helper()
	return runGit(t, dir, "rev-parse", "HEAD")
}

func writeAndCommit(t *testing.T, dir, relPath, contents, message string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, relPath), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", relPath)
	runGit(t, dir, "commit", "-m", message)
}
