package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	agentdetect "github.com/agentguard/agentguard/internal/agent_detect"
	"github.com/agentguard/agentguard/internal/store"
)

// newInitCmd builds `agentguard init`. The goal per requirements §3.1 is
// to take the user from "binary on disk" to "everything protected" in
// under 10 seconds.
//
// Behaviour (milestone 3 surface):
//  1. Create ~/.agentguard/{bin,data,logs,packs,config} with mode 0700.
//  2. Initialise SQLite + run migrations.
//  3. Detect installed agents.
//  4. Back up each detected config and rewrite MCP entries.
//  5. Insert one agents row per detection.
//  6. Print elapsed time.
//
// The interactive Bubble Tea checklist + browser auto-open land in M4 with
// the dashboard. For now --non-interactive is the default and accepts all
// detections.
func newInitCmd() *cobra.Command {
	var (
		nonInteractive bool
		interactive    bool
		dryRun         bool
		skipServers    []string
		homeOverride   string
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Detect installed AI agents and route their MCP traffic through AgentGuard",
		Long: `Find Claude Code, Cursor, Codex, Gemini CLI, and Windsurf on this
machine, back up their MCP configs, and rewrite each entry to invoke
'agentguard wrap'. Every original config is preserved at
<path>.agentguard.bak so 'agentguard uninstall' restores byte-identically.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			started := time.Now()
			out := cmd.OutOrStdout()

			paths, err := Paths()
			if err != nil {
				return err
			}
			home := homeOverride
			if home == "" {
				home, err = os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("resolve home: %w", err)
				}
			}
			fmt.Fprintf(out, "▶ AgentGuard init — root=%s\n", paths.Root)

			// 1. Make the directory tree.
			if !dryRun {
				if err := EnsurePaths(paths); err != nil {
					return err
				}
			}

			// 2. Open the store (runs migrations).
			if !dryRun {
				st, err := store.Open(cmd.Context(), paths.DBPath())
				if err != nil {
					return fmt.Errorf("open store: %w", err)
				}
				defer func() { _ = st.Close() }()
			}

			// 3. Detect agents.
			detections, errs := agentdetect.DetectAll(home)
			for _, e := range errs {
				fmt.Fprintf(out, "  ⚠ %v\n", e)
			}
			if len(detections) == 0 {
				fmt.Fprintln(out, "  · No supported agents detected. Install Claude Code, Cursor, Codex, Gemini CLI, or Windsurf and re-run.")
				return nil
			}

			// 4. Resolve our own binary path (for the wrap invocation).
			selfPath, err := os.Executable()
			if err != nil {
				selfPath = "agentguard"
			}
			selfPath, _ = filepath.Abs(selfPath)

			fmt.Fprintf(out, "  Found %d agent(s):\n", len(detections))
			for _, d := range detections {
				fmt.Fprintf(out, "    ✓ %-12s  %s  (%d server%s)\n",
					d.DisplayName, d.ConfigPath, len(d.Servers), pluralS(len(d.Servers)))
			}

			if interactive {
				kept, aborted, err := runInitTUI(detections)
				if err != nil {
					return fmt.Errorf("tui: %w", err)
				}
				if aborted {
					fmt.Fprintln(out, "  Aborted; no changes made.")
					return nil
				}
				if len(kept) == 0 {
					fmt.Fprintln(out, "  Nothing selected; no changes made.")
					return nil
				}
				detections = kept
			} else if !nonInteractive {
				if !confirm(out, cmd.InOrStdin(), "Patch all of these now?") {
					fmt.Fprintln(out, "  Aborted; no changes made.")
					return nil
				}
			}

			// 5. Patch + record.
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			st, err := store.Open(ctx, paths.DBPath())
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer func() { _ = st.Close() }()

			skip := agentdetect.SkipSet(skipServers)
			totalRewritten, totalUnchanged := 0, 0
			for _, d := range detections {
				if dryRun {
					fmt.Fprintf(out, "  · would patch %s\n", d.ConfigPath)
					continue
				}
				res, err := agentdetect.Apply(d, agentdetect.PatchOptions{
					AgentguardBinary: selfPath,
					SkipServers:      skip,
				})
				if err != nil {
					fmt.Fprintf(out, "  ✗ %s: %v\n", d.DisplayName, err)
					continue
				}
				totalRewritten += res.RewrittenCount
				totalUnchanged += res.UnchangedCount
				fmt.Fprintf(out, "    ↻ %s: %d rewritten, %d unchanged (backup at %s)\n",
					d.DisplayName, res.RewrittenCount, res.UnchangedCount, res.BackupPath)

				// Insert/update the agents row.
				now := store.NowMS()
				st.Writer().Submit(store.Event{
					Kind: store.EventAuditLog,
					Audit: &store.AuditEntry{
						ID:         store.NewID(),
						OccurredAt: now,
						Actor:      "cli",
						Action:     "agent.detected",
						TargetType: strPtr("agent"),
						TargetID:   strPtr(string(d.Kind)),
						Detail:     strPtr(fmt.Sprintf(`{"config":"%s","servers":%d}`, d.ConfigPath, len(d.Servers))),
					},
				})
				if err := upsertAgentRow(ctx, st, d, res.OriginalSHA256); err != nil {
					fmt.Fprintf(out, "    ⚠ persist agent row: %v\n", err)
				}
			}

			elapsed := time.Since(started)
			fmt.Fprintf(out, "✓ done in %s — %d entr%s rewritten, %d already wrapped\n",
				elapsed.Truncate(time.Millisecond), totalRewritten, plural(totalRewritten, "y", "ies"), totalUnchanged)
			fmt.Fprintf(out, "  Database at %s\n", paths.DBPath())
			fmt.Fprintln(out, "  Run 'agentguard tail' to watch traffic, or 'agentguard uninstall' to revert.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", true,
		"accept all detected agents without prompting (default true; pair with --interactive to opt into the TUI)")
	cmd.Flags().BoolVar(&interactive, "interactive", false,
		"show a Bubble Tea checklist so you can pick which agents to patch")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing anything")
	cmd.Flags().StringSliceVar(&skipServers, "skip-server", nil,
		"comma-separated MCP server names to leave untouched")
	cmd.Flags().StringVar(&homeOverride, "home", "",
		"override the home directory (for tests; not normally needed)")
	return cmd
}

// upsertAgentRow inserts or updates the agents row for one detection. We
// write this directly (not via the batched writer) so that downstream code
// like uninstall can read it back synchronously without race windows.
func upsertAgentRow(ctx context.Context, st *store.Store, d *agentdetect.Detection, originalHash string) error {
	now := store.NowMS()
	_, err := st.DB.ExecContext(ctx, `
		INSERT INTO agents (id, kind, display_name, config_path, config_backup,
			detected_at, last_seen_at, active)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT(id) DO UPDATE SET
			last_seen_at = excluded.last_seen_at,
			active = 1`,
		string(d.Kind), string(d.Kind), d.DisplayName,
		d.ConfigPath, d.ConfigPath+agentdetect.BackupSuffix,
		now, now)
	_ = originalHash
	return err
}

// confirm reads a yes/no answer from the user. Anything starting with y or Y
// counts as yes; empty input defaults to yes. Returns false if input is
// closed or the user types n.
func confirm(out io.Writer, in io.Reader, prompt string) bool {
	fmt.Fprintf(out, "  %s [Y/n] ", prompt)
	var buf [16]byte
	n, _ := in.Read(buf[:])
	if n == 0 {
		return true
	}
	c := buf[0]
	return c == '\n' || c == 'y' || c == 'Y' || c == '\r'
}

func plural(n int, singular, pluralWord string) string {
	if n == 1 {
		return singular
	}
	return pluralWord
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func strPtr(s string) *string { return &s }

// errors import kept for future expansion.
var _ = errors.New
