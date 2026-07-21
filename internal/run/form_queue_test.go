package run

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/ansi"
	"github.com/maxBRT/ship-cli/internal/ticket"
)

func TestQueuePickerForm_showsTicketsAndKeyhints(t *testing.T) {
	candidates := []ticket.Ticket{
		{Number: 7, Title: "seven"},
		{Number: 8, Title: "eight"},
		{Number: 9, Title: "nine"},
	}
	var selected []int
	f := newQueuePickerForm(candidates, &selected)
	f.Update(f.Init())
	view := ansi.Strip(f.View())

	// Independent source: issue #66 / #69 Queue confirmation mockup.
	for _, want := range []string{"#7", "seven", "#8", "eight", "#9", "nine"} {
		if !strings.Contains(view, want) {
			t.Errorf("picker missing %q; view:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "ship") {
		t.Errorf("picker missing ship brand; view:\n%s", view)
	}
	for _, hint := range []string{"move", "toggle", "reorder", "confirm", "cancel"} {
		if !strings.Contains(view, hint) {
			t.Errorf("picker missing keyhint %q; view:\n%s", hint, view)
		}
	}
}

func TestQueuePickerForm_toggleAndConfirmReturnsListOrder(t *testing.T) {
	candidates := []ticket.Ticket{
		{Number: 7, Title: "seven"},
		{Number: 8, Title: "eight"},
		{Number: 9, Title: "nine"},
	}
	var selected []int
	f := newQueuePickerForm(candidates, &selected)
	f.Update(f.Init())

	// Drop #7 (focused first), move to #9, drop #9 → keep #8 only.
	m, cmd := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = batchUpdate(m, cmd)
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = batchUpdate(m, cmd)
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = batchUpdate(m, cmd)
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = batchUpdate(m, cmd)
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = batchUpdate(m, cmd)

	f, ok := m.(*queuePicker)
	if !ok {
		t.Fatalf("Update returned %T, want *queuePicker", m)
	}
	if f.form.State != huh.StateCompleted {
		t.Fatalf("State = %v, want StateCompleted; selected=%v", f.form.State, selected)
	}
	if len(selected) != 1 || selected[0] != 8 {
		t.Fatalf("selected = %v, want [8]", selected)
	}
}

func TestQueuePickerForm_qAborts(t *testing.T) {
	var selected []int
	f := newQueuePickerForm([]ticket.Ticket{{Number: 7, Title: "seven"}}, &selected)
	f.Update(f.Init())

	m, _ := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	f, ok := m.(*queuePicker)
	if !ok {
		t.Fatalf("Update returned %T, want *queuePicker", m)
	}
	if f.form.State != huh.StateAborted {
		t.Fatalf("State = %v, want StateAborted", f.form.State)
	}
}

func TestQueuePickerForm_emptySelection(t *testing.T) {
	candidates := []ticket.Ticket{
		{Number: 7, Title: "seven"},
		{Number: 8, Title: "eight"},
	}
	var selected []int
	f := newQueuePickerForm(candidates, &selected)
	f.Update(f.Init())

	// Deselect both.
	m, cmd := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = batchUpdate(m, cmd)
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = batchUpdate(m, cmd)
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = batchUpdate(m, cmd)
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = batchUpdate(m, cmd)

	f, ok := m.(*queuePicker)
	if !ok {
		t.Fatalf("Update returned %T, want *queuePicker", m)
	}
	if f.form.State != huh.StateCompleted {
		t.Fatalf("State = %v, want StateCompleted; selected=%v", f.form.State, selected)
	}
	if len(selected) != 0 {
		t.Fatalf("selected = %v, want empty", selected)
	}
}

func TestQueuePickerForm_shiftDownReordersFocusedSelected(t *testing.T) {
	candidates := []ticket.Ticket{
		{Number: 7, Title: "seven"},
		{Number: 8, Title: "eight"},
		{Number: 9, Title: "nine"},
	}
	var selected []int
	f := newQueuePickerForm(candidates, &selected)
	f.Update(f.Init())

	// Focus starts on #7 (selected). Shift+↓ swaps with #8 → run order 8, 7, 9.
	m, cmd := f.Update(tea.KeyMsg{Type: tea.KeyShiftDown})
	m = batchUpdate(m, cmd)
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = batchUpdate(m, cmd)

	f, ok := m.(*queuePicker)
	if !ok {
		t.Fatalf("Update returned %T, want *queuePicker", m)
	}
	if f.form.State != huh.StateCompleted {
		t.Fatalf("State = %v, want StateCompleted; selected=%v", f.form.State, selected)
	}
	if len(selected) != 3 || selected[0] != 8 || selected[1] != 7 || selected[2] != 9 {
		t.Fatalf("selected = %v, want [8 7 9]", selected)
	}
}

func TestQueuePickerForm_shiftUpReordersFocusedSelected(t *testing.T) {
	candidates := []ticket.Ticket{
		{Number: 7, Title: "seven"},
		{Number: 8, Title: "eight"},
		{Number: 9, Title: "nine"},
	}
	var selected []int
	f := newQueuePickerForm(candidates, &selected)
	f.Update(f.Init())

	// Move focus to #8, then Shift+↑ swaps with #7 → run order 8, 7, 9.
	m, cmd := f.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = batchUpdate(m, cmd)
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyShiftUp})
	m = batchUpdate(m, cmd)
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = batchUpdate(m, cmd)

	f, ok := m.(*queuePicker)
	if !ok {
		t.Fatalf("Update returned %T, want *queuePicker", m)
	}
	if f.form.State != huh.StateCompleted {
		t.Fatalf("State = %v, want StateCompleted; selected=%v", f.form.State, selected)
	}
	if len(selected) != 3 || selected[0] != 8 || selected[1] != 7 || selected[2] != 9 {
		t.Fatalf("selected = %v, want [8 7 9]", selected)
	}
}
