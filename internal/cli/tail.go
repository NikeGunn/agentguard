package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/agentguard/agentguard/internal/store"
)

// newTailCmd builds `agentguard tail` — a Bubble Tea TUI that polls SQLite
// every 500 ms and shows the most recent N tool_calls. The full spec lists
// j/k navigation, filtering, search, pause, replay, etc. — those land in
// M4. M3 delivers the readable scrolling feed that's the demo moment.
func newTailCmd() *cobra.Command {
	var (
		dbPath   string
		limit    int
		interval time.Duration
		oneShot  bool
	)
	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Live feed of recent tool calls",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dbPath == "" {
				p, err := store.DefaultPath()
				if err != nil {
					return err
				}
				dbPath = p
			}
			db, err := sql.Open(store.DriverName, "file:"+dbPath+"?mode=ro")
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			defer db.Close()

			if oneShot {
				rows, err := fetchRecent(cmd.Context(), db, limit)
				if err != nil {
					return err
				}
				renderPlain(cmd.OutOrStdout(), rows)
				return nil
			}

			m := newTailModel(db, limit, interval)
			p := tea.NewProgram(m, tea.WithAltScreen())
			_, err = p.Run()
			return err
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "override the SQLite path")
	cmd.Flags().IntVar(&limit, "limit", 50, "rows to display")
	cmd.Flags().DurationVar(&interval, "interval", 500*time.Millisecond, "poll interval")
	cmd.Flags().BoolVar(&oneShot, "once", false, "print one snapshot and exit (no TUI)")
	return cmd
}

// tailRow is one displayed call.
type tailRow struct {
	Time     time.Time
	Server   string
	Tool     string
	Dir      string
	Verdict  string
	Reason   string
	LatencyMS int64
}

func fetchRecent(ctx context.Context, db *sql.DB, limit int) ([]tailRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			tc.started_at,
			COALESCE(s.name, ''),
			tc.tool_name,
			tc.direction,
			tc.verdict,
			COALESCE(tc.verdict_reason, ''),
			COALESCE(tc.latency_ms_proxy, 0)
		FROM tool_calls tc
		LEFT JOIN mcp_servers s ON s.id = tc.server_id
		ORDER BY tc.started_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []tailRow
	for rows.Next() {
		var r tailRow
		var ts int64
		if err := rows.Scan(&ts, &r.Server, &r.Tool, &r.Dir, &r.Verdict, &r.Reason, &r.LatencyMS); err != nil {
			return nil, err
		}
		r.Time = time.UnixMilli(ts)
		out = append(out, r)
	}
	return out, nil
}

// --- bubbletea model ---

type tickMsg time.Time
type rowsMsg struct {
	rows []tailRow
	err  error
}

type tailModel struct {
	db       *sql.DB
	limit    int
	interval time.Duration
	rows     []tailRow
	err      error
	width    int
	height   int
}

func newTailModel(db *sql.DB, limit int, interval time.Duration) tailModel {
	return tailModel{db: db, limit: limit, interval: interval}
}

func (m tailModel) Init() tea.Cmd {
	return tea.Batch(m.fetchCmd(), m.tickCmd())
}

func (m tailModel) tickCmd() tea.Cmd {
	return tea.Tick(m.interval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m tailModel) fetchCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		rs, err := fetchRecent(ctx, m.db, m.limit)
		return rowsMsg{rows: rs, err: err}
	}
}

func (m tailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tickMsg:
		return m, tea.Batch(m.fetchCmd(), m.tickCmd())
	case rowsMsg:
		m.rows, m.err = msg.rows, msg.err
	}
	return m, nil
}

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	allowStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	blockStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	flagStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	xformStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func verdictGlyph(v string) string {
	switch v {
	case "allow":
		return allowStyle.Render("✓")
	case "block":
		return blockStyle.Render("✗")
	case "flag":
		return flagStyle.Render("⚠")
	case "transform":
		return xformStyle.Render("✎")
	default:
		return dimStyle.Render("·")
	}
}

func (m tailModel) View() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("AgentGuard · live tail") + "  ")
	b.WriteString(dimStyle.Render(fmt.Sprintf("(refresh %s — q to quit)", m.interval)))
	b.WriteString("\n\n")
	if m.err != nil {
		b.WriteString(blockStyle.Render("db error: " + m.err.Error()))
		return b.String()
	}
	if len(m.rows) == 0 {
		b.WriteString(dimStyle.Render("  (no tool calls yet — try 'agentguard wrap …' in another shell)"))
		return b.String()
	}
	fmt.Fprintf(&b, "  %-12s %-1s %-10s %-7s %-30s %6s  %s\n",
		"TIME", "V", "SERVER", "DIR", "TOOL", "LAT(ms)", "REASON")
	for _, r := range m.rows {
		ts := r.Time.Format("15:04:05.000")
		fmt.Fprintf(&b, "  %-12s %s %-10s %-7s %-30s %6d  %s\n",
			ts, verdictGlyph(r.Verdict),
			truncate(r.Server, 10), r.Dir, truncate(r.Tool, 30),
			r.LatencyMS, dimStyle.Render(truncate(r.Reason, 40)))
	}
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

// renderPlain is the --once / non-TUI rendering path.
func renderPlain(w io.Writer, rows []tailRow) {
	fmt.Fprintf(w, "%-12s %-1s %-10s %-7s %-30s %6s  %s\n",
		"TIME", "V", "SERVER", "DIR", "TOOL", "LAT(ms)", "REASON")
	for _, r := range rows {
		fmt.Fprintf(w, "%-12s %-1s %-10s %-7s %-30s %6d  %s\n",
			r.Time.Format("15:04:05.000"),
			verdictAscii(r.Verdict),
			truncate(r.Server, 10), r.Dir, truncate(r.Tool, 30),
			r.LatencyMS, truncate(r.Reason, 40))
	}
}

func verdictAscii(v string) string {
	switch v {
	case "allow":
		return "+"
	case "block":
		return "x"
	case "flag":
		return "!"
	case "transform":
		return "~"
	default:
		return "."
	}
}
