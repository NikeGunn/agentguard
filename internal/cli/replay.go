package cli

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/agentguard/agentguard/internal/ml"
	"github.com/agentguard/agentguard/internal/pipeline"
	"github.com/agentguard/agentguard/internal/policy"
	"github.com/agentguard/agentguard/internal/store"
)

func newReplayCmd() *cobra.Command {
	var (
		sessionID string
		serverID  string
		since     string
		limit     int
		pack      string
		dbPath    string
	)
	cmd := &cobra.Command{
		Use:   "replay",
		Short: "Re-run the inspection pipeline against stored tool calls",
		Long: `Reads tool_calls from the local database and re-runs the full
inspection pipeline against the stored request/response bytes,
printing a verdict diff. Useful for testing a new rule pack against
real historic traffic before rolling it out.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if dbPath == "" {
				p, err := store.DefaultPath()
				if err != nil {
					return err
				}
				dbPath = p
			}
			s, err := store.Open(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer s.Close()

			rows, err := loadReplayRows(ctx, s.DB, sessionID, serverID, since, limit)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no tool calls matched the filter")
				return nil
			}

			chain, err := buildReplayChain(pack)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Replaying %d tool calls against pack %q\n\n", len(rows), pack)
			diffs := 0
			for _, r := range rows {
				m := &pipeline.Message{
					SessionID:  r.SessionID,
					ServerName: r.ServerName,
					ToolName:   r.ToolName,
					Direction:  pipeline.Outbound,
					Raw:        []byte(r.RequestInline),
				}
				_, v := chain.Run(ctx, m)
				marker := "="
				if string(v) != r.Verdict {
					marker = "≠"
					diffs++
				}
				fmt.Fprintf(out, "  %s  %s  %-12s %-20s  %s -> %s\n",
					marker, time.UnixMilli(r.StartedAt).Format("15:04:05"),
					trunc(r.ServerName, 12), trunc(r.ToolName, 20),
					r.Verdict, v)
			}
			fmt.Fprintf(out, "\n%d/%d verdict changes\n", diffs, len(rows))
			return nil
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "filter by session id")
	cmd.Flags().StringVar(&serverID, "server", "", "filter by server name")
	cmd.Flags().StringVar(&since, "since", "1h", "time window (e.g. 30m, 24h)")
	cmd.Flags().IntVar(&limit, "limit", 100, "max calls to replay")
	cmd.Flags().StringVar(&pack, "pack", "default", "rule pack to replay against")
	cmd.Flags().StringVar(&dbPath, "db", "", "override the SQLite path")
	return cmd
}

type replayRow struct {
	SessionID     string
	ServerName    string
	ToolName      string
	Verdict       string
	StartedAt     int64
	RequestInline string
}

func loadReplayRows(ctx context.Context, db *sql.DB, sessionID, serverID, since string, limit int) ([]replayRow, error) {
	q := `SELECT tc.session_id, COALESCE(s.name, ''), tc.tool_name, tc.verdict,
	             tc.started_at, COALESCE(tc.request_inline, '')
	      FROM tool_calls tc
	      LEFT JOIN mcp_servers s ON s.id = tc.server_id
	      WHERE 1=1`
	args := []any{}
	if sessionID != "" {
		q += " AND tc.session_id = ?"
		args = append(args, sessionID)
	}
	if serverID != "" {
		q += " AND s.name = ?"
		args = append(args, serverID)
	}
	if since != "" {
		if d, err := time.ParseDuration(since); err == nil {
			q += " AND tc.started_at >= ?"
			args = append(args, time.Now().Add(-d).UnixMilli())
		}
	}
	q += " ORDER BY tc.started_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []replayRow
	for rows.Next() {
		var r replayRow
		if err := rows.Scan(&r.SessionID, &r.ServerName, &r.ToolName, &r.Verdict, &r.StartedAt, &r.RequestInline); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func buildReplayChain(packName string) (*pipeline.Chain, error) {
	stages := []pipeline.Stage{pipeline.SchemaValidator{}}
	if packName != "" {
		_, rules, err := policy.LoadBuiltin(packName)
		if err == nil && len(rules) > 0 {
			stages = append(stages, &pipeline.PolicyStage{Engine: policy.NewEngine(rules)})
		}
	}
	stages = append(stages, pipeline.NewContentScanner())
	stages = append(stages, pipeline.NewMLStage(ml.New(0)))
	return pipeline.NewChain(stages...), nil
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
