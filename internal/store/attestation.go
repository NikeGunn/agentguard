package store

import (
	"context"
	"database/sql"
	"errors"
)

// GetServerHash returns the most recently stored tool-list hash for a
// server, or ("", false, nil) if none exists. We reuse the
// server_attestations.raw_attestation column to carry the JSON
// {"tools_hash":"..."} payload so no schema change is needed.
func (s *Store) GetServerHash(ctx context.Context, serverID string) (string, bool, error) {
	var raw sql.NullString
	err := s.DB.QueryRowContext(ctx, `
		SELECT raw_attestation FROM server_attestations
		WHERE server_id = ? AND raw_attestation IS NOT NULL
		ORDER BY attested_at DESC LIMIT 1`, serverID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if !raw.Valid {
		return "", false, nil
	}
	h := extractToolsHash(raw.String)
	return h, h != "", nil
}

// SetServerHash inserts a new server_attestations row carrying the new
// tools hash. We always insert (not update) so drift is auditable.
func (s *Store) SetServerHash(ctx context.Context, serverID, hash string) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO server_attestations
			(id, server_id, attested_at, raw_attestation)
		VALUES (?, ?, ?, ?)`,
		NewID(), serverID, NowMS(),
		`{"tools_hash":"`+hash+`"}`,
	)
	return err
}

// extractToolsHash pulls "tools_hash":"..." out of a tiny JSON object
// without paying for a full unmarshal. The format is fully owned by us.
func extractToolsHash(s string) string {
	const key = `"tools_hash":"`
	i := indexOfFast(s, key)
	if i < 0 {
		return ""
	}
	rest := s[i+len(key):]
	for j := 0; j < len(rest); j++ {
		if rest[j] == '"' {
			return rest[:j]
		}
	}
	return ""
}

func indexOfFast(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
