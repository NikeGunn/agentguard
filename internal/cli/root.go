// Package cli builds the cobra command tree.
// Milestone 1: wrap, version.
// Milestone 2: pack flag on wrap (no new command).
// Milestone 3: init, uninstall, doctor, tail, scan.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/agentguard/agentguard/internal/version"
)

// NewRoot returns the root `agentguard` command with every milestone-1 child
// attached.
func NewRoot() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "agentguard",
		Short:         "Zero-trust security sidecar for AI agents",
		Long:          "AgentGuard proxies MCP/A2A traffic and blocks prompt injection, tool poisoning, rug-pulls, runaway loops, and credential exfiltration before they reach your model.",
		Version:       version.String(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetVersionTemplate("agentguard {{.Version}}\n")
	cmd.AddCommand(newWrapCmd())
	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newUninstallCmd())
	cmd.AddCommand(newDoctorCmd())
	cmd.AddCommand(newTailCmd())
	cmd.AddCommand(newScanCmd())
	cmd.AddCommand(newDashboardCmd())
	cmd.AddCommand(newReplayCmd())
	cmd.AddCommand(newPackCmd())
	return cmd
}
