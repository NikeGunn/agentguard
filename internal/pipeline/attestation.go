package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/agentguard/agentguard/internal/crypto"
)

// AttestationStore is the persistence interface the attestation stage
// expects. The store package's *Store satisfies this; tests use an
// in-memory implementation.
type AttestationStore interface {
	GetServerHash(ctx context.Context, serverID string) (string, bool, error)
	SetServerHash(ctx context.Context, serverID, hash string) error
}

// AttestationStage detects MCP server "rug pulls" — silent tool-schema
// changes between sessions. On the first tools/list response for a
// server we compute a stable SHA-256 over the (name, description,
// inputSchema) triples and store it. Subsequent sessions compare; a
// mismatch yields VerdictFlag with the old and new hashes.
type AttestationStage struct {
	store AttestationStore
	mu    sync.Mutex
	cache map[string]string
}

func NewAttestationStage(s AttestationStore) *AttestationStage {
	return &AttestationStage{store: s, cache: make(map[string]string)}
}

func (s *AttestationStage) Name() string { return "attestation" }

func (s *AttestationStage) Run(ctx context.Context, m *Message) StageResult {
	if m.Direction != Inbound {
		return StageResult{Verdict: VerdictPass}
	}
	if m.Method != "tools/list" {
		return StageResult{Verdict: VerdictPass}
	}
	if m.ServerName == "" {
		return StageResult{Verdict: VerdictSkip, Reason: "no server id"}
	}
	hash, err := hashToolsList(m.Raw)
	if err != nil {
		return StageResult{Verdict: VerdictSkip, Reason: "parse: " + err.Error()}
	}

	s.mu.Lock()
	prev, ok := s.cache[m.ServerName]
	s.mu.Unlock()
	if !ok && s.store != nil {
		if p, found, err := s.store.GetServerHash(ctx, m.ServerName); err == nil && found {
			prev = p
			ok = true
		}
	}

	if !ok {
		s.persist(ctx, m.ServerName, hash)
		return StageResult{
			Verdict: VerdictPass,
			Reason:  "first seen",
			Detail:  fmt.Sprintf(`{"hash":"%s"}`, hash),
		}
	}

	if prev == hash {
		return StageResult{Verdict: VerdictPass}
	}

	s.persist(ctx, m.ServerName, hash)
	return StageResult{
		Verdict: VerdictFlag,
		Reason:  "tool schema drift",
		Detail:  fmt.Sprintf(`{"old_hash":"%s","new_hash":"%s"}`, prev, hash),
	}
}

func (s *AttestationStage) persist(ctx context.Context, serverID, hash string) {
	s.mu.Lock()
	s.cache[serverID] = hash
	s.mu.Unlock()
	if s.store != nil {
		_ = s.store.SetServerHash(ctx, serverID, hash)
	}
}

// hashToolsList parses a JSON-RPC tools/list response and returns a
// deterministic SHA-256 of the (sorted) tool definitions. The actual hashing
// lives in internal/crypto so the same canonicalisation is reused everywhere
// (DRY) and is independent of JSON key ordering.
func hashToolsList(raw []byte) (string, error) {
	var env struct {
		Result struct {
			Tools []map[string]any `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return "", err
	}
	defs := make([]crypto.ToolDef, 0, len(env.Result.Tools))
	for _, t := range env.Result.Tools {
		name, _ := t["name"].(string)
		desc, _ := t["description"].(string)
		defs = append(defs, crypto.ToolDef{Name: name, Description: desc, InputSchema: t["inputSchema"]})
	}
	return crypto.HashToolList(defs)
}
