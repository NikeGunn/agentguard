package cli

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agentguard/agentguard/internal/store"
)

// insertPolicy adds a policy row directly so the test controls the fixture.
func insertPolicy(ctx context.Context, t *testing.T, s *store.Store, name string, enabled int, priority int) string {
	t.Helper()
	id := store.NewID()
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO policies (id, name, source, scope, enabled, priority, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?)`,
		id, name, "builtin", "global", enabled, priority, store.NowMS(), store.NowMS())
	require.NoError(t, err)
	return id
}

func newPolicyTestStore(t *testing.T) (context.Context, *store.Store) {
	t.Helper()
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "policy.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return ctx, s
}

func TestQueryPoliciesOrdersByPriority(t *testing.T) {
	ctx, s := newPolicyTestStore(t)
	insertPolicy(ctx, t, s, "zeta", 1, 200)
	insertPolicy(ctx, t, s, "alpha", 0, 100)

	rows, err := queryPolicies(ctx, s.DB, "")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	// priority ASC: alpha (100) before zeta (200).
	require.Equal(t, "alpha", rows[0].name)
	require.False(t, rows[0].enabled)
	require.Equal(t, "zeta", rows[1].name)
	require.True(t, rows[1].enabled)
}

func TestQueryPoliciesWhere(t *testing.T) {
	ctx, s := newPolicyTestStore(t)
	insertPolicy(ctx, t, s, "only-me", 1, 100)
	insertPolicy(ctx, t, s, "other", 1, 100)

	rows, err := queryPolicies(ctx, s.DB, "name = ?", "only-me")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "only-me", rows[0].name)
}

func TestSetEnabledTogglesAndAudits(t *testing.T) {
	ctx, s := newPolicyTestStore(t)
	insertPolicy(ctx, t, s, "block-secrets", 1, 100)

	require.NoError(t, setEnabled(ctx, s, "block-secrets", false))

	var enabled int
	require.NoError(t, s.DB.QueryRowContext(ctx,
		`SELECT enabled FROM policies WHERE name = ?`, "block-secrets").Scan(&enabled))
	require.Equal(t, 0, enabled)

	// An audit_log row records the disable.
	var n int
	require.NoError(t, s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE action = ? AND target_id = ?`,
		"policy.disable", "block-secrets").Scan(&n))
	require.Equal(t, 1, n)

	// Re-enable writes a policy.enable audit row.
	require.NoError(t, setEnabled(ctx, s, "block-secrets", true))
	require.NoError(t, s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE action = ?`, "policy.enable").Scan(&n))
	require.Equal(t, 1, n)
}

func TestSetEnabledUnknownPolicy(t *testing.T) {
	ctx, s := newPolicyTestStore(t)
	err := setEnabled(ctx, s, "does-not-exist", true)
	require.Error(t, err)
}
