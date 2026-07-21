package agent_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/maxBRT/ship-cli/internal/agent"
	"github.com/maxBRT/ship-cli/internal/observe"
)

func TestCodex_RunPhase_succeedsWithHeadlessFlags(t *testing.T) {
	bin, capture := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout:   `{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}` + "\n",
	})
	workspace := t.TempDir()
	c := agent.Codex{Bin: bin}

	err := c.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "implement the ticket",
		Workspace: workspace,
		Timeout:   time.Minute,
	})
	if err != nil {
		t.Fatalf("RunPhase: %v", err)
	}

	args := capture.args(t)
	assertHasFlag(t, args, "exec")
	assertHasFlag(t, args, "--ephemeral")
	assertHasFlag(t, args, "--json")
	assertHasFlag(t, args, "--dangerously-bypass-approvals-and-sandbox")
	assertHasFlagValue(t, args, "--cd", workspace)
	assertNoFlag(t, args, "--model")
	assertNoFlag(t, args, "resume")
	// Full prompt from stdin: trailing "-" sentinel, never resume.
	if len(args) == 0 || args[len(args)-1] != "-" {
		t.Fatalf("args %v: want trailing \"-\" for full prompt from stdin", args)
	}
	if got := capture.stdin(t); got != "implement the ticket" {
		t.Fatalf("stdin = %q, want phase prompt", got)
	}
	if got := capture.cwd(t); got != workspace {
		t.Fatalf("cwd = %q, want workspace %q", got, workspace)
	}
}

func TestCodex_RunPhase_passesOptionalModel(t *testing.T) {
	bin, capture := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout:   `{"type":"turn.completed"}` + "\n",
	})
	c := agent.Codex{Bin: bin}

	err := c.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "review",
		Workspace: t.TempDir(),
		Model:     "gpt-5.4",
	})
	if err != nil {
		t.Fatalf("RunPhase: %v", err)
	}
	assertHasFlagValue(t, capture.args(t), "--model", "gpt-5.4")
}

func TestCodex_RunPhase_surfacesStderrOnNonZeroExit(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 1,
		stderr:   "model not found: bogus",
	})
	c := agent.Codex{Bin: bin}

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

func TestCodex_RunPhase_timeoutKillsHungAgent(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout:   `{"type":"turn.completed"}` + "\n",
		sleep:    2 * time.Second,
	})
	c := agent.Codex{Bin: bin}

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

func TestCodex_RunPhase_requiresTerminalTurnCompleted(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout:   `{"type":"turn.started"}` + "\n",
	})
	c := agent.Codex{Bin: bin}

	err := c.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "review",
		Workspace: t.TempDir(),
	})
	if err == nil {
		t.Fatal("RunPhase: want error when turn.completed is missing")
	}
	if !strings.Contains(err.Error(), "turn.completed") {
		t.Fatalf("RunPhase error = %q, want mention of turn.completed", err)
	}
}

func TestCodex_RunPhase_failsOnTurnFailed(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout: strings.Join([]string{
			`{"type":"turn.started"}`,
			`{"type":"turn.failed"}`,
			"",
		}, "\n"),
	})
	c := agent.Codex{Bin: bin}

	err := c.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "implement",
		Workspace: t.TempDir(),
	})
	if err == nil {
		t.Fatal("RunPhase: want error on turn.failed")
	}
	if !strings.Contains(err.Error(), "turn.failed") {
		t.Fatalf("RunPhase error = %q, want mention of turn.failed", err)
	}
}

func TestCodex_RunPhase_emitsCuratedToolEventsFromJSON(t *testing.T) {
	// Worked example from Codex --json docs: command_execution then agent_message.
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout: strings.Join([]string{
			`{"type":"thread.started","thread_id":"t1"}`,
			`{"type":"turn.started"}`,
			`{"type":"item.started","item":{"id":"item_1","type":"command_execution","command":"bash -lc ls","status":"in_progress"}}`,
			`{"type":"item.completed","item":{"id":"item_1","type":"command_execution","command":"bash -lc ls","status":"completed"}}`,
			`{"type":"item.completed","item":{"id":"item_2","type":"file_change","status":"failed"}}`,
			`{"type":"item.completed","item":{"id":"item_3","type":"agent_message","text":"done"}}`,
			`{"type":"turn.completed","usage":{"input_tokens":10,"output_tokens":2,"cached_input_tokens":0,"reasoning_output_tokens":0}}`,
			"",
		}, "\n"),
	})
	sink := &recordingSink{}
	c := agent.Codex{Bin: bin}

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
		t.Fatalf("tool events = %d, want 2 (agent_message excluded); events=%v", len(tools), sink.events)
	}
	if tools[0].Name != "command_execution" || tools[0].Status != observe.ToolOK {
		t.Errorf("tool[0] = %+v, want command_execution/ok", tools[0])
	}
	if tools[1].Name != "file_change" || tools[1].Status != observe.ToolError {
		t.Errorf("tool[1] = %+v, want file_change/error", tools[1])
	}
	for _, e := range sink.events {
		if e.Kind != observe.KindTool && e.Kind != observe.KindPhaseEnd {
			t.Errorf("unexpected event kind %q (no raw vendor lines)", e.Kind)
		}
	}
}

func TestCodex_RunPhase_emitsPhaseEndTokenCountsWhenUsagePresent(t *testing.T) {
	// Codex turn.completed exposes input_tokens / output_tokens / cached_input_tokens.
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout: strings.Join([]string{
			`{"type":"item.completed","item":{"id":"item_1","type":"command_execution","status":"completed"}}`,
			`{"type":"turn.completed","usage":{"input_tokens":24763,"cached_input_tokens":24448,"output_tokens":122,"reasoning_output_tokens":5}}`,
			"",
		}, "\n"),
	})
	sink := &recordingSink{}
	c := agent.Codex{Bin: bin}

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
	if end.ToolCount != 1 {
		t.Errorf("tool_count = %d, want 1", end.ToolCount)
	}
	if end.Tokens == nil {
		t.Fatal("tokens = nil, want counts from usage")
	}
	// output includes reasoning_output_tokens (122+5); cache_read from cached_input_tokens
	if end.Tokens.Input != 24763 || end.Tokens.Output != 127 || end.Tokens.CacheRead != 24448 {
		t.Errorf("tokens = %+v, want input=24763 output=127 cache_read=24448", end.Tokens)
	}
}

func TestCodex_RunPhase_missingUsageStillSucceedsWithoutTokenCounts(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout: strings.Join([]string{
			`{"type":"turn.completed"}`,
			"",
		}, "\n"),
	})
	sink := &recordingSink{}
	c := agent.Codex{Bin: bin}

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
	if end.Outcome != observe.OutcomeSuccess {
		t.Errorf("phase_end = %+v, want success", end)
	}
}

func TestCodex_RunPhase_nilSinkEmitsNothing(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout: strings.Join([]string{
			`{"type":"item.completed","item":{"id":"item_1","type":"command_execution","status":"completed"}}`,
			`{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1,"cached_input_tokens":0,"reasoning_output_tokens":0}}`,
			"",
		}, "\n"),
	})
	c := agent.Codex{Bin: bin}

	err := c.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "implement",
		Workspace: t.TempDir(),
		Events:    nil,
	})
	if err != nil {
		t.Fatalf("RunPhase: %v", err)
	}
}
