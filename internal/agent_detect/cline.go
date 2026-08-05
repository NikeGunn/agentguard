package agentdetect

// ClineDetector finds Cline's shared MCP settings. Current Cline versions
// store them at ~/.cline/data/settings/cline_mcp_settings.json.
type ClineDetector struct{}

func (ClineDetector) Kind() Kind          { return KindCline }
func (ClineDetector) DisplayName() string { return "Cline" }

func (ClineDetector) Detect(home string) (*Detection, error) {
	path := firstExisting([]string{
		joinHome(home, ".cline", "data", "settings", "cline_mcp_settings.json"),
	})
	if path == "" {
		return nil, ErrNotInstalled
	}
	parsed, err := readJSONMCPFile(path)
	if err != nil {
		return nil, err
	}
	return &Detection{
		Kind:        KindCline,
		DisplayName: "Cline",
		ConfigPath:  path,
		Format:      FormatJSON,
		Servers:     toEntries(parsed.Servers),
	}, nil
}
