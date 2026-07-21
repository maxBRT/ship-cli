package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/maxBRT/ship-cli/internal/observe"
)

// Codex is an Agent Port that runs OpenAI Codex CLI exec as a subprocess.
type Codex struct {
	Bin string // path to the codex binary; empty means "codex"
}

var _ Port = Codex{}

func (c Codex) bin() string {
	if c.Bin != "" {
		return c.Bin
	}
	return "codex"
}

// RunPhase spawns a fresh ephemeral Codex exec process for one Phase.
func (c Codex) RunPhase(ctx context.Context, req PhaseRequest) error {
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	args := []string{
		"exec",
		"--ephemeral",
		"--json",
		"--dangerously-bypass-approvals-and-sandbox",
		"--cd", req.Workspace,
	}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	// "-" forces the full Phase prompt from stdin (not prompt+context mode).
	args = append(args, "-")

	cmd := exec.CommandContext(ctx, c.bin(), args...) // #nosec G204 -- Codex agent CLI; args built by Ship
	cmd.Dir = req.Workspace
	cmd.Stdin = strings.NewReader(req.Prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("phase: %w", ctx.Err())
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("phase: %s", msg)
	}

	if err := consumeCodexJSON(stdout.Bytes(), req.Events); err != nil {
		return err
	}
	return nil
}

type codexEvent struct {
	Type  string          `json:"type"`
	Item  *codexItem      `json:"item"`
	Usage *codexUsage     `json:"usage"`
}

type codexItem struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Status  string `json:"status"`
	Command string `json:"command"`
}

type codexUsage struct {
	InputTokens           int64 `json:"input_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
}

func consumeCodexJSON(stdout []byte, sink observe.Sink) error {
	lines := bytes.Split(stdout, []byte("\n"))
	sawCompleted := false
	sawFailed := false
	toolCount := 0
	var tokens *observe.TokenCounts
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var ev codexEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "item.completed":
			if ev.Item == nil || !codexItemIsTool(ev.Item.Type) {
				continue
			}
			toolCount++
			if sink != nil {
				name := codexToolName(ev.Item)
				status := observe.ToolOK
				if ev.Item.Status == "failed" || ev.Item.Status == "error" {
					status = observe.ToolError
				}
				sink.Emit(observe.Event{
					Kind:   observe.KindTool,
					Name:   name,
					Status: status,
				})
			}
		case "turn.completed":
			sawCompleted = true
			if ev.Usage != nil {
				tokens = &observe.TokenCounts{
					Input:     ev.Usage.InputTokens,
					Output:    ev.Usage.OutputTokens + ev.Usage.ReasoningOutputTokens,
					CacheRead: ev.Usage.CachedInputTokens,
				}
			}
		case "turn.failed":
			sawFailed = true
		}
	}
	if sawFailed {
		return fmt.Errorf("phase: turn.failed")
	}
	if !sawCompleted {
		return fmt.Errorf("phase: missing terminal turn.completed event in json output")
	}
	if sink != nil {
		sink.Emit(observe.Event{
			Kind:      observe.KindPhaseEnd,
			Outcome:   observe.OutcomeSuccess,
			ToolCount: toolCount,
			Tokens:    tokens,
		})
	}
	return nil
}

func codexItemIsTool(itemType string) bool {
	switch itemType {
	case "command_execution", "file_change", "mcp_tool_call", "web_search", "tool_call":
		return true
	default:
		return false
	}
}

func codexToolName(item *codexItem) string {
	if item.Type == "" {
		return "unknown"
	}
	return item.Type
}
