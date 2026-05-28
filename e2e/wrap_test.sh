#!/usr/bin/env bash
# wrap_test.sh — POSIX end-to-end test for `agentguard wrap`.
#
# 1. Builds the agentguard binary.
# 2. Builds the mock MCP server.
# 3. Pipes three tool/call JSON-RPC requests through `agentguard wrap`.
# 4. Asserts the responses come back and that one sessions row + three
#    tool_calls rows exist in the SQLite DB.
#
# This file exists in addition to e2e/wrap_e2e_test.go (the Go-driven version
# that also runs on Windows) so the spec's promise of a shell-runnable e2e
# check is honoured. CI on linux/macOS runs this one; Windows CI runs the Go
# version.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

cd "$ROOT_DIR"
echo "==> building agentguard"
go build -o "$TMP_DIR/agentguard" ./cmd/agentguard
echo "==> building mock_mcp_server"
go build -o "$TMP_DIR/mock_mcp_server" ./e2e/mock_mcp_server

DB="$TMP_DIR/wrap_test.db"
INPUT="$TMP_DIR/calls.jsonl"
cat >"$INPUT" <<'EOF'
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"message":"hello"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"add","arguments":{"a":2,"b":3}}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"ping","arguments":{}}}
EOF

echo "==> driving agentguard wrap"
OUT="$("$TMP_DIR/agentguard" wrap --upstream-name mock --db "$DB" -- "$TMP_DIR/mock_mcp_server" <"$INPUT")"

# Response sanity.
LINES=$(printf '%s\n' "$OUT" | grep -c '"jsonrpc":"2.0"' || true)
if [ "$LINES" -ne 3 ]; then
  echo "FAIL: expected 3 JSON-RPC responses, got $LINES"
  echo "----- agentguard stdout -----"
  echo "$OUT"
  exit 1
fi
echo "$OUT" | grep -q '"hello"' || { echo "FAIL: echo response missing"; exit 1; }
echo "$OUT" | grep -q '"pong"'  || { echo "FAIL: ping response missing"; exit 1; }

# DB sanity. Use sqlite3 if available; otherwise fall back to a Go one-liner.
if command -v sqlite3 >/dev/null 2>&1; then
  SESSIONS=$(sqlite3 "$DB" "SELECT count(*) FROM sessions;")
  CALLS=$(sqlite3 "$DB" "SELECT count(*) FROM tool_calls WHERE direction='outbound' AND tool_name LIKE 'tools/call:%';")
else
  read -r SESSIONS CALLS < <(go run ./e2e/dbcheck "$DB")
fi

if [ "$SESSIONS" != "1" ]; then echo "FAIL: expected 1 sessions row, got $SESSIONS"; exit 1; fi
if [ "$CALLS" != "3" ];   then echo "FAIL: expected 3 tool_calls rows, got $CALLS"; exit 1; fi

echo "PASS: 3 responses, 1 session, 3 tool_calls"
