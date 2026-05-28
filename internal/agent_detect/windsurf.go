package agentdetect

// WindsurfDetector finds Codeium's Windsurf MCP config at
// ~/.codeium/windsurf/mcp_config.json (current) or the legacy
// ~/.config/windsurf/mcp_servers.json.
type WindsurfDetector struct{}

func (WindsurfDetector) Kind() Kind          { return KindWindsurf }
func (WindsurfDetector) DisplayName() string { return "Windsurf" }

func (WindsurfDetector) Detect(home string) (*Detection, error) {
	path := firstExisting([]string{
		joinHome(home, ".codeium", "windsurf", "mcp_config.json"),
		joinHome(home, ".config", "windsurf", "mcp_servers.json"),
	})
	if path == "" {
		return nil, ErrNotInstalled
	}
	parsed, err := readJSONMCPFile(path)
	if err != nil {
		return nil, err
	}
	return &Detection{
		Kind:        KindWindsurf,
		DisplayName: "Windsurf",
		ConfigPath:  path,
		Format:      FormatJSON,
		Servers:     toEntries(parsed.Servers),
	}, nil
}
