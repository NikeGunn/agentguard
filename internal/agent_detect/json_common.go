package agentdetect

import (
	"encoding/json"
	"fmt"
	"os"
)

// mcpJSONFile is the shared shape across Claude Code, Cursor, Gemini CLI,
// and Windsurf: a top-level "mcpServers" map (Cursor uses "mcp" or
// "mcpServers" depending on version — we accept both). We deliberately
// decode into a json.RawMessage map so we don't lose unknown fields when
// the patcher writes the file back.
type mcpJSONFile struct {
	// Raw holds every top-level key in the file so the patcher can
	// round-trip non-MCP keys.
	Raw map[string]json.RawMessage
	// ServersKey records which key in Raw held the MCP servers map so the
	// patcher writes back under the same name.
	ServersKey string
	// Servers is the parsed view.
	Servers map[string]mcpJSONEntry
}

// mcpJSONEntry mirrors the per-server shape used by every JSON-based agent.
type mcpJSONEntry struct {
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	URL     string            `json:"url,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	// Type is set by some agents (e.g. "stdio", "http") — preserved on
	// write but not interpreted on read; transport is inferred from
	// presence of Command vs URL.
	Type string `json:"type,omitempty"`
}

// readJSONMCPFile loads path and extracts the MCP server entries. It tries
// "mcpServers" first then "mcp"; whichever matches becomes ServersKey.
func readJSONMCPFile(path string) (*mcpJSONFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw := make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	out := &mcpJSONFile{Raw: raw, Servers: map[string]mcpJSONEntry{}}
	for _, key := range []string{"mcpServers", "mcp"} {
		if v, ok := raw[key]; ok {
			if err := json.Unmarshal(v, &out.Servers); err != nil {
				return nil, fmt.Errorf("parse %s.%s: %w", path, key, err)
			}
			out.ServersKey = key
			return out, nil
		}
	}
	// No servers configured yet, but config file exists.
	out.ServersKey = "mcpServers"
	return out, nil
}

// toEntries flattens the parsed map into MCPServerEntry slices the patcher
// and detector consumers prefer to work with. Sorted by name so output is
// deterministic.
func toEntries(parsed map[string]mcpJSONEntry) []MCPServerEntry {
	names := make([]string, 0, len(parsed))
	for n := range parsed {
		names = append(names, n)
	}
	sortStrings(names)
	out := make([]MCPServerEntry, 0, len(parsed))
	for _, n := range names {
		e := parsed[n]
		out = append(out, MCPServerEntry{
			Name: n, Command: e.Command, Args: e.Args,
			URL: e.URL, Env: e.Env,
		})
	}
	return out
}

// sortStrings is a tiny local helper to avoid importing sort just for one
// place. Stable insertion sort is fine for the dozens of MCP servers any
// real user has configured.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
