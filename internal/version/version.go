// Package version holds the Ship release version embedded at build time.
package version

// Version is the release version reported by ship --version.
// Release builds override this via -ldflags -X.
var Version = "dev"
