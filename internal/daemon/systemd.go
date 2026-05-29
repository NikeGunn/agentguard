package daemon

import "fmt"

// ServiceParams are the inputs every per-OS unit generator needs.
type ServiceParams struct {
	// Label is the reverse-DNS service identifier, e.g. "space.agentguard.daemon".
	Label string
	// ExecPath is the absolute path to the agentguard binary.
	ExecPath string
	// Args are the daemon subcommand args, e.g. ["daemon", "run"].
	Args []string
	// User is the account to run as (Linux systemd user units ignore this;
	// it's used for system-wide units only).
	User string
}

// DefaultParams returns ServiceParams with the canonical label and the given
// binary path, running `agentguard daemon run`.
func DefaultParams(execPath string) ServiceParams {
	return ServiceParams{
		Label:    "space.agentguard.daemon",
		ExecPath: execPath,
		Args:     []string{"daemon", "run"},
	}
}

// ServiceUnit is a generated, ready-to-install service description. AgentGuard
// only *generates* these (requirements.md §12) — installing them is the user's
// explicit, unprivileged step, printed by `agentguard daemon install`.
type ServiceUnit struct {
	// Kind is "systemd" | "launchd" | "windows".
	Kind string
	// Filename is the bare file name (e.g. "agentguard.service").
	Filename string
	// InstallPath is the recommended absolute install location.
	InstallPath string
	// Body is the file contents.
	Body string
	// InstallHint is the one-line command the user runs to activate it.
	InstallHint string
}

// quotedArgs renders Args as a space-joined exec line.
func (p ServiceParams) execLine() string {
	line := p.ExecPath
	for _, a := range p.Args {
		line += " " + a
	}
	return line
}

// SystemdUnit generates a systemd *user* unit (no root required), the Linux
// path for the supervisor.
func SystemdUnit(p ServiceParams) ServiceUnit {
	body := fmt.Sprintf(`[Unit]
Description=AgentGuard security sidecar daemon
Documentation=https://agentguard.space/docs
After=network.target

[Service]
Type=simple
ExecStart=%s
Restart=on-failure
RestartSec=2

[Install]
WantedBy=default.target
`, p.execLine())
	return ServiceUnit{
		Kind:        "systemd",
		Filename:    "agentguard.service",
		InstallPath: "~/.config/systemd/user/agentguard.service",
		Body:        body,
		InstallHint: "systemctl --user enable --now agentguard.service",
	}
}
