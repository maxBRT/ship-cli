package observe

import (
	"fmt"
	"io"
	"sync"
)

// Port is the Run-owned observability surface: it receives curated events
// during a Phase and presents them after the Phase ends. Durable Phase logs
// and terminal dumps share the same Event fields; only presentation differs.
type Port interface {
	// BeginPhase starts buffering for one Phase and returns the Sink Agent
	// adapters emit into. Mid-Phase Emit must not write tool lines to Out.
	BeginPhase(phase string) Sink
	// EndPhase dumps high-signal one-liners for tools and token totals
	// collected since BeginPhase, then clears the buffer.
	EndPhase()
}

// Observer buffers curated events for a Phase and dumps one-liners to Out
// when EndPhase is called. Safe for concurrent Emit from an Agent adapter.
type Observer struct {
	Out io.Writer

	mu     sync.Mutex
	events []Event
}

// New returns an Observer that dumps to out. A nil out discards dump output.
func New(out io.Writer) *Observer {
	if out == nil {
		out = io.Discard
	}
	return &Observer{Out: out}
}

var _ Port = (*Observer)(nil)
var _ Sink = (*Observer)(nil)

// BeginPhase clears any prior Phase buffer and returns this Observer as the
// Phase Sink so adapters stay path-agnostic.
func (o *Observer) BeginPhase(string) Sink {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.events = nil
	return o
}

// Emit records a curated event for the current Phase. It does not write to Out.
func (o *Observer) Emit(e Event) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.events = append(o.events, e)
}

// EndPhase writes high-signal one-liners for buffered tools (and later tokens)
// then clears the buffer.
func (o *Observer) EndPhase() {
	o.mu.Lock()
	events := o.events
	o.events = nil
	o.mu.Unlock()

	for _, e := range events {
		switch e.Kind {
		case KindTool:
			fmt.Fprintf(o.Out, "tool  %s  %dms  %s\n", e.Name, e.DurationMS, e.Status)
		case KindPhaseEnd:
			if e.Tokens == nil {
				continue
			}
			fmt.Fprintf(o.Out, "tokens  input=%d output=%d cache_read=%d cache_write=%d\n",
				e.Tokens.Input, e.Tokens.Output, e.Tokens.CacheRead, e.Tokens.CacheWrite)
		}
	}
}
