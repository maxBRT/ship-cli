package observe

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/maxBRT/ship-cli/internal/theme"
)

// Port is the Run-owned observability surface: it receives curated events
// during a Phase and presents them after the Phase ends. Durable Phase logs
// and terminal dumps share the same Event fields; only presentation differs.
type Port interface {
	// BeginPhase starts buffering for one Phase and returns the Sink Agent
	// adapters emit into. Mid-Phase Emit must not write tool lines to Out.
	BeginPhase(phase string) Sink
	// EndPhase dumps one dense strip with the tool total and compact
	// in/out token totals collected since BeginPhase, then clears the buffer.
	EndPhase()
	// Abort prints a loud stderr banner with the Run report directory, Phase
	// log path, and last tool lines, then clears the buffer. Used on Phase
	// failure instead of EndPhase so the banner does not rely on the
	// success dump.
	Abort()
}

// Observer buffers curated events for a Phase and dumps a summary line to Out
// when EndPhase is called. Safe for concurrent Emit from an Agent adapter.
// Dir is the workspace root; Phase reports land under Dir/.ship/runs/<run-id>/.
type Observer struct {
	Dir   string
	Out   io.Writer
	Color bool // red Abort chrome; plain when false

	mu       sync.Mutex
	events   []Event
	runDir   string
	phaseLog string
	phaseSeq int
}

// New returns an Observer that dumps to out and writes Phase reports under
// dir/.ship/runs. A nil out discards dump output. An empty dir skips durable
// report paths (terminal dump still works).
func New(dir string, out io.Writer) *Observer {
	if out == nil {
		out = io.Discard
	}
	return &Observer{Dir: dir, Out: out}
}

var _ Port = (*Observer)(nil)
var _ Sink = (*Observer)(nil)

// BeginPhase clears any prior Phase buffer and returns this Observer as the
// Phase Sink so adapters stay path-agnostic.
func (o *Observer) BeginPhase(phase string) Sink {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.events = nil
	o.phaseSeq++
	if o.Dir != "" {
		if o.runDir == "" {
			id := time.Now().Format("20060102-150405.000000000")
			o.runDir = filepath.Join(o.Dir, ".ship", "runs", id)
			_ = os.MkdirAll(o.runDir, 0o750)
		}
		name := fmt.Sprintf("%03d-%s.jsonl", o.phaseSeq, strings.ToLower(phase))
		o.phaseLog = filepath.Join(o.runDir, name)
		f, err := os.OpenFile(o.phaseLog, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err == nil {
			_ = f.Close()
		}
	}
	return o
}

// Emit records a curated event for the current Phase and appends it to the
// Phase log when durable reports are enabled. It does not write to Out.
func (o *Observer) Emit(e Event) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.events = append(o.events, e)
	if o.phaseLog == "" {
		return
	}
	f, err := os.OpenFile(o.phaseLog, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	_ = enc.Encode(e)
}

// EndPhase writes one dense strip with the tool total and compact in/out
// token totals, then clears the buffer. Per-tool detail and cache fields
// stay in the Phase log.
func (o *Observer) EndPhase() {
	o.mu.Lock()
	events := o.events
	o.events = nil
	o.mu.Unlock()

	var tools int
	var tokens *TokenCounts
	for _, e := range events {
		switch e.Kind {
		case KindTool:
			tools++
		case KindPhaseEnd:
			if e.Tokens != nil {
				tokens = e.Tokens
			}
		}
	}
	if tools == 0 && tokens == nil {
		return
	}
	if tokens == nil {
		fmt.Fprintf(o.Out, "  tools  %d\n", tools)
		return
	}
	fmt.Fprintf(o.Out, "  tools  %d  ·  in %s  ·  out %s\n",
		tools, compactCount(tokens.Input), compactCount(tokens.Output))
}

// Abort writes a loud banner with Run report and Phase log paths plus the
// last handful of tool one-liners, then clears the buffer without running the
// success dump. When Color is false, marks stay plain (no ANSI).
func (o *Observer) Abort() {
	o.mu.Lock()
	runDir := o.runDir
	phaseLog := o.phaseLog
	events := o.events
	o.events = nil
	o.mu.Unlock()

	mark := "✗ Abort"
	sep := "────────────────────────────────────────────"
	if o.Color {
		style := lipgloss.NewStyle().Foreground(theme.Red)
		mark = style.Render("✗ Abort")
		sep = style.Render(sep)
	}
	fmt.Fprintf(o.Out, "%s  Phase failed\n", mark)
	if runDir != "" {
		fmt.Fprintf(o.Out, "  report  %s\n", runDir)
	}
	if phaseLog != "" {
		fmt.Fprintf(o.Out, "  phase log  %s\n", phaseLog)
	}

	var tools []Event
	for _, e := range events {
		if e.Kind == KindTool {
			tools = append(tools, e)
		}
	}
	if n := len(tools); n > abortToolLimit {
		tools = tools[n-abortToolLimit:]
	}
	if len(tools) > 0 {
		fmt.Fprintln(o.Out, sep)
	}
	for _, e := range tools {
		fmt.Fprintf(o.Out, "  %s  %dms  %s\n", e.Name, e.DurationMS, e.Status)
	}
}

// compactCount renders token totals for the dense observe strip (e.g. 18.4k).
func compactCount(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1_000_000 {
		v := float64(n) / 1000
		if v == float64(int64(v)) {
			return fmt.Sprintf("%.0fk", v)
		}
		return fmt.Sprintf("%.1fk", v)
	}
	v := float64(n) / 1_000_000
	if v == float64(int64(v)) {
		return fmt.Sprintf("%.0fM", v)
	}
	return fmt.Sprintf("%.1fM", v)
}

// abortToolLimit is how many trailing tool lines the Abort banner keeps.
const abortToolLimit = 5
