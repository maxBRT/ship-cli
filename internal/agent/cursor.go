package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"unicode"

	"github.com/maxBRT/ship-cli/internal/observe"
)

// Cursor is an Agent Port that runs Cursor's headless agent CLI as a subprocess.
type Cursor struct {
	Bin string // path to the agent binary; empty means "agent"
}

var _ Port = Cursor{}

func (c Cursor) bin() string {
	if c.Bin != "" {
		return c.Bin
	}
	return "agent"
}

// RunPhase spawns a fresh headless agent process for one Phase.
func (c Cursor) RunPhase(ctx context.Context, req PhaseRequest) error {
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	args := []string{
		"-p",
		"--trust",
		"--force",
		"--approve-mcps",
		"--workspace", req.Workspace,
		"--output-format", "stream-json",
	}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}

	cmd := exec.CommandContext(ctx, c.bin(), args...) // #nosec G204 -- Cursor agent CLI; args built by Ship
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

	if err := consumeStreamJSON(stdout.Bytes(), req.Events); err != nil {
		return err
	}
	return nil
}

type streamEvent struct {
	Type       string          `json:"type"`
	Subtype    string          `json:"subtype"`
	CallID     string          `json:"call_id"`
	DurationMS int64           `json:"duration_ms"`
	ToolCall   json.RawMessage `json:"tool_call"`
	Usage      *streamUsage    `json:"usage"`
}

type streamUsage struct {
	InputTokens      int64 `json:"inputTokens"`
	OutputTokens     int64 `json:"outputTokens"`
	CacheReadTokens  int64 `json:"cacheReadTokens"`
	CacheWriteTokens int64 `json:"cacheWriteTokens"`
}

func consumeStreamJSON(stdout []byte, sink observe.Sink) error {
	lines := bytes.Split(stdout, []byte("\n"))
	var last *streamEvent
	toolCount := 0
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var ev streamEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "tool_call":
			if ev.Subtype == "completed" {
				toolCount++
				if sink != nil {
					name, status := parseToolCall(ev.ToolCall)
					sink.Emit(observe.Event{
						Kind:       observe.KindTool,
						Name:       name,
						DurationMS: ev.DurationMS,
						Status:     status,
					})
				}
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
				CacheRead:  last.Usage.CacheReadTokens,
				CacheWrite: last.Usage.CacheWriteTokens,
			}
		}
		sink.Emit(end)
	}
	return nil
}

func parseToolCall(raw json.RawMessage) (name, status string) {
	status = observe.ToolOK
	if len(raw) == 0 {
		return "unknown", status
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "unknown", status
	}
	for key, val := range obj {
		name = toolCallKeyName(key, val)
		status = toolCallStatus(val)
		return name, status
	}
	return "unknown", status
}

func toolCallKeyName(key string, val json.RawMessage) string {
	if key == "function" {
		var fn struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(val, &fn) == nil && fn.Name != "" {
			return fn.Name
		}
	}
	if strings.HasSuffix(key, "ToolCall") {
		base := strings.TrimSuffix(key, "ToolCall")
		if base == "" {
			return key
		}
		r := []rune(base)
		r[0] = unicode.ToUpper(r[0])
		return string(r)
	}
	return key
}

func toolCallStatus(val json.RawMessage) string {
	var body struct {
		Result map[string]json.RawMessage `json:"result"`
	}
	if json.Unmarshal(val, &body) != nil || body.Result == nil {
		return observe.ToolOK
	}
	if _, ok := body.Result["error"]; ok {
		return observe.ToolError
	}
	if _, ok := body.Result["success"]; ok {
		return observe.ToolOK
	}
	return observe.ToolOK
}
