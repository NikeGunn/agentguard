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
	"github.com/agentguard/agentguard/internal/policy"
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
		packName     string
		ratePerSec   float64
		maxBytes     int
		noInspect    bool
	)
	cmd := &cobra.Command{
		Use:   "wrap [--upstream-name <name>] [--pack <name>] -- <command> [args...]",
		Short: "Transparent MCP stdio proxy for a single upstream server",
		Long: `Spawn the given upstream MCP server and proxy its stdio JSON-RPC
traffic, running every frame through the cheap-path inspection chain
(transport guard, schema validator, policy engine, content scanner) and
recording the verdict in the local SQLite database.`,
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

			chain, err := buildChain(packName, ratePerSec, maxBytes, noInspect)
			if err != nil {
				return fmt.Errorf("build inspection chain: %w", err)
			}

			exit, runErr := proxy.RunStdio(ctx, proxy.StdioConfig{
				UpstreamName: upstreamName,
				Command:      args,
				ClientIn:     cmd.InOrStdin(),
				ClientOut:    cmd.OutOrStdout(),
				Store:        st,
				Chain:        chain,
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
	cmd.Flags().StringVar(&packName, "pack", "default",
		"builtin rule pack to load (default | strict)")
	cmd.Flags().Float64Var(&ratePerSec, "rate-per-second", 100,
		"per-session request rate limit (stage 0)")
	cmd.Flags().IntVar(&maxBytes, "max-frame-bytes", 8<<20,
		"hard cap on inspected frame size (stage 0); 0 to disable")
	cmd.Flags().BoolVar(&noInspect, "no-inspect", false,
		"disable the inspection chain entirely (transparent pass-through)")
	return cmd
}

// buildChain assembles the milestone-2 cheap-path chain:
//
//	transport → schema → policy → content scanner
//
// Stage 2 (server attestation), Stage 5 (ML), and Stage 6 (circuit breaker)
// land in later milestones.
func buildChain(packName string, ratePerSec float64, maxBytes int, disable bool) (*pipeline.Chain, error) {
	if disable {
		return pipeline.NewChain(), nil
	}
	_, rules, err := policy.LoadBuiltin(packName)
	if err != nil {
		return nil, err
	}
	return pipeline.NewChain(
		&pipeline.TransportGuard{RatePerSecond: ratePerSec, MaxBytes: maxBytes},
		pipeline.SchemaValidator{},
		&pipeline.PolicyStage{Engine: policy.NewEngine(rules)},
		pipeline.NewContentScanner(),
	), nil
}
