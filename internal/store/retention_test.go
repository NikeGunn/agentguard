package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// insertCall writes a tool_call (and one stage) directly with an explicit
// started_at so the test controls age. Uses the DB, not the writer, to be
// synchronous and precise.
func insertCall(t *testing.T, s *Store, sessID, srvID string, startedAt int64) string {
	t.Helper()
	callID := NewID()
	_, err := s.DB.Exec(`INSERT INTO tool_calls
		(id, session_id, server_id, tool_name, direction, started_at, verdict)
		VALUES (?,?,?,?,?,?,?)`,
		callID, sessID, srvID, "create_issue", "outbound", startedAt, "allow")
	require.NoError(t, err)
	_, err = s.DB.Exec(`INSERT INTO tool_call_stages
		(id, tool_call_id, stage, stage_order, started_at_ns, duration_ns, outcome)
		VALUES (?,?,?,?,?,?,?)`,
		NewID(), callID, "policy", 0, 0, 1000, "pass")
	require.NoError(t, err)
	return callID
}

func seedSessionAndServer(t *testing.T, s *Store) (string, string) {
	t.Helper()
	sessID, srvID := NewID(), NewID()
	now := NowMS()
	_, err := s.DB.Exec(`INSERT INTO sessions (id, started_at, mode) VALUES (?,?,?)`, sessID, now, "enforce")
	require.NoError(t, err)
	_, err = s.DB.Exec(`INSERT INTO mcp_servers (id, name, canonical_uri, transport, first_seen_at, last_seen_at)
		VALUES (?,?,?,?,?,?)`, srvID, "github", "test://gh", "stdio", now, now)
	require.NoError(t, err)
	return sessID, srvID
}

func TestRetention_DeletesExpiredAndCascades(t *testing.T) {
	s := tempStore(t)
	defer s.Close()
	ctx := context.Background()
	sessID, srvID := seedSessionAndServer(t, s)

	now := time.Now()
	fresh := insertCall(t, s, sessID, srvID, now.UnixMilli())
	old := insertCall(t, s, sessID, srvID, now.Add(-40*24*time.Hour).UnixMilli())

	job := NewRetentionJob(s, Retention{Days: 30})
	res, err := job.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), res.DeletedCalls, "exactly the 40-day-old call should be deleted")
	require.True(t, res.Checkpointed)

	// Fresh call survives; old call and its cascaded stage are gone.
	require.Equal(t, 1, countRows(t, s, `SELECT COUNT(*) FROM tool_calls`))
	require.Equal(t, 1, countRows(t, s, `SELECT COUNT(*) FROM tool_calls WHERE id=?`, fresh))
	require.Equal(t, 0, countRows(t, s, `SELECT COUNT(*) FROM tool_calls WHERE id=?`, old))
	require.Equal(t, 0, countRows(t, s, `SELECT COUNT(*) FROM tool_call_stages WHERE tool_call_id=?`, old),
		"stages of a deleted call must cascade away")
	require.Equal(t, 1, countRows(t, s, `SELECT COUNT(*) FROM tool_call_stages`),
		"the fresh call's stage must remain")
}

func TestRetention_DaysZeroKeepsEverything(t *testing.T) {
	s := tempStore(t)
	defer s.Close()
	sessID, srvID := seedSessionAndServer(t, s)
	insertCall(t, s, sessID, srvID, time.Now().Add(-365*24*time.Hour).UnixMilli())

	job := NewRetentionJob(s, Retention{Days: 0})
	res, err := job.RunOnce(context.Background())
	require.NoError(t, err)
	require.Zero(t, res.DeletedCalls)
	require.Equal(t, 1, countRows(t, s, `SELECT COUNT(*) FROM tool_calls`))
}

func TestRetention_VacuumCadence(t *testing.T) {
	s := tempStore(t)
	defer s.Close()
	seedSessionAndServer(t, s)

	clock := time.Now()
	job := NewRetentionJob(s, Retention{Days: 30, VacuumEvery: 7 * 24 * time.Hour})
	job.now = func() time.Time { return clock }

	// First pass vacuums (lastVacuum is zero, so elapsed is huge).
	res, err := job.RunOnce(context.Background())
	require.NoError(t, err)
	require.True(t, res.Vacuumed, "first pass should vacuum")

	// A pass one day later must NOT vacuum again.
	clock = clock.Add(24 * time.Hour)
	res, err = job.RunOnce(context.Background())
	require.NoError(t, err)
	require.False(t, res.Vacuumed, "vacuum should respect the weekly cadence")

	// Eight days after the first vacuum it should vacuum again.
	clock = clock.Add(7 * 24 * time.Hour)
	res, err = job.RunOnce(context.Background())
	require.NoError(t, err)
	require.True(t, res.Vacuumed, "vacuum should run again after the interval elapses")
}

func TestRetention_DefaultsFillZeroes(t *testing.T) {
	s := tempStore(t)
	defer s.Close()
	job := NewRetentionJob(s, Retention{Days: 30}) // no Interval/VacuumEvery
	require.Equal(t, 24*time.Hour, job.cfg.Interval)
	require.Equal(t, 7*24*time.Hour, job.cfg.VacuumEvery)
}

func countRows(t *testing.T, s *Store, q string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, s.DB.QueryRow(q, args...).Scan(&n))
	return n
}
