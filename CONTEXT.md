# Ship

Orchestrates a sequential implement/review loop over tagged tickets in a local checkout, then finishes with a final review and pull request.

## Language

**Ship**:
The CLI product that orchestrates ticket work in a local checkout.
_Avoid_: ship-cli (except as the repo/binary name), ralph, loop tool

**Run**:
One invocation of ship: confirm a ship queue from Ready for Agent tickets, process that frozen set in sequence, then (when applicable) finish with a final review and pull request.
_Avoid_: session, job, voyage, loop (as a noun for the whole invocation)

**Ticket**:
A unit of work a run may process through an Iteration until it is Done. Today each ticket is backed by a GitHub issue; other trackers may back tickets later.
_Avoid_: Issue, task, item

**Ready for Agent**:
Ticket state meaning the ticket is eligible to appear as a run queue candidate.
_Avoid_: ready, open, backlog, todo

**ship (label)**:
Tracker label meaning the ticket is a member of a run's remaining confirmed queue. Stamped when the queue is confirmed; cleared when the ticket is Done. Unfinished tickets keep it after Abort or Partial Progress.
_Avoid_: in-progress, claimed, lock (as the run membership model)

**Done**:
Ticket state meaning the ticket's work for this run is finished and it should not be queued again.
_Avoid_: closed (as the domain name), complete, finished, resolved

**Queue confirmation**:
The act of selecting the ordered ticket set a run will process via an interactive picker over Ready for Agent candidates (fakeable in tests) and stamping that set with ship.
_Avoid_: Claim, pickup, lock, assign (unless talking about GitHub assignee specifically)

**Iteration**:
Processing exactly one ticket in a run: one Implement phase followed by one Review phase. A run's max-iterations limit caps how many tickets it will process this way before the Final phase.
_Avoid_: loop, cycle, pass (as synonyms for this whole ticket unit)

**Phase**:
A single fresh agent invocation within a run. The phases are Implement, Review, and Final.
_Avoid_: turn, step, stage, session

**Implement**:
The phase where an agent does the ticket's work and commits it.
_Avoid_: coding phase, builder, worker

**Review**:
The phase where a fresh agent reviews the ticket's commits and may adjust them in a single pass before the run moves on.
_Avoid_: check, QA (for this per-ticket step), critique

**Final**:
The end-of-run phase where a fresh agent reviews the whole branch, runs tests/e2e, and opens the pull request.
_Avoid_: wrap-up, finalize, PR phase, ship phase

**Agent**:
The external coding agent that executes a phase (e.g. Cursor). A run uses one agent for every phase.
_Avoid_: backend, runner, harness, model (model is a setting of an agent, not the agent itself)

**Abort**:
Stopping a run after a failed or timed-out phase, undoing that ticket's commits and stopping further iterations, while leaving ship on unfinished tickets and reporting the failure on stderr.
_Avoid_: revert, rollback, bail, crash (crash is unplanned; abort is deliberate stop)

**Partial Progress**:
A run outcome where the Final phase still opens a pull request, but some confirmed queue tickets remain because the run hit its max iterations. Remaining tickets keep ship.
_Avoid_: incomplete run, truncated run, WIP (as the name for this outcome)
