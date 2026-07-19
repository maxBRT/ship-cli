package prompt

// TicketInput is Ticket identity and body for phase prompts.
type TicketInput struct {
	Number int
	Title  string
	Body   string
}

// ImplementInput is structured input for the Implement phase prompt.
type ImplementInput struct {
	Ticket TicketInput
	Branch string
}

// ReviewInput is structured input for the Review phase prompt.
type ReviewInput struct {
	Ticket TicketInput
	Branch string
}

// FinalInput is structured input for the Final phase prompt.
type FinalInput struct {
	Branch          string
	Tickets         []TicketInput // Done Tickets from this Run (linked in the PR)
	PartialProgress bool
	MaxIterations   int
}
