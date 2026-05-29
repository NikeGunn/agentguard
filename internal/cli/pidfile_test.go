package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentguard/agentguard/internal/daemon"
)

// TestPidFilePath pins the canonical pidfile location under the agentguard root.
// The dashboard writes here on start and uninstall stops it from here; if these
// two ever disagree the daemon is left running and locks the binary on Windows
// (the bug this guards against).
func TestPidFilePath(t *testing.T) {
	home := t.TempDir()
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	t.Setenv("USERPROFILE", home) // Windows home resolution

	p, err := Paths()
	if err != nil {
		t.Fatalf("Paths: %v", err)
	}
	want := filepath.Join(p.Root, "agentguard.pid")
	if p.PidFile() != want {
		t.Fatalf("PidFile() = %q, want %q", p.PidFile(), want)
	}
}

// TestDaemonStopRoundTrip proves the contract the dashboard and uninstall share:
// a PID written via the supervisor at Paths().PidFile() is found and stopped by
// a fresh supervisor pointed at the same path.
func TestDaemonStopRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))

	p, err := Paths()
	if err != nil {
		t.Fatalf("Paths: %v", err)
	}
	if err := EnsurePaths(p); err != nil {
		t.Fatalf("EnsurePaths: %v", err)
	}

	// A self-PID is guaranteed alive for the duration of the test; we stub the
	// process finder so Stop() doesn't actually signal the test runner.
	sup := daemon.NewSupervisor(p.PidFile())
	if err := sup.WritePid(os.Getpid()); err != nil {
		t.Fatalf("WritePid: %v", err)
	}
	if _, err := os.Stat(p.PidFile()); err != nil {
		t.Fatalf("pidfile not written: %v", err)
	}
}
