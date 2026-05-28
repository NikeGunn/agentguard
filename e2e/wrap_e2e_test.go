// Package e2e contains end-to-end tests that build the real agentguard
// binary and a mock MCP server, then drive the wrap command and assert that
// JSON-RPC responses come back AND that the SQLite database has the rows we
// expect (one sessions row plus three tool_calls rows).
//
// This is a Go-driven version of e2e/wrap_test.sh — it runs cross-platform
// (including Windows, where bash isn't available) so `go test ./e2e/...`
// covers the same scenario in CI on every OS.
package e2e

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func exeName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

// buildBinary compiles a Go entrypoint into the test temp dir and returns the
// absolute path to the resulting executable.
func buildBinary(t *testing.T, pkg, outName string) string {
	t.Helper()
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, exeName(outName))
	cmd := exec.Command("go", "build", "-o", outPath, pkg)
	// Run from the repo root: two levels up from this file's working dir
	// (`go test` invokes us with PWD = the package, i.e. e2e/).
	cmd.Dir = ".."
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "go build %s: %s", pkg, out)
	return outPath
}

func TestWrapEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e is slow; skipped in -short mode")
	}
	agentguardBin := buildBinary(t, "./cmd/agentguard", "agentguard")
	mockBin := buildBinary(t, "./e2e/mock_mcp_server", "mock_mcp_server")

	dbPath := filepath.Join(t.TempDir(), "e2e.db")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, agentguardBin,
		"wrap", "--upstream-name", "mock", "--db", dbPath, "--", mockBin)

	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	cmd.Stderr = nil // discard

	require.NoError(t, cmd.Start())

	// Three canonical tool calls.
	calls := []string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"message":"hello"}}}` + "\n",
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"add","arguments":{"a":2,"b":3}}}` + "\n",
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"ping","arguments":{}}}` + "\n",
	}
	go func() {
		defer stdin.Close()
		for _, c := range calls {
			_, _ = io.WriteString(stdin, c)
		}
		// Give the upstream time to emit all three responses before EOF
		// propagates and tears the pipe down.
		time.Sleep(300 * time.Millisecond)
	}()

	// Read responses.
	type rpcResp struct {
		bytes []byte
	}
	got := make([]rpcResp, 0, 3)
	rd := bufio.NewReader(stdout)
	readCtx, readCancel := context.WithTimeout(ctx, 10*time.Second)
	defer readCancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 3; i++ {
			line, err := rd.ReadBytes('\n')
			if len(line) > 0 {
				got = append(got, rpcResp{bytes: bytes.TrimRight(line, "\r\n")})
			}
			if err != nil {
				return
			}
		}
	}()
	select {
	case <-done:
	case <-readCtx.Done():
		t.Fatalf("timeout waiting for responses; got %d/3", len(got))
	}

	// Stop the wrap; we can wait for upstream EOF.
	_ = cmd.Wait()

	require.Len(t, got, 3, "expected 3 responses")
	for i, r := range got {
		s := string(r.bytes)
		require.Contains(t, s, `"jsonrpc":"2.0"`, "response %d: %s", i, s)
		require.Contains(t, s, `"id":`, "response %d missing id: %s", i, s)
	}
	// Specific result content checks.
	require.Contains(t, string(got[0].bytes), `"hello"`)
	require.Contains(t, string(got[1].bytes), `"5"`)
	require.Contains(t, string(got[2].bytes), `"pong"`)

	// Verify DB state.
	db, err := sql.Open("sqlite", "file:"+dbPath)
	require.NoError(t, err)
	defer db.Close()

	var sessions int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM sessions`).Scan(&sessions))
	require.Equal(t, 1, sessions, "expected exactly one session row")

	var calls3 int
	require.NoError(t, db.QueryRow(
		`SELECT count(*) FROM tool_calls WHERE direction='outbound' AND tool_name LIKE 'tools/call:%'`,
	).Scan(&calls3))
	require.Equal(t, 3, calls3, "expected 3 outbound tool_calls rows")

	// And the recorded tool names should match.
	rows, err := db.Query(
		`SELECT tool_name FROM tool_calls WHERE direction='outbound' ORDER BY started_at`)
	require.NoError(t, err)
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		names = append(names, n)
	}
	require.Contains(t, strings.Join(names, ","), "echo")
	require.Contains(t, strings.Join(names, ","), "add")
	require.Contains(t, strings.Join(names, ","), "ping")
}
