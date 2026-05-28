package agentdetect

// CursorDetector finds Cursor's MCP config at ~/.cursor/mcp.json (global).
// Project-scoped .cursor/mcp.json files are surfaced via init's --project
// flag in a later milestone.
type CursorDetector struct{}

func (CursorDetector) Kind() Kind          { return KindCursor }
func (CursorDetector) DisplayName() string { return "Cursor" }

func (CursorDetector) Detect(home string) (*Detection, error) {
	path := firstExisting([]string{
		joinHome(home, ".cursor", "mcp.json"),
	})
	if path == "" {
		return nil, ErrNotInstalled
	}
	parsed, err := readJSONMCPFile(path)
	if err != nil {
		return nil, err
	}
	return &Detection{
		Kind:        KindCursor,
		DisplayName: "Cursor",
		ConfigPath:  path,
		Format:      FormatJSON,
		Servers:     toEntries(parsed.Servers),
	}, nil
}
