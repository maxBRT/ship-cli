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
        issueFieldValues(first: 20) {
          nodes {
            __typename
            ... on IssueFieldSingleSelectValue {
              value
              field {
                ... on IssueFieldSingleSelect { name }
              }
            }
          }
        }
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
	IssueFieldValues struct {
		Nodes []gqlFieldValue `json:"nodes"`
	} `json:"issueFieldValues"`
}

type gqlFieldValue struct {
	Typename string `json:"__typename"`
	Value    string `json:"value"`
	Field    struct {
		Name string `json:"name"`
	} `json:"field"`
}

type rankedTicket struct {
	Ticket
	rank      int
	createdAt time.Time
}

// ListReady returns Ready for Agent Tickets ordered by priority then oldest.
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

	ranked := make([]rankedTicket, 0, len(resp.Data.Repository.Issues.Nodes))
	for _, issue := range resp.Data.Repository.Issues.Nodes {
		if feature != "" && !hasLabel(issue, feature) {
			continue
		}
		ranked = append(ranked, rankedTicket{
			Ticket:    Ticket{Number: issue.Number, Title: issue.Title},
			rank:      priorityRank(issue),
			createdAt: issue.CreatedAt,
		})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].rank != ranked[j].rank {
			return ranked[i].rank < ranked[j].rank
		}
		return ranked[i].createdAt.Before(ranked[j].createdAt)
	})

	outTickets := make([]Ticket, len(ranked))
	for i, r := range ranked {
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

func priorityRank(issue gqlIssue) int {
	for _, fv := range issue.IssueFieldValues.Nodes {
		if !strings.EqualFold(fv.Field.Name, "Priority") {
			continue
		}
		return rankPriorityValue(fv.Value)
	}
	return 100
}

func rankPriorityValue(v string) int {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "p0", "critical", "urgent":
		return 0
	case "p1", "high":
		return 1
	case "p2", "medium", "med":
		return 2
	case "p3", "low":
		return 3
	case "":
		return 100
	default:
		return 50
	}
}

// Claim moves a Ticket from Ready for Agent to In Progress.
func (g *GitHub) Claim(ctx context.Context, t Ticket) error {
	_, err := g.exec()(ctx, "issue", "edit", fmt.Sprintf("%d", t.Number),
		"--remove-label", LabelReadyForAgent,
		"--add-label", LabelInProgress,
	)
	return err
}

// Done marks a Ticket Done by closing its GitHub issue.
func (g *GitHub) Done(ctx context.Context, t Ticket) error {
	_, err := g.exec()(ctx, "issue", "close", fmt.Sprintf("%d", t.Number))
	return err
}

// Abort restores a Ticket from In Progress to Ready for Agent.
func (g *GitHub) Abort(ctx context.Context, t Ticket) error {
	_, err := g.exec()(ctx, "issue", "edit", fmt.Sprintf("%d", t.Number),
		"--remove-label", LabelInProgress,
		"--add-label", LabelReadyForAgent,
	)
	return err
}

var _ Port = (*GitHub)(nil)

