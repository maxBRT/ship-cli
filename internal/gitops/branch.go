package gitops

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Repo is a git checkout Ship can operate on. Dir is the repository root
// (or any directory inside it); cwd for git commands is Dir.
type Repo struct {
	Dir string
}

// EnsureBranch creates name if missing, then checks it out.
// If name is empty, it generates ship/<id> using a UTC timestamp.
// Returns the branch that is checked out.
func (r Repo) EnsureBranch(name string) (string, error) {
	if name == "" {
		name = fmt.Sprintf("ship/%s", time.Now().UTC().Format("20060102-150405.000000000"))
	}
	exists, err := r.branchExists(name)
	if err != nil {
		return "", err
	}
	if !exists {
		if err := r.git("checkout", "-b", name); err != nil {
			return "", err
		}
		return name, nil
	}
	if err := r.git("checkout", name); err != nil {
		return "", err
	}
	return name, nil
}

func (r Repo) branchExists(name string) (bool, error) {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	cmd.Dir = r.Dir
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git show-ref %s: %w", name, err)
}

func (r Repo) git(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
		return fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return nil
}
