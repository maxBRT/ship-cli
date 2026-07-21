package run

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/ansi"
)

func TestAgentPickerForm_showsKindsBlurbsAndKeyhints(t *testing.T) {
	var kind string
	f := newAgentPickerForm(&kind)
	f.Update(f.Init())
	view := ansi.Strip(f.View())

	// Independent source: lavished Init mockup (issue #66 / #68).
	for _, kindName := range []string{"cursor", "pi", "codex", "claude"} {
		if !strings.Contains(view, kindName) {
			t.Errorf("picker missing kind %q; view:\n%s", kindName, view)
		}
	}
	for _, blurb := range []string{
		"Cursor Agent CLI",
		"Pi coding agent",
		"OpenAI Codex CLI",
		"Claude Code",
	} {
		if !strings.Contains(view, blurb) {
			t.Errorf("picker missing blurb %q; view:\n%s", blurb, view)
		}
	}
	if !strings.Contains(view, "ship") {
		t.Errorf("picker missing ship brand; view:\n%s", view)
	}
	if !strings.Contains(view, "Choose Agent") {
		t.Errorf("picker missing title; view:\n%s", view)
	}
	for _, hint := range []string{"move", "confirm", "cancel"} {
		if !strings.Contains(view, hint) {
			t.Errorf("picker missing keyhint %q; view:\n%s", hint, view)
		}
	}
}

func TestAgentPickerForm_qAborts(t *testing.T) {
	var kind string
	f := newAgentPickerForm(&kind)
	f.Update(f.Init())

	m, _ := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	f, ok := m.(*huh.Form)
	if !ok {
		t.Fatalf("Update returned %T, want *huh.Form", m)
	}
	if f.State != huh.StateAborted {
		t.Fatalf("State = %v, want StateAborted", f.State)
	}
}

func TestAgentPickerErr_userAbortedIsCanceled(t *testing.T) {
	err := mapAgentPickerErr(huh.ErrUserAborted)
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("mapAgentPickerErr = %v, want ErrCanceled", err)
	}
}

func TestAgentPickerForm_enterConfirmsFocusedKind(t *testing.T) {
	var kind string
	f := newAgentPickerForm(&kind)
	f.Update(f.Init())

	// Move to pi, then confirm.
	m, cmd := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = batchUpdate(m, cmd)
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = batchUpdate(m, cmd)

	f, ok := m.(*huh.Form)
	if !ok {
		t.Fatalf("Update returned %T, want *huh.Form", m)
	}
	if f.State != huh.StateCompleted {
		t.Fatalf("State = %v, want StateCompleted; kind=%q", f.State, kind)
	}
	if kind != AgentKindPi {
		t.Errorf("kind = %q, want pi", kind)
	}
}

func batchUpdate(m tea.Model, cmd tea.Cmd) tea.Model {
	if cmd == nil {
		return m
	}
	msg := cmd()
	if msg == nil {
		return m
	}
	switch msg := msg.(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			m = batchUpdate(m, c)
		}
		return m
	default:
		m, cmd = m.Update(msg)
		return batchUpdate(m, cmd)
	}
}
