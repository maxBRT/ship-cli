package agent_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/maxBRT/ship-cli/internal/agent"
	"github.com/maxBRT/ship-cli/internal/observe"
)

func TestClaude_RunPhase_succeedsWithHeadlessFlags(t *testing.T) {
	bin, capture := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout:   `{"type":"result","subtype":"success","is_error":false}` + "\n",
	})
	workspace := t.TempDir()
	c := agent.Claude{Bin: bin}

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
	assertHasFlag(t, args, "--dangerously-skip-permissions")
	assertHasFlag(t, args, "--no-session-persistence")
	assertHasFlagValue(t, args, "--output-format", "stream-json")
	assertHasFlag(t, args, "--verbose")
	assertNoFlag(t, args, "--model")
	assertNoFlag(t, args, "--bare")
	assertNoFlag(t, args, "--worktree")
	assertNoFlag(t, args, "--resume")
	assertNoFlag(t, args, "--continue")
	assertNoFlag(t, args, "-c")
	assertNoFlag(t, args, "-r")
	assertNoFlag(t, args, "-w")
	if got := capture.stdin(t); got != "implement the ticket" {
		t.Fatalf("stdin = %q, want phase prompt", got)
	}
	if got := capture.cwd(t); got != workspace {
		t.Fatalf("cwd = %q, want workspace %q", got, workspace)
	}
}

func TestClaude_RunPhase_passesOptionalModel(t *testing.T) {
	bin, capture := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout:   `{"type":"result","subtype":"success"}` + "\n",
	})
	c := agent.Claude{Bin: bin}

	err := c.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "review",
		Workspace: t.TempDir(),
		Model:     "sonnet",
	})
	if err != nil {
		t.Fatalf("RunPhase: %v", err)
	}
	assertHasFlagValue(t, capture.args(t), "--model", "sonnet")
}

func TestClaude_RunPhase_surfacesStderrOnNonZeroExit(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 1,
		stderr:   "model not found: bogus",
	})
	c := agent.Claude{Bin: bin}

	err := c.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "implement",
		Workspace: t.TempDir(),
	})
	if err == nil {
		t.Fatal("RunPhase: want error on non-zero exit")
	}
	if !strings.Contains(err.Error(), "model not found: bogus") {
		t.Fatalf("RunPhase error = %q, want stderr contents", err)
	}
}

func TestClaude_RunPhase_timeoutKillsHungAgent(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout:   `{"type":"result","subtype":"success"}` + "\n",
		sleep:    2 * time.Second,
	})
	c := agent.Claude{Bin: bin}

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

func TestClaude_RunPhase_requiresStreamJSONSuccessSubtype(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout:   `{"type":"result","subtype":"error"}` + "\n",
	})
	c := agent.Claude{Bin: bin}

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

func TestClaude_RunPhase_missingResultFails(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout:   `{"type":"system","subtype":"init"}` + "\n",
	})
	c := agent.Claude{Bin: bin}

	err := c.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "review",
		Workspace: t.TempDir(),
	})
	if err == nil {
		t.Fatal("RunPhase: want error when result is missing")
	}
	if !strings.Contains(err.Error(), "result") {
		t.Fatalf("RunPhase error = %q, want mention of result", err)
	}
}

func TestClaude_RunPhase_emitsCuratedToolEventsFromStreamJSON(t *testing.T) {
	// Worked example from Claude Code stream-json: Bash then Read; second tool_result errors.
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout: strings.Join([]string{
			`{"type":"system","subtype":"init","tools":["Bash","Read"]}`,
			`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"ls"}}]}}`,
			`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"README.md"}]}}`,
			`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_2","name":"Read","input":{"path":"missing"}}]}}`,
			`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_2","content":"not found","is_error":true}]}}`,
			`{"type":"result","subtype":"success","is_error":false,"duration_ms":100}`,
			"",
		}, "\n"),
	})
	sink := &recordingSink{}
	c := agent.Claude{Bin: bin}

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
	if tools[0].Name != "Bash" || tools[0].Status != observe.ToolOK {
		t.Errorf("tool[0] = %+v, want Bash/ok", tools[0])
	}
	if tools[1].Name != "Read" || tools[1].Status != observe.ToolError {
		t.Errorf("tool[1] = %+v, want Read/error", tools[1])
	}
	for _, e := range sink.events {
		if e.Kind != observe.KindTool && e.Kind != observe.KindPhaseEnd {
			t.Errorf("unexpected event kind %q (no raw vendor lines)", e.Kind)
		}
	}
}

func TestClaude_RunPhase_emitsPhaseEndTokenCountsWhenUsagePresent(t *testing.T) {
	// Claude result usage uses snake_case input_tokens / output_tokens /
	// cache_read_input_tokens / cache_creation_input_tokens.
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout: strings.Join([]string{
			`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"true"}}]}}`,
			`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"ok"}]}}`,
			`{"type":"result","subtype":"success","is_error":false,"duration_ms":2500,"usage":{"input_tokens":120,"output_tokens":45,"cache_read_input_tokens":10,"cache_creation_input_tokens":2}}`,
			"",
		}, "\n"),
	})
	sink := &recordingSink{}
	c := agent.Claude{Bin: bin}

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

func TestClaude_RunPhase_missingUsageStillSucceedsWithoutTokenCounts(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout: strings.Join([]string{
			`{"type":"result","subtype":"success","is_error":false,"duration_ms":99}`,
			"",
		}, "\n"),
	})
	sink := &recordingSink{}
	c := agent.Claude{Bin: bin}

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

func TestClaude_RunPhase_nilSinkEmitsNothing(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout: strings.Join([]string{
			`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"Read","input":{"path":"a"}}]}}`,
			`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"ok"}]}}`,
			`{"type":"result","subtype":"success","usage":{"input_tokens":1,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}`,
			"",
		}, "\n"),
	})
	c := agent.Claude{Bin: bin}

	err := c.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "implement",
		Workspace: t.TempDir(),
		Events:    nil,
	})
	if err != nil {
		t.Fatalf("RunPhase: %v", err)
	}
}
