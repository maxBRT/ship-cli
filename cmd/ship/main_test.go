package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain_helpDocumentsDomainLanguage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Main([]string{"--help"}, func(string) string { return "" }, &stdout, &stderr, t.TempDir())
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
	var stdout, stderr bytes.Buffer
	code := Main([]string{"--not-a-real-flag"}, func(string) string { return "" }, &stdout, &stderr, t.TempDir())
	if code == 0 {
		t.Fatal("exit = 0, want non-zero")
	}
	if stderr.Len() == 0 {
		t.Fatal("want error message on stderr")
	}
}

func TestMain_invalidTimeoutExitsNonZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Main([]string{"--timeout", "nope"}, func(string) string { return "" }, &stdout, &stderr, t.TempDir())
	if code == 0 {
		t.Fatal("exit = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "timeout") && !strings.Contains(stderr.String(), "duration") {
		t.Errorf("stderr should mention timeout/duration; got %q", stderr.String())
	}
}

func TestMain_ensuresNamedBranchInCheckout(t *testing.T) {
	dir := initTempRepo(t)
	var stdout, stderr bytes.Buffer
	code := Main([]string{"--branch", "ship/from-cli"}, os.Getenv, &stdout, &stderr, dir)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr.String())
	}
	branch := runGit(t, dir, "branch", "--show-current")
	if branch != "ship/from-cli" {
		t.Fatalf("current branch = %q, want ship/from-cli", branch)
	}
}

func TestMain_emptyBranchGeneratesShipPrefixed(t *testing.T) {
	dir := initTempRepo(t)
	var stdout, stderr bytes.Buffer
	code := Main(nil, func(string) string { return "" }, &stdout, &stderr, dir)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr.String())
	}
	branch := runGit(t, dir, "branch", "--show-current")
	if !strings.HasPrefix(branch, "ship/") || branch == "ship/" {
		t.Fatalf("current branch = %q, want ship/<id>", branch)
	}
}

func initTempRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "ship@example.com")
	runGit(t, dir, "config", "user.name", "Ship Test")
	path := filepath.Join(dir, "README")
	if err := os.WriteFile(path, []byte("init\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "README")
	runGit(t, dir, "commit", "-m", "init")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}
