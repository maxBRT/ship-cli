package run_test

import (
	"testing"
	"time"

	"github.com/maxBRT/ship-cli/internal/run"
)

func TestParseConfig_defaults(t *testing.T) {
	cfg, err := run.ParseConfig(nil, func(string) string { return "" })
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if cfg.Branch != "" {
		t.Errorf("Branch = %q, want empty (generate ship/<id>)", cfg.Branch)
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
	if cfg.Timeout != 10*time.Minute {
		t.Errorf("Timeout = %v, want 10m", cfg.Timeout)
	}
}

func TestParseConfig_envOverridesDefaults(t *testing.T) {
	env := map[string]string{
		"SHIP_BRANCH":         "feat/widget",
		"SHIP_FEATURE":        "widget",
		"SHIP_AGENT":          "cursor-agent",
		"SHIP_MODEL":          "composer",
		"SHIP_MAX_ITERATIONS": "3",
		"SHIP_PHASE_TIMEOUT":  "5m",
	}
	cfg, err := run.ParseConfig(nil, func(k string) string { return env[k] })
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

func TestParseConfig_flagsOverrideEnv(t *testing.T) {
	env := map[string]string{
		"SHIP_BRANCH":         "from-env",
		"SHIP_FEATURE":        "env-feature",
		"SHIP_AGENT":          "env-agent",
		"SHIP_MODEL":          "env-model",
		"SHIP_MAX_ITERATIONS": "7",
		"SHIP_PHASE_TIMEOUT":  "2m",
	}
	args := []string{
		"--branch", "from-flag",
		"--feature", "flag-feature",
		"--agent", "flag-agent",
		"--model", "flag-model",
		"--max-iterations", "4",
		"--timeout", "30s",
	}
	cfg, err := run.ParseConfig(args, func(k string) string { return env[k] })
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

func TestParseConfig_invalidMaxIterationsEnv(t *testing.T) {
	env := map[string]string{"SHIP_MAX_ITERATIONS": "nope"}
	_, err := run.ParseConfig(nil, func(k string) string { return env[k] })
	if err == nil {
		t.Fatal("ParseConfig: want error for invalid SHIP_MAX_ITERATIONS")
	}
}

func TestParseConfig_invalidTimeoutEnv(t *testing.T) {
	env := map[string]string{"SHIP_PHASE_TIMEOUT": "not-a-duration"}
	_, err := run.ParseConfig(nil, func(k string) string { return env[k] })
	if err == nil {
		t.Fatal("ParseConfig: want error for invalid SHIP_PHASE_TIMEOUT")
	}
}

func TestParseConfig_invalidMaxIterationsFlag(t *testing.T) {
	_, err := run.ParseConfig([]string{"--max-iterations", "xyz"}, func(string) string { return "" })
	if err == nil {
		t.Fatal("ParseConfig: want error for invalid --max-iterations")
	}
}

func TestParseConfig_invalidTimeoutFlag(t *testing.T) {
	_, err := run.ParseConfig([]string{"--timeout", "zzz"}, func(string) string { return "" })
	if err == nil {
		t.Fatal("ParseConfig: want error for invalid --timeout")
	}
}
