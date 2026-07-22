package observe_test

import (
	"strings"
	"testing"

	"github.com/maxBRT/ship-cli/internal/observe"
)

func TestObserver_EndPhase_dumpsOneToolsAndTokensLine(t *testing.T) {
	// Terminal dump is one dense strip: tool total plus compact in/out tokens.
	// Per-tool detail and cache fields stay in the Phase JSON log only.
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
			Input:      18420,
			Output:     931,
			CacheRead:  10,
			CacheWrite: 2,
		},
	})
	obs.EndPhase()

	got := out.String()
	if !strings.Contains(got, "tools") || !strings.Contains(got, "2") {
		t.Errorf("dump missing tools count; got %q", got)
	}
	if !strings.Contains(got, "in") || !strings.Contains(got, "18.4k") {
		t.Errorf("dump missing compact input tokens; got %q", got)
	}
	if !strings.Contains(got, "out") || !strings.Contains(got, "931") {
		t.Errorf("dump missing output tokens; got %q", got)
	}
	if strings.Contains(got, "cache_read") || strings.Contains(got, "cache_write") {
		t.Errorf("dense strip must omit cache fields; got %q", got)
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
	want := "  tools  1\n"
	if got != want {
		t.Errorf("dump = %q, want %q", got, want)
	}
	if strings.Contains(got, "tokens") || strings.Contains(got, "in ") {
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
	if got := out.String(); got != "  tools  1\n" {
		t.Errorf("after EndPhase dump = %q, want tools  1\\n", got)
	}
}

func TestObserver_Abort_keepsLastHandfulOfToolLines(t *testing.T) {
	// Abort banner keeps only the trailing handful of tool lines and stays loud.
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
	if !strings.Contains(got, "Abort") {
		t.Errorf("Abort banner missing Abort; got:\n%s", got)
	}
	if !strings.Contains(got, "✗") {
		t.Errorf("Abort banner missing red failure mark; got:\n%s", got)
	}
	if strings.Contains(got, " A ") && strings.Contains(got, "1ms") {
		lines := strings.Split(got, "\n")
		for _, line := range lines {
			if strings.Contains(line, " A ") && strings.Contains(line, "1ms") {
				t.Errorf("Abort kept tool A beyond handful; got:\n%s", got)
				break
			}
		}
	}
	for _, want := range []string{"B", "C", "D", "E", "F"} {
		if !strings.Contains(got, want) {
			t.Errorf("Abort banner missing tool %q; got:\n%s", want, got)
		}
	}
}

func TestObserver_Abort_plainWhenColorOff(t *testing.T) {
	var out strings.Builder
	obs := observe.New(t.TempDir(), &out)
	obs.Color = false
	sink := obs.BeginPhase("Implement")
	sink.Emit(observe.Event{
		Kind:       observe.KindTool,
		Name:       "Read",
		DurationMS: 42,
		Status:     observe.ToolOK,
	})
	obs.Abort()

	got := out.String()
	if strings.Contains(got, "\033[") {
		t.Errorf("plain Abort must not emit ANSI; got %q", got)
	}
	for _, want := range []string{"Abort", "✗", "report", "phase log", "Read", "42ms", "ok"} {
		if !strings.Contains(got, want) {
			t.Errorf("Abort banner missing %q; got:\n%s", want, got)
		}
	}
}
