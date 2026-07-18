package gitops

import (
	"fmt"
	"os/exec"
	"strings"
)

// RestorePoint is a HEAD SHA recorded before a Ticket Iteration begins.
type RestorePoint string

// RecordRestorePoint captures the current HEAD before a Ticket Iteration.
func (r Repo) RecordRestorePoint() (RestorePoint, error) {
	sha, err := r.gitOutput("rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return RestorePoint(sha), nil
}

// UndoToRestorePoint hard-resets the current branch to rp, dropping commits
// made for the Ticket since the restore point was recorded.
//
// MVP assumption: the working tree is clean or ship-controlled when Abort runs.
func (r Repo) UndoToRestorePoint(rp RestorePoint) error {
	if rp == "" {
		return fmt.Errorf("empty restore point")
	}
	return r.git("reset", "--hard", string(rp))
}

func (r Repo) gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(string(out)), nil
}
