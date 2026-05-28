package agentdetect

// GeminiCLIDetector finds Google's Gemini CLI. Current versions ship
// ~/.gemini/settings.json with an "mcpServers" key; older betas used
// ~/.config/gemini-cli/mcp.json.
type GeminiCLIDetector struct{}

func (GeminiCLIDetector) Kind() Kind          { return KindGeminiCLI }
func (GeminiCLIDetector) DisplayName() string { return "Gemini CLI" }

func (GeminiCLIDetector) Detect(home string) (*Detection, error) {
	path := firstExisting([]string{
		joinHome(home, ".gemini", "settings.json"),
		joinHome(home, ".config", "gemini-cli", "mcp.json"),
	})
	if path == "" {
		return nil, ErrNotInstalled
	}
	parsed, err := readJSONMCPFile(path)
	if err != nil {
		return nil, err
	}
	return &Detection{
		Kind:        KindGeminiCLI,
		DisplayName: "Gemini CLI",
		ConfigPath:  path,
		Format:      FormatJSON,
		Servers:     toEntries(parsed.Servers),
	}, nil
}
