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

// Claude is an Agent Port that runs Claude Code print mode as a subprocess.
type Claude struct {
	Bin string // path to the claude binary; empty means "claude"
}

var _ Port = Claude{}

func (c Claude) bin() string {
	if c.Bin != "" {
		return c.Bin
	}
	return "claude"
}

// RunPhase spawns a fresh Claude Code print-mode process for one Phase.
func (c Claude) RunPhase(ctx context.Context, req PhaseRequest) error {
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	args := []string{
		"-p",
		"--dangerously-skip-permissions",
		"--no-session-persistence",
		"--output-format", "stream-json",
		"--verbose",
	}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}

	cmd := exec.CommandContext(ctx, c.bin(), args...) // #nosec G204 -- Claude Code CLI; args built by Ship
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

	if err := consumeClaudeStreamJSON(stdout.Bytes(), req.Events); err != nil {
		return err
	}
	return nil
}

type claudeEvent struct {
	Type       string         `json:"type"`
	Subtype    string         `json:"subtype"`
	DurationMS int64          `json:"duration_ms"`
	Message    *claudeMessage `json:"message"`
	Usage      *claudeUsage   `json:"usage"`
}

type claudeMessage struct {
	Content []claudeContent `json:"content"`
}

type claudeContent struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	ToolUseID string `json:"tool_use_id"`
	IsError   bool   `json:"is_error"`
}

type claudeUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
}

func consumeClaudeStreamJSON(stdout []byte, sink observe.Sink) error {
	lines := bytes.Split(stdout, []byte("\n"))
	var last *claudeEvent
	toolCount := 0
	pending := map[string]string{} // tool_use id -> name
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var ev claudeEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "assistant":
			if ev.Message == nil {
				continue
			}
			for _, block := range ev.Message.Content {
				if block.Type != "tool_use" {
					continue
				}
				name := block.Name
				if name == "" {
					name = "unknown"
				}
				if block.ID != "" {
					pending[block.ID] = name
				}
			}
		case "user":
			if ev.Message == nil {
				continue
			}
			for _, block := range ev.Message.Content {
				if block.Type != "tool_result" {
					continue
				}
				toolCount++
				if sink == nil {
					continue
				}
				name := pending[block.ToolUseID]
				if name == "" {
					name = "unknown"
				}
				status := observe.ToolOK
				if block.IsError {
					status = observe.ToolError
				}
				sink.Emit(observe.Event{
					Kind:   observe.KindTool,
					Name:   name,
					Status: status,
				})
			}
		case "result":
			last = &ev
		}
	}
	if last == nil {
		return fmt.Errorf("phase: missing terminal result event in stream-json output")
	}
	if last.Subtype != "success" {
		return fmt.Errorf("phase: result subtype %q, want success", last.Subtype)
	}
	if sink != nil {
		end := observe.Event{
			Kind:       observe.KindPhaseEnd,
			Outcome:    observe.OutcomeSuccess,
			DurationMS: last.DurationMS,
			ToolCount:  toolCount,
		}
		if last.Usage != nil {
			end.Tokens = &observe.TokenCounts{
				Input:      last.Usage.InputTokens,
				Output:     last.Usage.OutputTokens,
				CacheRead:  last.Usage.CacheReadInputTokens,
				CacheWrite: last.Usage.CacheCreationInputTokens,
			}
		}
		sink.Emit(end)
	}
	return nil
}
