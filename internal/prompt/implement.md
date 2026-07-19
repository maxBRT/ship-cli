You are in the Implement phase of a Ship Run on branch {{.Branch}}.

Ticket #{{.Ticket.Number}}: {{.Ticket.Title}}
{{if .Ticket.Body}}
Ticket body:
{{.Ticket.Body}}
{{end}}
Do the Ticket work described above.
You must commit your changes before finishing. Ship verifies that Implement produces at least one commit; a commitless exit is a failure.
