// inspection_e2e_test.go drives the milestone-2 cheap-path inspection chain
// end-to-end: a real agentguard binary, the mock MCP server, and a tool that
// deliberately leaks a fake AWS key in its response. Asserts:
//
//  1. The inbound frame the agent receives has the key replaced with
//     [REDACTED:aws_access_key] (stage 4 transformed the response).
//  2. The tool_calls row recorded for that frame has verdict='transform' and
//     a non-empty verdict_reason.
package e2e

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func TestWrapRedactsSecretInTooLResponse(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e is slow; skipped in -short mode")
	}
	agentguardBin := buildBinary(t, "./cmd/agentguard", "agentguard")
	mockBin := buildBinary(t, "./e2e/mock_mcp_server", "mock_mcp_server")
	dbPath := filepath.Join(t.TempDir(), "redact.db")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, agentguardBin,
		"wrap", "--upstream-name", "mock", "--db", dbPath, "--", mockBin)
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	cmd.Stderr = nil
	require.NoError(t, cmd.Start())

	go func() {
		defer stdin.Close()
		_, _ = io.WriteString(stdin,
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"leak_secret"}}`+"\n")
		time.Sleep(200 * time.Millisecond)
	}()

	var got []byte
	done := make(chan struct{})
	go func() {
		defer close(done)
		rd := bufio.NewReader(stdout)
		line, _ := rd.ReadBytes('\n')
		got = bytes.TrimRight(line, "\r\n")
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for redacted response")
	}
	_ = cmd.Wait()

	require.NotEmpty(t, got, "no response received")
	s := string(got)
	require.NotContains(t, s, "AKIAIOSFODNN7EXAMPLE", "secret was NOT redacted on the wire")
	require.Contains(t, s, "[REDACTED:aws_access_key]")

	db, err := sql.Open("sqlite", "file:"+dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Allow the batched writer up to ~500ms to flush.
	deadline := time.Now().Add(500 * time.Millisecond)
	var verdict, reason string
	for time.Now().Before(deadline) {
		err := db.QueryRow(`
			SELECT verdict, COALESCE(verdict_reason,'') FROM tool_calls
			WHERE direction='inbound' ORDER BY started_at DESC LIMIT 1`,
		).Scan(&verdict, &reason)
		if err == nil && verdict != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.Equal(t, "transform", verdict)
	require.True(t, strings.HasPrefix(reason, "content_scan:"), "reason was %q", reason)
}

func TestWrapBlocksOnPolicyMatch(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e is slow; skipped in -short mode")
	}
	agentguardBin := buildBinary(t, "./cmd/agentguard", "agentguard")
	mockBin := buildBinary(t, "./e2e/mock_mcp_server", "mock_mcp_server")
	dbPath := filepath.Join(t.TempDir(), "block.db")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use the strict pack — it blocks inbound "ignore previous instructions".
	cmd := exec.CommandContext(ctx, agentguardBin,
		"wrap", "--upstream-name", "mock", "--db", dbPath,
		"--pack", "strict", "--", mockBin)
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	cmd.Stderr = nil
	require.NoError(t, cmd.Start())

	go func() {
		defer stdin.Close()
		_, _ = io.WriteString(stdin,
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"injection"}}`+"\n")
		time.Sleep(200 * time.Millisecond)
	}()

	rd := bufio.NewReader(stdout)
	done := make(chan []byte, 1)
	go func() {
		line, _ := rd.ReadBytes('\n')
		done <- bytes.TrimRight(line, "\r\n")
	}()
	var got []byte
	select {
	case got = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
	_ = cmd.Wait()

	s := string(got)
	require.Contains(t, s, "Blocked by AgentGuard", "agent should receive a synthesized block error: %s", s)
	require.NotContains(t, s, "Ignore previous instructions", "injection payload must NOT reach the agent")
}
