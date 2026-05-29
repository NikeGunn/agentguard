package store

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Retention configures the nightly cleanup job (database.md §7). The hot
// event tables roll off after Days; the small config/audit tables are never
// auto-deleted.
type Retention struct {
	// Days is the rolling window kept for tool_calls and their children.
	// Zero or negative disables deletion (keep everything).
	Days int
	// VacuumEvery controls how often a full VACUUM runs (expensive). A
	// wal_checkpoint(TRUNCATE) runs on every pass regardless. Default weekly.
	VacuumEvery time.Duration
	// Interval is how often the scheduler fires. Default 24h.
	Interval time.Duration
}

// DefaultRetention is the spec default: 30-day window, weekly vacuum, nightly.
func DefaultRetention() Retention {
	return Retention{Days: 30, VacuumEvery: 7 * 24 * time.Hour, Interval: 24 * time.Hour}
}

// RetentionResult reports what one pass did, for logging and tests.
type RetentionResult struct {
	DeletedCalls int64
	Checkpointed bool
	Vacuumed     bool
}

// RetentionJob runs retention against a Store. Foreign keys are ON with
// ON DELETE CASCADE (see migrations), so deleting a tool_calls row also
// removes its stages and artifacts in one statement.
type RetentionJob struct {
	store      *Store
	cfg        Retention
	lastVacuum time.Time
	now        func() time.Time // injectable clock for tests
}

// NewRetentionJob constructs a job. A zero VacuumEvery/Interval is filled with
// the defaults so callers can pass just Days.
func NewRetentionJob(s *Store, cfg Retention) *RetentionJob {
	if cfg.VacuumEvery <= 0 {
		cfg.VacuumEvery = 7 * 24 * time.Hour
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 24 * time.Hour
	}
	return &RetentionJob{store: s, cfg: cfg, now: time.Now}
}

// RunOnce performs a single retention pass: delete expired event rows,
// checkpoint the WAL, and VACUUM if enough time has elapsed since the last one.
func (j *RetentionJob) RunOnce(ctx context.Context) (RetentionResult, error) {
	var res RetentionResult

	if j.cfg.Days > 0 {
		cutoff := j.now().Add(-time.Duration(j.cfg.Days) * 24 * time.Hour).UnixMilli()
		// Only tool_calls needs a WHERE; stages/artifacts cascade. We delete
		// from tool_calls and rely on FK cascade for the rest.
		r, err := j.store.DB.ExecContext(ctx, `DELETE FROM tool_calls WHERE started_at < ?`, cutoff)
		if err != nil {
			return res, fmt.Errorf("retention delete: %w", err)
		}
		if n, err := r.RowsAffected(); err == nil {
			res.DeletedCalls = n
		}
	}

	// wal_checkpoint(TRUNCATE) keeps the -wal file from growing without bound
	// after a large delete. Cheap; run every pass.
	if _, err := j.store.DB.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return res, fmt.Errorf("retention checkpoint: %w", err)
	}
	res.Checkpointed = true

	if j.now().Sub(j.lastVacuum) >= j.cfg.VacuumEvery {
		// VACUUM cannot run inside a transaction and reclaims free pages left
		// by the delete. It's the slow part, hence the weekly cadence.
		if _, err := j.store.DB.ExecContext(ctx, `VACUUM`); err != nil {
			return res, fmt.Errorf("retention vacuum: %w", err)
		}
		j.lastVacuum = j.now()
		res.Vacuumed = true
	}
	return res, nil
}

// Start runs RunOnce immediately, then on every Interval tick until ctx is
// cancelled. It's meant to be launched as a goroutine by the daemon. Errors
// are logged, not fatal — a failed retention pass must never take down the
// proxy.
func (j *RetentionJob) Start(ctx context.Context) {
	run := func() {
		res, err := j.RunOnce(ctx)
		if err != nil {
			slog.Warn("retention pass failed", slog.Any("err", err))
			return
		}
		slog.Info("retention pass",
			slog.Int64("deleted_calls", res.DeletedCalls),
			slog.Bool("vacuumed", res.Vacuumed))
	}
	run()
	ticker := time.NewTicker(j.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
