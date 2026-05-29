package cli

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/spf13/cobra"

	"github.com/agentguard/agentguard/internal/store"
)

// demoUser tags every row this command creates so --reset can remove exactly
// the demo data and nothing the user actually captured.
const demoUser = "demo"

// newSeedDemoCmd builds the hidden `agentguard seed-demo` command. It is hidden
// because it is a developer/recording aid, not part of the supported surface,
// but it is fully functional and writes through the real store layer so the
// dashboard, tail, and SSE stream all light up exactly as they would with real
// traffic.
func newSeedDemoCmd() *cobra.Command {
	var (
		dbPath string
		live   bool
		reset  bool
		count  int
	)
	cmd := &cobra.Command{
		Use:    "seed-demo",
		Short:  "Populate the database with realistic demo traffic (dev/recording aid)",
		Hidden: true,
		Long: `seed-demo writes a believable stream of demo tool calls — a mix of
allow/block/flag/transform verdicts across several mock MCP servers, including
blocked prompt-injection attempts, an indirect injection via a GitHub issue, a
rug-pull / schema-drift event, a loop-detection trip, and a couple of low-trust
servers.

All demo rows are tagged session.client_user="demo" and can be removed with
--reset. Use --live to keep emitting new calls every 1-3s for recordings.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
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

			out := cmd.OutOrStdout()

			if reset {
				n, err := resetDemo(ctx, s)
				if err != nil {
					return fmt.Errorf("reset demo data: %w", err)
				}
				fmt.Fprintf(out, "Removed demo data (%d sessions and their calls).\n", n)
				return nil
			}

			sd := newSeeder(s)
			if err := sd.ensureServers(ctx); err != nil {
				return fmt.Errorf("seed servers: %w", err)
			}
			if err := sd.ensureAgents(ctx); err != nil {
				return fmt.Errorf("seed agents: %w", err)
			}

			fmt.Fprintf(out, "Seeding %d historical demo calls across %d servers…\n", count, len(sd.servers))
			sd.history(count)
			// Drain the writer so the history is queryable immediately.
			if err := s.Close(); err != nil {
				return fmt.Errorf("flush history: %w", err)
			}

			// trust_score has no Event path, so apply it after the server rows
			// have landed. Re-open briefly to do the direct UPDATEs.
			s3, err := store.Open(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("reopen for trust scores: %w", err)
			}
			sd.s = s3
			if err := sd.applyTrustScores(ctx); err != nil {
				_ = s3.Close()
				return fmt.Errorf("apply trust scores: %w", err)
			}
			_ = s3.Close()

			fmt.Fprintln(out, "Demo data ready. Open the dashboard: agentguard dashboard")

			if !live {
				return nil
			}

			// Re-open for the live phase (Close above drained and shut the writer).
			s2, err := store.Open(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("reopen for live: %w", err)
			}
			defer s2.Close()
			sd2 := newSeeder(s2)
			if err := sd2.loadServers(ctx); err != nil {
				return fmt.Errorf("reload servers: %w", err)
			}
			fmt.Fprintln(out, "Live mode: emitting a new call every 1-3s. Ctrl+C to stop.")
			return sd2.live(ctx, out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "override the SQLite database path")
	cmd.Flags().BoolVar(&live, "live", false, "keep emitting new calls every 1-3s")
	cmd.Flags().BoolVar(&reset, "reset", false, "remove all demo data and exit")
	cmd.Flags().IntVar(&count, "count", 500, "number of historical calls to generate")
	return cmd
}

// resetDemo deletes every session tagged client_user='demo'. ON DELETE CASCADE
// in the schema removes the dependent tool_calls, stages, and artifacts.
func resetDemo(ctx context.Context, s *store.Store) (int64, error) {
	res, err := s.DB.ExecContext(ctx,
		`DELETE FROM sessions WHERE client_user = ?`, demoUser)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	// Demo servers/agents are harmless to leave, but drop the obviously-demo
	// low-trust ones so a reset returns to a clean slate.
	_, _ = s.DB.ExecContext(ctx,
		`DELETE FROM mcp_servers WHERE canonical_uri LIKE 'demo://%'`)
	_, _ = s.DB.ExecContext(ctx,
		`DELETE FROM agents WHERE config_path LIKE '%/demo/%'`)
	return n, nil
}

// demoServer describes one mock MCP server the seeder creates.
type demoServer struct {
	id        string
	name      string
	uri       string
	transport string
	trust     int
	tools     []string
}

type seeder struct {
	s       *store.Store
	rng     *rand.Rand
	servers []demoServer
}

func newSeeder(s *store.Store) *seeder {
	return &seeder{
		s:   s,
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// demoServerSpecs is the fixed cast of servers the demo tells a story with:
// two trustworthy ones, two low-trust ones (one of which rug-pulls).
func demoServerSpecs() []demoServer {
	return []demoServer{
		{name: "github-mcp", uri: "demo://github", transport: "stdio", trust: 92,
			tools: []string{"create_issue", "list_repos", "get_file", "create_pr"}},
		{name: "filesystem-mcp", uri: "demo://fs", transport: "stdio", trust: 88,
			tools: []string{"read_file", "write_file", "list_dir"}},
		{name: "sketchy-search", uri: "demo://search", transport: "http", trust: 41,
			tools: []string{"web_search", "fetch_url"}},
		{name: "rugpull-tools", uri: "demo://rugpull", transport: "stdio", trust: 23,
			tools: []string{"run_command", "exfiltrate", "harmless_lookup"}},
	}
}

func (sd *seeder) ensureServers(ctx context.Context) error {
	now := store.NowMS()
	specs := demoServerSpecs()
	for i := range specs {
		specs[i].id = store.DeterministicID(specs[i].uri)
		sp := specs[i]
		sd.s.Writer().Submit(store.Event{Kind: store.EventMCPServerUpsert, Server: &store.MCPServer{
			ID: sp.id, Name: sp.name, CanonicalURI: sp.uri, Transport: sp.transport,
			FirstSeenAt: now - int64(48*time.Hour/time.Millisecond), LastSeenAt: now,
		}})
		// trust_score has no Event path; set it directly after the upsert lands.
	}
	sd.servers = specs
	return nil
}

// loadServers repopulates sd.servers from the DB (used in live mode after a
// fresh Open, when the in-memory list from ensureServers is gone).
func (sd *seeder) loadServers(ctx context.Context) error {
	specs := demoServerSpecs()
	for i := range specs {
		specs[i].id = store.DeterministicID(specs[i].uri)
	}
	sd.servers = specs
	return nil
}

func (sd *seeder) ensureAgents(ctx context.Context) error {
	now := store.NowMS()
	agents := []struct{ kind, name string }{
		{"claude-code", "Claude Code"},
		{"cursor", "Cursor"},
	}
	for _, a := range agents {
		id := store.DeterministicID("demo-agent:" + a.kind)
		_, err := sd.s.DB.ExecContext(ctx, `
			INSERT INTO agents (id, kind, display_name, config_path, config_backup, detected_at, last_seen_at, active)
			VALUES (?, ?, ?, ?, ?, ?, ?, 1)
			ON CONFLICT(id) DO UPDATE SET last_seen_at = excluded.last_seen_at, active = 1`,
			id, a.kind, a.name, "/demo/"+a.kind+"/config.json", "/demo/"+a.kind+"/config.json.bak", now, now)
		if err != nil {
			return err
		}
	}
	return nil
}

// applyTrustScores writes each demo server's trust_score directly. Called after
// the writer has drained so the rows exist.
func (sd *seeder) applyTrustScores(ctx context.Context) error {
	now := store.NowMS()
	for _, sp := range sd.servers {
		if _, err := sd.s.DB.ExecContext(ctx,
			`UPDATE mcp_servers SET trust_score = ?, trust_score_updated_at = ? WHERE id = ?`,
			sp.trust, now, sp.id); err != nil {
			return err
		}
	}
	return nil
}
