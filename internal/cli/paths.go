package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

// AgentguardPaths captures the canonical ~/.agentguard layout.
type AgentguardPaths struct {
	Root   string
	Bin    string
	Data   string
	Logs   string
	Packs  string
	Config string
}

// Paths returns the on-disk layout under the user's home directory. Missing
// directories are NOT created here — `ensure` is a separate step so dry-run
// callers (doctor) don't have side effects.
func Paths() (AgentguardPaths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return AgentguardPaths{}, fmt.Errorf("resolve home: %w", err)
	}
	root := filepath.Join(home, ".agentguard")
	return AgentguardPaths{
		Root:   root,
		Bin:    filepath.Join(root, "bin"),
		Data:   filepath.Join(root, "data"),
		Logs:   filepath.Join(root, "logs"),
		Packs:  filepath.Join(root, "packs"),
		Config: filepath.Join(root, "config"),
	}, nil
}

// EnsurePaths creates every directory in p with mode 0700. Idempotent.
func EnsurePaths(p AgentguardPaths) error {
	for _, dir := range []string{p.Root, p.Bin, p.Data, p.Logs, p.Packs, p.Config} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	return nil
}

// DBPath is the canonical SQLite path under p.
func (p AgentguardPaths) DBPath() string {
	return filepath.Join(p.Data, "agentguard.db")
}

// PidFile is the canonical pidfile path for the long-lived background process
// (dashboard + proxy). Used by daemon.Supervisor so `uninstall` can stop it.
func (p AgentguardPaths) PidFile() string {
	return filepath.Join(p.Root, "agentguard.pid")
}
