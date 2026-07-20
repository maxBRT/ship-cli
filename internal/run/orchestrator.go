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

// Orchestrator wires the Ship Run loop over its ports: it confirms a ship
// queue from Ready for Agent candidates, stamps that set, drives an Implement
// then a Review Phase per Iteration, marks each Ticket Done, then runs Final
// and verifies an open pull request. Phase failure Aborts (undo commits,
// stop) while leaving ship on unfinished Tickets. Ports stay behind interfaces
// so tests can fake them.
type Orchestrator struct {
	Tickets  ticket.Port
	Queue    Queue // optional; nil means AllCandidates
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
// It first ensures the Ready for Agent and ship tracker labels exist. With no
// Ready for Agent Tickets it reports that and returns without touching the
// branch. Otherwise it confirms a queue (default: all candidates), stamps
// ship on that ordered set, prepares the Run branch, then walks that frozen
// queue one Iteration each (Implement Phase then Review Phase) up to the
// max-iterations limit, stopping when the queue drains or the limit is hit,
// then runs Final. Mid-Run tracker changes do not rewrite which Tickets are
// processed or in what order. A failed Phase, missing side effect, or timeout
// Aborts the Run, leaving ship on unfinished Tickets.
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

	queue, err := r.queue().Confirm(ctx, ready)
	if err != nil {
		return fmt.Errorf("confirm ship queue: %w", err)
	}
	if len(queue) == 0 {
		fmt.Fprintln(r.stdout(), "No Tickets selected; nothing to Run.")
		return nil
	}

	if err := r.Tickets.Stamp(ctx, queue); err != nil {
		return fmt.Errorf("stamp ship queue: %w", err)
	}

	branch, err := r.Repo.EnsureBranch(r.Config.Branch)
	if err != nil {
		return fmt.Errorf("prepare Run branch: %w", err)
	}
	fmt.Fprintf(r.stdout(), "Run branch: %s\n", branch)

	var done []ticket.Ticket
	iterations := 0
	for iterations < r.Config.MaxIterations && iterations < len(queue) {
		t := queue[iterations]
		iterations++
		fmt.Fprintf(r.stdout(), "Iteration %d: Ticket #%d %s\n", iterations, t.Number, t.Title)

		if err := r.iterate(ctx, t, branch, iterations); err != nil {
			return err
		}
		done = append(done, t)
	}

	// Final only when at least one Iteration succeeded ("when there was work").
	if len(done) == 0 {
		return nil
	}
	partial := iterations >= r.Config.MaxIterations && iterations < len(queue)
	return r.final(ctx, branch, done, partial)
}

// iterate runs one Ticket through a single Iteration: the Implement Phase
// (which must commit), the Review Phase (which may be commitless), then Done.
// A failed Phase or missing side effect Aborts: undoes that Ticket's commits
// and stops the Run. Unfinished Tickets keep ship queue membership.
func (r Orchestrator) iterate(ctx context.Context, t ticket.Ticket, branch string, iteration int) error {
	restore, err := r.Repo.RecordRestorePoint()
	if err != nil {
		return r.abort(t, gitops.RestorePoint(""), fmt.Errorf("record restore point for Ticket #%d: %w", t.Number, err))
	}

	ticketLabel := fmt.Sprintf("#%d %s", t.Number, t.Title)
	implInput := prompt.ImplementInput{Ticket: ticketInput(t), Branch: branch}
	if err := r.runPhase(ctx, throbber.Status{Phase: "Implement", Iteration: iteration, Ticket: ticketLabel}, prompt.Implement(implInput)); err != nil {
		return r.abort(t, restore, fmt.Errorf("Implement Phase for Ticket #%d: %w", t.Number, err))
	}

	committed, err := r.Repo.HasCommitsSince(restore)
	if err != nil {
		return r.abort(t, restore, fmt.Errorf("check Implement commits for Ticket #%d: %w", t.Number, err))
	}
	if !committed {
		return r.abort(t, restore, fmt.Errorf("Implement Phase for Ticket #%d produced no commit(s)", t.Number))
	}

	reviewInput := prompt.ReviewInput{Ticket: ticketInput(t), Branch: branch}
	if err := r.runPhase(ctx, throbber.Status{Phase: "Review", Iteration: iteration, Ticket: ticketLabel}, prompt.Review(reviewInput)); err != nil {
		return r.abort(t, restore, fmt.Errorf("Review Phase for Ticket #%d: %w", t.Number, err))
	}

	if err := r.Tickets.Done(ctx, t); err != nil {
		return fmt.Errorf("mark Ticket #%d Done: %w", t.Number, err)
	}
	fmt.Fprintf(r.stdout(), "Ticket #%d Done\n", t.Number)
	return nil
}

// abort undoes that Ticket's commits on the Run branch and returns a clear
// Abort summary wrapping cause. Tracker labels are left alone so unfinished
// Tickets keep ship queue membership.
func (r Orchestrator) abort(t ticket.Ticket, restore gitops.RestorePoint, cause error) error {
	if restore != "" {
		if err := r.Repo.UndoToRestorePoint(restore); err != nil {
			return fmt.Errorf("Abort: undo commits for Ticket #%d failed after (%v): %w", t.Number, cause, err)
		}
	}
	return fmt.Errorf("Abort: %w", cause)
}

func (r Orchestrator) queue() Queue {
	if r.Queue != nil {
		return r.Queue
	}
	return AllCandidates{}
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
