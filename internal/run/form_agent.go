package run

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/huh"
	"github.com/maxBRT/ship-cli/internal/theme"
)

func (Forms) PickAgent() (string, error) {
	var kind string
	if err := mapAgentPickerErr(newAgentPickerForm(&kind).Run()); err != nil {
		return "", err
	}
	if err := validateAgentKind(kind); err != nil {
		return "", err
	}
	return kind, nil
}

func mapAgentPickerErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, huh.ErrUserAborted) {
		return ErrCanceled
	}
	return err
}

type agentChoice struct {
	kind  string
	blurb string
}

func agentChoices() []agentChoice {
	return []agentChoice{
		{AgentKindCursor, "Cursor Agent CLI"},
		{AgentKindPi, "Pi coding agent"},
		{AgentKindCodex, "OpenAI Codex CLI"},
		{AgentKindClaude, "Claude Code"},
	}
}

const agentPickerKeyhints = "↑↓ move · enter confirm · q cancel"

func newAgentPickerForm(kind *string) *huh.Form {
	opts := make([]huh.Option[string], 0, len(agentChoices()))
	for _, c := range agentChoices() {
		label := fmt.Sprintf("%-8s %s", c.kind, c.blurb)
		opts = append(opts, huh.NewOption(label, c.kind))
	}
	*kind = AgentKindCursor

	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(
		key.WithKeys("ctrl+c", "q"),
		key.WithHelp("q", "cancel"),
	)
	km.Select.Filter.SetEnabled(false)
	km.Select.Up = key.NewBinding(
		key.WithKeys("up", "k", "ctrl+k", "ctrl+p"),
		key.WithHelp("↑↓", "move"),
	)
	km.Select.Down = key.NewBinding(
		key.WithKeys("down", "j", "ctrl+j", "ctrl+n"),
		key.WithHelp("↑↓", "move"),
	)
	km.Select.Next = key.NewBinding(
		key.WithKeys("enter", "tab"),
		key.WithHelp("enter", "confirm"),
	)
	km.Select.Submit = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "confirm"),
	)

	title := theme.Brand + "  Choose Agent for this checkout"
	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(title).
				Description(agentPickerKeyhints).
				Options(opts...).
				Value(kind),
		),
	).WithKeyMap(km).WithTheme(huh.ThemeBase())
}
