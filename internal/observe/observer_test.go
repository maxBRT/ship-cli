package observe_test

import (
	"strings"
	"testing"

	"github.com/maxBRT/ship-cli/internal/observe"
)

func TestObserver_EndPhase_dumpsOneToolsAndTokensLine(t *testing.T) {
	// Terminal dump is one high-signal line: tool total plus tokens.
	// Per-tool detail stays in the Phase JSON log only.
	var out strings.Builder
	obs := observe.New(t.TempDir(), &out)

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
	want := "tools  2  tokens  input=120 output=45 cache_read=10 cache_write=2\n"
	if got != want {
		t.Errorf("dump = %q, want %q", got, want)
	}
	if strings.Contains(got, "tool  Read") || strings.Contains(got, "tool  Write") {
		t.Errorf("dump must not list per-tool lines; got:\n%s", got)
	}
}

func TestObserver_EndPhase_toolsOnlyWhenUsageAbsent(t *testing.T) {
	var out strings.Builder
	obs := observe.New(t.TempDir(), &out)

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
	})
	obs.EndPhase()

	got := out.String()
	want := "tools  1\n"
	if got != want {
		t.Errorf("dump = %q, want %q", got, want)
	}
	if strings.Contains(got, "tokens") {
		t.Errorf("dump = %q, want no tokens when usage absent", got)
	}
}

func TestObserver_Emit_doesNotWriteUntilEndPhase(t *testing.T) {
	// Throbber stays the only live UI during the Phase: buffered Emit must
	// not interleave dump lines before EndPhase.
	var out strings.Builder
	obs := observe.New(t.TempDir(), &out)

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
	if got := out.String(); got != "tools  1\n" {
		t.Errorf("after EndPhase dump = %q, want tools  1\\n", got)
	}
}

func TestObserver_Abort_keepsLastHandfulOfToolLines(t *testing.T) {
	// Abort banner keeps only the trailing handful of tool lines.
	var out strings.Builder
	obs := observe.New(t.TempDir(), &out)
	sink := obs.BeginPhase("Implement")
	names := []string{"A", "B", "C", "D", "E", "F"}
	for i, name := range names {
		sink.Emit(observe.Event{
			Kind:       observe.KindTool,
			Name:       name,
			DurationMS: int64(i + 1),
			Status:     observe.ToolOK,
		})
	}
	obs.Abort()

	got := out.String()
	if strings.Contains(got, "tool  A  1ms  ok") {
		t.Errorf("Abort kept tool A beyond handful; got:\n%s", got)
	}
	for _, want := range []string{
		"tool  B  2ms  ok",
		"tool  C  3ms  ok",
		"tool  D  4ms  ok",
		"tool  E  5ms  ok",
		"tool  F  6ms  ok",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Abort banner missing %q; got:\n%s", want, got)
		}
	}
}
