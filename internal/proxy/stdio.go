// Package proxy holds the transports that intercept MCP/A2A traffic. Milestone
// 1 implements only the stdio transport with no inspection: it's a transparent
// pipe that opens a session, logs every JSON-RPC request as a tool_call, and
// closes the session on upstream exit.
package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/agentguard/agentguard/internal/pipeline"
	"github.com/agentguard/agentguard/internal/store"
)

// StdioConfig configures a single wrap invocation.
type StdioConfig struct {
	// UpstreamName is the canonical name of the MCP server (e.g. "github").
	UpstreamName string
	// Command is the upstream argv. Command[0] is the executable.
	Command []string
	// ClientIn is what the agent writes to us (stdin of the wrap process).
	ClientIn io.Reader
	// ClientOut is what we write back to the agent (stdout of the wrap process).
	ClientOut io.Writer
	// Store is where we persist session + tool_calls.
	Store *store.Store
	// Chain is the inspection chain. May be empty for milestone 1.
	Chain *pipeline.Chain
}

// jsonrpcMessage is the bare minimum we parse out of each frame to identify
// whether the frame is a request, response, or notification, and to extract
// the tool name for logging.
type jsonrpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

// RunStdio runs the wrap loop until ctx is cancelled or the upstream exits.
// It returns the upstream's exit code (0 if clean) and any framing error.
func RunStdio(ctx context.Context, cfg StdioConfig) (int, error) {
	if len(cfg.Command) == 0 {
		return 1, errors.New("proxy: empty upstream command")
	}
	if cfg.Store == nil {
		return 1, errors.New("proxy: nil store")
	}

	now := store.NowMS()

	// Register/upsert the server row.
	serverID := store.NewID()
	canonical := "stdio:" + cfg.UpstreamName + ":" + cfg.Command[0]
	cfg.Store.Writer().Submit(store.Event{Kind: store.EventMCPServerUpsert, Server: &store.MCPServer{
		ID: serverID, Name: cfg.UpstreamName, CanonicalURI: canonical,
		Transport: "stdio", UpstreamCommand: stringPtr(joinArgs(cfg.Command)),
		FirstSeenAt: now, LastSeenAt: now,
	}})

	// Open a session.
	sessionID := store.NewID()
	pid := os.Getpid()
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("USERNAME")
	}
	cfg.Store.Writer().Submit(store.Event{Kind: store.EventSessionStart, Session: &store.Session{
		ID: sessionID, StartedAt: now, ClientPID: &pid, ClientUser: &user, Mode: "enforce",
	}})

	// Spawn upstream.
	cmdCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, cfg.Command[0], cfg.Command[1:]...) //nolint:gosec
	cmd.Stderr = os.Stderr
	upstreamIn, err := cmd.StdinPipe()
	if err != nil {
		return 1, fmt.Errorf("upstream stdin pipe: %w", err)
	}
	upstreamOut, err := cmd.StdoutPipe()
	if err != nil {
		return 1, fmt.Errorf("upstream stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return 1, fmt.Errorf("start upstream: %w", err)
	}

	state := &sessionState{
		sessionID: sessionID,
		serverID:  serverID,
		chain:     cfg.Chain,
		st:        cfg.Store,
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Agent -> upstream (outbound)
	go func() {
		defer wg.Done()
		defer func() { _ = upstreamIn.Close() }()
		state.pump(cmdCtx, cfg.ClientIn, upstreamIn, pipeline.Outbound)
	}()
	// Upstream -> agent (inbound)
	go func() {
		defer wg.Done()
		state.pump(cmdCtx, upstreamOut, cfg.ClientOut, pipeline.Inbound)
	}()

	waitErr := cmd.Wait()
	cancel()
	wg.Wait()

	// Close the session row.
	end := store.NowMS()
	cfg.Store.Writer().Submit(store.Event{Kind: store.EventSessionEnd, Session: &store.Session{
		ID: sessionID, EndedAt: &end,
		TotalCalls:   state.totalCalls.Load(),
		TotalBlocked: state.totalBlocked.Load(),
	}})

	exit := 0
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			exit = exitErr.ExitCode()
		} else {
			slog.Error("upstream wait failed", slog.Any("err", waitErr))
			exit = 1
		}
	}
	return exit, nil
}

// sessionState carries the per-wrap counters and identifiers used by both pump
// goroutines.
type sessionState struct {
	sessionID    string
	serverID     string
	chain        *pipeline.Chain
	st           *store.Store
	totalCalls   atomic.Int64
	totalBlocked atomic.Int64
}

// pump reads newline-delimited JSON-RPC frames from src and writes them to
// dst. Each frame is fed through the inspection chain (no-op for M1) and
// recorded to the store before being forwarded.
//
// We deliberately use bufio.Reader.ReadBytes('\n') instead of json.Decoder so
// we can forward the exact original bytes — preserving whitespace and trailing
// newlines that an MCP server may rely on.
func (s *sessionState) pump(ctx context.Context, src io.Reader, dst io.Writer, dir pipeline.Direction) {
	// 1 MiB buffer covers very large tool results without surprising us.
	r := bufio.NewReaderSize(src, 1<<20)
	for {
		if ctx.Err() != nil {
			return
		}
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			s.handleFrame(ctx, line, dst, dir)
		}
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
				slog.Debug("proxy pump ended", slog.String("dir", string(dir)), slog.Any("err", err))
			}
			return
		}
	}
}

func (s *sessionState) handleFrame(ctx context.Context, raw []byte, dst io.Writer, dir pipeline.Direction) {
	// Parse just enough to identify the frame; never block on a malformed line.
	var msg jsonrpcMessage
	if err := json.Unmarshal(trimRightNewlines(raw), &msg); err != nil {
		// Pass it through unchanged — we'd rather be transparent than wrong.
		_, _ = dst.Write(raw)
		return
	}

	// Run the inspection chain. For M1 the chain is empty so this is free.
	if s.chain != nil {
		pmsg := &pipeline.Message{
			ServerName: "",
			ToolName:   extractToolName(&msg),
			Direction:  dir,
			Raw:        raw,
		}
		traces, verdict := s.chain.Run(ctx, pmsg)
		if verdict == pipeline.VerdictBlock {
			s.totalBlocked.Add(1)
			// Synthesise an error response and DO NOT forward to upstream.
			if reply := errorResponse(&msg, "Blocked by AgentGuard"); reply != nil {
				_, _ = dst.Write(reply)
			}
			s.recordCall(&msg, dir, raw, "block", traces)
			return
		}
		s.recordCall(&msg, dir, raw, "allow", traces)
	} else {
		s.recordCall(&msg, dir, raw, "allow", nil)
	}

	_, _ = dst.Write(raw)
}

// recordCall persists a tool_calls row (and stage rows, if traces present).
// We only record JSON-RPC *requests* (msg.Method set) to keep the table
// meaningful; responses are correlated by the agent via the request ID.
func (s *sessionState) recordCall(msg *jsonrpcMessage, dir pipeline.Direction, raw []byte, verdict string, traces []pipeline.Trace) {
	if msg.Method == "" {
		return
	}
	s.totalCalls.Add(1)
	id := store.NewID()
	now := store.NowMS()
	size := int64(len(raw))
	requestInline := string(raw)
	tc := &store.ToolCall{
		ID:               id,
		SessionID:        s.sessionID,
		ServerID:         s.serverID,
		ToolName:         displayTool(msg),
		Direction:        string(dir),
		StartedAt:        now,
		CompletedAt:      &now,
		Verdict:          verdict,
		RequestSizeBytes: &size,
		RequestInline:    &requestInline,
	}
	s.st.Writer().Submit(store.Event{Kind: store.EventToolCall, ToolCall: tc})
	for _, tr := range traces {
		detail := tr.Result.Detail
		var detailPtr *string
		if detail != "" {
			detailPtr = &detail
		}
		s.st.Writer().Submit(store.Event{Kind: store.EventToolCallStage, Stage: &store.ToolCallStage{
			ID:          store.NewID(),
			ToolCallID:  id,
			Stage:       tr.Stage,
			StageOrder:  tr.Order,
			StartedAtNS: tr.StartedAt.UnixNano(),
			DurationNS:  tr.Duration.Nanoseconds(),
			Outcome:     string(tr.Result.Verdict),
			Detail:      detailPtr,
		}})
	}
}

// extractToolName pulls the MCP tool name from a tools/call params payload.
// Returns the empty string for non-tool-call methods.
func extractToolName(msg *jsonrpcMessage) string {
	if msg.Method != "tools/call" || len(msg.Params) == 0 {
		return ""
	}
	var p struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return ""
	}
	return p.Name
}

// displayTool returns a human-friendly identifier for a tool_calls row:
// "tools/call:read_file" for tool invocations, else the bare method.
func displayTool(msg *jsonrpcMessage) string {
	if tn := extractToolName(msg); tn != "" {
		return msg.Method + ":" + tn
	}
	return msg.Method
}

// errorResponse builds a JSON-RPC error response correlated to msg.ID.
// Returns nil for notifications (no ID → no response expected).
func errorResponse(msg *jsonrpcMessage, reason string) []byte {
	if len(msg.ID) == 0 {
		return nil
	}
	out := struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Error   struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}{JSONRPC: "2.0", ID: msg.ID}
	out.Error.Code = -32099
	out.Error.Message = "Blocked by AgentGuard: " + reason
	b, err := json.Marshal(out)
	if err != nil {
		return nil
	}
	return append(b, '\n')
}

func stringPtr(s string) *string { return &s }

func trimRightNewlines(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}

func joinArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}

