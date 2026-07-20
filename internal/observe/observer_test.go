package observe_test

import (
	"strings"
	"testing"

	"github.com/maxBRT/ship-cli/internal/observe"
)

func TestObserver_EndPhase_dumpsToolOneLiners(t *testing.T) {
	// Worked example: same curated tool fields as Phase logs (name,
	// duration_ms, status); terminal presentation is one line each.
	var out strings.Builder
	obs := observe.New(&out)

	sink := obs.BeginPhase("Implement")
	sink.Emit(observe.Event{
		Kind:       observe.KindTool,
		Name:       "Read",
		DurationMS: 42,
		Status:     observe.ToolOK,
	})
	sink.Emit(observe.Event{
		Kind:       observe.KindTool,
		Name:       "Write",
		DurationMS: 7,
		Status:     observe.ToolError,
	})
	obs.EndPhase()

	got := out.String()
	wantLines := []string{
		"tool  Read  42ms  ok",
		"tool  Write  7ms  error",
	}
	for _, want := range wantLines {
		if !strings.Contains(got, want) {
			t.Errorf("dump missing %q; got:\n%s", want, got)
		}
	}
}

func TestObserver_EndPhase_dumpsPhaseTokenTotals(t *testing.T) {
	// Same curated token fields as phase_end in Phase logs; one terminal line.
	var out strings.Builder
	obs := observe.New(&out)

	sink := obs.BeginPhase("Review")
	sink.Emit(observe.Event{
		Kind:       observe.KindTool,
		Name:       "Shell",
		DurationMS: 10,
		Status:     observe.ToolOK,
	})
	sink.Emit(observe.Event{
		Kind:    observe.KindPhaseEnd,
		Outcome: observe.OutcomeSuccess,
		Tokens: &observe.TokenCounts{
			Input:      120,
			Output:     45,
			CacheRead:  10,
			CacheWrite: 2,
		},
	})
	obs.EndPhase()

	got := out.String()
	if !strings.Contains(got, "tool  Shell  10ms  ok") {
		t.Errorf("dump missing tool line; got:\n%s", got)
	}
	wantTokens := "tokens  input=120 output=45 cache_read=10 cache_write=2"
	if !strings.Contains(got, wantTokens) {
		t.Errorf("dump missing %q; got:\n%s", wantTokens, got)
	}
}

func TestObserver_Emit_doesNotWriteUntilEndPhase(t *testing.T) {
	// Throbber stays the only live UI during the Phase: buffered Emit must
	// not interleave tool lines before EndPhase.
	var out strings.Builder
	obs := observe.New(&out)

	sink := obs.BeginPhase("Implement")
	sink.Emit(observe.Event{
		Kind:       observe.KindTool,
		Name:       "Read",
		DurationMS: 42,
		Status:     observe.ToolOK,
	})
	if got := out.String(); got != "" {
		t.Fatalf("mid-Phase Out = %q, want empty until EndPhase", got)
	}

	obs.EndPhase()
	if !strings.Contains(out.String(), "tool  Read  42ms  ok") {
		t.Errorf("after EndPhase dump missing tool line; got:\n%s", out.String())
	}
}

func TestObserver_EndPhase_omitsTokensLineWhenUsageAbsent(t *testing.T) {
	var out strings.Builder
	obs := observe.New(&out)

	sink := obs.BeginPhase("Final")
	sink.Emit(observe.Event{
		Kind:       observe.KindTool,
		Name:       "Shell",
		DurationMS: 3,
		Status:     observe.ToolOK,
	})
	sink.Emit(observe.Event{
		Kind:    observe.KindPhaseEnd,
		Outcome: observe.OutcomeSuccess,
		// Tokens absent: degrade gracefully — no tokens line.
	})
	obs.EndPhase()

	got := out.String()
	if !strings.Contains(got, "tool  Shell  3ms  ok") {
		t.Errorf("dump missing tool line; got:\n%s", got)
	}
	if strings.Contains(got, "tokens") {
		t.Errorf("dump = %q, want no tokens line when usage absent", got)
	}
}
