package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	shipDir        = ".ship"
	shipConfigFile = "config.yaml"
)

func shipConfigPath(dir string) string {
	return filepath.Join(dir, shipDir, shipConfigFile)
}

// Init creates .ship/config.yaml. On a TTY it asks FormRunner for an Agent
// kind; otherwise it records agent: cursor with no prompt.
type Init struct {
	// IsTerminal reports whether an interactive session is available.
	// Optional; default requires stdin and stderr to be character devices.
	IsTerminal func() bool
	// Forms prompts for Agent kind on a TTY. Optional; default Forms (huh).
	Forms FormRunner
}

// Config writes .ship/config.yaml with filled defaults and the chosen Agent kind.
// If the file already exists, created is false and the file is not overwritten.
func (i Init) Config(dir string) (created bool, err error) {
	path := shipConfigPath(dir)
	_, err = os.Stat(path)
	switch {
	case err == nil:
		return false, nil
	case os.IsNotExist(err):
		cfg := defaultConfig()
		if i.terminal() {
			kind, err := i.forms().PickAgent()
			if err != nil {
				return false, err
			}
			cfg.Agent = kind
		}
		content := fmt.Sprintf(`branch: %q
agent: %s
model: %q
max_iterations: %d
timeout: %s
`, cfg.Branch, cfg.Agent, cfg.Model, cfg.MaxIterations, formatDuration(cfg.Timeout))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return false, err
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return false, err
		}
		return true, nil
	default:
		return false, err
	}
}

func (i Init) forms() FormRunner {
	if i.Forms != nil {
		return i.Forms
	}
	return Forms{}
}

func (i Init) terminal() bool {
	if i.IsTerminal != nil {
		return i.IsTerminal()
	}
	return isTerminalFile(os.Stdin) && isTerminalFile(os.Stderr)
}

// InitConfig writes .ship/config.yaml with filled defaults.
// If the file already exists, created is false and the file is not overwritten.
// On a TTY it prompts for Agent kind; otherwise it records agent: cursor.
func InitConfig(dir string) (created bool, err error) {
	return Init{}.Config(dir)
}

// LoadConfig reads .ship/config.yaml from dir. Every known key must be present;
// unknown keys are ignored.
func LoadConfig(dir string) (Config, error) {
	rel := filepath.Join(shipDir, shipConfigFile)
	data, err := os.ReadFile(shipConfigPath(dir))
	if err != nil {
		return Config{}, err
	}

	var raw shipYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("%s: %w", rel, err)
	}

	missing := raw.missingKeys()
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("%s: missing required keys: %s", rel, strings.Join(missing, ", "))
	}

	timeout, err := time.ParseDuration(*raw.Timeout)
	if err != nil {
		return Config{}, fmt.Errorf("%s: timeout: invalid duration %q", rel, *raw.Timeout)
	}

	return Config{
		Branch:        *raw.Branch,
		Agent:         *raw.Agent,
		Model:         *raw.Model,
		MaxIterations: *raw.MaxIterations,
		Timeout:       timeout,
	}, nil
}

type shipYAML struct {
	Branch        *string `yaml:"branch"`
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
