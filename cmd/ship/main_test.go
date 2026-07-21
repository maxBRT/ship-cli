package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maxBRT/ship-cli/internal/update"
)

type fakeUpdater struct {
	result update.Result
	err    error
	called bool
}

func (f *fakeUpdater) Update(context.Context) (update.Result, error) {
	f.called = true
	return f.result, f.err
}

func TestMain_updateRoutesToUpdaterNotTicketRun(t *testing.T) {
	up := &fakeUpdater{result: update.Result{
		AlreadyCurrent: true,
		OldVersion:     "v1.0.0",
		NewVersion:     "v1.0.0",
	}}
	var stdout, stderr bytes.Buffer
	code := MainWith([]string{"update"}, &stdout, &stderr, t.TempDir(), up)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !up.called {
		t.Fatal("Updater.Update was not called; ship update must not start a Ticket Run")
	}
}

func TestMain_updateAlreadyCurrentPrintsSuccess(t *testing.T) {
	up := &fakeUpdater{result: update.Result{
		AlreadyCurrent: true,
		OldVersion:     "v1.2.3",
		NewVersion:     "v1.2.3",
	}}
	var stdout, stderr bytes.Buffer
	code := MainWith([]string{"update"}, &stdout, &stderr, t.TempDir(), up)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "up to date") {
		t.Errorf("stdout missing up-to-date message; got %q", got)
	}
	if !strings.Contains(got, "v1.2.3") {
		t.Errorf("stdout missing version; got %q", got)
	}
	if stderr.Len() != 0 {
		t.Errorf("want empty stderr, got %q", stderr.String())
	}
}

func TestMain_updateNewerReleasePrintsOldAndNewVersions(t *testing.T) {
	up := &fakeUpdater{result: update.Result{
		OldVersion: "v1.0.0",
		NewVersion: "v1.2.3",
	}}
	var stdout, stderr bytes.Buffer
	code := MainWith([]string{"update"}, &stdout, &stderr, t.TempDir(), up)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "v1.0.0") || !strings.Contains(got, "v1.2.3") {
		t.Errorf("stdout should print old and new versions; got %q", got)
	}
	if stderr.Len() != 0 {
		t.Errorf("want empty stderr, got %q", stderr.String())
	}
}

func TestMain_updateFailureLeavesNonZeroAndStderr(t *testing.T) {
	up := &fakeUpdater{err: errString("checksum mismatch")}
	var stdout, stderr bytes.Buffer
	code := MainWith([]string{"update"}, &stdout, &stderr, t.TempDir(), up)
	if code == 0 {
		t.Fatal("exit = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "checksum mismatch") {
		t.Errorf("stderr should carry updater error; got %q", stderr.String())
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestMain_versionPrintsEmbeddedVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Main([]string{"--version"}, &stdout, &stderr, t.TempDir())
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("want empty stderr, got %q", stderr.String())
	}
	// Non-release builds report the known default "dev".
	got := strings.TrimSpace(stdout.String())
	if got != "dev" {
		t.Errorf("version = %q, want %q", got, "dev")
	}
}

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
	for _, term := range []string{"update", "version"} {
		if !strings.Contains(out, term) {
			t.Errorf("help missing %q; got:\n%s", term, out)
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
