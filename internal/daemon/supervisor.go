// Package daemon supervises AgentGuard's background process: the long-lived
// process that runs the dashboard (:7878) and HTTP proxy (:7879) and owns the
// nightly retention job (system-design.md §10).
//
// The Supervisor here manages a user-mode background process via a pidfile —
// it does NOT install a privileged system service (requirements.md §12 rules
// out a Windows service installer and anything needing root). The per-OS files
// (launchd.go / systemd.go / windows_svc.go) *generate* service-unit bodies a
// user can install themselves, but the supervisor's start/stop/status path is
// the same on every platform.
package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// ErrNotRunning is returned by Stop/Status when no live daemon is found.
var ErrNotRunning = errors.New("daemon: not running")

// Supervisor manages the daemon process lifecycle through a pidfile.
type Supervisor struct {
	// PidFile is the path to the pidfile, e.g. ~/.agentguard/agentguard.pid.
	PidFile string
	// findProcess is injectable so tests don't spawn real processes.
	findProcess func(pid int) (Process, error)
}

// Process is the minimal slice of *os.Process the supervisor needs; an
// interface so tests can stub it.
type Process interface {
	Signal(os.Signal) error
	Kill() error
}

// NewSupervisor returns a Supervisor for the given pidfile.
func NewSupervisor(pidFile string) *Supervisor {
	return &Supervisor{PidFile: pidFile, findProcess: osFindProcess}
}

func osFindProcess(pid int) (Process, error) {
	p, err := os.FindProcess(pid)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// Status returns the running daemon's PID, or ErrNotRunning. It treats a
// pidfile pointing at a dead process as "not running" and removes the stale
// file so a subsequent start succeeds.
func (s *Supervisor) Status() (int, error) {
	pid, err := s.readPid()
	if err != nil {
		return 0, ErrNotRunning
	}
	if !s.alive(pid) {
		_ = os.Remove(s.PidFile)
		return 0, ErrNotRunning
	}
	return pid, nil
}

// WritePid records the current process (or a child's) PID. Called by the
// daemon process itself once it has started its listeners.
func (s *Supervisor) WritePid(pid int) error {
	if err := os.MkdirAll(filepath.Dir(s.PidFile), 0o700); err != nil {
		return err
	}
	return os.WriteFile(s.PidFile, []byte(strconv.Itoa(pid)), 0o600)
}

// Stop signals the running daemon to terminate gracefully and removes the
// pidfile. Returns ErrNotRunning if none is live.
func (s *Supervisor) Stop() error {
	pid, err := s.Status()
	if err != nil {
		return err
	}
	proc, ferr := s.finder()(pid)
	if ferr != nil {
		return ferr
	}
	// os.Interrupt maps to a graceful stop where supported; the daemon traps
	// it and drains the writer before exiting. Fall back to Kill if the
	// platform can't deliver Interrupt (e.g. Windows).
	if sigErr := proc.Signal(os.Interrupt); sigErr != nil {
		if killErr := proc.Kill(); killErr != nil {
			return fmt.Errorf("daemon: stop pid %d: %w", pid, killErr)
		}
	}
	return os.Remove(s.PidFile)
}

// alive reports whether pid refers to a live process. On Unix, signal 0 is the
// canonical liveness probe (FindProcess always succeeds there, so we must
// probe). On Windows FindProcess fails for dead PIDs, which is enough.
func (s *Supervisor) alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := s.finder()(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func (s *Supervisor) finder() func(int) (Process, error) {
	if s.findProcess != nil {
		return s.findProcess
	}
	return osFindProcess
}

func (s *Supervisor) readPid() (int, error) {
	b, err := os.ReadFile(s.PidFile)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(b)))
}
