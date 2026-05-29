package cli

import (
	"context"
	"path/filepath"
	"testing"

	agentdetect "github.com/agentguard/agentguard/internal/agent_detect"
	"github.com/agentguard/agentguard/internal/store"
)

// Regression: re-running init for the same agent kind but a DIFFERENT config
// path must update the recorded config_path/config_backup — not keep the stale
// ones. The original ON CONFLICT(id) clause only touched last_seen_at/active,
// so a second init left the DB pointing at the first config. uninstall then
// read the wrong path and restored the wrong (or a missing) backup, leaving the
// actually-patched file routed through `agentguard wrap` forever.
func TestUpsertAgentRow_UpdatesConfigPathOnReinit(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "agentguard.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = st.Close() }()

	first := &agentdetect.Detection{
		Kind:        agentdetect.KindClaudeCode,
		DisplayName: "Claude Code",
		ConfigPath:  `C:\Users\alice\.claude.json`,
	}
	if err := upsertAgentRow(ctx, st, first, ""); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	// Same kind (same primary key) but the config now lives elsewhere.
	second := &agentdetect.Detection{
		Kind:        agentdetect.KindClaudeCode,
		DisplayName: "Claude Code",
		ConfigPath:  `D:\profiles\bob\.claude.json`,
	}
	if err := upsertAgentRow(ctx, st, second, ""); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	var gotPath, gotBackup string
	var rows int
	if err := st.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM agents WHERE id = ?`, string(agentdetect.KindClaudeCode)).
		Scan(&rows); err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected exactly 1 row for the kind, got %d", rows)
	}
	if err := st.DB.QueryRowContext(ctx,
		`SELECT config_path, config_backup FROM agents WHERE id = ?`,
		string(agentdetect.KindClaudeCode)).Scan(&gotPath, &gotBackup); err != nil {
		t.Fatalf("select: %v", err)
	}

	if gotPath != second.ConfigPath {
		t.Errorf("config_path not updated on re-init: got %q, want %q", gotPath, second.ConfigPath)
	}
	wantBackup := second.ConfigPath + agentdetect.BackupSuffix
	if gotBackup != wantBackup {
		t.Errorf("config_backup not updated on re-init: got %q, want %q", gotBackup, wantBackup)
	}
}
