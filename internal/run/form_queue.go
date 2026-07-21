package run

import (
	"errors"
	"fmt"
	"slices"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/maxBRT/ship-cli/internal/theme"
	"github.com/maxBRT/ship-cli/internal/ticket"
)

func (Forms) ConfirmQueue(candidates []ticket.Ticket) ([]ticket.Ticket, error) {
	if len(candidates) == 0 {
		return []ticket.Ticket{}, nil
	}
	var selected []int
	picker := newQueuePickerForm(candidates, &selected)
	picker.form.SubmitCmd = tea.Quit
	picker.form.CancelCmd = tea.Interrupt

	final, err := tea.NewProgram(picker).Run()
	if errors.Is(err, tea.ErrInterrupted) {
		return nil, ErrCanceled
	}
	if err != nil {
		return nil, err
	}
	q, ok := final.(*queuePicker)
	if !ok {
		return nil, fmt.Errorf("queue picker: unexpected model %T", final)
	}
	if q.form.State == huh.StateAborted {
		return nil, ErrCanceled
	}

	byNum := make(map[int]ticket.Ticket, len(candidates))
	for _, tk := range candidates {
		byNum[tk.Number] = tk
	}
	out := make([]ticket.Ticket, 0, len(selected))
	for _, n := range selected {
		tk, ok := byNum[n]
		if !ok {
			continue
		}
		out = append(out, tk)
	}
	return out, nil
}

const queuePickerKeyhints = "↑↓ move · space toggle · shift+↑↓ reorder · enter confirm · q cancel"

// queuePicker wraps a huh MultiSelect form so Shift+↑/↓ can reorder the
// focused selected Ticket. Selected list order is Run order.
type queuePicker struct {
	form     *huh.Form
	field    *huh.MultiSelect[int]
	options  []huh.Option[int]
	selected *[]int
}

func newQueuePickerForm(candidates []ticket.Ticket, selected *[]int) *queuePicker {
	opts := make([]huh.Option[int], 0, len(candidates))
	for _, tk := range candidates {
		label := fmt.Sprintf("#%d %s", tk.Number, tk.Title)
		opts = append(opts, huh.NewOption(label, tk.Number).Selected(true))
	}
	*selected = make([]int, len(candidates))
	for i, tk := range candidates {
		(*selected)[i] = tk.Number
	}

	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(
		key.WithKeys("ctrl+c", "q"),
		key.WithHelp("q", "cancel"),
	)
	km.MultiSelect.Filter.SetEnabled(false)
	km.MultiSelect.Toggle = key.NewBinding(
		key.WithKeys(" ", "x"),
		key.WithHelp("space", "toggle"),
	)
	km.MultiSelect.Up = key.NewBinding(
		key.WithKeys("up", "k", "ctrl+k", "ctrl+p"),
		key.WithHelp("↑↓", "move"),
	)
	km.MultiSelect.Down = key.NewBinding(
		key.WithKeys("down", "j", "ctrl+j", "ctrl+n"),
		key.WithHelp("↑↓", "move"),
	)
	km.MultiSelect.Next = key.NewBinding(
		key.WithKeys("enter", "tab"),
		key.WithHelp("enter", "confirm"),
	)
	km.MultiSelect.Submit = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "confirm"),
	)

	field := huh.NewMultiSelect[int]().
		Title(theme.Brand + "  Confirm Run queue").
		Description(queuePickerKeyhints).
		Options(opts...).
		Value(selected).
		Filterable(false)

	form := huh.NewForm(huh.NewGroup(field)).WithKeyMap(km).WithTheme(huh.ThemeBase())
	return &queuePicker{
		form:     form,
		field:    field,
		options:  opts,
		selected: selected,
	}
}

func (q *queuePicker) Init() tea.Cmd {
	return q.form.Init()
}

func (q *queuePicker) View() string {
	return q.form.View()
}

func (q *queuePicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "shift+up":
			q.reorder(-1)
			return q, nil
		case "shift+down":
			q.reorder(1)
			return q, nil
		}
	}
	m, cmd := q.form.Update(msg)
	q.form = m.(*huh.Form)
	return q, cmd
}

func (q *queuePicker) reorder(delta int) {
	hovered, ok := q.field.Hovered()
	if !ok || !slices.Contains(*q.selected, hovered) {
		return
	}
	idx := -1
	for i, o := range q.options {
		if o.Value == hovered {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	j := idx + delta
	if j < 0 || j >= len(q.options) {
		return
	}
	q.options[idx], q.options[j] = q.options[j], q.options[idx]
	q.field.Options(q.options...)
	q.nudgeCursorTo(j)
}

// nudgeCursorTo moves the MultiSelect cursor to target after Options() resets
// it to the first selected row.
func (q *queuePicker) nudgeCursorTo(target int) {
	firstSel := -1
	for i, o := range q.options {
		if slices.Contains(*q.selected, o.Value) {
			firstSel = i
			break
		}
	}
	if firstSel < 0 {
		return
	}
	steps := target - firstSel
	keyType := tea.KeyDown
	if steps < 0 {
		keyType = tea.KeyUp
		steps = -steps
	}
	for i := 0; i < steps; i++ {
		m, cmd := q.form.Update(tea.KeyMsg{Type: keyType})
		q.form = m.(*huh.Form)
		if cmd != nil {
			_ = cmd
		}
	}
}
