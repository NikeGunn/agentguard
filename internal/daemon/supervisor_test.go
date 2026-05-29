package daemon

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// fakeProc is a stub Process for tests.
type fakeProc struct {
	aliveSig  bool // Signal(0) returns nil when true
	interrupt error
	killed    bool
}

func (f *fakeProc) Signal(s os.Signal) error {
	if s == syscall.Signal(0) {
		if f.aliveSig {
			return nil
		}
		return os.ErrProcessDone
	}
	return f.interrupt
}
func (f *fakeProc) Kill() error { f.killed = true; return nil }

func newTestSupervisor(t *testing.T, proc *fakeProc) *Supervisor {
	t.Helper()
	s := NewSupervisor(filepath.Join(t.TempDir(), "agentguard.pid"))
	s.findProcess = func(int) (Process, error) { return proc, nil }
	return s
}

func TestSupervisor_StatusNotRunningWhenNoPidfile(t *testing.T) {
	s := newTestSupervisor(t, &fakeProc{})
	if _, err := s.Status(); err != ErrNotRunning {
		t.Fatalf("want ErrNotRunning, got %v", err)
	}
}

func TestSupervisor_WritePidAndStatusAlive(t *testing.T) {
	s := newTestSupervisor(t, &fakeProc{aliveSig: true})
	if err := s.WritePid(12345); err != nil {
		t.Fatal(err)
	}
	pid, err := s.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if pid != 12345 {
		t.Fatalf("pid = %d", pid)
	}
}

func TestSupervisor_StalePidfileCleared(t *testing.T) {
	s := newTestSupervisor(t, &fakeProc{aliveSig: false}) // process is dead
	if err := s.WritePid(999); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Status(); err != ErrNotRunning {
		t.Fatalf("dead process should yield ErrNotRunning, got %v", err)
	}
	// Stale pidfile must be removed.
	if _, err := os.Stat(s.PidFile); !os.IsNotExist(err) {
		t.Fatal("stale pidfile should have been removed")
	}
}

func TestSupervisor_StopSignalsAndRemovesPidfile(t *testing.T) {
	proc := &fakeProc{aliveSig: true}
	s := newTestSupervisor(t, proc)
	if err := s.WritePid(4242); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if _, err := os.Stat(s.PidFile); !os.IsNotExist(err) {
		t.Fatal("pidfile should be removed after stop")
	}
}

func TestSupervisor_StopNotRunning(t *testing.T) {
	s := newTestSupervisor(t, &fakeProc{})
	if err := s.Stop(); err != ErrNotRunning {
		t.Fatalf("want ErrNotRunning, got %v", err)
	}
}

func TestSupervisor_StopFallsBackToKill(t *testing.T) {
	proc := &fakeProc{aliveSig: true, interrupt: os.ErrInvalid} // Interrupt unsupported
	s := newTestSupervisor(t, proc)
	if err := s.WritePid(7); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if !proc.killed {
		t.Fatal("expected fallback to Kill when Interrupt fails")
	}
}
