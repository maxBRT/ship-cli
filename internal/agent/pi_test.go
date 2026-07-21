package agent_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/maxBRT/ship-cli/internal/agent"
	"github.com/maxBRT/ship-cli/internal/observe"
)

func TestPi_RunPhase_succeedsWithHeadlessFlags(t *testing.T) {
	bin, capture := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout:   `{"type":"agent_end","messages":[]}` + "\n",
	})
	workspace := t.TempDir()
	p := agent.Pi{Bin: bin}

	err := p.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "implement the ticket",
		Workspace: workspace,
		Timeout:   time.Minute,
	})
	if err != nil {
		t.Fatalf("RunPhase: %v", err)
	}

	args := capture.args(t)
	assertHasFlag(t, args, "-p")
	assertHasFlagValue(t, args, "--mode", "json")
	assertHasFlag(t, args, "--no-session")
	assertHasFlag(t, args, "--approve")
	assertNoFlag(t, args, "--model")
	assertNoFlag(t, args, "--resume")
	assertNoFlag(t, args, "--continue")
	assertNoFlag(t, args, "--session")
	assertNoFlag(t, args, "-c")
	assertNoFlag(t, args, "-r")
	if got := capture.stdin(t); got != "implement the ticket" {
		t.Fatalf("stdin = %q, want phase prompt", got)
	}
	if got := capture.cwd(t); got != workspace {
		t.Fatalf("cwd = %q, want workspace %q", got, workspace)
	}
}

func TestPi_RunPhase_passesOptionalModel(t *testing.T) {
	bin, capture := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout:   `{"type":"agent_end","messages":[]}` + "\n",
	})
	p := agent.Pi{Bin: bin}

	err := p.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "review",
		Workspace: t.TempDir(),
		Model:     "openai/gpt-4o",
	})
	if err != nil {
		t.Fatalf("RunPhase: %v", err)
	}
	assertHasFlagValue(t, capture.args(t), "--model", "openai/gpt-4o")
}

func TestPi_RunPhase_surfacesStderrOnNonZeroExit(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 1,
		stderr:   "model not found: bogus",
	})
	p := agent.Pi{Bin: bin}

	err := p.RunPhase(context.Background(), agent.PhaseRequest{
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

func TestPi_RunPhase_timeoutKillsHungAgent(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout:   `{"type":"agent_end","messages":[]}` + "\n",
		sleep:    2 * time.Second,
	})
	p := agent.Pi{Bin: bin}

	start := time.Now()
	err := p.RunPhase(context.Background(), agent.PhaseRequest{
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

func TestPi_RunPhase_requiresTerminalAgentEnd(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout:   `{"type":"agent_start"}` + "\n",
	})
	p := agent.Pi{Bin: bin}

	err := p.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "review",
		Workspace: t.TempDir(),
	})
	if err == nil {
		t.Fatal("RunPhase: want error when agent_end is missing")
	}
	if !strings.Contains(err.Error(), "agent_end") {
		t.Fatalf("RunPhase error = %q, want mention of agent_end", err)
	}
}

func TestPi_RunPhase_emitsCuratedToolEventsFromJSON(t *testing.T) {
	// Worked example from Pi JSON mode: read then bash, second fails.
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout: strings.Join([]string{
			`{"type":"session","version":3,"id":"s1"}`,
			`{"type":"agent_start"}`,
			`{"type":"tool_execution_start","toolCallId":"c1","toolName":"read","args":{"path":"README.md"}}`,
			`{"type":"tool_execution_end","toolCallId":"c1","toolName":"read","result":"# Hi","isError":false}`,
			`{"type":"tool_execution_start","toolCallId":"c2","toolName":"bash","args":{"command":"false"}}`,
			`{"type":"tool_execution_end","toolCallId":"c2","toolName":"bash","result":"exit 1","isError":true}`,
			`{"type":"agent_end","messages":[]}`,
			"",
		}, "\n"),
	})
	sink := &recordingSink{}
	p := agent.Pi{Bin: bin}

	err := p.RunPhase(context.Background(), agent.PhaseRequest{
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
	if tools[0].Name != "read" || tools[0].Status != observe.ToolOK {
		t.Errorf("tool[0] = %+v, want read/ok", tools[0])
	}
	if tools[1].Name != "bash" || tools[1].Status != observe.ToolError {
		t.Errorf("tool[1] = %+v, want bash/error", tools[1])
	}
	for _, e := range sink.events {
		if e.Kind != observe.KindTool && e.Kind != observe.KindPhaseEnd {
			t.Errorf("unexpected event kind %q (no raw vendor lines)", e.Kind)
		}
	}
}

func TestPi_RunPhase_emitsPhaseEndTokenCountsWhenUsagePresent(t *testing.T) {
	// Pi assistant messages expose usage as input/output/cacheRead/cacheWrite.
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout: strings.Join([]string{
			`{"type":"tool_execution_end","toolCallId":"c1","toolName":"ls","result":"ok","isError":false}`,
			`{"type":"agent_end","messages":[{"role":"assistant","content":[{"type":"text","text":"done"}],"usage":{"input":100,"output":50,"cacheRead":10,"cacheWrite":2,"totalTokens":162}},{"role":"assistant","content":[{"type":"text","text":"more"}],"usage":{"input":20,"output":5,"cacheRead":0,"cacheWrite":0,"totalTokens":25}}]}`,
			"",
		}, "\n"),
	})
	sink := &recordingSink{}
	p := agent.Pi{Bin: bin}

	err := p.RunPhase(context.Background(), agent.PhaseRequest{
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
		t.Fatal("tokens = nil, want summed counts from assistant usage")
	}
	// 100+20 input, 50+5 output, 10+0 cache_read, 2+0 cache_write
	if end.Tokens.Input != 120 || end.Tokens.Output != 55 || end.Tokens.CacheRead != 10 || end.Tokens.CacheWrite != 2 {
		t.Errorf("tokens = %+v, want input=120 output=55 cache_read=10 cache_write=2", end.Tokens)
	}
}

func TestPi_RunPhase_missingUsageStillSucceedsWithoutTokenCounts(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout: strings.Join([]string{
			`{"type":"agent_end","messages":[{"role":"assistant","content":[{"type":"text","text":"ok"}],"stopReason":"stop"}]}`,
			"",
		}, "\n"),
	})
	sink := &recordingSink{}
	p := agent.Pi{Bin: bin}

	err := p.RunPhase(context.Background(), agent.PhaseRequest{
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

func TestPi_RunPhase_nilSinkEmitsNothing(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout: strings.Join([]string{
			`{"type":"tool_execution_end","toolCallId":"c1","toolName":"read","result":"ok","isError":false}`,
			`{"type":"agent_end","messages":[{"role":"assistant","usage":{"input":1,"output":1,"cacheRead":0,"cacheWrite":0}}]}`,
			"",
		}, "\n"),
	})
	p := agent.Pi{Bin: bin}

	err := p.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "implement",
		Workspace: t.TempDir(),
		Events:    nil,
	})
	if err != nil {
		t.Fatalf("RunPhase: %v", err)
	}
}
