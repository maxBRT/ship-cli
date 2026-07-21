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

func TestNew_unimplementedKindErrors(t *testing.T) {
	_, err := agent.New("pi")
	if err == nil {
		t.Fatal("New(pi): want error until adapter lands")
	}
	if !strings.Contains(err.Error(), "pi") {
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

func TestRequireOnPATH_okWhenDefaultBinaryPresent(t *testing.T) {
	bin, _ := writeFakeAgent(t, fakeAgentConfig{exitCode: 0})
	t.Setenv("PATH", filepath.Dir(bin)+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := agent.RequireOnPATH("cursor"); err != nil {
		t.Fatalf("RequireOnPATH(cursor): %v", err)
	}
}
