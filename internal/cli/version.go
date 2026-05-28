package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agentguard/agentguard/internal/version"
)

// newVersionCmd renders the version + commit + build date.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "agentguard %s\n", version.String())
			return err
		},
	}
}
