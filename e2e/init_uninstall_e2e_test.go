// init_uninstall_e2e_test.go validates the milestone-3 install story end to
// end: a real agentguard binary, a faked Claude Code config tree under a
// temp HOME, `agentguard init` rewrites the config, the rewrite actually
// invokes our binary, then `agentguard uninstall` restores byte-identical
// to the original. The "byte-identical" promise is the hard one — that's
// what the §13 definition of done calls out.
package e2e

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// runWithHome runs argv with HOME (and USERPROFILE on Windows) overridden so
// the agentguard process sees the given fake home dir.
func runWithHome(t *testing.T, ctx context.Context, home, bin string, argv ...string) (string, string, int) {
	t.Helper()
	cmd := exec.CommandContext(ctx, bin, argv...)
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"USERPROFILE="+home, // Windows
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = bytes.NewReader([]byte("y\n"))
	err := cmd.Run()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		code = -1
		t.Logf("run %s: %v", argv, err)
	}
	return stdout.String(), stderr.String(), code
}

func TestInitAndUninstallRoundtrip(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e is slow; skipped in -short mode")
	}
	bin := buildBinary(t, "./cmd/agentguard", "agentguard")
	home := t.TempDir()

	originalConfig := `{
  "model": "sonnet",
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"]
    },
    "filesystem": {
      "command": "node",
      "args": ["fs.js"]
    }
  }
}
`
	cfgPath := filepath.Join(home, ".claude.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(cfgPath), 0o700))
	require.NoError(t, os.WriteFile(cfgPath, []byte(originalConfig), 0o600))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// init.
	out, errOut, code := runWithHome(t, ctx, home, bin, "init", "--non-interactive")
	require.Equal(t, 0, code, "init exit=%d stdout=%s stderr=%s", code, out, errOut)
	require.Contains(t, out, "Claude Code")

	// Patched config must reference agentguard.
	patched, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	require.Contains(t, string(patched), "wrap")
	require.Contains(t, string(patched), "--upstream-name")

	// Backup file present and byte-identical to original.
	bak, err := os.ReadFile(cfgPath + ".agentguard.bak")
	require.NoError(t, err)
	require.Equal(t, originalConfig, string(bak))

	// agents row exists in the DB under the fake HOME.
	dbPath := filepath.Join(home, ".agentguard", "data", "agentguard.db")
	_, err = os.Stat(dbPath)
	require.NoError(t, err, "expected db at %s", dbPath)

	// uninstall (without purge, with --yes).
	out, errOut, code = runWithHome(t, ctx, home, bin, "uninstall", "-y")
	require.Equal(t, 0, code, "uninstall exit=%d stdout=%s stderr=%s", code, out, errOut)

	// Config restored byte-identical.
	restored, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	require.Equal(t, originalConfig, string(restored), "uninstall must restore byte-identical")

	// Backup file removed.
	_, err = os.Stat(cfgPath + ".agentguard.bak")
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestDoctorReportsHealthyAfterInit(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e is slow; skipped in -short mode")
	}
	bin := buildBinary(t, "./cmd/agentguard", "agentguard")
	home := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"),
		[]byte(`{"mcpServers":{"x":{"command":"node","args":["x.js"]}}}`), 0o600))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _, code := runWithHome(t, ctx, home, bin, "init", "--non-interactive")
	require.Equal(t, 0, code)

	out, _, code := runWithHome(t, ctx, home, bin, "doctor")
	require.Equal(t, 0, code, "doctor should exit 0 on a freshly-init'd install: %s", out)
	require.Contains(t, out, "agents: 1 active")
	require.Contains(t, out, "ok,")
	require.Contains(t, out, "builtin packs:")
}
