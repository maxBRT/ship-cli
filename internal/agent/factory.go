package agent

import (
	"fmt"
	"os/exec"
)

// New returns the Agent Port for kind. cursor uses the default agent binary on
// PATH. Other supported kinds error until their adapters land.
func New(kind string) (Port, error) {
	switch kind {
	case "cursor":
		return Cursor{}, nil
	case "pi", "codex", "claude":
		return nil, fmt.Errorf("agent kind %q: adapter not implemented yet", kind)
	default:
		return nil, fmt.Errorf("unknown agent kind %q", kind)
	}
}

// DefaultBinary returns the fixed PATH binary name for kind.
func DefaultBinary(kind string) (string, error) {
	switch kind {
	case "cursor":
		return "agent", nil
	case "pi":
		return "pi", nil
	case "codex":
		return "codex", nil
	case "claude":
		return "claude", nil
	default:
		return "", fmt.Errorf("unknown agent kind %q", kind)
	}
}

// RequireOnPATH fails when kind's default binary is not findable on PATH.
func RequireOnPATH(kind string) error {
	bin, err := DefaultBinary(kind)
	if err != nil {
		return err
	}
	if _, err := exec.LookPath(bin); err != nil {
		return fmt.Errorf("agent kind %q: %q not found on PATH", kind, bin)
	}
	return nil
}
