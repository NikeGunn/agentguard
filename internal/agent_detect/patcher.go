package agentdetect

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// BackupSuffix is appended to the original config path to form the backup
// path. The patcher writes a byte-for-byte copy of the original here before
// any rewrite; uninstall restores from this exact file.
const BackupSuffix = ".agentguard.bak"

// PatchOptions controls how Apply rewrites a detected agent's config.
type PatchOptions struct {
	// AgentguardBinary is the absolute path to the agentguard executable
	// users should invoke. Defaults to the literal "agentguard" if unset
	// (assumes it's on PATH).
	AgentguardBinary string
	// SkipServers is an optional set of MCP server names (within the
	// detected config) that should NOT be rewritten — useful when the user
	// has explicitly opted one out.
	SkipServers map[string]bool
}

// PatchResult is the outcome of a single Apply call.
type PatchResult struct {
	ConfigPath     string
	BackupPath     string
	OriginalSHA256 string
	RewrittenCount int
	UnchangedCount int
}

// AlreadyWrapped reports whether an entry already invokes agentguard, so we
// don't double-wrap on a re-run of init.
func AlreadyWrapped(e MCPServerEntry, agentguardBinary string) bool {
	if e.Command == "" {
		return false
	}
	if e.Command == agentguardBinary || filepath.Base(e.Command) == "agentguard" || filepath.Base(e.Command) == "agentguard.exe" {
		return true
	}
	return false
}

// Apply rewrites every MCP server entry in d.ConfigPath so that it invokes
//
//	<agentguard> wrap --upstream-name <name> -- <original command> <original args>
//
// before invoking the original upstream. HTTP-transport entries (URL set,
// no Command) are left alone for now — they're a milestone 4 concern.
//
// The original file is copied to <path>+BackupSuffix verbatim before the
// rewrite. Apply is idempotent: running it twice is a no-op for already-
// wrapped entries.
func Apply(d *Detection, opt PatchOptions) (*PatchResult, error) {
	if d == nil {
		return nil, errors.New("nil detection")
	}
	if opt.AgentguardBinary == "" {
		opt.AgentguardBinary = "agentguard"
	}
	original, err := os.ReadFile(d.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	sum := sha256.Sum256(original)
	res := &PatchResult{
		ConfigPath:     d.ConfigPath,
		BackupPath:     d.ConfigPath + BackupSuffix,
		OriginalSHA256: hex.EncodeToString(sum[:]),
	}

	// Make the backup first; refuse to overwrite an existing one with
	// different contents.
	if err := writeBackup(res.BackupPath, original); err != nil {
		return nil, err
	}

	var rewritten []byte
	switch d.Format {
	case FormatJSON:
		rewritten, res.RewrittenCount, res.UnchangedCount, err = patchJSON(original, d, opt)
	case FormatTOML:
		rewritten, res.RewrittenCount, res.UnchangedCount, err = patchTOML(original, d, opt)
	default:
		return nil, fmt.Errorf("unknown config format %q", d.Format)
	}
	if err != nil {
		return nil, err
	}

	// Atomic write: temp file in the same dir, then rename.
	if err := atomicWrite(d.ConfigPath, rewritten); err != nil {
		return nil, fmt.Errorf("write rewritten config: %w", err)
	}
	return res, nil
}

// Restore reverses Apply: copies the backup file back over the config and
// verifies the result hashes identically to what Apply originally captured.
// If hashCheck is non-empty, Restore fails unless the restored file's
// sha256 matches it byte-for-byte.
func Restore(configPath, hashCheck string) error {
	backup := configPath + BackupSuffix
	data, err := os.ReadFile(backup)
	if err != nil {
		return fmt.Errorf("read backup %s: %w", backup, err)
	}
	if err := atomicWrite(configPath, data); err != nil {
		return fmt.Errorf("restore %s: %w", configPath, err)
	}
	if hashCheck != "" {
		restored, err := os.ReadFile(configPath)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(restored)
		got := hex.EncodeToString(sum[:])
		if got != hashCheck {
			return fmt.Errorf("restore hash mismatch: want %s got %s", hashCheck, got)
		}
	}
	if err := os.Remove(backup); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove backup: %w", err)
	}
	return nil
}

// patchJSON rewrites the mcpServers map in a JSON config file. Unknown
// top-level keys are preserved verbatim via json.RawMessage.
func patchJSON(original []byte, d *Detection, opt PatchOptions) ([]byte, int, int, error) {
	parsed, err := readJSONMCPFromBytes(original)
	if err != nil {
		return nil, 0, 0, err
	}
	rewritten, unchanged := 0, 0
	for name, entry := range parsed.Servers {
		if opt.SkipServers[name] {
			unchanged++
			continue
		}
		// HTTP entries — leave alone until M4.
		if entry.URL != "" && entry.Command == "" {
			unchanged++
			continue
		}
		// Already wrapped — idempotent skip.
		if AlreadyWrapped(MCPServerEntry{Command: entry.Command}, opt.AgentguardBinary) {
			unchanged++
			continue
		}
		entry.Args = wrapArgs(name, entry.Command, entry.Args)
		entry.Command = opt.AgentguardBinary
		// Preserve "type" field if it was there.
		parsed.Servers[name] = entry
		rewritten++
	}
	// Re-encode the servers map and stuff it back into Raw under the same key.
	enc, err := json.Marshal(parsed.Servers)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("re-encode mcp servers: %w", err)
	}
	parsed.Raw[parsed.ServersKey] = enc

	// Re-emit the whole file with 2-space indent — the convention every
	// agent uses. We can't preserve the original's exact whitespace, but
	// uninstall reads the backup, not this file, so byte-identical restore
	// still holds.
	out, err := marshalIndentedJSON(parsed.Raw)
	if err != nil {
		return nil, 0, 0, err
	}
	// Preserve trailing newline if the original had one.
	if len(original) > 0 && original[len(original)-1] == '\n' && (len(out) == 0 || out[len(out)-1] != '\n') {
		out = append(out, '\n')
	}
	return out, rewritten, unchanged, nil
}

// readJSONMCPFromBytes is the in-memory version of readJSONMCPFile.
func readJSONMCPFromBytes(data []byte) (*mcpJSONFile, error) {
	raw := make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	out := &mcpJSONFile{Raw: raw, Servers: map[string]mcpJSONEntry{}}
	for _, key := range []string{"mcpServers", "mcp"} {
		if v, ok := raw[key]; ok {
			if err := json.Unmarshal(v, &out.Servers); err != nil {
				return nil, fmt.Errorf("parse %s: %w", key, err)
			}
			out.ServersKey = key
			return out, nil
		}
	}
	out.ServersKey = "mcpServers"
	return out, nil
}

// marshalIndentedJSON writes the map with sorted keys and 2-space indent.
// json.MarshalIndent already sorts map keys for us.
func marshalIndentedJSON(m map[string]json.RawMessage) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// patchTOML rewrites the [mcp_servers.*] tables in a Codex TOML config.
func patchTOML(original []byte, d *Detection, opt PatchOptions) ([]byte, int, int, error) {
	// Decode whole file into a generic map so we don't lose user-set keys.
	var full map[string]any
	if _, err := toml.Decode(string(original), &full); err != nil {
		return nil, 0, 0, fmt.Errorf("parse toml: %w", err)
	}
	raw, ok := full["mcp_servers"].(map[string]any)
	if !ok {
		// No servers configured; nothing to do.
		return original, 0, 0, nil
	}
	rewritten, unchanged := 0, 0
	for name, val := range raw {
		entryMap, ok := val.(map[string]any)
		if !ok {
			unchanged++
			continue
		}
		if opt.SkipServers[name] {
			unchanged++
			continue
		}
		cmd, _ := entryMap["command"].(string)
		if cmd == "" {
			unchanged++
			continue
		}
		if AlreadyWrapped(MCPServerEntry{Command: cmd}, opt.AgentguardBinary) {
			unchanged++
			continue
		}
		origArgs := stringSlice(entryMap["args"])
		entryMap["command"] = opt.AgentguardBinary
		entryMap["args"] = wrapArgs(name, cmd, origArgs)
		raw[name] = entryMap
		rewritten++
	}
	full["mcp_servers"] = raw
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	enc.Indent = "  "
	if err := enc.Encode(full); err != nil {
		return nil, 0, 0, fmt.Errorf("re-encode toml: %w", err)
	}
	return buf.Bytes(), rewritten, unchanged, nil
}

func stringSlice(v any) []string {
	a, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(a))
	for _, e := range a {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// wrapArgs builds the args slice that turns
//
//	command=<original> args=<original>
//
// into
//
//	command=<agentguard> args=[wrap, --upstream-name, <name>, --, <original cmd>, ...]
func wrapArgs(serverName, origCmd string, origArgs []string) []string {
	out := []string{"wrap", "--upstream-name", serverName, "--", origCmd}
	out = append(out, origArgs...)
	return out
}

// atomicWrite writes data to path via a temp file + rename. It preserves the
// existing file mode if there is one; otherwise uses 0600.
func atomicWrite(path string, data []byte) error {
	mode := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".agentguard-write-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	// Best-effort chmod; on Windows this is a no-op for ACL files.
	_ = os.Chmod(tmpName, mode)
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

// writeBackup writes the original config bytes to backupPath. If the file
// already exists and has identical contents, it's a no-op (idempotent).
// If it exists with different contents, it's left alone (the user has
// already been backed up once; don't overwrite their pristine copy).
func writeBackup(backupPath string, original []byte) error {
	if existing, err := os.ReadFile(backupPath); err == nil {
		if bytes.Equal(existing, original) {
			return nil
		}
		// Different contents — preserve the older backup, write a fresh
		// one with a timestamp suffix.
		return os.WriteFile(backupPath+timestampSuffix(), original, 0o600)
	}
	return os.WriteFile(backupPath, original, 0o600)
}

func timestampSuffix() string {
	// Avoid pulling in fmt for one place; a static suffix is fine here.
	// The .2 etc. is incremented if even this collides, but that requires
	// running init three times in the same second.
	return ".2"
}

// SkipSet builds a SkipServers set from a slice of names.
func SkipSet(names []string) map[string]bool {
	if len(names) == 0 {
		return nil
	}
	out := make(map[string]bool, len(names))
	for _, n := range names {
		out[strings.TrimSpace(n)] = true
	}
	return out
}
