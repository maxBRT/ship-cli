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

	cmd := exec.Command(c.bin(), args...) // #nosec G204 -- Codex agent CLI; args built by Ship
	cmd.Dir = req.Workspace
	cmd.Stdin = strings.NewReader(req.Prompt)
	return runStreamedPhase(ctx, req.Timeout, cmd, newCodexStream(req.Events))
}

type codexEvent struct {
	Type  string      `json:"type"`
	Item  *codexItem  `json:"item"`
	Usage *codexUsage `json:"usage"`
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

type codexStream struct {
	sink         observe.Sink
	sawCompleted bool
	sawFailed    bool
	toolCount    int
	tokens       *observe.TokenCounts
}

func newCodexStream(sink observe.Sink) *codexStream {
	return &codexStream{sink: sink}
}

func (s *codexStream) ProcessLine(line []byte) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return
	}
	var ev codexEvent
	if err := json.Unmarshal(line, &ev); err != nil {
		return
	}
	switch ev.Type {
	case "item.completed":
		if ev.Item == nil || !codexItemIsTool(ev.Item.Type) {
			return
		}
		s.toolCount++
		if s.sink != nil {
			name := codexToolName(ev.Item)
			status := observe.ToolOK
			if ev.Item.Status == "failed" || ev.Item.Status == "error" {
				status = observe.ToolError
			}
			s.sink.Emit(observe.Event{Kind: observe.KindTool, Name: name, Status: status})
		}
	case "turn.completed":
		s.sawCompleted = true
		if ev.Usage != nil {
			s.tokens = &observe.TokenCounts{Input: ev.Usage.InputTokens, Output: ev.Usage.OutputTokens + ev.Usage.ReasoningOutputTokens, CacheRead: ev.Usage.CachedInputTokens}
		}
	case "turn.failed":
		s.sawFailed = true
	}
}

func (s *codexStream) Finish() error {
	if s.sawFailed {
		return fmt.Errorf("phase: turn.failed")
	}
	if !s.sawCompleted {
		return fmt.Errorf("phase: missing terminal turn.completed event in json output")
	}
	if s.sink != nil {
		s.sink.Emit(observe.Event{
			Kind:      observe.KindPhaseEnd,
			Outcome:   observe.OutcomeSuccess,
			ToolCount: s.toolCount,
			Tokens:    s.tokens,
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
