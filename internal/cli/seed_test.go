package cli

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agentguard/agentguard/internal/store"
)

// TestSeedHistoryProducesVariedTaggedData drives the seeder directly (not the
// cobra command) so it can assert on the resulting rows: a believable verdict
// mix, the scripted security scenarios, stage waterfalls, trust scores, and
// the demo tag that --reset keys off.
func TestSeedHistoryProducesVariedTaggedData(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "seed.db")

	s, err := store.Open(ctx, dbPath)
	require.NoError(t, err)
	sd := newSeeder(s)
	require.NoError(t, sd.ensureServers(ctx))
	require.NoError(t, sd.ensureAgents(ctx))
	sd.history(150)
	require.NoError(t, s.Close()) // drain

	s2, err := store.Open(ctx, dbPath)
	require.NoError(t, err)
	defer s2.Close()
	sd.s = s2
	require.NoError(t, sd.applyTrustScores(ctx))

	count := func(q string, args ...any) int64 {
		t.Helper()
		var n int64
		require.NoError(t, s2.DB.QueryRowContext(ctx, q, args...).Scan(&n))
		return n
	}

	require.EqualValues(t, 150, count(`SELECT COUNT(*) FROM tool_calls`))
	// All four verdict classes must be present so the donut has every slice.
	for _, v := range []string{"allow", "block", "flag", "transform"} {
		require.Positive(t, count(`SELECT COUNT(*) FROM tool_calls WHERE verdict=?`, v),
			"expected at least one %q verdict", v)
	}
	// Every call has a stage waterfall.
	require.Positive(t, count(`SELECT COUNT(*) FROM tool_call_stages`))
	// The scripted injection scenarios are guaranteed present.
	require.Positive(t, count(
		`SELECT COUNT(*) FROM tool_calls WHERE verdict_reason LIKE '%injection%'`))
	require.Positive(t, count(
		`SELECT COUNT(*) FROM tool_calls WHERE verdict_reason LIKE '%schema-drift%'`))
	require.Positive(t, count(
		`SELECT COUNT(*) FROM tool_calls WHERE verdict_reason LIKE '%loop-detected%'`))
	// Two low-trust servers for the trust surface.
	require.EqualValues(t, 2, count(`SELECT COUNT(*) FROM mcp_servers WHERE trust_score < 50`))
	require.EqualValues(t, 4, count(`SELECT COUNT(*) FROM mcp_servers WHERE trust_score IS NOT NULL`))
	// Active agents for the overview KPI.
	require.EqualValues(t, 2, count(`SELECT COUNT(*) FROM agents WHERE active=1`))
	// Everything is demo-tagged.
	require.EqualValues(t, 1, count(`SELECT COUNT(*) FROM sessions WHERE client_user='demo'`))
}

func TestResetDemoRemovesEverything(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "reset.db")

	s, err := store.Open(ctx, dbPath)
	require.NoError(t, err)
	sd := newSeeder(s)
	require.NoError(t, sd.ensureServers(ctx))
	require.NoError(t, sd.ensureAgents(ctx))
	sd.history(40)
	require.NoError(t, s.Close())

	s2, err := store.Open(ctx, dbPath)
	require.NoError(t, err)
	defer s2.Close()

	n, err := resetDemo(ctx, s2)
	require.NoError(t, err)
	require.EqualValues(t, 1, n)

	var calls int64
	require.NoError(t, s2.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM tool_calls`).Scan(&calls))
	require.Zero(t, calls, "cascade delete should remove all demo tool calls")
}
