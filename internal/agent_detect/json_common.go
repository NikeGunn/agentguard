package agentdetect

import (
	"encoding/json"
	"fmt"
	"os"
)

// mcpJSONFile is the shared shape across Claude Code, Cline, Cursor, Gemini
// CLI, and Windsurf: a top-level "mcpServers" map (Cursor uses "mcp" or
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
	raw     map[string]json.RawMessage
	Command string
	Args    []string
	URL     string
	Env     map[string]string
	// Type is set by some agents (e.g. "stdio", "http") — preserved on
	// write but not interpreted on read; transport is inferred from
	// presence of Command vs URL.
	Type      string
	Transport json.RawMessage
}

// mcpJSONTransport is Cline's nested transport form. It retains the raw
// object so Cline-specific fields survive a rewrite.
type mcpJSONTransport struct {
	raw     map[string]json.RawMessage
	Type    string
	Command string
	Args    []string
	URL     string
	Env     map[string]string
}

func (e *mcpJSONEntry) UnmarshalJSON(data []byte) error {
	type entryFields struct {
		Command   string            `json:"command"`
		Args      []string          `json:"args"`
		URL       string            `json:"url"`
		Env       map[string]string `json:"env"`
		Type      string            `json:"type"`
		Transport json.RawMessage   `json:"transport"`
	}
	var fields entryFields
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	e.raw = raw
	e.Command = fields.Command
	e.Args = fields.Args
	e.URL = fields.URL
	e.Env = fields.Env
	e.Type = fields.Type
	e.Transport = fields.Transport
	return nil
}

func (e mcpJSONEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.raw)
}

func (e mcpJSONEntry) nestedTransport() (*mcpJSONTransport, bool) {
	if len(e.Transport) == 0 {
		return nil, false
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(e.Transport, &raw); err != nil || raw == nil {
		return nil, false
	}
	type transportFields struct {
		Type    string            `json:"type"`
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		URL     string            `json:"url"`
		Env     map[string]string `json:"env"`
	}
	var fields transportFields
	if err := json.Unmarshal(e.Transport, &fields); err != nil {
		return nil, false
	}
	if fields.Command == "" && fields.URL == "" {
		return nil, false
	}
	return &mcpJSONTransport{
		raw:     raw,
		Type:    fields.Type,
		Command: fields.Command,
		Args:    fields.Args,
		URL:     fields.URL,
		Env:     fields.Env,
	}, true
}

func (e mcpJSONEntry) values() (command string, args []string, url string, env map[string]string) {
	if transport, ok := e.nestedTransport(); ok {
		return transport.Command, transport.Args, transport.URL, transport.Env
	}
	return e.Command, e.Args, e.URL, e.Env
}

func setJSONValue(raw map[string]json.RawMessage, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	raw[key] = data
	return nil
}

func (e *mcpJSONEntry) setInvocation(command string, args []string) error {
	if transport, ok := e.nestedTransport(); ok {
		if err := setJSONValue(transport.raw, "command", command); err != nil {
			return err
		}
		if err := setJSONValue(transport.raw, "args", args); err != nil {
			return err
		}
		data, err := json.Marshal(transport.raw)
		if err != nil {
			return err
		}
		e.raw["transport"] = data
		e.Transport = data
		return nil
	}
	if err := setJSONValue(e.raw, "command", command); err != nil {
		return err
	}
	if err := setJSONValue(e.raw, "args", args); err != nil {
		return err
	}
	e.Command = command
	e.Args = args
	return nil
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
		command, args, url, env := e.values()
		out = append(out, MCPServerEntry{
			Name: n, Command: command, Args: args,
			URL: url, Env: env,
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
