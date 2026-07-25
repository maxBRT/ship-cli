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

	cmd := exec.Command(c.bin(), args...) // #nosec G204 -- Cursor agent CLI; args built by Ship
	cmd.Dir = req.Workspace
	cmd.Stdin = strings.NewReader(req.Prompt)
	return runStreamedPhase(ctx, req.Timeout, cmd, newCursorStream(req.Events))
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
	stream := newCursorStream(sink)
	for _, line := range lines {
		stream.ProcessLine(line)
	}
	return stream.Finish()
}

type cursorStream struct {
	sink      observe.Sink
	last      *streamEvent
	toolCount int
}

func newCursorStream(sink observe.Sink) *cursorStream {
	return &cursorStream{sink: sink}
}

func (s *cursorStream) ProcessLine(line []byte) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return
	}
	var ev streamEvent
	if err := json.Unmarshal(line, &ev); err != nil {
		return
	}
	switch ev.Type {
	case "tool_call":
		if ev.Subtype == "completed" {
			s.toolCount++
			if s.sink != nil {
				name, status := parseToolCall(ev.ToolCall)
				s.sink.Emit(observe.Event{
					Kind:       observe.KindTool,
					Name:       name,
					DurationMS: ev.DurationMS,
					Status:     status,
				})
			}
		}
	case "result":
		s.last = &ev
	}
}

func (s *cursorStream) Finish() error {
	if s.last == nil {
		return fmt.Errorf("phase: missing terminal result event in stream-json output")
	}
	if s.last.Subtype != "success" {
		return fmt.Errorf("phase: result subtype %q, want success", s.last.Subtype)
	}
	if s.sink != nil {
		end := observe.Event{
			Kind:       observe.KindPhaseEnd,
			Outcome:    observe.OutcomeSuccess,
			DurationMS: s.last.DurationMS,
			ToolCount:  s.toolCount,
		}
		if s.last.Usage != nil {
			end.Tokens = &observe.TokenCounts{
				Input:      s.last.Usage.InputTokens,
				Output:     s.last.Usage.OutputTokens,
				CacheRead:  s.last.Usage.CacheReadTokens,
				CacheWrite: s.last.Usage.CacheWriteTokens,
			}
		}
		s.sink.Emit(end)
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
