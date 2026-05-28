package agentdetect

import "runtime"

// ClaudeCodeDetector finds Anthropic's Claude Code CLI by its MCP config
// file. The path has shifted between versions, so we try the union of
// known locations.
type ClaudeCodeDetector struct{}

func (ClaudeCodeDetector) Kind() Kind          { return KindClaudeCode }
func (ClaudeCodeDetector) DisplayName() string { return "Claude Code" }

func (ClaudeCodeDetector) Detect(home string) (*Detection, error) {
	candidates := []string{
		joinHome(home, ".claude.json"),
		joinHome(home, ".claude", "mcp_servers.json"),
		joinHome(home, ".claude", "settings.json"),
	}
	if runtime.GOOS != "windows" {
		candidates = append(candidates,
			joinHome(home, ".config", "claude", "config.json"))
	}
	path := firstExisting(candidates)
	if path == "" {
		return nil, ErrNotInstalled
	}
	parsed, err := readJSONMCPFile(path)
	if err != nil {
		return nil, err
	}
	return &Detection{
		Kind:        KindClaudeCode,
		DisplayName: "Claude Code",
		ConfigPath:  path,
		Format:      FormatJSON,
		Servers:     toEntries(parsed.Servers),
	}, nil
}
