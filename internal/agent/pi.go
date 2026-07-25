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

// Pi is an Agent Port that runs Pi's headless coding-agent CLI as a subprocess.
type Pi struct {
	Bin string // path to the pi binary; empty means "pi"
}

var _ Port = Pi{}

func (p Pi) bin() string {
	if p.Bin != "" {
		return p.Bin
	}
	return "pi"
}

// RunPhase spawns a fresh headless Pi process for one Phase.
func (p Pi) RunPhase(ctx context.Context, req PhaseRequest) error {
	args := []string{
		"-p",
		"--mode", "json",
		"--no-session",
		"--approve",
	}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}

	cmd := exec.Command(p.bin(), args...) // #nosec G204 -- Pi agent CLI; args built by Ship
	cmd.Dir = req.Workspace
	cmd.Stdin = strings.NewReader(req.Prompt)
	return runStreamedPhase(ctx, req.Timeout, cmd, newPiStream(req.Events))
}

type piEvent struct {
	Type     string      `json:"type"`
	ToolName string      `json:"toolName"`
	IsError  bool        `json:"isError"`
	Messages []piMessage `json:"messages"`
}

type piMessage struct {
	Role  string   `json:"role"`
	Usage *piUsage `json:"usage"`
}

type piUsage struct {
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	CacheRead  int64 `json:"cacheRead"`
	CacheWrite int64 `json:"cacheWrite"`
}

func consumePiJSON(stdout []byte, sink observe.Sink) error {
	lines := bytes.Split(stdout, []byte("\n"))
	stream := newPiStream(sink)
	for _, line := range lines {
		stream.ProcessLine(line)
	}
	return stream.Finish()
}

type piStream struct {
	sink      observe.Sink
	sawEnd    bool
	toolCount int
	tokens    *observe.TokenCounts
}

func newPiStream(sink observe.Sink) *piStream {
	return &piStream{sink: sink}
}

func (s *piStream) ProcessLine(line []byte) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return
	}
	var ev piEvent
	if err := json.Unmarshal(line, &ev); err != nil {
		return
	}
	switch ev.Type {
	case "tool_execution_end":
		s.toolCount++
		if s.sink != nil {
			status := observe.ToolOK
			if ev.IsError {
				status = observe.ToolError
			}
			name := ev.ToolName
			if name == "" {
				name = "unknown"
			}
			s.sink.Emit(observe.Event{Kind: observe.KindTool, Name: name, Status: status})
		}
	case "agent_end":
		s.sawEnd = true
		s.tokens = sumPiUsage(ev.Messages)
	}
}

func (s *piStream) Finish() error {
	if !s.sawEnd {
		return fmt.Errorf("phase: missing terminal agent_end event in json output")
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

func sumPiUsage(msgs []piMessage) *observe.TokenCounts {
	var total observe.TokenCounts
	found := false
	for _, m := range msgs {
		if m.Usage == nil {
			continue
		}
		found = true
		total.Input += m.Usage.Input
		total.Output += m.Usage.Output
		total.CacheRead += m.Usage.CacheRead
		total.CacheWrite += m.Usage.CacheWrite
	}
	if !found {
		return nil
	}
	return &total
}
