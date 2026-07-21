package agent_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/maxBRT/ship-cli/internal/agent"
)

func TestNew_cursorReturnsPortUsingAgentOnPATH(t *testing.T) {
	bin, capture := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout:   `{"type":"result","subtype":"success"}` + "\n",
	})
	t.Setenv("PATH", filepath.Dir(bin)+string(os.PathListSeparator)+os.Getenv("PATH"))

	port, err := agent.New("cursor")
	if err != nil {
		t.Fatalf("New(cursor): %v", err)
	}

	err = port.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "implement",
		Workspace: t.TempDir(),
		Timeout:   time.Minute,
	})
	if err != nil {
		t.Fatalf("RunPhase: %v", err)
	}
	assertHasFlag(t, capture.args(t), "-p")
}

func TestNew_piReturnsPortUsingPiOnPATH(t *testing.T) {
	bin, capture := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout:   `{"type":"agent_end","messages":[]}` + "\n",
	})
	piBin := filepath.Join(filepath.Dir(bin), "pi")
	if err := os.Rename(bin, piBin); err != nil {
		t.Fatalf("rename fake to pi: %v", err)
	}
	t.Setenv("PATH", filepath.Dir(piBin)+string(os.PathListSeparator)+os.Getenv("PATH"))

	port, err := agent.New("pi")
	if err != nil {
		t.Fatalf("New(pi): %v", err)
	}

	err = port.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "implement",
		Workspace: t.TempDir(),
		Timeout:   time.Minute,
	})
	if err != nil {
		t.Fatalf("RunPhase: %v", err)
	}
	assertHasFlag(t, capture.args(t), "--no-session")
	assertHasFlag(t, capture.args(t), "--approve")
}

func TestNew_codexReturnsPortUsingCodexOnPATH(t *testing.T) {
	bin, capture := writeFakeAgent(t, fakeAgentConfig{
		exitCode: 0,
		stdout:   `{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}` + "\n",
	})
	codexBin := filepath.Join(filepath.Dir(bin), "codex")
	if err := os.Rename(bin, codexBin); err != nil {
		t.Fatalf("rename fake to codex: %v", err)
	}
	t.Setenv("PATH", filepath.Dir(codexBin)+string(os.PathListSeparator)+os.Getenv("PATH"))

	port, err := agent.New("codex")
	if err != nil {
		t.Fatalf("New(codex): %v", err)
	}

	err = port.RunPhase(context.Background(), agent.PhaseRequest{
		Prompt:    "implement",
		Workspace: t.TempDir(),
		Timeout:   time.Minute,
	})
	if err != nil {
		t.Fatalf("RunPhase: %v", err)
	}
	args := capture.args(t)
	assertHasFlag(t, args, "exec")
	assertHasFlag(t, args, "--ephemeral")
	assertHasFlag(t, args, "--json")
}

func TestNew_unimplementedKindErrors(t *testing.T) {
	_, err := agent.New("claude")
	if err == nil {
		t.Fatal("New(claude): want error until adapter lands")
	}
	if !strings.Contains(err.Error(), "claude") {
		t.Errorf("error should name kind; got %v", err)
	}
}

func TestNew_unknownKindErrors(t *testing.T) {
	_, err := agent.New("opencode")
	if err == nil {
		t.Fatal("New(opencode): want error")
	}
}

func TestRequireOnPATH_failsWhenDefaultBinaryMissing(t *testing.T) {
	empty := t.TempDir()
	t.Setenv("PATH", empty)

	err := agent.RequireOnPATH("cursor")
	if err == nil {
		t.Fatal("RequireOnPATH(cursor): want error when agent missing")
	}
	if !strings.Contains(err.Error(), "agent") {
		t.Errorf("error should mention default binary; got %v", err)
	}
}

func TestRequireOnPATH_failsWhenPiBinaryMissing(t *testing.T) {
	empty := t.TempDir()
	t.Setenv("PATH", empty)

	err := agent.RequireOnPATH("pi")
	if err == nil {
		t.Fatal("RequireOnPATH(pi): want error when pi missing")
	}
	if !strings.Contains(err.Error(), "pi") {
		t.Errorf("error should mention default binary; got %v", err)
	}
}

func TestRequireOnPATH_failsWhenCodexBinaryMissing(t *testing.T) {
	empty := t.TempDir()
	t.Setenv("PATH", empty)

	err := agent.RequireOnPATH("codex")
	if err == nil {
		t.Fatal("RequireOnPATH(codex): want error when codex missing")
	}
	if !strings.Contains(err.Error(), "codex") {
		t.Errorf("error should mention default binary; got %v", err)
	}
}

func TestRequireOnPATH_okWhenDefaultBinaryPresent(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{exitCode: 0})
	t.Setenv("PATH", filepath.Dir(bin)+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := agent.RequireOnPATH("cursor"); err != nil {
		t.Fatalf("RequireOnPATH(cursor): %v", err)
	}
}
