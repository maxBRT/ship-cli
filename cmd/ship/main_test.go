package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain_helpDescribesAgentAsKind(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Main([]string{"--help"}, &stdout, &stderr, t.TempDir())
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Agent kind") {
		t.Errorf("help should describe agent as Agent kind; got:\n%s", out)
	}
	if strings.Contains(out, "Agent binary") {
		t.Errorf("help should not describe agent as binary; got:\n%s", out)
	}
}

func TestMain_helpDocumentsDomainLanguage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Main([]string{"--help"}, &stdout, &stderr, t.TempDir())
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr.String())
	}
	out := stdout.String() + stderr.String()
	for _, term := range []string{"Run", "Ticket", "Iteration", "Phase", "Agent"} {
		if !strings.Contains(out, term) {
			t.Errorf("help missing domain term %q; got:\n%s", term, out)
		}
	}
}

func TestMain_invalidFlagExitsNonZero(t *testing.T) {
	dir := t.TempDir()
	writeShipYAML(t, dir, `branch: ""
feature: ""
agent: cursor
model: ""
max_iterations: 10
timeout: 20m
`)
	var stdout, stderr bytes.Buffer
	code := Main([]string{"--not-a-real-flag"}, &stdout, &stderr, dir)
	if code == 0 {
		t.Fatal("exit = 0, want non-zero")
	}
	if stderr.Len() == 0 {
		t.Fatal("want error message on stderr")
	}
}

func TestMain_invalidTimeoutExitsNonZero(t *testing.T) {
	dir := t.TempDir()
	writeShipYAML(t, dir, `branch: ""
feature: ""
agent: cursor
model: ""
max_iterations: 10
timeout: 20m
`)
	var stdout, stderr bytes.Buffer
	code := Main([]string{"--timeout", "nope"}, &stdout, &stderr, dir)
	if code == 0 {
		t.Fatal("exit = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "timeout") && !strings.Contains(stderr.String(), "duration") {
		t.Errorf("stderr should mention timeout/duration; got %q", stderr.String())
	}
}

func TestMain_missingAgentBinaryExitsBeforeRun(t *testing.T) {
	dir := t.TempDir()
	writeShipYAML(t, dir, `branch: ""
feature: ""
agent: cursor
model: ""
max_iterations: 10
timeout: 20m
`)
	t.Setenv("PATH", t.TempDir())

	var stdout, stderr bytes.Buffer
	code := Main(nil, &stdout, &stderr, dir)
	if code == 0 {
		t.Fatal("exit = 0, want non-zero when agent binary missing")
	}
	if !strings.Contains(stderr.String(), "agent") {
		t.Errorf("stderr should mention missing agent binary; got %q", stderr.String())
	}
}

func writeShipYAML(t *testing.T, dir, content string) {
	t.Helper()
	path := filepath.Join(dir, ".ship", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
