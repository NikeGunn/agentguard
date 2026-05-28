// Package cli builds the cobra command tree. Only `wrap` and `--version` are
// implemented in milestone 1; the rest of the commands are file-stubs that
// land in subsequent milestones.
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
	return cmd
}
