package cli

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/agentguard/agentguard/internal/dashboard"
	"github.com/agentguard/agentguard/internal/store"
)

func newDashboardCmd() *cobra.Command {
	var (
		addr      string
		dbPath    string
		noBrowser bool
	)
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Run the local dashboard at http://localhost:7878",
		Long: `Starts the AgentGuard dashboard — a single-page web UI that
shows live tool calls, server inventory, top tools, and per-minute
timeseries. Bound to 127.0.0.1 only.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			if dbPath == "" {
				p, err := store.DefaultPath()
				if err != nil {
					return fmt.Errorf("default db path: %w", err)
				}
				dbPath = p
			}
			s, err := store.Open(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer s.Close()

			srv := dashboard.New(dashboard.Config{Store: s, Addr: addr})
			url := "http://" + srv.Addr()
			fmt.Fprintf(cmd.OutOrStdout(), "AgentGuard dashboard listening on %s\n", url)

			if !noBrowser {
				go func() {
					time.Sleep(300 * time.Millisecond)
					_ = openBrowser(url)
				}()
			}
			return srv.Run(ctx)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:7878", "listen address (localhost only)")
	cmd.Flags().StringVar(&dbPath, "db", "", "override the SQLite database path")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "don't try to open the dashboard in a browser")
	return cmd
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
