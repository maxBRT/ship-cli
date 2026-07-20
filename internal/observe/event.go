// Package observe defines curated Run observability events and the sink
// Agent adapters emit into. Ship owns durable Phase logs and terminal
// presentation; adapters stay path-agnostic.
package observe

// Kind is a curated observability event kind.
type Kind string

const (
	KindPhaseStart Kind = "phase_start"
	KindTool       Kind = "tool"
	KindPhaseEnd   Kind = "phase_end"
)

// Tool status values for KindTool events.
const (
	ToolOK    = "ok"
	ToolError = "error"
)

// Phase outcome values for KindPhaseEnd events.
const (
	OutcomeSuccess = "success"
	OutcomeError   = "error"
)

// TokenCounts are optional per-Phase usage totals when an Agent stream
// exposes them. Absence means the Agent omitted usage; it is not a failure.
type TokenCounts struct {
	Input      int64 `json:"input,omitempty"`
	Output     int64 `json:"output,omitempty"`
	CacheRead  int64 `json:"cache_read,omitempty"`
	CacheWrite int64 `json:"cache_write,omitempty"`
}

// Event is one curated observability record shared by Phase logs and the
// terminal. Adapters must not emit raw vendor stream lines or message bodies.
type Event struct {
	Kind Kind `json:"kind"`

	// KindTool
	Name       string `json:"name,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
	Status     string `json:"status,omitempty"`

	// KindPhaseStart / KindPhaseEnd
	Phase string `json:"phase,omitempty"`

	// KindPhaseEnd
	Outcome    string       `json:"outcome,omitempty"`
	Tokens     *TokenCounts `json:"tokens,omitempty"`
	ToolCount  int          `json:"tool_count,omitempty"`
}

// Sink receives curated events from Agent adapters during a Phase.
// Implementations may write Phase logs, buffer for a terminal dump, or both.
// A nil Sink on a PhaseRequest means the adapter emits nothing.
type Sink interface {
	Emit(Event)
}
