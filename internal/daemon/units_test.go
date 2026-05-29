package daemon

import (
	"strings"
	"testing"
)

func testParams() ServiceParams {
	return DefaultParams("/usr/local/bin/agentguard")
}

func TestDefaultParams(t *testing.T) {
	p := DefaultParams("/opt/agentguard")
	if p.Label != "space.agentguard.daemon" {
		t.Fatalf("label = %q", p.Label)
	}
	if p.ExecPath != "/opt/agentguard" {
		t.Fatalf("execpath = %q", p.ExecPath)
	}
	if got := p.execLine(); got != "/opt/agentguard daemon run" {
		t.Fatalf("execLine = %q", got)
	}
}

func TestSystemdUnit(t *testing.T) {
	u := SystemdUnit(testParams())
	if u.Kind != "systemd" {
		t.Fatalf("kind = %q", u.Kind)
	}
	if u.Filename != "agentguard.service" {
		t.Fatalf("filename = %q", u.Filename)
	}
	for _, want := range []string{"[Unit]", "[Service]", "[Install]", "ExecStart=/usr/local/bin/agentguard daemon run", "WantedBy=default.target"} {
		if !strings.Contains(u.Body, want) {
			t.Fatalf("systemd body missing %q:\n%s", want, u.Body)
		}
	}
	if !strings.Contains(u.InstallHint, "systemctl --user") {
		t.Fatalf("install hint = %q", u.InstallHint)
	}
}

func TestLaunchdUnit(t *testing.T) {
	u := LaunchdUnit(testParams())
	if u.Kind != "launchd" {
		t.Fatalf("kind = %q", u.Kind)
	}
	if !strings.HasSuffix(u.Filename, ".plist") {
		t.Fatalf("filename = %q", u.Filename)
	}
	for _, want := range []string{"<plist version=\"1.0\">", "space.agentguard.daemon", "<string>/usr/local/bin/agentguard</string>", "<string>daemon</string>", "<string>run</string>", "RunAtLoad"} {
		if !strings.Contains(u.Body, want) {
			t.Fatalf("launchd body missing %q:\n%s", want, u.Body)
		}
	}
}

func TestLaunchdUnitEscapesXML(t *testing.T) {
	p := testParams()
	p.Args = []string{"daemon", "run", "--note=a<b>c&d"}
	u := LaunchdUnit(p)
	if strings.Contains(u.Body, "a<b>c&d") {
		t.Fatalf("launchd body did not escape XML special chars:\n%s", u.Body)
	}
	if !strings.Contains(u.Body, "a&lt;b&gt;c&amp;d") {
		t.Fatalf("expected escaped form in body:\n%s", u.Body)
	}
}

func TestWindowsUnit(t *testing.T) {
	u := WindowsUnit(testParams())
	if u.Kind != "windows" {
		t.Fatalf("kind = %q", u.Kind)
	}
	for _, want := range []string{"schtasks /Create", "ONLOGON", "/RL LIMITED", "schtasks /Delete"} {
		if !strings.Contains(u.Body, want) {
			t.Fatalf("windows body missing %q:\n%s", want, u.Body)
		}
	}
}

func TestEscapeForSchtasks(t *testing.T) {
	if got := escapeForSchtasks(`a "b" c`); got != `a \"b\" c` {
		t.Fatalf("escapeForSchtasks = %q", got)
	}
}
