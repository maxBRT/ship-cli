package ticket_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/maxBRT/ship-cli/internal/ticket"
)

func TestListReady_ordersByNumberThenCreatedDate(t *testing.T) {
	gh := &ticket.GitHub{Exec: scriptedExec(t, map[string]string{
		"repo view --json nameWithOwner": `{"nameWithOwner":"maxBRT/ship-cli"}`,
		"api graphql": `{
			"data": {
				"repository": {
					"issues": {
						"nodes": [
							{
								"number": 10,
								"title": "newer higher number",
								"createdAt": "2026-06-01T00:00:00Z",
								"labels": {"nodes": [{"name": "ship"}]}
							},
							{
								"number": 3,
								"title": "older lower number",
								"createdAt": "2026-01-01T00:00:00Z",
								"labels": {"nodes": [{"name": "ship"}]}
							},
							{
								"number": 7,
								"title": "middle",
								"createdAt": "2026-03-01T00:00:00Z",
								"labels": {"nodes": [{"name": "ship"}]}
							}
						]
					}
				}
			}
		}`,
	})}

	got, err := gh.ListReady(context.Background())
	if err != nil {
		t.Fatalf("ListReady: %v", err)
	}
	// Stable default: issue number ascending (Priority must not reorder).
	want := []int{3, 7, 10}
	if len(got) != len(want) {
		t.Fatalf("ListReady len=%d, want %d (%v)", len(got), len(want), got)
	}
	for i, n := range want {
		if got[i].Number != n {
			t.Fatalf("ListReady[%d].Number=%d, want %d (full=%v)", i, got[i].Number, n, numbers(got))
		}
	}
}

func TestEnsureLabels_createsMissingShipLabels(t *testing.T) {
	var creates [][]string
	gh := &ticket.GitHub{Exec: func(ctx context.Context, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if joined == "label list --json name --limit 1000" {
			return []byte(`[{"name":"bug"}]`), nil
		}
		if len(args) >= 3 && args[0] == "label" && args[1] == "create" {
			creates = append(creates, append([]string(nil), args...))
			return nil, nil
		}
		return nil, fmt.Errorf("unexpected gh %s", joined)
	}}

	if err := gh.EnsureLabels(context.Background()); err != nil {
		t.Fatalf("EnsureLabels: %v", err)
	}
	wantNames := []string{ticket.LabelReadyForAgent, ticket.LabelShip}
	if len(creates) != len(wantNames) {
		t.Fatalf("created=%v, want %v", creates, wantNames)
	}
	for i, name := range wantNames {
		got := strings.Join(creates[i], " ")
		if creates[i][2] != name {
			t.Fatalf("created[%d] name=%q, want %q", i, creates[i][2], name)
		}
		if !strings.Contains(got, "--description ") || !strings.Contains(got, "--color ") {
			t.Fatalf("create %q missing description/color: %q", name, got)
		}
	}
	for _, create := range creates {
		if create[2] == "in-progress" {
			t.Fatal("EnsureLabels must not create in-progress")
		}
	}
}

func TestEnsureLabels_skipsLabelsThatAlreadyExist(t *testing.T) {
	var creates []string
	gh := &ticket.GitHub{Exec: func(ctx context.Context, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if joined == "label list --json name --limit 1000" {
			return []byte(fmt.Sprintf(
				`[{"name":%q},{"name":%q},{"name":"bug"}]`,
				ticket.LabelReadyForAgent, ticket.LabelShip,
			)), nil
		}
		if len(args) >= 3 && args[0] == "label" && args[1] == "create" {
			creates = append(creates, args[2])
			return nil, nil
		}
		return nil, fmt.Errorf("unexpected gh %s", joined)
	}}

	if err := gh.EnsureLabels(context.Background()); err != nil {
		t.Fatalf("EnsureLabels: %v", err)
	}
	if len(creates) != 0 {
		t.Fatalf("created=%v, want none when labels already exist", creates)
	}
}

func TestEnsureLabels_createsOnlyMissingLabel(t *testing.T) {
	var creates []string
	gh := &ticket.GitHub{Exec: func(ctx context.Context, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if joined == "label list --json name --limit 1000" {
			return []byte(fmt.Sprintf(`[{"name":%q}]`, ticket.LabelReadyForAgent)), nil
		}
		if len(args) >= 3 && args[0] == "label" && args[1] == "create" {
			creates = append(creates, args[2])
			return nil, nil
		}
		return nil, fmt.Errorf("unexpected gh %s", joined)
	}}

	if err := gh.EnsureLabels(context.Background()); err != nil {
		t.Fatalf("EnsureLabels: %v", err)
	}
	if len(creates) != 1 || creates[0] != ticket.LabelShip {
		t.Fatalf("created=%v, want only %q", creates, ticket.LabelShip)
	}
}

func TestStamp_addsShipWithoutRemovingReadyForAgent(t *testing.T) {
	var calls []string
	gh := &ticket.GitHub{Exec: func(ctx context.Context, args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		return nil, nil
	}}

	err := gh.Stamp(context.Background(), []ticket.Ticket{
		{Number: 7, Title: "seven"},
		{Number: 8, Title: "eight"},
	})
	if err != nil {
		t.Fatalf("Stamp: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("calls=%v, want 2 edits", calls)
	}
	for i, n := range []int{7, 8} {
		got := calls[i]
		wantAdd := fmt.Sprintf("issue edit %d --add-label %s", n, ticket.LabelShip)
		if got != wantAdd {
			t.Fatalf("Stamp call[%d]=%q, want %q", i, got, wantAdd)
		}
		if strings.Contains(got, "--remove-label") {
			t.Fatalf("Stamp must keep Ready for Agent; got %q", got)
		}
		if strings.Contains(got, "in-progress") {
			t.Fatalf("Stamp must not write in-progress; got %q", got)
		}
	}
}

func TestDone_removesShipAndClosesIssue(t *testing.T) {
	var calls []string
	gh := &ticket.GitHub{Exec: func(ctx context.Context, args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		return nil, nil
	}}

	err := gh.Done(context.Background(), ticket.Ticket{Number: 7, Title: "port"})
	if err != nil {
		t.Fatalf("Done: %v", err)
	}
	want := []string{
		"issue edit 7 --remove-label " + ticket.LabelShip,
		"issue close 7",
	}
	if len(calls) != len(want) {
		t.Fatalf("Done calls=%v, want %v", calls, want)
	}
	for i, w := range want {
		if calls[i] != w {
			t.Fatalf("Done calls[%d]=%q, want %q", i, calls[i], w)
		}
	}
}

func TestOpenPRURL_returnsURLWhenOpenPRExistsForBranch(t *testing.T) {
	gh := &ticket.GitHub{Exec: scriptedExec(t, map[string]string{
		"pr list --head ship/run --state open --json url": `[{"url":"https://github.com/acme/ship/pull/42"}]`,
	})}

	got, err := gh.OpenPRURL(context.Background(), "ship/run")
	if err != nil {
		t.Fatalf("OpenPRURL: %v", err)
	}
	want := "https://github.com/acme/ship/pull/42"
	if got != want {
		t.Fatalf("OpenPRURL = %q, want %q", got, want)
	}
}

func TestOpenPRURL_emptyWhenNone(t *testing.T) {
	gh := &ticket.GitHub{Exec: scriptedExec(t, map[string]string{
		"pr list --head ship/run --state open --json url": `[]`,
	})}

	got, err := gh.OpenPRURL(context.Background(), "ship/run")
	if err != nil {
		t.Fatalf("OpenPRURL: %v", err)
	}
	if got != "" {
		t.Fatalf("OpenPRURL = %q, want empty when no open PR", got)
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
