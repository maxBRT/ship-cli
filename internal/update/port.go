// Package update brings a Ship installation up to date with the latest
// published GitHub Release.
package update

import "context"

// Port owns “bring this installation up to date with the latest published
// release.” The real adapter talks to GitHub Releases; tests fake this port.
type Port interface {
	Update(ctx context.Context) (Result, error)
}

// Result is the observable outcome of an Update call.
type Result struct {
	// AlreadyCurrent is true when the installed binary matches the latest release.
	AlreadyCurrent bool
	OldVersion     string
	NewVersion     string
}
