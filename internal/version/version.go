// Package version exposes build-time identity injected via -ldflags.
package version

// These three are overridden at link time:
//   go build -ldflags "-X github.com/agentguard/agentguard/internal/version.Version=v0.1.0 \
//                      -X github.com/agentguard/agentguard/internal/version.Commit=abcdef \
//                      -X github.com/agentguard/agentguard/internal/version.Date=2026-05-28"
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a single-line human-readable identifier.
func String() string {
	return Version + " (" + Commit + ", " + Date + ")"
}
