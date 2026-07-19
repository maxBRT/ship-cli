## Goal

In a **single pass**, review the recent commits for issue #{{.Ticket.Number}} on branch `{{.Branch}}`. Fix problems you find in this same pass when needed. A new commit is optional - if the work is already correct, finish without committing.

**Issue #{{.Ticket.Number}}: {{.Ticket.Title}}**
{{if .Ticket.Body}}
{{.Ticket.Body}}
{{end}}

## What to review

Review only commits that belong to this issue (not unrelated history on the branch). Identify that range from the git log; do not ask for a fixed point. Confirm the diff is non-empty before deep review.

Review along **two separate axes**. Do not merge findings across axes.

### Spec

Does the diff satisfy this issue (the body above is the primary spec)?
- Missing or partial requirements
- Extra behaviour that wasn't asked for (scope creep)
- Requirements that look done but are wrong

Quote the issue line for each finding.

### Standards

Does the diff follow repo standards (`CONTEXT.md`, ADRs, `CODING_STANDARDS.md` / `CONTRIBUTING.md` if present)?

Also apply the smell baseline below as judgement calls only. Repo standards override the baseline. Skip anything tooling already enforces.

Smell baseline (what it is → how to fix):
- **Mysterious Name** — name doesn't reveal intent → rename
- **Duplicated Code** — same logic shape in multiple places → extract shared shape
- **Feature Envy** — method uses another object's data more than its own → move it
- **Data Clumps** — same fields travel together → bundle into a type
- **Primitive Obsession** — primitive standing in for a domain concept → small type
- **Repeated Switches** — same type cascade in multiple places → polymorphism or shared map
- **Shotgun Surgery** — one change scatters across many files → gather what changes together
- **Divergent Change** — one module edited for unrelated reasons → split by reason
- **Speculative Generality** — abstraction for unneeded future → delete until a real need
- **Message Chains** — long `a.b().c().d()` → hide behind one method
- **Middle Man** — mostly delegates → call the real target
- **Refused Bequest** — ignores most of what it inherits → composition instead

## Done when

You may adjust code or commits if Spec or Standards findings need fixing now.
You may finish **without a new commit** if nothing needs changing.
