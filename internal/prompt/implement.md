## Goal

Do the work described in issue #{{.Ticket.Number}} on branch `{{.Branch}}`. Finish with at least one git commit.

**Issue #{{.Ticket.Number}}: {{.Ticket.Title}}**
{{if .Ticket.Body}}
{{.Ticket.Body}}
{{end}}

## How to work (TDD)

Where the issue has a testable seam, work in the red → green loop. Do not write production code before a failing test.

**Good tests** verify behavior through public interfaces, not implementation details. Code can change entirely; tests shouldn't. A good test reads like a specification — it names a capability that exists — and survives refactors because it doesn't care about internal structure.

**Seams** are the public boundaries you test at: the interface where you observe behavior without reaching inside. Tests live at seams, never against internals.

Test only at pre-agreed seams already implied by this issue, `CONTEXT.md`, and ADRs in the area you touch. Do not invent new seams or ask for confirmation — this session is non-interactive.

**Anti-patterns to avoid**
- **Implementation-coupled** — mocks internal collaborators, tests private methods, or verifies through a side channel (querying the database instead of using the interface). The tell: the test breaks when you refactor but behavior hasn't changed.
- **Tautological** — the assertion recomputes the expected value the way the code does (`expect(add(a, b)).toBe(a + b)`, a snapshot derived by hand the same way, a constant asserted equal to itself), so it passes by construction and can never disagree with the code. Expected values must come from an independent source of truth — a known-good literal, a worked example, the issue/spec.
- **Horizontal slicing** — writing all tests first, then all implementation. Bulk tests verify imagined behavior: you test the shape of things rather than user-facing behavior. Work in **vertical slices** instead — one test → one implementation → repeat.

**Rules of the loop**
- **Red before green.** Write the failing test first, then only enough code to pass it. Don't anticipate future tests or add speculative features.
- **One slice at a time.** One seam, one test, one minimal implementation per cycle.
- **Do not refactor during the loop.** Leave cleanup for a later review pass.

## Verification while working

Run typechecking regularly, single test files regularly, and the full test suite once at the end.

## Done when

You must commit your changes before finishing. Exiting without a commit is a failure.
