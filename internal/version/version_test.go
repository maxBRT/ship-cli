package version_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestVersion_ldflagsInjection(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "../.."))

	dir := t.TempDir()
	bin := filepath.Join(dir, "ship")
	ldflags := "-X github.com/maxBRT/ship-cli/internal/version.Version=v1.2.3"
	build := exec.Command("go", "build", "-o", bin, "-ldflags", ldflags, "./cmd/ship")
	build.Dir = root
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	cmd := exec.Command(bin, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ship --version: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	if got != "v1.2.3" {
		t.Errorf("version = %q, want %q", got, "v1.2.3")
	}
}
