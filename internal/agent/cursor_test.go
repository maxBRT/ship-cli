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
