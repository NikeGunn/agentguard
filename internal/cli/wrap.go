package cli

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/agentguard/agentguard/internal/pipeline"
	"github.com/agentguard/agentguard/internal/proxy"
	"github.com/agentguard/agentguard/internal/store"
)

// newWrapCmd builds `agentguard wrap`. Invocation pattern:
//
//	agentguard wrap --upstream-name github -- npx -y @modelcontextprotocol/server-github
//
// The `--` separator is mandatory; everything after it is the upstream argv.
func newWrapCmd() *cobra.Command {
	var (
		upstreamName string
		dbPath       string
	)
	cmd := &cobra.Command{
		Use:   "wrap [--upstream-name <name>] -- <command> [args...]",
		Short: "Transparent MCP stdio proxy for a single upstream server",
		Long: `Spawn the given upstream MCP server and proxy its stdio JSON-RPC
traffic, recording every tool call to the local SQLite database. This is the
command the rewritten agent configs invoke.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if upstreamName == "" {
				upstreamName = args[0]
			}

			path := dbPath
			if path == "" {
				p, err := store.DefaultPath()
				if err != nil {
					return fmt.Errorf("resolve db path: %w", err)
				}
				path = p
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			st, err := store.Open(ctx, path)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer func() { _ = st.Close() }()

			exit, runErr := proxy.RunStdio(ctx, proxy.StdioConfig{
				UpstreamName: upstreamName,
				Command:      args,
				ClientIn:     cmd.InOrStdin(),
				ClientOut:    cmd.OutOrStdout(),
				Store:        st,
				Chain:        pipeline.NewChain(), // empty for milestone 1
			})
			if runErr != nil {
				return runErr
			}
			if exit != 0 {
				return errors.New("upstream exited with non-zero status: " + strconv.Itoa(exit))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&upstreamName, "upstream-name", "",
		"canonical name for the upstream server (defaults to argv[0])")
	cmd.Flags().StringVar(&dbPath, "db", "",
		"override the SQLite database path (default ~/.agentguard/data/agentguard.db)")
	return cmd
}

