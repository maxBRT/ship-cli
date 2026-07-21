package run_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/maxBRT/ship-cli/internal/run"
)

func shipConfigPath(dir string) string {
	return filepath.Join(dir, ".ship", "config.yaml")
}

func writeShipConfig(t *testing.T, dir, content string) {
	t.Helper()
	path := shipConfigPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInit_nonTTYWritesCursorWithoutPrompt(t *testing.T) {
	dir := t.TempDir()

	created, err := (run.Init{
		IsTerminal: func() bool {
			return false
		},
	}).Config(dir)
	if err != nil {
		t.Fatalf("Init.Config: %v", err)
	}
	if !created {
		t.Fatal("Init.Config: want created=true")
	}

	cfg, err := run.LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Agent != "cursor" {
		t.Errorf("Agent = %q, want cursor", cfg.Agent)
	}
}

func initNonTTY(dir string) (bool, error) {
	return (run.Init{
		IsTerminal: func() bool { return false },
	}).Config(dir)
}

func TestInitConfig_writesFilledDefaults(t *testing.T) {
	dir := t.TempDir()

	created, err := initNonTTY(dir)
	if err != nil {
		t.Fatalf("InitConfig: %v", err)
	}
	if !created {
		t.Fatal("InitConfig: want created=true")
	}

	data, err := os.ReadFile(shipConfigPath(dir))
	if err != nil {
		t.Fatalf("read .ship/config.yaml: %v", err)
	}
	got := string(data)
	for _, want := range []string{
		"branch:",
		"agent: cursor",
		"model:",
		"max_iterations: 10",
		"timeout: 20m",
	} {
		if !strings.Contains(got, want) {
			t.Errorf(".ship/config.yaml missing %q; got:\n%s", want, got)
		}
	}
}

func TestInitConfig_alreadyExistsDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := shipConfigPath(dir)
	original := "branch: keep-me\n"
	writeShipConfig(t, dir, original)

	created, err := initNonTTY(dir)
	if err != nil {
		t.Fatalf("InitConfig: %v", err)
	}
	if created {
		t.Fatal("InitConfig: want created=false when file exists")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Errorf("file overwritten: got %q", data)
	}
}

func TestLoadConfig_readsAllFields(t *testing.T) {
	dir := t.TempDir()
	content := `branch: feat/widget
agent: pi
model: composer
max_iterations: 3
timeout: 5m
`
	writeShipConfig(t, dir, content)

	cfg, err := run.LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Branch != "feat/widget" {
		t.Errorf("Branch = %q, want feat/widget", cfg.Branch)
	}
	if cfg.Agent != "pi" {
		t.Errorf("Agent = %q, want pi", cfg.Agent)
	}
	if cfg.Model != "composer" {
		t.Errorf("Model = %q, want composer", cfg.Model)
	}
	if cfg.MaxIterations != 3 {
		t.Errorf("MaxIterations = %d, want 3", cfg.MaxIterations)
	}
	if cfg.Timeout != 5*time.Minute {
		t.Errorf("Timeout = %v, want 5m", cfg.Timeout)
	}
}

func TestLoadConfig_missingKeyErrors(t *testing.T) {
	dir := t.TempDir()
	content := `branch: ""
agent: cursor
model: ""
timeout: 10m
`
	writeShipConfig(t, dir, content)

	_, err := run.LoadConfig(dir)
	if err == nil {
		t.Fatal("LoadConfig: want error for missing max_iterations")
	}
	if !strings.Contains(err.Error(), "max_iterations") {
		t.Errorf("error should mention max_iterations; got %v", err)
	}
}

func TestLoadConfig_ignoresUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	content := `branch: ""
agent: cursor
model: ""
max_iterations: 10
timeout: 10m
feature: leftover
extra_thing: ignored
`
	writeShipConfig(t, dir, content)

	cfg, err := run.LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Agent != "cursor" {
		t.Errorf("Agent = %q, want cursor", cfg.Agent)
	}
}

func TestInitConfig_roundTripLoadable(t *testing.T) {
	dir := t.TempDir()
	if _, err := initNonTTY(dir); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}
	cfg, err := run.LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig after Init: %v", err)
	}
	if cfg.Agent != "cursor" || cfg.MaxIterations != 10 || cfg.Timeout != 20*time.Minute {
		t.Errorf("unexpected defaults after init: %+v", cfg)
	}
}
