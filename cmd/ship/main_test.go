package main

import (
	"bytes"
	"strings"
	"testing"
)

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
	var stdout, stderr bytes.Buffer
	code := Main([]string{"--not-a-real-flag"}, &stdout, &stderr, t.TempDir())
	if code == 0 {
		t.Fatal("exit = 0, want non-zero")
	}
	if stderr.Len() == 0 {
		t.Fatal("want error message on stderr")
	}
}

func TestMain_invalidTimeoutExitsNonZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Main([]string{"--timeout", "nope"}, &stdout, &stderr, t.TempDir())
	if code == 0 {
		t.Fatal("exit = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "timeout") && !strings.Contains(stderr.String(), "duration") {
		t.Errorf("stderr should mention timeout/duration; got %q", stderr.String())
	}
}
