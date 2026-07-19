package run

import (
	"context"
	"fmt"
	"io"

	"github.com/maxBRT/ship-cli/internal/agent"
	"github.com/maxBRT/ship-cli/internal/gitops"
	"github.com/maxBRT/ship-cli/internal/prompt"
	"github.com/maxBRT/ship-cli/internal/ticket"
)

// Orchestrator wires the Ship Run loop over its ports: it Claims Ready for
// Agent Tickets, drives an Implement then a Review Phase per Iteration, and
// marks each Ticket Done. Ports stay behind interfaces so tests can fake them.
type Orchestrator struct {
	Tickets ticket.Port
	Agent   agent.Port
	Repo    gitops.Repo
	Config  Config
	Stdout  io.Writer
}

// Run executes one Ship Run in the current checkout.
//
// With no Ready for Agent Tickets it reports that and returns without touching
// the branch. Otherwise it prepares the Run branch, then processes Tickets one
// Iteration each (Implement Phase then Review Phase) up to the max-iterations
// limit, stopping when the queue drains or the limit is hit. The Final Phase is
// wired in a later Ticket; here it is a clean stop point.
func (r Orchestrator) Run(ctx context.Context) error {
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
	fmt.Fprintf(r.stdout(), "Run branch: %s\n", branch)

	var done []ticket.Ticket
	iterations := 0
	for iterations < r.Config.MaxIterations && len(ready) > 0 {
		t := ready[0]
		iterations++
		fmt.Fprintf(r.stdout(), "Iteration %d: Ticket #%d %s\n", iterations, t.Number, t.Title)

		if err := r.iterate(ctx, t, branch); err != nil {
			return err
		}
		done = append(done, t)

		ready, err = r.Tickets.ListReady(ctx, r.Config.Feature)
		if err != nil {
			return fmt.Errorf("list Ready for Agent Tickets: %w", err)
		}
	}

	partial := iterations >= r.Config.MaxIterations && len(ready) > 0
	return r.final(ctx, branch, done, partial)
}

// iterate runs one Ticket through a single Iteration: Claim, the Implement
// Phase (which must commit), the Review Phase (which may be commitless), then
// Done. A failed Phase or missing side effect returns an error; full Abort
// restore is a later Ticket.
func (r Orchestrator) iterate(ctx context.Context, t ticket.Ticket, branch string) error {
	if err := r.Tickets.Claim(ctx, t); err != nil {
		return fmt.Errorf("claim Ticket #%d: %w", t.Number, err)
	}

	restore, err := r.Repo.RecordRestorePoint()
	if err != nil {
		return fmt.Errorf("record restore point for Ticket #%d: %w", t.Number, err)
	}

	implInput := prompt.ImplementInput{Ticket: ticketInput(t), Branch: branch}
	if err := r.runPhase(ctx, prompt.Implement(implInput)); err != nil {
		return fmt.Errorf("Implement Phase for Ticket #%d: %w", t.Number, err)
	}

	committed, err := r.Repo.HasCommitsSince(restore)
	if err != nil {
		return fmt.Errorf("check Implement commits for Ticket #%d: %w", t.Number, err)
	}
	if !committed {
		return fmt.Errorf("Implement Phase for Ticket #%d produced no commit(s)", t.Number)
	}

	reviewInput := prompt.ReviewInput{Ticket: ticketInput(t), Branch: branch}
	if err := r.runPhase(ctx, prompt.Review(reviewInput)); err != nil {
		return fmt.Errorf("Review Phase for Ticket #%d: %w", t.Number, err)
	}

	if err := r.Tickets.Done(ctx, t); err != nil {
		return fmt.Errorf("mark Ticket #%d Done: %w", t.Number, err)
	}
	fmt.Fprintf(r.stdout(), "Ticket #%d Done\n", t.Number)
	return nil
}

// final is the end-of-run stop point. The Final Phase (branch review and pull
// request) is wired in a later Ticket; for now it only reports the outcome so
// the Run ends cleanly before Final.
func (r Orchestrator) final(_ context.Context, _ string, done []ticket.Ticket, partial bool) error {
	if partial {
		fmt.Fprintf(r.stdout(), "Stopped at max iterations (%d) with Ready for Agent Tickets remaining; %d Ticket(s) Done.\n", r.Config.MaxIterations, len(done))
	} else {
		fmt.Fprintf(r.stdout(), "Queue drained; %d Ticket(s) Done.\n", len(done))
	}
	return nil
}

func (r Orchestrator) runPhase(ctx context.Context, promptText string) error {
	return r.Agent.RunPhase(ctx, agent.PhaseRequest{
		Prompt:    promptText,
		Workspace: r.Repo.Dir,
		Model:     r.Config.Model,
		Timeout:   r.Config.Timeout,
	})
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
