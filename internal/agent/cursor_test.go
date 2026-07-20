package agent_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/maxBRT/ship-cli/internal/agent"
	"github.com/maxBRT/ship-cli/internal/observe"
)

func TestCursor_RunPhase_succeedsWithHeadlessFlags(t *testing.T) {
	bin, capture := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout:   `{"type":"result","subtype":"success","is_error":false}` + "\n",
	})
	workspace := t.TempDir()
	c := agent.Cursor{Bin: bin}

	err := c.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "implement the ticket",
		Workspace: workspace,
		Timeout:   time.Minute,
	})
	if err != nil {
		t.Fatalf("RunPhase: %v", err)
	}

	args := capture.args(t)
	assertHasFlag(t, args, "-p")
	assertHasFlag(t, args, "--trust")
	assertHasFlag(t, args, "--force")
	assertHasFlag(t, args, "--approve-mcps")
	assertHasFlagValue(t, args, "--workspace", workspace)
	assertHasFlagValue(t, args, "--output-format", "stream-json")
	assertNoFlag(t, args, "--model")
	assertNoFlag(t, args, "--resume")
	assertNoFlag(t, args, "--continue")
	assertNoFlag(t, args, "--worktree")
	if got := capture.stdin(t); got != "implement the ticket" {
		t.Fatalf("stdin = %q, want phase prompt", got)
	}
}

func TestCursor_RunPhase_passesOptionalModel(t *testing.T) {
	bin, capture := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout:   `{"type":"result","subtype":"success"}` + "\n",
	})
	c := agent.Cursor{Bin: bin}

	err := c.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "review",
		Workspace: t.TempDir(),
		Model:     "composer-2.5",
	})
	if err != nil {
		t.Fatalf("RunPhase: %v", err)
	}
	assertHasFlagValue(t, capture.args(t), "--model", "composer-2.5")
}

func TestCursor_RunPhase_surfacesStderrOnNonZeroExit(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 1,
		stderr:   "Cannot use this model: bogus",
	})
	c := agent.Cursor{Bin: bin}

	err := c.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "implement",
		Workspace: t.TempDir(),
	})
	if err == nil {
		t.Fatal("RunPhase: want error on non-zero exit")
	}
	if !strings.Contains(err.Error(), "Cannot use this model: bogus") {
		t.Fatalf("RunPhase error = %q, want stderr contents", err)
	}
}

func TestCursor_RunPhase_timeoutKillsHungAgent(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout:   `{"type":"result","subtype":"success"}` + "\n",
		sleep:    2 * time.Second,
	})
	c := agent.Cursor{Bin: bin}

	start := time.Now()
	err := c.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "hang",
		Workspace: t.TempDir(),
		Timeout:   200 * time.Millisecond,
	})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("RunPhase: want timeout error")
	}
	if !strings.Contains(err.Error(), "deadline exceeded") && !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("RunPhase error = %q, want context timeout", err)
	}
	if elapsed > time.Second {
		t.Fatalf("RunPhase took %v, want kill near timeout", elapsed)
	}
}

func TestCursor_RunPhase_requiresStreamJSONSuccessSubtype(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout:   `{"type":"result","subtype":"error"}` + "\n",
	})
	c := agent.Cursor{Bin: bin}

	err := c.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "review",
		Workspace: t.TempDir(),
	})
	if err == nil {
		t.Fatal("RunPhase: want error when subtype is not success")
	}
	if !strings.Contains(err.Error(), "success") {
		t.Fatalf("RunPhase error = %q, want mention of success subtype", err)
	}
}

func TestCursor_RunPhase_acceptsTerminalSuccessAfterStreamEvents(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout: strings.Join([]string{
			`{"type":"system","subtype":"init"}`,
			`{"type":"assistant","message":{"content":[{"type":"text","text":"ok"}]}}`,
			`{"type":"result","subtype":"success","is_error":false}`,
			"",
		}, "\n"),
	})
	c := agent.Cursor{Bin: bin}

	err := c.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "final",
		Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("RunPhase: %v", err)
	}
}

func TestCursor_RunPhase_emitsCuratedToolEventsFromStreamJSON(t *testing.T) {
	// Worked example from Cursor stream-json docs: Read then Write, both succeed.
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout: strings.Join([]string{
			`{"type":"system","subtype":"init"}`,
			`{"type":"tool_call","subtype":"started","call_id":"c1","tool_call":{"readToolCall":{"args":{"path":"README.md"}}}}`,
			`{"type":"tool_call","subtype":"completed","call_id":"c1","duration_ms":42,"tool_call":{"readToolCall":{"args":{"path":"README.md"},"result":{"success":{"content":"# Hi"}}}}}`,
			`{"type":"tool_call","subtype":"started","call_id":"c2","tool_call":{"writeToolCall":{"args":{"path":"out.txt"}}}}`,
			`{"type":"tool_call","subtype":"completed","call_id":"c2","duration_ms":7,"tool_call":{"writeToolCall":{"args":{"path":"out.txt"},"result":{"success":{"path":"/tmp/out.txt"}}}}}`,
			`{"type":"result","subtype":"success","is_error":false,"duration_ms":100}`,
			"",
		}, "\n"),
	})
	sink := &recordingSink{}
	c := agent.Cursor{Bin: bin}

	err := c.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "implement",
		Workspace: t.TempDir(),
		Events:    sink,
	})
	if err != nil {
		t.Fatalf("RunPhase: %v", err)
	}

	tools := sink.tools()
	if len(tools) != 2 {
		t.Fatalf("tool events = %d, want 2; events=%v", len(tools), sink.events)
	}
	if tools[0].Name != "Read" || tools[0].DurationMS != 42 || tools[0].Status != observe.ToolOK {
		t.Errorf("tool[0] = %+v, want Read/42/ok", tools[0])
	}
	if tools[1].Name != "Write" || tools[1].DurationMS != 7 || tools[1].Status != observe.ToolOK {
		t.Errorf("tool[1] = %+v, want Write/7/ok", tools[1])
	}
}

func TestCursor_RunPhase_emitsPhaseEndTokenCountsWhenUsagePresent(t *testing.T) {
	// Cursor stream-json exposes camelCase usage on the terminal result
	// (inputTokens / outputTokens / cacheReadTokens / cacheWriteTokens).
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout: strings.Join([]string{
			`{"type":"tool_call","subtype":"completed","call_id":"c1","duration_ms":10,"tool_call":{"shellToolCall":{"args":{"command":"true"},"result":{"success":{}}}}}`,
			`{"type":"result","subtype":"success","is_error":false,"duration_ms":2500,"usage":{"inputTokens":120,"outputTokens":45,"cacheReadTokens":10,"cacheWriteTokens":2}}`,
			"",
		}, "\n"),
	})
	sink := &recordingSink{}
	c := agent.Cursor{Bin: bin}

	err := c.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "review",
		Workspace: t.TempDir(),
		Events:    sink,
	})
	if err != nil {
		t.Fatalf("RunPhase: %v", err)
	}

	end := sink.phaseEnd()
	if end == nil {
		t.Fatalf("want phase_end event, got %v", sink.events)
	}
	if end.Outcome != observe.OutcomeSuccess {
		t.Errorf("outcome = %q, want %q", end.Outcome, observe.OutcomeSuccess)
	}
	if end.DurationMS != 2500 {
		t.Errorf("duration_ms = %d, want 2500", end.DurationMS)
	}
	if end.ToolCount != 1 {
		t.Errorf("tool_count = %d, want 1", end.ToolCount)
	}
	if end.Tokens == nil {
		t.Fatal("tokens = nil, want counts from usage")
	}
	if end.Tokens.Input != 120 || end.Tokens.Output != 45 || end.Tokens.CacheRead != 10 || end.Tokens.CacheWrite != 2 {
		t.Errorf("tokens = %+v, want input=120 output=45 cache_read=10 cache_write=2", end.Tokens)
	}
}

func TestCursor_RunPhase_missingUsageStillSucceedsWithoutTokenCounts(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout: strings.Join([]string{
			`{"type":"result","subtype":"success","is_error":false,"duration_ms":99}`,
			"",
		}, "\n"),
	})
	sink := &recordingSink{}
	c := agent.Cursor{Bin: bin}

	err := c.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "final",
		Workspace: t.TempDir(),
		Events:    sink,
	})
	if err != nil {
		t.Fatalf("RunPhase: %v (missing usage must not fail the Phase)", err)
	}
	end := sink.phaseEnd()
	if end == nil {
		t.Fatalf("want phase_end event, got %v", sink.events)
	}
	if end.Tokens != nil {
		t.Errorf("tokens = %+v, want nil when usage absent", end.Tokens)
	}
	if end.Outcome != observe.OutcomeSuccess || end.DurationMS != 99 {
		t.Errorf("phase_end = %+v, want success/99ms", end)
	}
}

func TestCursor_RunPhase_emitsToolErrorStatusFromFailedToolResult(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout: strings.Join([]string{
			`{"type":"tool_call","subtype":"completed","call_id":"c1","duration_ms":3,"tool_call":{"readToolCall":{"args":{"path":"missing"},"result":{"error":{"message":"not found"}}}}}`,
			`{"type":"result","subtype":"success","is_error":false}`,
			"",
		}, "\n"),
	})
	sink := &recordingSink{}
	c := agent.Cursor{Bin: bin}

	err := c.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "implement",
		Workspace: t.TempDir(),
		Events:    sink,
	})
	if err != nil {
		t.Fatalf("RunPhase: %v", err)
	}
	tools := sink.tools()
	if len(tools) != 1 {
		t.Fatalf("tool events = %d, want 1", len(tools))
	}
	if tools[0].Name != "Read" || tools[0].Status != observe.ToolError || tools[0].DurationMS != 3 {
		t.Errorf("tool = %+v, want Read/error/3", tools[0])
	}
}

type fakeAgentConfig struct {
	exitCode int
	stdout   string
	stderr   string
	sleep    time.Duration
}

type fakeCapture struct {
	dir string
}

func (c fakeCapture) args(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(c.dir, "args"))
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	s := strings.TrimSuffix(string(b), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func (c fakeCapture) stdin(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(c.dir, "stdin"))
	if err != nil {
		t.Fatalf("read stdin: %v", err)
	}
	return string(b)
}

func writeFakeAgent(t *testing.T, cfg fakeAgentConfig) (string, fakeCapture) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "agent")

	if err := os.WriteFile(filepath.Join(dir, "stdout"), []byte(cfg.stdout), 0o644); err != nil {
		t.Fatalf("write stdout fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stderr"), []byte(cfg.stderr), 0o644); err != nil {
		t.Fatalf("write stderr fixture: %v", err)
	}

	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString("dir=$(dirname \"$0\")\n")
	b.WriteString("printf '%s\\n' \"$@\" > \"$dir/args\"\n")
	b.WriteString("cat > \"$dir/stdin\"\n")
	if cfg.sleep > 0 {
		// exec so CommandContext kill targets the sleeper, not a parent shell
		fmt.Fprintf(&b, "exec sleep %g\n", cfg.sleep.Seconds())
	}
	b.WriteString("cat \"$dir/stderr\" >&2\n")
	b.WriteString("cat \"$dir/stdout\"\n")
	fmt.Fprintf(&b, "exit %d\n", cfg.exitCode)

	if err := os.WriteFile(bin, []byte(b.String()), 0o755); err != nil {
		t.Fatalf("write fake agent: %v", err)
	}
	return bin, fakeCapture{dir: dir}
}

func assertHasFlag(t *testing.T, args []string, flag string) {
	t.Helper()
	for _, a := range args {
		if a == flag {
			return
		}
	}
	t.Fatalf("args %v missing %q", args, flag)
}

func assertHasFlagValue(t *testing.T, args []string, flag, want string) {
	t.Helper()
	for i, a := range args {
		if a == flag {
			if i+1 >= len(args) {
				t.Fatalf("args %v: %q has no value", args, flag)
			}
			if args[i+1] != want {
				t.Fatalf("args %v: %q = %q, want %q", args, flag, args[i+1], want)
			}
			return
		}
	}
	t.Fatalf("args %v missing %q", args, flag)
}

func assertNoFlag(t *testing.T, args []string, flag string) {
	t.Helper()
	for _, a := range args {
		if a == flag || strings.HasPrefix(a, flag+"=") {
			t.Fatalf("args %v must not include %q", args, flag)
		}
	}
}

type recordingSink struct {
	events []observe.Event
}

func (s *recordingSink) Emit(e observe.Event) {
	s.events = append(s.events, e)
}

func (s *recordingSink) tools() []observe.Event {
	var out []observe.Event
	for _, e := range s.events {
		if e.Kind == observe.KindTool {
			out = append(out, e)
		}
	}
	return out
}

func (s *recordingSink) phaseEnd() *observe.Event {
	for i := len(s.events) - 1; i >= 0; i-- {
		if s.events[i].Kind == observe.KindPhaseEnd {
			e := s.events[i]
			return &e
		}
	}
	return nil
}
