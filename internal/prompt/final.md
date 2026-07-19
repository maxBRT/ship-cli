## Goal

On branch `{{.Branch}}`, make this work safe for a human to merge, then **open a pull request** against the repo default branch (usually `main`). Exiting without an opened PR is a failure.

Do **not** start new feature work or pick up leftover issues. Only fix blockers on what already landed, get the green bar, and open the PR.
{{if .PartialProgress}}
## Mode: Partial Progress (light Final)

This automation stopped early after max iterations ({{.MaxIterations}}) with more ready work still queued. A later run will do the full Final review.

For this PR:
1. Skip the deep branch/integration review below.
2. Still hit the **green bar** (tests, e2e, lint/typecheck). Fix blockers that keep the bar red; commit those fixes.
3. Open the PR with a loud **Partial Progress** banner in the body (see PR body).
{{else}}
## 1. Branch-level integration review

Review the full diff of `{{.Branch}}` against the default branch (`main`). This is **not** a redo of each issue’s per-commit review. Focus on the branch as one coherent change:

- Do the landed issues fit together? Missing glue, conflicting approaches, broken imports/APIs across commits?
- Domain language and docs (`CONTEXT.md`, ADRs) still consistent?
- Anything unsafe or merge-blocking that a per-issue review would miss?

Keep Spec vs Standards concerns separate in your own notes if helpful, but optimize for **integration and merge readiness**, not nits.
{{end}}

## 2. Green bar (required)

Before opening the PR, make these pass locally (use the repo’s real commands from README/Makefile/CI config when present):

- Unit/integration tests
- e2e checks if the repo has them
- Lint and typecheck

If something fails, **fix the blocker**, commit the fix, and re-run until green. Do not open a knowingly red PR.

## 3. Fix policy

- **Fix:** test/lint/typecheck failures, broken integration across the branch, clear bugs that would bite a merger.
- **Do not:** drive-by refactors, style-only polish, or new feature work. Leave taste and nits for the human reviewer.
- **Commit** any Final fixes before opening the PR. Do not leave a dirty tree.

## 4. Open the pull request

Open a PR from `{{.Branch}}` into the default branch. The body must be concrete — no `TBD` / placeholder RISK or QA.

### PR body structure

**Summary** — what landed and why, in a few sentences.

**Linked issues**
{{if .Tickets}}{{range .Tickets}}- #{{.Number}}: {{.Title}}
{{end}}{{else}}- (none provided)
{{end}}
**RISK** — concrete failure modes if this merges (what breaks, who is affected, **how likely**). Not vague hedges.

**QA — stress-test plan for a human** — enough that someone unfamiliar with this branch knows exactly what to exercise, make sure to use the domain language of this repo when it applies:

1. **Happy path** — primary flows to verify the new work works end-to-end (exact commands, URLs, or UI paths).
2. **Edges / failure cases** — what to try to stress the feature (empty input, auth failures, concurrency, bad data, etc. as relevant).
3. **Out of scope** — what they can skip so they don’t waste time on unrelated areas.
{{if .PartialProgress}}
**Partial Progress** (required banner)

State clearly at the top of the PR body:

> **Partial Progress:** this run stopped after max iterations ({{.MaxIterations}}). More ready work remains. Treat this PR as an intermediate checkpoint; a later run should perform the full Final review.
{{end}}
## Done when

- Green bar is green
- Any Final fixes are committed
- A pull request is open with Summary, Linked issues, RISK, and the QA stress-test plan
{{if .PartialProgress}}- The Partial Progress banner is unmistakable in the PR body
{{end}}
