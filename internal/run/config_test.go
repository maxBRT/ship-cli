package run_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/maxBRT/ship-cli/internal/run"
)

func writeShipYAML(t *testing.T, dir, content string) {
	t.Helper()
	path := filepath.Join(dir, ".ship", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const fullYAML = `branch: ""
feature: ""
agent: agent
model: ""
max_iterations: 10
timeout: 10m
`

func TestParseConfig_yamlDefaults(t *testing.T) {
	dir := t.TempDir()
	writeShipYAML(t, dir, fullYAML)

	cfg, err := run.ParseConfig(nil, dir)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if cfg.Branch != "" {
		t.Errorf("Branch = %q, want empty", cfg.Branch)
	}
	if cfg.Feature != "" {
		t.Errorf("Feature = %q, want empty", cfg.Feature)
	}
	if cfg.Agent != "agent" {
		t.Errorf("Agent = %q, want %q", cfg.Agent, "agent")
	}
	if cfg.Model != "" {
		t.Errorf("Model = %q, want empty", cfg.Model)
	}
	if cfg.MaxIterations != 10 {
		t.Errorf("MaxIterations = %d, want 10", cfg.MaxIterations)
	}
	if cfg.Timeout != 20*time.Minute {
		t.Errorf("Timeout = %v, want 20m", cfg.Timeout)
	}
}

func TestParseConfig_yamlOverridesBuiltInDefaults(t *testing.T) {
	dir := t.TempDir()
	writeShipYAML(t, dir, `branch: feat/widget
feature: widget
agent: cursor-agent
model: composer
max_iterations: 3
timeout: 5m
`)

	cfg, err := run.ParseConfig(nil, dir)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if cfg.Branch != "feat/widget" {
		t.Errorf("Branch = %q, want feat/widget", cfg.Branch)
	}
	if cfg.Feature != "widget" {
		t.Errorf("Feature = %q, want widget", cfg.Feature)
	}
	if cfg.Agent != "cursor-agent" {
		t.Errorf("Agent = %q, want cursor-agent", cfg.Agent)
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

func TestParseConfig_flagsOverrideYAML(t *testing.T) {
	dir := t.TempDir()
	writeShipYAML(t, dir, `branch: from-yaml
feature: yaml-feature
agent: yaml-agent
model: yaml-model
max_iterations: 7
timeout: 2m
`)
	args := []string{
		"--branch", "from-flag",
		"--feature", "flag-feature",
		"--agent", "flag-agent",
		"--model", "flag-model",
		"--max-iterations", "4",
		"--timeout", "30s",
	}
	cfg, err := run.ParseConfig(args, dir)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if cfg.Branch != "from-flag" {
		t.Errorf("Branch = %q, want from-flag", cfg.Branch)
	}
	if cfg.Feature != "flag-feature" {
		t.Errorf("Feature = %q, want flag-feature", cfg.Feature)
	}
	if cfg.Agent != "flag-agent" {
		t.Errorf("Agent = %q, want flag-agent", cfg.Agent)
	}
	if cfg.Model != "flag-model" {
		t.Errorf("Model = %q, want flag-model", cfg.Model)
	}
	if cfg.MaxIterations != 4 {
		t.Errorf("MaxIterations = %d, want 4", cfg.MaxIterations)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", cfg.Timeout)
	}
}

func TestParseConfig_invalidMaxIterationsFlag(t *testing.T) {
	dir := t.TempDir()
	writeShipYAML(t, dir, fullYAML)
	_, err := run.ParseConfig([]string{"--max-iterations", "xyz"}, dir)
	if err == nil {
		t.Fatal("ParseConfig: want error for invalid --max-iterations")
	}
}

func TestParseConfig_invalidTimeoutFlag(t *testing.T) {
	dir := t.TempDir()
	writeShipYAML(t, dir, fullYAML)
	_, err := run.ParseConfig([]string{"--timeout", "zzz"}, dir)
	if err == nil {
		t.Fatal("ParseConfig: want error for invalid --timeout")
	}
}
