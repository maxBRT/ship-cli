package run_test

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	if len(tickets.stamped) != 0 {
		t.Errorf("stamped = %v, want none on empty queue", tickets.stamped)
	}
	if len(ag.reqs) != 0 {
		t.Errorf("agent called %d times, want 0", len(ag.reqs))
	}
}

func TestRun_emptyPickerSelectionDoesNotStampOrStartPhases(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{{Number: 7, Title: "seven"}, {Number: 8, Title: "eight"}}}
	ag := &fakeAgent{}
	queue := &fakeQueue{selectFn: func([]ticket.Ticket) ([]ticket.Ticket, error) {
		return nil, nil // confirmed empty selection
	}}
	var out strings.Builder

	r := run.Orchestrator{
		Tickets: tickets,
		Queue:   queue,
		Agent:   ag,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", MaxIterations: 10},
		Stdout:  &out,
	}
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(out.String(), "No Tickets selected") {
		t.Errorf("stdout = %q, want empty-selection message", out.String())
	}
	if len(tickets.stamped) != 0 {
		t.Errorf("stamped = %v, want none when picker confirms empty", tickets.stamped)
	}
	if len(ag.reqs) != 0 {
		t.Errorf("agent called %d times, want 0", len(ag.reqs))
	}
	if b := currentBranch(t, dir); b != "main" {
		t.Errorf("branch = %q, want main (no branch prepared)", b)
	}
}

func TestRun_canceledPickerSelectionDoesNotStampOrStartPhases(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{{Number: 7, Title: "seven"}}}
	ag := &fakeAgent{}
	queue := &fakeQueue{selectFn: func([]ticket.Ticket) ([]ticket.Ticket, error) {
		return nil, errors.New("picker canceled")
	}}

	r := run.Orchestrator{
		Tickets: tickets,
		Queue:   queue,
		Agent:   ag,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", MaxIterations: 10},
		Stdout:  &strings.Builder{},
	}
	err := r.Run(context.Background())
	if err == nil {
		t.Fatal("Run error = nil, want picker cancel failure")
	}
	if !strings.Contains(err.Error(), "confirm ship queue") {
		t.Errorf("Run error = %v, want confirm ship queue wrap", err)
	}
	if !strings.Contains(err.Error(), "picker canceled") {
		t.Errorf("Run error = %v, want underlying cancel", err)
	}
	if len(tickets.stamped) != 0 {
		t.Errorf("stamped = %v, want none when picker is canceled", tickets.stamped)
	}
	if len(ag.reqs) != 0 {
		t.Errorf("agent called %d times, want 0", len(ag.reqs))
	}
	if b := currentBranch(t, dir); b != "main" {
		t.Errorf("branch = %q, want main (no branch prepared)", b)
	}
}

func TestRun_nonInteractivePickerFailsWithoutStamping(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{{Number: 7, Title: "seven"}}}
	ag := &fakeAgent{}
	queue := run.Interactive{
		In:  strings.NewReader(""),
		Out: io.Discard,
		IsTerminal: func() bool {
			return false
		},
	}

	r := run.Orchestrator{
		Tickets: tickets,
		Queue:   queue,
		Agent:   ag,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", MaxIterations: 10},
		Stdout:  &strings.Builder{},
	}
	err := r.Run(context.Background())
	if err == nil {
		t.Fatal("Run error = nil, want non-interactive picker failure")
	}
	if !errors.Is(err, run.ErrNonInteractive) {
		t.Errorf("Run error = %v, want ErrNonInteractive", err)
	}
	if len(tickets.stamped) != 0 {
		t.Errorf("stamped = %v, want none on non-interactive misuse", tickets.stamped)
	}
	if len(ag.reqs) != 0 {
		t.Errorf("agent called %d times, want 0", len(ag.reqs))
	}
}

func TestRun_ensureLabelsFailureStopsBeforeStamp(t *testing.T) {
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
	if len(tickets.stamped) != 0 {
		t.Errorf("stamped = %v, want none when EnsureLabels fails", tickets.stamped)
	}
	if len(ag.reqs) != 0 {
		t.Errorf("agent called %d times, want 0", len(ag.reqs))
	}
	if b := currentBranch(t, dir); b != "main" {
		t.Errorf("branch = %q, want main (no branch prepared)", b)
	}
}

func TestRun_stampsShipQueueMembershipBeforeIterations(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{{Number: 7, Title: "seven"}, {Number: 8, Title: "eight"}}}
	prs := &fakePRs{}
	ag := committingAgent(t, prs, "ship/run")
	queue := &fakeQueue{} // confirms all candidates in order
	var out strings.Builder

	r := run.Orchestrator{
		Tickets: tickets,
		Queue:   queue,
		Agent:   ag,
		PRs:     prs,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", MaxIterations: 10, Model: "composer"},
		Stdout:  &out,
	}
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !equalInts(queue.confirmed, []int{7, 8}) {
		t.Errorf("queue confirmed = %v, want [7 8] (all candidates)", queue.confirmed)
	}
	if got := tickets.stamped; !equalInts(got, []int{7, 8}) {
		t.Errorf("stamped = %v, want [7 8] (ship queue membership)", got)
	}
	if got := tickets.doneList; !equalInts(got, []int{7, 8}) {
		t.Errorf("done = %v, want [7 8]", got)
	}
	if got := tickets.shipCleared; !equalInts(got, []int{7, 8}) {
		t.Errorf("ship cleared = %v, want [7 8] (Done removes ship)", got)
	}
	if b := currentBranch(t, dir); b != "ship/run" {
		t.Errorf("branch = %q, want ship/run", b)
	}
	// Two Phases (Implement, Review) per Ticket, then Final.
	if len(ag.reqs) != 5 {
		t.Fatalf("agent called %d times, want 5", len(ag.reqs))
	}
	for i, req := range ag.reqs {
		if req.Workspace != dir {
			t.Errorf("req %d workspace = %q, want %q", i, req.Workspace, dir)
		}
		if req.Model != "composer" {
			t.Errorf("req %d model = %q, want composer", i, req.Model)
		}
	}
	if !strings.Contains(ag.reqs[0].Prompt, "#7") || !strings.Contains(ag.reqs[0].Prompt, "ship/run") {
		t.Errorf("first prompt missing Ticket/branch context:\n%s", ag.reqs[0].Prompt)
	}
	if !strings.Contains(out.String(), "Queue drained") {
		t.Errorf("stdout = %q, want drain summary", out.String())
	}
}

func TestRun_pickerConfirmedOrderIsRunOrder(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{
		{Number: 7, Title: "seven"},
		{Number: 8, Title: "eight"},
		{Number: 9, Title: "nine"},
	}}
	prs := &fakePRs{}
	ag := committingAgent(t, prs, "ship/run")
	// Drop #7, reorder so #9 runs before #8.
	queue := &fakeQueue{selectFn: func(candidates []ticket.Ticket) ([]ticket.Ticket, error) {
		return []ticket.Ticket{candidates[2], candidates[1]}, nil
	}}
	var out strings.Builder

	r := run.Orchestrator{
		Tickets: tickets,
		Queue:   queue,
		Agent:   ag,
		PRs:     prs,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", MaxIterations: 10},
		Stdout:  &out,
	}
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !equalInts(queue.confirmed, []int{9, 8}) {
		t.Errorf("queue confirmed = %v, want [9 8]", queue.confirmed)
	}
	if got := tickets.stamped; !equalInts(got, []int{9, 8}) {
		t.Errorf("stamped = %v, want [9 8] (picker order)", got)
	}
	if got := tickets.doneList; !equalInts(got, []int{9, 8}) {
		t.Errorf("done = %v, want [9 8] (picker order)", got)
	}
	if !strings.Contains(out.String(), "Iteration 1: Ticket #9") {
		t.Errorf("stdout = %q, want first Iteration on #9", out.String())
	}
	if !strings.Contains(out.String(), "Iteration 2: Ticket #8") {
		t.Errorf("stdout = %q, want second Iteration on #8", out.String())
	}
	if strings.Contains(out.String(), "Ticket #7") {
		t.Errorf("stdout = %q, dropped Ticket #7 must not run", out.String())
	}
}

func TestRun_processesFrozenShipQueueDespiteMidRunReadyChanges(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{
		ready: []ticket.Ticket{{Number: 7, Title: "seven"}, {Number: 8, Title: "eight"}},
		afterStamp: func(f *fakeTickets) {
			// Mid-Run tracker rewrite after stamp: #99 jumps ahead and #8
			// disappears from Ready. The frozen ship queue must still walk #7
			// then #8.
			f.ready = []ticket.Ticket{{Number: 99, Title: "intruder"}}
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

	if got := tickets.stamped; !equalInts(got, []int{7, 8}) {
		t.Errorf("stamped = %v, want [7 8] (frozen ship queue)", got)
	}
	if got := tickets.doneList; !equalInts(got, []int{7, 8}) {
		t.Errorf("done = %v, want [7 8] (frozen ship queue)", got)
	}
	if slices.Contains(tickets.stamped, 99) || slices.Contains(tickets.doneList, 99) {
		t.Errorf("Run must ignore mid-Run Ready for Agent changes; stamped=%v done=%v", tickets.stamped, tickets.doneList)
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
	if got := tickets.stamped; !equalInts(got, []int{7, 8, 9}) {
		t.Errorf("stamped = %v, want [7 8 9]; Ticket 9 keeps ship under Partial Progress", got)
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
	if got := tickets.stamped; !equalInts(got, []int{7}) {
		t.Errorf("stamped = %v, want [7]", got)
	}
	if len(tickets.doneList) != 0 {
		t.Errorf("done = %v, want none (Iteration failed)", tickets.doneList)
	}
	// Review Phase must not run after a failed Implement.
	if len(ag.reqs) != 1 {
		t.Errorf("agent called %d times, want 1 (Implement only)", len(ag.reqs))
	}
}

func TestRun_implementWithoutCommitAbortsWithoutLabelRestore(t *testing.T) {
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
	if got := tickets.stamped; !equalInts(got, []int{7, 8}) {
		t.Errorf("stamped = %v, want [7 8] (unfinished Tickets keep ship)", got)
	}
	if len(tickets.doneList) != 0 {
		t.Errorf("done = %v, want none", tickets.doneList)
	}
	if len(tickets.shipCleared) != 0 {
		t.Errorf("ship cleared = %v, want none (Abort does not clear ship)", tickets.shipCleared)
	}
	if len(ag.reqs) != 1 {
		t.Errorf("agent called %d times, want 1 (stopped after first Implement)", len(ag.reqs))
	}
}

func TestRun_phaseTimeoutAbortsWithoutLabelRestore(t *testing.T) {
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
	if got := tickets.stamped; !equalInts(got, []int{7, 8}) {
		t.Errorf("stamped = %v, want [7 8] (unfinished Tickets keep ship)", got)
	}
	if len(tickets.shipCleared) != 0 {
		t.Errorf("ship cleared = %v, want none (Abort does not clear ship)", tickets.shipCleared)
	}
	if len(tickets.doneList) != 0 {
		t.Errorf("done = %v, want none", tickets.doneList)
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

func TestRun_phaseFailureAbortsWithoutLabelRestore(t *testing.T) {
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
	if got := tickets.stamped; !equalInts(got, []int{7, 8}) {
		t.Errorf("stamped = %v, want [7 8] (unfinished Tickets keep ship)", got)
	}
	if len(tickets.shipCleared) != 0 {
		t.Errorf("ship cleared = %v, want none (Abort does not clear ship)", tickets.shipCleared)
	}
	if len(tickets.doneList) != 0 {
		t.Errorf("done = %v, want none", tickets.doneList)
	}
	// Abort must stop the Run: no Review, no Ticket 8 Iteration, no Final.
	if len(ag.reqs) != 1 {
		t.Errorf("agent called %d times, want 1 (Implement only)", len(ag.reqs))
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
	if got := tickets.stamped; !equalInts(got, []int{7, 8}) {
		t.Errorf("stamped = %v, want [7 8] (unfinished Tickets keep ship)", got)
	}
	if len(tickets.shipCleared) != 0 {
		t.Errorf("ship cleared = %v, want none (Abort does not clear ship)", tickets.shipCleared)
	}
}

func TestRun_nextRunAfterAbortAlwaysOpensPicker(t *testing.T) {
	dir := initTempRepo(t)
	tickets := &fakeTickets{ready: []ticket.Ticket{
		{Number: 7, Title: "seven", OnShip: true},
		{Number: 8, Title: "eight", OnShip: true},
	}}
	ag := &fakeAgent{handler: func(int, agent.PhaseRequest) error {
		return errors.New("agent boom")
	}}
	queue := &fakeQueue{}

	r := run.Orchestrator{
		Tickets: tickets,
		Queue:   queue,
		Agent:   ag,
		Repo:    gitops.Repo{Dir: dir},
		Config:  run.Config{Branch: "ship/run", MaxIterations: 10},
		Stdout:  &strings.Builder{},
	}
	if err := r.Run(context.Background()); err == nil {
		t.Fatal("first Run: want Abort error")
	}
	if queue.calls != 1 {
		t.Fatalf("picker Confirm calls after Abort = %d, want 1", queue.calls)
	}
	if len(queue.saw[0]) != 2 || !queue.saw[0][0].OnShip || !queue.saw[0][1].OnShip {
		t.Errorf("first picker candidates = %+v, want leftover ship hint on both", queue.saw[0])
	}

	// Leftover ship Tickets remain Ready; the next Run must open the picker
	// again (no auto-resume), still showing leftover ship membership.
	prs := &fakePRs{}
	ag2 := committingAgent(t, prs, "ship/run")
	r.Agent = ag2
	r.PRs = prs
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if queue.calls != 2 {
		t.Errorf("picker Confirm calls across Runs = %d, want 2 (always re-open)", queue.calls)
	}
	if len(queue.saw[1]) != 2 || !queue.saw[1][0].OnShip || !queue.saw[1][1].OnShip {
		t.Errorf("second picker candidates = %+v, want leftover ship hint without skipping picker", queue.saw[1])
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

// fakeQueue is the injectable picker/queue port for Orchestrator tests.
// Without selectFn it confirms every candidate in order. With selectFn it
// returns that selection (drop / reorder / empty / cancel) instead.
type fakeQueue struct {
	confirmed []int
	calls     int
	saw       [][]ticket.Ticket
	selectFn  func(candidates []ticket.Ticket) ([]ticket.Ticket, error)
}

func (f *fakeQueue) Confirm(_ context.Context, candidates []ticket.Ticket) ([]ticket.Ticket, error) {
	f.calls++
	cp := make([]ticket.Ticket, len(candidates))
	copy(cp, candidates)
	f.saw = append(f.saw, cp)

	var out []ticket.Ticket
	var err error
	if f.selectFn != nil {
		out, err = f.selectFn(candidates)
	} else {
		out = make([]ticket.Ticket, len(candidates))
		copy(out, candidates)
	}
	if err != nil {
		return nil, err
	}
	for _, t := range out {
		f.confirmed = append(f.confirmed, t.Number)
	}
	return out, nil
}

// fakeTickets is an in-memory Ticket port. Done removes a Ticket from the
// Ready for Agent queue so the in-memory set can change mid-Run like a tracker.
type fakeTickets struct {
	ready       []ticket.Ticket
	stamped     []int
	shipCleared []int
	doneList    []int
	ensured     bool
	ensureErr   error
	afterStamp  func(*fakeTickets)
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

func (f *fakeTickets) Stamp(_ context.Context, tickets []ticket.Ticket) error {
	for _, t := range tickets {
		f.stamped = append(f.stamped, t.Number)
	}
	if f.afterStamp != nil {
		f.afterStamp(f)
	}
	return nil
}

func (f *fakeTickets) Done(_ context.Context, t ticket.Ticket) error {
	f.shipCleared = append(f.shipCleared, t.Number)
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
