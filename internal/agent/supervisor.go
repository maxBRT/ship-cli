package agent

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type streamProcessor interface {
	ProcessLine([]byte)
	Finish() error
}

func runStreamedPhase(ctx context.Context, timeout time.Duration, cmd *exec.Cmd, stream streamProcessor) error {
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("phase: stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("phase: stderr pipe: %w", err)
	}

	setProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("phase: start: %w", err)
	}

	var wg sync.WaitGroup
	var recentStderr recentLines
	var stdoutErr, stderrErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		stdoutErr = scanLines(stdout, stream.ProcessLine)
	}()
	go func() {
		defer wg.Done()
		stderrErr = scanLines(stderr, func(line []byte) {
			recentStderr.Add(string(line))
		})
	}()

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	var waitErr error
	select {
	case waitErr = <-waitCh:
	case <-ctx.Done():
		terminateProcessGroup(cmd.Process)
		waitErr = <-waitCh
		wg.Wait()
		return timeoutError(timeout, ctx.Err(), recentStderr.String())
	}
	wg.Wait()

	if stdoutErr != nil {
		return fmt.Errorf("phase: stdout: %w", stdoutErr)
	}
	if stderrErr != nil {
		return fmt.Errorf("phase: stderr: %w", stderrErr)
	}
	if waitErr != nil {
		msg := strings.TrimSpace(recentStderr.String())
		if msg == "" {
			msg = waitErr.Error()
		}
		return fmt.Errorf("phase: %s", msg)
	}
	return stream.Finish()
}

func scanLines(r io.Reader, fn func([]byte)) error {
	s := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	s.Buffer(buf, 1024*1024)
	for s.Scan() {
		line := append([]byte(nil), s.Bytes()...)
		fn(line)
	}
	if err := s.Err(); err != nil && !errors.Is(err, fs.ErrClosed) && !strings.Contains(err.Error(), "file already closed") {
		return err
	}
	return nil
}

func timeoutError(timeout time.Duration, cause error, stderr string) error {
	msg := fmt.Sprintf("phase: timeout after %s: %v", timeout, cause)
	stderr = strings.TrimSpace(stderr)
	if stderr != "" {
		msg += "; recent stderr: " + stderr
	}
	return fmt.Errorf("%s", msg)
}

type recentLines struct {
	mu    sync.Mutex
	lines []string
}

func (r *recentLines) Add(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, line)
	if len(r.lines) > 10 {
		r.lines = r.lines[len(r.lines)-10:]
	}
}

func (r *recentLines) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.lines, "\n")
}
