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

	cmd := exec.Command(c.bin(), args...) // #nosec G204 -- Claude Code CLI; args built by Ship
	cmd.Dir = req.Workspace
	cmd.Stdin = strings.NewReader(req.Prompt)
	return runStreamedPhase(ctx, req.Timeout, cmd, newClaudeStream(req.Events))
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
	stream := newClaudeStream(sink)
	for _, line := range lines {
		stream.ProcessLine(line)
	}
	return stream.Finish()
}

type claudeStream struct {
	sink      observe.Sink
	last      *claudeEvent
	toolCount int
	pending   map[string]string
}

func newClaudeStream(sink observe.Sink) *claudeStream {
	return &claudeStream{sink: sink, pending: map[string]string{}}
}

func (s *claudeStream) ProcessLine(line []byte) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return
	}
	var ev claudeEvent
	if err := json.Unmarshal(line, &ev); err != nil {
		return
	}
	switch ev.Type {
	case "assistant":
		if ev.Message == nil {
			return
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
				s.pending[block.ID] = name
			}
		}
	case "user":
		if ev.Message == nil {
			return
		}
		for _, block := range ev.Message.Content {
			if block.Type != "tool_result" {
				continue
			}
			s.toolCount++
			if s.sink == nil {
				continue
			}
			name := s.pending[block.ToolUseID]
			if name == "" {
				name = "unknown"
			}
			status := observe.ToolOK
			if block.IsError {
				status = observe.ToolError
			}
			s.sink.Emit(observe.Event{Kind: observe.KindTool, Name: name, Status: status})
		}
	case "result":
		s.last = &ev
	}
}

func (s *claudeStream) Finish() error {
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
				CacheRead:  s.last.Usage.CacheReadInputTokens,
				CacheWrite: s.last.Usage.CacheCreationInputTokens,
			}
		}
		s.sink.Emit(end)
	}
	return nil
}
