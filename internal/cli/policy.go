package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/agentguard/agentguard/internal/store"
)

// newPolicyCmd builds `agentguard policy …`, operating on the policies table
// (database.md §3.9). list/show are read-only; enable/disable toggle the
// `enabled` flag and append to audit_log so policy changes are forensically
// traceable (database.md §3.12). YAML editing of rule bodies stays in the
// `pack` workflow; this command manages activation, which is what the
// dashboard's policy toggles also do.
func newPolicyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "List and toggle security policies",
	}
	cmd.AddCommand(newPolicyListCmd(), newPolicyShowCmd(), newPolicyEnableCmd(), newPolicyDisableCmd())
	return cmd
}

// openStore opens the on-disk store at the canonical DB path. Shared by every
// DB-backed policy subcommand.
func openStore(ctx context.Context) (*store.Store, error) {
	p, err := Paths()
	if err != nil {
		return nil, err
	}
	return store.Open(ctx, p.DBPath())
}

type policyRow struct {
	id       string
	name     string
	source   string
	scope    string
	enabled  bool
	priority int
}

func queryPolicies(ctx context.Context, db *sql.DB, where string, args ...any) ([]policyRow, error) {
	q := `SELECT id, name, source, scope, enabled, priority FROM policies`
	if where != "" {
		q += " WHERE " + where
	}
	q += " ORDER BY priority ASC, name ASC"
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []policyRow
	for rows.Next() {
		var p policyRow
		var enabled int
		if err := rows.Scan(&p.id, &p.name, &p.source, &p.scope, &enabled, &p.priority); err != nil {
			return nil, err
		}
		p.enabled = enabled != 0
		out = append(out, p)
	}
	return out, rows.Err()
}

func newPolicyListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all policies and their enabled state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			st, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()
			rows, err := queryPolicies(cmd.Context(), st.DB, "")
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				out := cmd.OutOrStdout()
				fmt.Fprintln(out, "No custom policies installed.")
				fmt.Fprintln(out, "Inspection still runs the built-in 'default' rule pack on every call")
				fmt.Fprintln(out, "(see 'agentguard pack show builtin/default'). Use --pack strict on")
				fmt.Fprintln(out, "'agentguard wrap' for the stricter block-by-default set.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "STATE\tPRIORITY\tSOURCE\tSCOPE\tNAME")
			for _, p := range rows {
				state := "disabled"
				if p.enabled {
					state = "enabled"
				}
				fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\n", state, p.priority, p.source, p.scope, p.name)
			}
			return tw.Flush()
		},
	}
}

func newPolicyShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show a policy and its rules",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()
			rows, err := queryPolicies(cmd.Context(), st.DB, "name = ?", args[0])
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return fmt.Errorf("policy %q not found", args[0])
			}
			p := rows[0]
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Name:     %s\nSource:   %s\nScope:    %s\nPriority: %d\nEnabled:  %t\n",
				p.name, p.source, p.scope, p.priority, p.enabled)
			ruleRows, err := st.DB.QueryContext(cmd.Context(),
				`SELECT rule_order, action, match FROM policy_rules WHERE policy_id = ? ORDER BY rule_order`, p.id)
			if err != nil {
				return err
			}
			defer func() { _ = ruleRows.Close() }()
			fmt.Fprintln(out, "Rules:")
			any := false
			for ruleRows.Next() {
				var order int
				var action, match string
				if err := ruleRows.Scan(&order, &action, &match); err != nil {
					return err
				}
				any = true
				fmt.Fprintf(out, "  [%d] %-16s %s\n", order, action, match)
			}
			if !any {
				fmt.Fprintln(out, "  (none)")
			}
			return ruleRows.Err()
		},
	}
}

// setEnabled flips the enabled flag for a policy by name and writes an
// audit_log row. Returns an error if no such policy exists.
func setEnabled(ctx context.Context, st *store.Store, name string, enabled bool) error {
	val := 0
	action := "policy.disable"
	if enabled {
		val = 1
		action = "policy.enable"
	}
	res, err := st.DB.ExecContext(ctx,
		`UPDATE policies SET enabled = ?, updated_at = ? WHERE name = ?`, val, store.NowMS(), name)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("policy %q not found", name)
	}
	// Forensic trail (database.md §3.12). Best-effort: a failed audit write
	// must not mask a successful toggle, but we surface it.
	detail := fmt.Sprintf(`{"name":%q,"enabled":%t}`, name, enabled)
	if _, err := st.DB.ExecContext(ctx,
		`INSERT INTO audit_log (id, occurred_at, actor, action, target_type, target_id, detail)
		 VALUES (?,?,?,?,?,?,?)`,
		store.NewID(), store.NowMS(), "cli", action, "policy", name, detail); err != nil {
		return fmt.Errorf("toggle applied but audit write failed: %w", err)
	}
	return nil
}

func newPolicyEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable a policy",
		Args:  cobra.ExactArgs(1),
		RunE:  toggleRunE(true),
	}
}

func newPolicyDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable a policy",
		Args:  cobra.ExactArgs(1),
		RunE:  toggleRunE(false),
	}
}

// toggleRunE is the shared enable/disable handler (DRY) parameterised by the
// target state.
func toggleRunE(enabled bool) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		st, err := openStore(cmd.Context())
		if err != nil {
			return err
		}
		defer func() { _ = st.Close() }()
		if err := setEnabled(cmd.Context(), st, args[0], enabled); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("policy %q not found", args[0])
			}
			return err
		}
		verb := "disabled"
		if enabled {
			verb = "enabled"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "policy %q %s\n", args[0], verb)
		return nil
	}
}
