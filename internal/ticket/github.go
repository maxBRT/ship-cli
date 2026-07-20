package ticket

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// Exec runs the gh CLI with the given args and returns stdout.
// Tests inject a fake; nil means the real gh binary.
type Exec func(ctx context.Context, args ...string) ([]byte, error)

// GitHub is a Ticket Port backed by the gh CLI.
type GitHub struct {
	Exec Exec
}

func (g *GitHub) exec() Exec {
	if g.Exec != nil {
		return g.Exec
	}
	return defaultExec
}

func defaultExec(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

const listReadyQuery = `
query($owner: String!, $name: String!) {
  repository(owner: $owner, name: $name) {
    issues(first: 100, states: OPEN, labels: ["ready-for-agent"], orderBy: {field: CREATED_AT, direction: ASC}) {
      nodes {
        number
        title
        createdAt
        labels(first: 20) { nodes { name } }
      }
    }
  }
}`

type repoView struct {
	NameWithOwner string `json:"nameWithOwner"`
}

type gqlListResponse struct {
	Data struct {
		Repository struct {
			Issues struct {
				Nodes []gqlIssue `json:"nodes"`
			} `json:"issues"`
		} `json:"repository"`
	} `json:"data"`
}

type gqlIssue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"createdAt"`
	Labels    struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
}

type orderedTicket struct {
	Ticket
	createdAt time.Time
}

// ListReady returns Ready for Agent Tickets in stable default order:
// ascending issue number, then oldest created date.
func (g *GitHub) ListReady(ctx context.Context, feature string) ([]Ticket, error) {
	execGH := g.exec()

	repoOut, err := execGH(ctx, "repo", "view", "--json", "nameWithOwner")
	if err != nil {
		return nil, err
	}
	var repo repoView
	if err := json.Unmarshal(repoOut, &repo); err != nil {
		return nil, fmt.Errorf("parse repo view: %w", err)
	}
	owner, name, ok := strings.Cut(repo.NameWithOwner, "/")
	if !ok || owner == "" || name == "" {
		return nil, fmt.Errorf("invalid nameWithOwner %q", repo.NameWithOwner)
	}

	out, err := execGH(ctx, "api", "graphql",
		"-f", "query="+listReadyQuery,
		"-F", "owner="+owner,
		"-F", "name="+name,
	)
	if err != nil {
		return nil, err
	}

	var resp gqlListResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("parse graphql list: %w", err)
	}

	ordered := make([]orderedTicket, 0, len(resp.Data.Repository.Issues.Nodes))
	for _, issue := range resp.Data.Repository.Issues.Nodes {
		if feature != "" && !hasLabel(issue, feature) {
			continue
		}
		ordered = append(ordered, orderedTicket{
			Ticket:    Ticket{Number: issue.Number, Title: issue.Title},
			createdAt: issue.CreatedAt,
		})
	}

	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Number != ordered[j].Number {
			return ordered[i].Number < ordered[j].Number
		}
		return ordered[i].createdAt.Before(ordered[j].createdAt)
	})

	outTickets := make([]Ticket, len(ordered))
	for i, r := range ordered {
		outTickets[i] = r.Ticket
	}
	return outTickets, nil
}

func hasLabel(issue gqlIssue, want string) bool {
	for _, l := range issue.Labels.Nodes {
		if l.Name == want {
			return true
		}
	}
	return false
}

// requiredLabels are the tracker labels Ship needs for Ready for Agent triage
// and ship queue membership.
var requiredLabels = []struct {
	Name        string
	Description string
	Color       string
}{
	{LabelReadyForAgent, "Eligible for a Ship Run queue", "0E8A16"},
	{LabelShip, "Remaining queue membership for a Ship Run", "9ADD98"},
}

// EnsureLabels creates Ready for Agent and ship when missing.
func (g *GitHub) EnsureLabels(ctx context.Context) error {
	execGH := g.exec()
	out, err := execGH(ctx, "label", "list", "--json", "name", "--limit", "1000")
	if err != nil {
		return fmt.Errorf("list labels: %w", err)
	}
	var existing []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &existing); err != nil {
		return fmt.Errorf("parse label list: %w", err)
	}
	have := make(map[string]struct{}, len(existing))
	for _, l := range existing {
		have[l.Name] = struct{}{}
	}
	for _, label := range requiredLabels {
		if _, ok := have[label.Name]; ok {
			continue
		}
		_, err := execGH(ctx, "label", "create", label.Name,
			"--description", label.Description,
			"--color", label.Color,
		)
		if err != nil {
			return fmt.Errorf("create label %q: %w", label.Name, err)
		}
	}
	return nil
}

// Stamp adds ship to each Ticket without removing Ready for Agent.
func (g *GitHub) Stamp(ctx context.Context, tickets []Ticket) error {
	for _, t := range tickets {
		_, err := g.exec()(ctx, "issue", "edit", fmt.Sprintf("%d", t.Number),
			"--add-label", LabelShip,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// Done removes ship and closes the Ticket's GitHub issue.
func (g *GitHub) Done(ctx context.Context, t Ticket) error {
	execGH := g.exec()
	_, err := execGH(ctx, "issue", "edit", fmt.Sprintf("%d", t.Number),
		"--remove-label", LabelShip,
	)
	if err != nil {
		return err
	}
	_, err = execGH(ctx, "issue", "close", fmt.Sprintf("%d", t.Number))
	return err
}

// HasOpenPR reports whether an open pull request exists for branch.
// Ship uses this to verify the Final Phase opened a PR.
func (g *GitHub) HasOpenPR(ctx context.Context, branch string) (bool, error) {
	out, err := g.exec()(ctx, "pr", "list",
		"--head", branch,
		"--state", "open",
		"--json", "number",
	)
	if err != nil {
		return false, err
	}
	var prs []struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(out, &prs); err != nil {
		return false, fmt.Errorf("parse pr list: %w", err)
	}
	return len(prs) > 0, nil
}

var _ Port = (*GitHub)(nil)

