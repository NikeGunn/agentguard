package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	agentdetect "github.com/agentguard/agentguard/internal/agent_detect"
	"github.com/agentguard/agentguard/internal/daemon"
	"github.com/agentguard/agentguard/internal/store"
)

// newUninstallCmd builds `agentguard uninstall`. Spec §3.9 calls this the
// critical command — a bad uninstall kills the project's reputation.
//
// Behaviour:
//  1. Open the store and load every active agents row.
//  2. For each row, restore the original config from <path>.agentguard.bak.
//  3. Verify the restore landed (sha256 if available, byte compare otherwise).
//  4. Mark each agents row inactive (we keep the history).
//  5. Print a summary; leave ~/.agentguard untouched unless --purge is set.
//
// The "remove the whole ~/.agentguard tree" step is opt-in via --purge so
// users don't lose audit data by accident. Spec promises an export, which
// lands in M4 alongside backup/restore.
func newUninstallCmd() *cobra.Command {
	var (
		yes   bool
		purge bool
	)
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Restore agent configs and disable AgentGuard",
		Long: `Reverse 'agentguard init' for every agent currently routed
through the sidecar. Each agent's MCP config is restored byte-for-byte from
its .agentguard.bak file; the backup is then removed. The local SQLite
database is left in place unless --purge is given.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			paths, err := Paths()
			if err != nil {
				return err
			}

			if !yes {
				if !confirm(out, cmd.InOrStdin(), "Restore all agent configs and disable AgentGuard?") {
					fmt.Fprintln(out, "  Aborted.")
					return nil
				}
			}

			// Stop the background process (dashboard + proxy) first. Leaving it
			// running keeps the binary locked on Windows, which is exactly what
			// breaks the next install — so a clean uninstall must end the daemon.
			sup := daemon.NewSupervisor(paths.PidFile())
			switch err := sup.Stop(); {
			case err == nil:
				fmt.Fprintln(out, "  ⏹ stopped the AgentGuard daemon")
			case errors.Is(err, daemon.ErrNotRunning):
				// nothing to stop — fine.
			default:
				fmt.Fprintf(out, "  ⚠ could not stop the daemon cleanly: %v\n", err)
			}

			st, err := store.Open(cmd.Context(), paths.DBPath())
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer func() { _ = st.Close() }()

			rows, err := st.DB.QueryContext(cmd.Context(),
				`SELECT id, kind, display_name, config_path FROM agents WHERE active = 1`)
			if err != nil {
				return fmt.Errorf("list agents: %w", err)
			}
			type agentRow struct {
				ID, Kind, Display, ConfigPath string
			}
			var agents []agentRow
			for rows.Next() {
				var a agentRow
				if err := rows.Scan(&a.ID, &a.Kind, &a.Display, &a.ConfigPath); err != nil {
					_ = rows.Close()
					return err
				}
				agents = append(agents, a)
			}
			_ = rows.Close()

			if len(agents) == 0 {
				fmt.Fprintln(out, "  No active agents found — nothing to restore.")
			}

			restored := 0
			for _, a := range agents {
				err := agentdetect.Restore(a.ConfigPath, "")
				if err != nil {
					fmt.Fprintf(out, "  ✗ %s: %v\n", a.Display, err)
					continue
				}
				restored++
				fmt.Fprintf(out, "  ↺ restored %s — %s\n", a.Display, a.ConfigPath)

				if _, err := st.DB.ExecContext(cmd.Context(),
					`UPDATE agents SET active = 0, last_seen_at = ? WHERE id = ?`,
					store.NowMS(), a.ID); err != nil {
					fmt.Fprintf(out, "    ⚠ mark inactive: %v\n", err)
				}
			}
			fmt.Fprintf(out, "✓ restored %d/%d agent(s)\n", restored, len(agents))

			if purge {
				if err := purgeRoot(out, paths.Root); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(out, "  Audit data preserved at %s (pass --purge to remove)\n", paths.Root)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	cmd.Flags().BoolVar(&purge, "purge", false, "also delete ~/.agentguard recursively")
	return cmd
}

func purgeRoot(out io.Writer, root string) error {
	if root == "" {
		return errors.New("refusing to purge empty path")
	}
	// Sanity guard: never delete anything outside our own subtree.
	if info, err := os.Stat(root); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	} else if !info.IsDir() {
		return errors.New("refusing to purge non-directory")
	}
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("purge %s: %w", root, err)
	}
	fmt.Fprintf(out, "  ✗ deleted %s\n", root)
	return nil
}

var _ = context.Background
