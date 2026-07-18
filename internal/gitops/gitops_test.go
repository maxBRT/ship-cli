package gitops_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maxBRT/ship-cli/internal/gitops"
)

func TestEnsureBranch_createsAndChecksOutNamedBranch(t *testing.T) {
	dir := initTempRepo(t)
	repo := gitops.Repo{Dir: dir}

	got, err := repo.EnsureBranch("ship/ticket-6")
	if err != nil {
		t.Fatalf("EnsureBranch: %v", err)
	}
	if got != "ship/ticket-6" {
		t.Fatalf("EnsureBranch = %q, want ship/ticket-6", got)
	}
	if branch := currentBranch(t, dir); branch != "ship/ticket-6" {
		t.Fatalf("current branch = %q, want ship/ticket-6", branch)
	}
}

func TestEnsureBranch_checksOutExistingBranch(t *testing.T) {
	dir := initTempRepo(t)
	runGit(t, dir, "branch", "ship/existing")
	if branch := currentBranch(t, dir); branch != "main" {
		t.Fatalf("precondition: current branch = %q, want main", branch)
	}

	repo := gitops.Repo{Dir: dir}
	got, err := repo.EnsureBranch("ship/existing")
	if err != nil {
		t.Fatalf("EnsureBranch: %v", err)
	}
	if got != "ship/existing" {
		t.Fatalf("EnsureBranch = %q, want ship/existing", got)
	}
	if branch := currentBranch(t, dir); branch != "ship/existing" {
		t.Fatalf("current branch = %q, want ship/existing", branch)
	}
}

func TestEnsureBranch_emptyNameGeneratesShipPrefixedBranch(t *testing.T) {
	dir := initTempRepo(t)
	repo := gitops.Repo{Dir: dir}

	got, err := repo.EnsureBranch("")
	if err != nil {
		t.Fatalf("EnsureBranch: %v", err)
	}
	if !strings.HasPrefix(got, "ship/") {
		t.Fatalf("EnsureBranch = %q, want ship/<id> prefix", got)
	}
	if got == "ship/" {
		t.Fatal("EnsureBranch generated empty id after ship/")
	}
	if branch := currentBranch(t, dir); branch != got {
		t.Fatalf("current branch = %q, want %q", branch, got)
	}
}

func TestUndoToRestorePoint_dropsTicketCommits(t *testing.T) {
	dir := initTempRepo(t)
	repo := gitops.Repo{Dir: dir}
	if _, err := repo.EnsureBranch("ship/run"); err != nil {
		t.Fatalf("EnsureBranch: %v", err)
	}

	rp, err := repo.RecordRestorePoint()
	if err != nil {
		t.Fatalf("RecordRestorePoint: %v", err)
	}
	before := headSHA(t, dir)
	if string(rp) != before {
		t.Fatalf("RestorePoint = %q, want HEAD %q", rp, before)
	}

	writeAndCommit(t, dir, "work.txt", "ticket work\n", "ticket commit")
	after := headSHA(t, dir)
	if after == before {
		t.Fatal("expected new commit after ticket work")
	}

	if err := repo.UndoToRestorePoint(rp); err != nil {
		t.Fatalf("UndoToRestorePoint: %v", err)
	}
	if head := headSHA(t, dir); head != before {
		t.Fatalf("HEAD after undo = %q, want restore point %q", head, before)
	}
	if currentBranch(t, dir) != "ship/run" {
		t.Fatalf("branch after undo = %q, want ship/run", currentBranch(t, dir))
	}
}

func initTempRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "ship@example.com")
	runGit(t, dir, "config", "user.name", "Ship Test")
	path := filepath.Join(dir, "README")
	if err := os.WriteFile(path, []byte("init\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "README")
	runGit(t, dir, "commit", "-m", "init")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func currentBranch(t *testing.T, dir string) string {
	t.Helper()
	return runGit(t, dir, "branch", "--show-current")
}

func headSHA(t *testing.T, dir string) string {
	t.Helper()
	return runGit(t, dir, "rev-parse", "HEAD")
}

func writeAndCommit(t *testing.T, dir, relPath, contents, message string) {
	t.Helper()
	path := filepath.Join(dir, relPath)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", relPath)
	runGit(t, dir, "commit", "-m", message)
}
