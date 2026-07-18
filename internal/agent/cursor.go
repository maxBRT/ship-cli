package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
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

	cmd := exec.CommandContext(ctx, c.bin(), args...)
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

	if err := requireStreamJSONSuccess(stdout.Bytes()); err != nil {
		return err
	}
	return nil
}

type streamEvent struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
}

func requireStreamJSONSuccess(stdout []byte) error {
	lines := bytes.Split(stdout, []byte("\n"))
	var last *streamEvent
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var ev streamEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		if ev.Type == "result" {
			last = &ev
		}
	}
	if last == nil {
		return fmt.Errorf("phase: missing terminal result event in stream-json output")
	}
	if last.Subtype != "success" {
		return fmt.Errorf("phase: result subtype %q, want success", last.Subtype)
	}
	return nil
}
