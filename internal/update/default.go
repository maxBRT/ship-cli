package update

import (
	"fmt"
	"os"

	"github.com/maxBRT/ship-cli/internal/version"
)

// DefaultOwner and DefaultRepo are the GitHub project that publishes Ship releases.
const (
	DefaultOwner = "maxBRT"
	DefaultRepo  = "ship-cli"
)

// Default returns a Releases Updater for the running executable and embedded version.
func Default() (Port, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("update: resolve executable: %w", err)
	}
	return &Releases{
		Owner:      DefaultOwner,
		Repo:       DefaultRepo,
		Version:    version.Version,
		Executable: exe,
	}, nil
}
