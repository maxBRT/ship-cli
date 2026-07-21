package herdr

import (
	"context"
	"os"
	"os/exec"
)

// Source and AgentID identify Ship's lifecycle reports to Herdr.
// AgentID is the Herdr --agent value for Ship-the-product, not the coding Agent port.
const (
	Source  = "ship:run"
	AgentID = "ship"
)

// Exec runs the herdr CLI with the given args. Tests inject a fake; nil means
// the real herdr binary (or HERDR_BIN_PATH when set).
type Exec func(ctx context.Context, args ...string) error

// Reporter is the production Port: it shells to `herdr pane report-agent`
// when running under Herdr, and is a no-op otherwise.
type Reporter struct {
	// LookupEnv reads process environment. Nil means os.LookupEnv.
	LookupEnv func(key string) (string, bool)
	// Exec runs herdr. Nil means the real binary.
	Exec Exec
	// Bin overrides the herdr binary name/path. Empty uses HERDR_BIN_PATH or "herdr".
	Bin string
}

var _ Port = Reporter{}

// Working reports state=working when under Herdr; otherwise it is a no-op.
func (r Reporter) Working(ctx context.Context, message string) {
	r.report(ctx, "working", message)
}

// Idle reports state=idle when under Herdr; otherwise it is a no-op.
func (r Reporter) Idle(ctx context.Context) {
	r.report(ctx, "idle", "")
}

func (r Reporter) report(ctx context.Context, state, message string) {
	paneID, ok := r.paneID()
	if !ok {
		return
	}
	args := []string{
		"pane", "report-agent", paneID,
		"--source", Source,
		"--agent", AgentID,
		"--state", state,
	}
	if message != "" {
		args = append(args, "--message", message)
	}
	execFn := r.Exec
	if execFn == nil {
		execFn = r.defaultExec
	}
	_ = execFn(ctx, args...) // best-effort; missing herdr must not fail the Phase
}

func (r Reporter) paneID() (string, bool) {
	if !r.active() {
		return "", false
	}
	lookup := r.lookup()
	id, _ := lookup("HERDR_PANE_ID")
	return id, true
}

func (r Reporter) active() bool {
	lookup := r.lookup()
	if v, ok := lookup("HERDR_ENV"); !ok || v != "1" {
		return false
	}
	if v, ok := lookup("HERDR_SOCKET_PATH"); !ok || v == "" {
		return false
	}
	if v, ok := lookup("HERDR_PANE_ID"); !ok || v == "" {
		return false
	}
	return true
}

func (r Reporter) lookup() func(string) (string, bool) {
	if r.LookupEnv != nil {
		return r.LookupEnv
	}
	return os.LookupEnv
}

func (r Reporter) bin() string {
	if r.Bin != "" {
		return r.Bin
	}
	if v, ok := r.lookup()("HERDR_BIN_PATH"); ok && v != "" {
		return v
	}
	return "herdr"
}

func (r Reporter) defaultExec(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, r.bin(), args...) // #nosec G204 -- fixed herdr binary; args built by Ship
	return cmd.Run()
}
