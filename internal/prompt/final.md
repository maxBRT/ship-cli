You are in the Final phase of a Ship Run on branch {{.Branch}}.

Review the whole branch diff for this Run.
Run the project's tests and e2e checks; fix any failures before opening the pull request.
Open a pull request for this branch. The PR body must include RISK, QA notes, and linked Tickets.
Ship verifies that Final opens a pull request; exiting without an opened PR is a failure.
{{if .Tickets}}
Linked Tickets from this Run:
{{range .Tickets}}- #{{.Number}}: {{.Title}}
{{end}}
{{end}}{{if .PartialProgress}}This Run is Partial Progress: it stopped because it hit max iterations ({{.MaxIterations}}) with Ready for Agent Tickets still remaining. Disclose that clearly in the PR body so reviewers know more work remains.
{{end}}
