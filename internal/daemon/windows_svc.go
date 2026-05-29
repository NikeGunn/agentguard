package daemon

import (
	"fmt"
	"strings"
)

// WindowsUnit generates a Windows "run at logon" helper. requirements.md §12
// explicitly rules out a Windows *service* installer, so rather than an SCM
// service we emit a Scheduled Task definition that runs the daemon at user
// logon — unprivileged, no SCM, removable with one schtasks command. This is
// the "use scoop / manual" path the spec calls for.
func WindowsUnit(p ServiceParams) ServiceUnit {
	// schtasks /TR wants a single quoted command line.
	tr := p.ExecPath
	for _, a := range p.Args {
		tr += " " + a
	}
	taskName := "AgentGuard"

	body := fmt.Sprintf(`# AgentGuard — run the daemon at logon (no admin, no Windows service).
# Install:
schtasks /Create /TN "%s" /SC ONLOGON /TR "%s" /RL LIMITED /F

# Status:
schtasks /Query /TN "%s"

# Remove:
schtasks /Delete /TN "%s" /F
`, taskName, escapeForSchtasks(tr), taskName, taskName)

	return ServiceUnit{
		Kind:        "windows",
		Filename:    "agentguard-task.ps1",
		InstallPath: "%USERPROFILE%\\.agentguard\\agentguard-task.ps1",
		Body:        body,
		InstallHint: fmt.Sprintf(`schtasks /Create /TN "%s" /SC ONLOGON /TR "%s" /RL LIMITED /F`, taskName, escapeForSchtasks(tr)),
	}
}

// escapeForSchtasks escapes embedded double quotes for the schtasks /TR value.
func escapeForSchtasks(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}
