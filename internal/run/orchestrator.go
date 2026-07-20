package run

import (
	"context"
	"fmt"
	"io"

	"github.com/maxBRT/ship-cli/internal/agent"
	"github.com/maxBRT/ship-cli/internal/gitops"
	"github.com/maxBRT/ship-cli/internal/prompt"
	"github.com/maxBRT/ship-cli/internal/throbber"
	"github.com/maxBRT/ship-cli/internal/ticket"
)

// Orchestrator wires the Ship Run loop over its ports: it Claims Ready for
// Agent Tickets, drives an Implement then a Review Phase per Iteration, marks
// each Ticket Done, then runs Final and verifies an open pull request. Phase
// failure Aborts (restore Ticket, undo commits, stop). Ports stay behind
// interfaces so tests can fake them.
type Orchestrator struct {
	Tickets  ticket.Port
	Agent    agent.Port
	PRs      PullRequests
	Repo     gitops.Repo
	Config   Config
	Throbber throbber.Port // optional; nil means no wait UI
	Stdout   io.Writer
}

// PullRequests is the side-effect seam Final success is checked against: an
// open pull request for the Run branch after the Final Agent exits 0.
type PullRequests interface {
	HasOpenPR(ctx context.Context, branch string) (bool, error)
}

// Run executes one Ship Run in the current checkout.
//
// It first ensures the Ready for Agent and In Progress tracker labels exist.
// With no Ready for Agent Tickets it reports that and returns without touching
// the branch. Otherwise it prepares the Run branch, then processes Tickets one
// Iteration each (Implement Phase then Review Phase) up to the max-iterations
// limit, stopping when the queue drains or the limit is hit, then runs Final.
// A failed Phase, missing side effect, or timeout Aborts the Run.
func (r Orchestrator) Run(ctx context.Context) error {
	if err := r.Tickets.EnsureLabels(ctx); err != nil {
		return fmt.Errorf("ensure tracker labels: %w", err)
	}

	ready, err := r.Tickets.ListReady(ctx, r.Config.Feature)
	if err != nil {
		return fmt.Errorf("list Ready for Agent Tickets: %w", err)
	}
	if len(ready) == 0 {
		fmt.Fprintln(r.stdout(), "No Ready for Agent Tickets; nothing to Run.")
		return nil
	}

	branch, err := r.Repo.EnsureBranch(r.Config.Branch)
	if err != nil {
		return fmt.Errorf("prepare Run branch: %w", err)
	}
	var done []ticket.Ticket
	iterations := 0
	for iterations < r.Config.MaxIterations && len(ready) > 0 {
		t := ready[0]
		iterations++

		if err := r.iterate(ctx, t, branch, iterations); err != nil {
			return err
		}
		done = append(done, t)

		ready, err = r.Tickets.ListReady(ctx, r.Config.Feature)
		if err != nil {
			return fmt.Errorf("list Ready for Agent Tickets: %w", err)
		}
	}

	// Final only when at least one Iteration succeeded ("when there was work").
	if len(done) == 0 {
		return nil
	}
	partial := iterations >= r.Config.MaxIterations && len(ready) > 0
	return r.final(ctx, branch, done, partial)
}

// iterate runs one Ticket through a single Iteration: Claim, the Implement
// Phase (which must commit), the Review Phase (which may be commitless), then
// Done. A failed Phase or missing side effect Aborts: restores the Ticket to
// Ready for Agent, undoes that Ticket's commits, and stops the Run.
func (r Orchestrator) iterate(ctx context.Context, t ticket.Ticket, branch string, iteration int) error {
	if err := r.Tickets.Claim(ctx, t); err != nil {
		return fmt.Errorf("claim Ticket #%d: %w", t.Number, err)
	}

	restore, err := r.Repo.RecordRestorePoint()
	if err != nil {
		return r.abort(ctx, t, gitops.RestorePoint(""), fmt.Errorf("record restore point for Ticket #%d: %w", t.Number, err))
	}

	ticketLabel := fmt.Sprintf("#%d %s", t.Number, t.Title)
	implInput := prompt.ImplementInput{Ticket: ticketInput(t), Branch: branch}
	if err := r.runPhase(ctx, throbber.Status{Phase: "Implement", Iteration: iteration, Ticket: ticketLabel}, prompt.Implement(implInput)); err != nil {
		return r.abort(ctx, t, restore, fmt.Errorf("Implement Phase for Ticket #%d: %w", t.Number, err))
	}

	committed, err := r.Repo.HasCommitsSince(restore)
	if err != nil {
		return r.abort(ctx, t, restore, fmt.Errorf("check Implement commits for Ticket #%d: %w", t.Number, err))
	}
	if !committed {
		return r.abort(ctx, t, restore, fmt.Errorf("Implement Phase for Ticket #%d produced no commit(s)", t.Number))
	}

	reviewInput := prompt.ReviewInput{Ticket: ticketInput(t), Branch: branch}
	if err := r.runPhase(ctx, throbber.Status{Phase: "Review", Iteration: iteration, Ticket: ticketLabel}, prompt.Review(reviewInput)); err != nil {
		return r.abort(ctx, t, restore, fmt.Errorf("Review Phase for Ticket #%d: %w", t.Number, err))
	}

	if err := r.Tickets.Done(ctx, t); err != nil {
		return fmt.Errorf("mark Ticket #%d Done: %w", t.Number, err)
	}
	fmt.Fprintf(r.stdout(), "Ticket #%d Done\n", t.Number)
	return nil
}

// abort restores the Ticket to Ready for Agent, undoes that Ticket's commits
// on the Run branch, and returns a clear Abort summary wrapping cause.
func (r Orchestrator) abort(ctx context.Context, t ticket.Ticket, restore gitops.RestorePoint, cause error) error {
	if err := r.Tickets.Abort(ctx, t); err != nil {
		return fmt.Errorf("Abort: restore Ticket #%d failed after (%v): %w", t.Number, cause, err)
	}
	if restore != "" {
		if err := r.Repo.UndoToRestorePoint(restore); err != nil {
			return fmt.Errorf("Abort: undo commits for Ticket #%d failed after (%v): %w", t.Number, cause, err)
		}
	}
	return fmt.Errorf("Abort: %w", cause)
}

// final runs the Final Phase: a fresh Agent invocation with the built-in Final
// prompt (branch review / green bar / open PR). Partial Progress is disclosed
// in the prompt when the Run stopped at max iterations with work remaining.
func (r Orchestrator) final(ctx context.Context, branch string, done []ticket.Ticket, partial bool) error {
	if partial {
		fmt.Fprintf(r.stdout(), "Stopped at max iterations (%d) with Ready for Agent Tickets remaining; %d Ticket(s) Done.\n", r.Config.MaxIterations, len(done))
	} else {
		fmt.Fprintf(r.stdout(), "Queue drained; %d Ticket(s) Done.\n", len(done))
	}

	tickets := make([]prompt.TicketInput, len(done))
	for i, t := range done {
		tickets[i] = ticketInput(t)
	}
	finalPrompt := prompt.Final(prompt.FinalInput{
		Branch:          branch,
		Tickets:         tickets,
		PartialProgress: partial,
		MaxIterations:   r.Config.MaxIterations,
	})
	if err := r.runPhase(ctx, throbber.Status{Phase: "Final"}, finalPrompt); err != nil {
		return fmt.Errorf("Abort: Final Phase: %w", err)
	}
	open, err := r.PRs.HasOpenPR(ctx, branch)
	if err != nil {
		return fmt.Errorf("Abort: check Final pull request for %s: %w", branch, err)
	}
	if !open {
		return fmt.Errorf("Abort: Final Phase produced no open pull request for branch %s", branch)
	}
	return nil
}

func (r Orchestrator) runPhase(ctx context.Context, status throbber.Status, promptText string) error {
	work := func(ctx context.Context) error {
		return r.Agent.RunPhase(ctx, agent.PhaseRequest{
			Prompt:    promptText,
			Workspace: r.Repo.Dir,
			Model:     r.Config.Model,
			Timeout:   r.Config.Timeout,
		})
	}
	if r.Throbber == nil {
		return work(ctx)
	}
	return r.Throbber.During(ctx, status, work)
}

func (r Orchestrator) stdout() io.Writer {
	if r.Stdout != nil {
		return r.Stdout
	}
	return io.Discard
}

func ticketInput(t ticket.Ticket) prompt.TicketInput {
	return prompt.TicketInput{Number: t.Number, Title: t.Title}
}
