// Package agentdetect finds installed AI-agent CLIs by their config-file
// fingerprints, parses their MCP server entries, and surfaces them to the
// patcher. The patcher in patcher.go does the rewriting.
//
// Each agent is a Detector: it knows where its config lives, what shape it
// is (JSON vs TOML), and how to enumerate the MCP server entries inside.
// init.go iterates Detect(home) across every registered Detector and
// presents the union as a checklist.
package agentdetect

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Kind identifies which agent flavour we're talking to. The value is stored
// as the `agents.kind` column.
type Kind string

const (
	KindClaudeCode Kind = "claude-code"
	KindCursor     Kind = "cursor"
	KindCodex      Kind = "codex"
	KindGeminiCLI  Kind = "gemini-cli"
	KindWindsurf   Kind = "windsurf"
)

// MCPServerEntry is one row inside an agent's MCP config file. The Name is
// the key it lives under; Command + Args are the upstream stdio invocation
// when present; URL is set for HTTP transports.
type MCPServerEntry struct {
	Name    string
	Command string
	Args    []string
	URL     string
	Env     map[string]string
}

// Detection is what a Detector returns when it finds a usable config file
// for its agent. ConfigPath is the absolute path on disk; Servers is the
// parsed entry list (may be empty if the user has configured no MCP servers
// yet, which is still a valid "we found this agent" signal).
type Detection struct {
	Kind        Kind
	DisplayName string
	ConfigPath  string
	Format      ConfigFormat
	Servers     []MCPServerEntry
}

// ConfigFormat tells the patcher which serialiser to use when writing back.
type ConfigFormat string

const (
	FormatJSON ConfigFormat = "json"
	FormatTOML ConfigFormat = "toml"
)

// Detector is the per-agent interface.
type Detector interface {
	Kind() Kind
	DisplayName() string
	// Detect resolves the agent's config path under the given home dir, or
	// returns ErrNotInstalled if no config is found. Returning a Detection
	// with an empty Servers slice is valid.
	Detect(home string) (*Detection, error)
}

// ErrNotInstalled is returned by Detectors when no config is found.
var ErrNotInstalled = errors.New("agent not installed")

// AllDetectors returns the canonical detector list. The order is the order
// shown to the user during init.
func AllDetectors() []Detector {
	return []Detector{
		ClaudeCodeDetector{},
		CursorDetector{},
		CodexDetector{},
		GeminiCLIDetector{},
		WindsurfDetector{},
	}
}

// DetectAll runs every detector against home and returns the agents that
// were found. Errors other than ErrNotInstalled are returned as a slice so
// init can surface them without aborting.
func DetectAll(home string) (detections []*Detection, errs []error) {
	for _, d := range AllDetectors() {
		dt, err := d.Detect(home)
		switch {
		case err == nil:
			detections = append(detections, dt)
		case errors.Is(err, ErrNotInstalled):
			continue
		default:
			errs = append(errs, fmt.Errorf("%s: %w", d.Kind(), err))
		}
	}
	return detections, errs
}

// firstExisting returns the first path from candidates that exists on disk,
// or "" if none do.
func firstExisting(candidates []string) string {
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

// joinHome is filepath.Join(home, rest...) but tolerant of an empty home
// (returns "" in that case so detectors short-circuit cleanly in tests).
func joinHome(home string, rest ...string) string {
	if home == "" {
		return ""
	}
	return filepath.Join(append([]string{home}, rest...)...)
}
