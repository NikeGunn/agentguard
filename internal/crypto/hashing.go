// Package crypto holds AgentGuard's signing and attestation helpers: stable
// content hashing (used by Stage-2 attestation to detect tool-schema
// rug-pulls) and a thin wrapper over the sigstore `cosign` CLI for verifying
// release-artifact signatures.
//
// Hashing here is deliberately deterministic: the same logical input always
// produces the same digest regardless of map ordering or insignificant
// whitespace, so a hash stored today still matches an equal schema seen
// tomorrow.
package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// SHA256Hex returns the lowercase hex SHA-256 of b.
func SHA256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// SHA256HexString is the string convenience form of SHA256Hex.
func SHA256HexString(s string) string {
	return SHA256Hex([]byte(s))
}

// CanonicalJSON returns a deterministic JSON encoding of v: object keys are
// sorted recursively and insignificant whitespace is removed. Two values that
// are semantically equal produce byte-identical output, which is what makes a
// hash over the result stable across runs.
//
// It works by round-tripping through encoding/json's `any` model (maps become
// map[string]any, which we then re-encode key-sorted). This handles arbitrary
// nested structures — exactly what MCP `inputSchema` blobs are.
func CanonicalJSON(v any) ([]byte, error) {
	// Normalise concrete Go types into the generic any-tree first so that a
	// struct and an equivalent map hash identically.
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	return marshalCanonical(generic)
}

// marshalCanonical encodes an any-tree with sorted object keys. We hand-roll
// the encoder rather than rely on json.Marshal's map-key sorting alone because
// we also want to recurse deterministically into slices and avoid re-escaping
// surprises.
func marshalCanonical(v any) ([]byte, error) {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf := make([]byte, 0, 64)
		buf = append(buf, '{')
		for i, k := range keys {
			if i > 0 {
				buf = append(buf, ',')
			}
			kb, err := json.Marshal(k)
			if err != nil {
				return nil, err
			}
			buf = append(buf, kb...)
			buf = append(buf, ':')
			vb, err := marshalCanonical(t[k])
			if err != nil {
				return nil, err
			}
			buf = append(buf, vb...)
		}
		buf = append(buf, '}')
		return buf, nil
	case []any:
		buf := make([]byte, 0, 32)
		buf = append(buf, '[')
		for i, e := range t {
			if i > 0 {
				buf = append(buf, ',')
			}
			eb, err := marshalCanonical(e)
			if err != nil {
				return nil, err
			}
			buf = append(buf, eb...)
		}
		buf = append(buf, ']')
		return buf, nil
	default:
		// Scalars (string, float64, bool, nil) encode unambiguously.
		return json.Marshal(t)
	}
}

// ToolDef is the subset of an MCP tool definition that matters for rug-pull
// detection: a server that silently changes a tool's behaviour will change one
// of these three fields.
type ToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"inputSchema,omitempty"`
}

// HashToolSchema returns a deterministic SHA-256 over a tool's
// (name, description, inputSchema), independent of input-schema key ordering.
// Stage-2 attestation stores this per tool and compares it on every
// subsequent tools/list to catch schema drift.
func HashToolSchema(name, description string, inputSchema any) (string, error) {
	canonical, err := CanonicalJSON(ToolDef{
		Name:        name,
		Description: description,
		InputSchema: inputSchema,
	})
	if err != nil {
		return "", err
	}
	return SHA256Hex(canonical), nil
}

// HashToolList returns a single deterministic digest over a whole set of tool
// definitions, sorted by tool name so the order the server lists them in does
// not affect the hash. This is the server-level attestation fingerprint.
func HashToolList(tools []ToolDef) (string, error) {
	sorted := make([]ToolDef, len(tools))
	copy(sorted, tools)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	canonical, err := CanonicalJSON(sorted)
	if err != nil {
		return "", err
	}
	return SHA256Hex(canonical), nil
}
