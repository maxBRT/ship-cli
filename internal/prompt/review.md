You are in the Review phase of a Ship Run on branch {{.Branch}}.

Ticket #{{.Ticket.Number}}: {{.Ticket.Title}}
{{if .Ticket.Body}}
Ticket body:
{{.Ticket.Body}}
{{end}}
Perform a single pass review of this Ticket's commits on the branch.
You may adjust the code or commits if you find issues that need fixing in this same pass.
Commitless success is allowed: if the Implement work is already correct, finish without a new commit. Ship does not require Review to produce commits.
