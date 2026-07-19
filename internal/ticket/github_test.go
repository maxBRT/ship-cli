package ticket_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/maxBRT/ship-cli/internal/ticket"
)

func TestListReady_ordersByPriorityThenOldest(t *testing.T) {
	gh := &ticket.GitHub{Exec: scriptedExec(t, map[string]string{
		"repo view --json nameWithOwner": `{"nameWithOwner":"maxBRT/ship-cli"}`,
		"api graphql": `{
			"data": {
				"repository": {
					"issues": {
						"nodes": [
							{
								"number": 1,
								"title": "old low",
								"createdAt": "2026-01-01T00:00:00Z",
								"labels": {"nodes": [{"name": "ready-for-agent"}]},
								"issueFieldValues": {"nodes": [{
									"__typename": "IssueFieldSingleSelectValue",
									"value": "Low",
									"field": {"name": "Priority"}
								}]}
							},
							{
								"number": 2,
								"title": "new high",
								"createdAt": "2026-06-01T00:00:00Z",
								"labels": {"nodes": [{"name": "ready-for-agent"}]},
								"issueFieldValues": {"nodes": [{
									"__typename": "IssueFieldSingleSelectValue",
									"value": "High",
									"field": {"name": "Priority"}
								}]}
							},
							{
								"number": 3,
								"title": "older high",
								"createdAt": "2026-03-01T00:00:00Z",
								"labels": {"nodes": [{"name": "ready-for-agent"}]},
								"issueFieldValues": {"nodes": [{
									"__typename": "IssueFieldSingleSelectValue",
									"value": "High",
									"field": {"name": "Priority"}
								}]}
							},
							{
								"number": 4,
								"title": "unset priority oldest",
								"createdAt": "2025-01-01T00:00:00Z",
								"labels": {"nodes": [{"name": "ready-for-agent"}]},
								"issueFieldValues": {"nodes": []}
							}
						]
					}
				}
			}
		}`,
	})}

	got, err := gh.ListReady(context.Background(), "")
	if err != nil {
		t.Fatalf("ListReady: %v", err)
	}
	want := []int{3, 2, 1, 4} // High oldest, High newer, Low, then unset
	if len(got) != len(want) {
		t.Fatalf("ListReady len=%d, want %d (%v)", len(got), len(want), got)
	}
	for i, n := range want {
		if got[i].Number != n {
			t.Fatalf("ListReady[%d].Number=%d, want %d (full=%v)", i, got[i].Number, n, numbers(got))
		}
	}
}

func TestListReady_filtersByFeatureLabel(t *testing.T) {
	gh := &ticket.GitHub{Exec: scriptedExec(t, map[string]string{
		"repo view --json nameWithOwner": `{"nameWithOwner":"maxBRT/ship-cli"}`,
		"api graphql": `{
			"data": {
				"repository": {
					"issues": {
						"nodes": [
							{
								"number": 10,
								"title": "other feature",
								"createdAt": "2026-01-01T00:00:00Z",
								"labels": {"nodes": [
									{"name": "ready-for-agent"},
									{"name": "feat-other"}
								]},
								"issueFieldValues": {"nodes": []}
							},
							{
								"number": 11,
								"title": "wanted feature",
								"createdAt": "2026-02-01T00:00:00Z",
								"labels": {"nodes": [
									{"name": "ready-for-agent"},
									{"name": "feat-ship"}
								]},
								"issueFieldValues": {"nodes": []}
							}
						]
					}
				}
			}
		}`,
	})}

	got, err := gh.ListReady(context.Background(), "feat-ship")
	if err != nil {
		t.Fatalf("ListReady: %v", err)
	}
	if len(got) != 1 || got[0].Number != 11 {
		t.Fatalf("ListReady = %v, want only #11", numbers(got))
	}
}

func TestClaim_transitionsReadyToInProgress(t *testing.T) {
	var calls []string
	gh := &ticket.GitHub{Exec: func(ctx context.Context, args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		return nil, nil
	}}

	err := gh.Claim(context.Background(), ticket.Ticket{Number: 7, Title: "port"})
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("calls=%v, want 1 edit", calls)
	}
	got := calls[0]
	for _, want := range []string{
		"issue edit 7",
		"--remove-label " + ticket.LabelReadyForAgent,
		"--add-label " + ticket.LabelInProgress,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Claim args %q missing %q", got, want)
		}
	}
}

func TestDone_closesIssue(t *testing.T) {
	var calls []string
	gh := &ticket.GitHub{Exec: func(ctx context.Context, args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		return nil, nil
	}}

	err := gh.Done(context.Background(), ticket.Ticket{Number: 7, Title: "port"})
	if err != nil {
		t.Fatalf("Done: %v", err)
	}
	if len(calls) != 1 || calls[0] != "issue close 7" {
		t.Fatalf("Done calls=%v, want [issue close 7]", calls)
	}
}

func TestAbort_restoresReadyForAgent(t *testing.T) {
	var calls []string
	gh := &ticket.GitHub{Exec: func(ctx context.Context, args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		return nil, nil
	}}

	err := gh.Abort(context.Background(), ticket.Ticket{Number: 7, Title: "port"})
	if err != nil {
		t.Fatalf("Abort: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("calls=%v, want 1 edit", calls)
	}
	got := calls[0]
	for _, want := range []string{
		"issue edit 7",
		"--remove-label " + ticket.LabelInProgress,
		"--add-label " + ticket.LabelReadyForAgent,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Abort args %q missing %q", got, want)
		}
	}
}

func TestHasOpenPR_trueWhenOpenPRExistsForBranch(t *testing.T) {
	gh := &ticket.GitHub{Exec: scriptedExec(t, map[string]string{
		"pr list --head ship/run --state open --json number": `[{"number":42}]`,
	})}

	open, err := gh.HasOpenPR(context.Background(), "ship/run")
	if err != nil {
		t.Fatalf("HasOpenPR: %v", err)
	}
	if !open {
		t.Fatal("HasOpenPR = false, want true when an open PR exists")
	}
}

func TestHasOpenPR_falseWhenNone(t *testing.T) {
	gh := &ticket.GitHub{Exec: scriptedExec(t, map[string]string{
		"pr list --head ship/run --state open --json number": `[]`,
	})}

	open, err := gh.HasOpenPR(context.Background(), "ship/run")
	if err != nil {
		t.Fatalf("HasOpenPR: %v", err)
	}
	if open {
		t.Fatal("HasOpenPR = true, want false when no open PR")
	}
}

func numbers(ts []ticket.Ticket) []int {
	out := make([]int, len(ts))
	for i, t := range ts {
		out[i] = t.Number
	}
	return out
}

// scriptedExec matches gh argv joined by spaces against keys (exact or prefix).
// "api graphql" matches any graphql invocation.
func scriptedExec(t *testing.T, scripts map[string]string) ticket.Exec {
	t.Helper()
	return func(ctx context.Context, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		for key, out := range scripts {
			if joined == key || strings.HasPrefix(joined, key+" ") || (key == "api graphql" && strings.HasPrefix(joined, "api graphql")) {
				return []byte(out), nil
			}
		}
		return nil, fmt.Errorf("unexpected gh %s", joined)
	}
}
