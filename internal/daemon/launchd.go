package daemon

import (
	"fmt"
	"strings"
)

// LaunchdUnit generates a macOS launchd LaunchAgent plist (per-user, no root),
// the macOS path for the supervisor.
func LaunchdUnit(p ServiceParams) ServiceUnit {
	var args strings.Builder
	args.WriteString(fmt.Sprintf("\t\t<string>%s</string>\n", xmlEscape(p.ExecPath)))
	for _, a := range p.Args {
		args.WriteString(fmt.Sprintf("\t\t<string>%s</string>\n", xmlEscape(a)))
	}

	body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
%s	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
</dict>
</plist>
`, xmlEscape(p.Label), args.String())

	return ServiceUnit{
		Kind:        "launchd",
		Filename:    p.Label + ".plist",
		InstallPath: "~/Library/LaunchAgents/" + p.Label + ".plist",
		Body:        body,
		InstallHint: "launchctl load -w ~/Library/LaunchAgents/" + p.Label + ".plist",
	}
}

// xmlEscape escapes the five XML special characters for safe plist embedding.
func xmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}
