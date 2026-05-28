package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	agentdetect "github.com/agentguard/agentguard/internal/agent_detect"
	"github.com/agentguard/agentguard/internal/policy"
	"github.com/agentguard/agentguard/internal/store"
)

// newDoctorCmd builds `agentguard doctor`. Each finding is one of ✓ / ⚠ / ✗
// with a remediation hint. Exit code is 0 if no ✗ findings, 1 otherwise.
func newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Health check: configs, database, rule packs, agents",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			rep := newReport(out)
			runChecks(cmd.Context(), rep)
			rep.Summary()
			if rep.Fails() > 0 {
				return errors.New("doctor: one or more checks failed")
			}
			return nil
		},
	}
	return cmd
}

func runChecks(ctx context.Context, rep *report) {
	paths, err := Paths()
	if err != nil {
		rep.Fail("paths", err.Error())
		return
	}

	for _, dir := range []string{paths.Root, paths.Data, paths.Bin, paths.Packs} {
		if info, err := os.Stat(dir); err != nil {
			if os.IsNotExist(err) {
				rep.Warn("dir:"+dir, "missing — run 'agentguard init'")
			} else {
				rep.Fail("dir:"+dir, err.Error())
			}
		} else if !info.IsDir() {
			rep.Fail("dir:"+dir, "not a directory")
		} else {
			rep.OK("dir:" + dir)
		}
	}

	if _, err := os.Stat(paths.DBPath()); err != nil {
		rep.Warn("db", "database file missing — run 'agentguard init'")
	} else {
		st, err := store.Open(ctx, paths.DBPath())
		if err != nil {
			rep.Fail("db", err.Error())
		} else {
			defer func() { _ = st.Close() }()
			var ok string
			if err := st.DB.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&ok); err != nil {
				rep.Fail("db", "integrity_check failed: "+err.Error())
			} else if ok != "ok" {
				rep.Fail("db", "integrity_check returned "+ok)
			} else {
				rep.OK("db integrity_check")
			}

			var agentCount, callCount int
			if err := st.DB.QueryRowContext(ctx,
				`SELECT count(*) FROM agents WHERE active = 1`).Scan(&agentCount); err == nil {
				if agentCount == 0 {
					rep.Warn("agents", "no agents registered — run 'agentguard init'")
				} else {
					rep.OK(fmt.Sprintf("agents: %d active", agentCount))
				}
			}
			if err := st.DB.QueryRowContext(ctx,
				`SELECT count(*) FROM tool_calls`).Scan(&callCount); err == nil {
				rep.OK(fmt.Sprintf("tool_calls: %d recorded", callCount))
			}

			rows, err := st.DB.QueryContext(ctx,
				`SELECT display_name, config_path FROM agents WHERE active = 1`)
			if err == nil {
				for rows.Next() {
					var name, cfg string
					if err := rows.Scan(&name, &cfg); err != nil {
						continue
					}
					verifyAgentConfig(rep, name, cfg)
				}
				_ = rows.Close()
			}
		}
	}

	names, err := policy.ListBuiltin()
	if err != nil {
		rep.Fail("packs", err.Error())
	} else {
		rep.OK(fmt.Sprintf("builtin packs: %d (%v)", len(names), names))
	}
}

func verifyAgentConfig(rep *report, displayName, configPath string) {
	home, err := os.UserHomeDir()
	if err != nil {
		rep.Warn("agent:"+displayName, "could not resolve home: "+err.Error())
		return
	}
	for _, d := range agentdetect.AllDetectors() {
		dt, derr := d.Detect(home)
		if derr != nil || dt.ConfigPath != configPath {
			continue
		}
		wrapped := 0
		for _, s := range dt.Servers {
			if agentdetect.AlreadyWrapped(s, "agentguard") || agentdetect.AlreadyWrapped(s, "agentguard.exe") {
				wrapped++
			}
		}
		if wrapped == 0 && len(dt.Servers) > 0 {
			rep.Warn("agent:"+displayName,
				"config exists but no entries route through AgentGuard — was init reverted?")
		} else {
			rep.OK(fmt.Sprintf("agent:%s (%d/%d wrapped)", displayName, wrapped, len(dt.Servers)))
		}
		return
	}
	rep.Warn("agent:"+displayName, "config path no longer detectable: "+configPath)
}

// report tallies findings. Methods are upper-case so the field is plainly
// distinct from the counters.
type report struct {
	out   io.Writer
	oks   int
	warns int
	fails int
}

func newReport(out io.Writer) *report { return &report{out: out} }

func (r *report) OK(name string) {
	r.oks++
	fmt.Fprintf(r.out, "  ✓ %s\n", name)
}
func (r *report) Warn(name, hint string) {
	r.warns++
	fmt.Fprintf(r.out, "  ⚠ %s — %s\n", name, hint)
}
func (r *report) Fail(name, hint string) {
	r.fails++
	fmt.Fprintf(r.out, "  ✗ %s — %s\n", name, hint)
}
func (r *report) Fails() int { return r.fails }
func (r *report) Summary() {
	fmt.Fprintf(r.out, "— %d ok, %d warn, %d fail —\n", r.oks, r.warns, r.fails)
}
