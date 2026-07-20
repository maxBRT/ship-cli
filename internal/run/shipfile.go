package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const shipConfigFile = ".ship.yaml"

// InitConfig writes .ship.yaml with filled defaults and ensures .gitignore lists it.
// If the file already exists, created is false and the file is not overwritten;
// .gitignore is still ensured.
func InitConfig(dir string) (created bool, err error) {
	path := filepath.Join(dir, shipConfigFile)
	_, err = os.Stat(path)
	switch {
	case err == nil:
		// already exists
	case os.IsNotExist(err):
		cfg := defaultConfig()
		content := fmt.Sprintf(`branch: %q
feature: %q
agent: %s
model: %q
max_iterations: %d
timeout: %s
`, cfg.Branch, cfg.Feature, cfg.Agent, cfg.Model, cfg.MaxIterations, formatDuration(cfg.Timeout))
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return false, err
		}
		created = true
	default:
		return false, err
	}

	if err := ensureGitignore(dir); err != nil {
		return created, err
	}
	return created, nil
}

// LoadConfig reads .ship.yaml from dir. Every known key must be present;
// unknown keys are ignored.
func LoadConfig(dir string) (Config, error) {
	data, err := os.ReadFile(filepath.Join(dir, shipConfigFile))
	if err != nil {
		return Config{}, err
	}

	var raw shipYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("%s: %w", shipConfigFile, err)
	}

	missing := raw.missingKeys()
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("%s: missing required keys: %s", shipConfigFile, strings.Join(missing, ", "))
	}

	timeout, err := time.ParseDuration(*raw.Timeout)
	if err != nil {
		return Config{}, fmt.Errorf("%s: timeout: invalid duration %q", shipConfigFile, *raw.Timeout)
	}

	return Config{
		Branch:        *raw.Branch,
		Feature:       *raw.Feature,
		Agent:         *raw.Agent,
		Model:         *raw.Model,
		MaxIterations: *raw.MaxIterations,
		Timeout:       timeout,
	}, nil
}

type shipYAML struct {
	Branch        *string `yaml:"branch"`
	Feature       *string `yaml:"feature"`
	Agent         *string `yaml:"agent"`
	Model         *string `yaml:"model"`
	MaxIterations *int    `yaml:"max_iterations"`
	Timeout       *string `yaml:"timeout"`
}

func (s shipYAML) missingKeys() []string {
	var missing []string
	if s.Branch == nil {
		missing = append(missing, "branch")
	}
	if s.Feature == nil {
		missing = append(missing, "feature")
	}
	if s.Agent == nil {
		missing = append(missing, "agent")
	}
	if s.Model == nil {
		missing = append(missing, "model")
	}
	if s.MaxIterations == nil {
		missing = append(missing, "max_iterations")
	}
	if s.Timeout == nil {
		missing = append(missing, "timeout")
	}
	return missing
}

func formatDuration(d time.Duration) string {
	if d%time.Minute == 0 {
		return fmt.Sprintf("%dm", d/time.Minute)
	}
	return d.String()
}

func ensureGitignore(dir string) error {
	path := filepath.Join(dir, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		return os.WriteFile(path, []byte(shipConfigFile+"\n"), 0o644)
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == shipConfigFile {
			return nil
		}
	}
	var b strings.Builder
	b.Write(data)
	if len(data) > 0 && data[len(data)-1] != '\n' {
		b.WriteByte('\n')
	}
	b.WriteString(shipConfigFile)
	b.WriteByte('\n')
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
