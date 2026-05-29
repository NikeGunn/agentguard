package proxy

import (
	"bufio"
	"context"
	"io"
	"strings"

	"github.com/agentguard/agentguard/internal/pipeline"
)

// SSEScanner inspects a legacy MCP Server-Sent-Events stream. SSE MCP servers
// push `data:`-framed JSON-RPC messages to the agent (inbound direction); this
// is precisely the channel that carries indirect prompt injection in tool
// results (the "GitHub MCP heist" pattern, system-design.md §5), so it must go
// through the pipeline just like stdio frames do.
//
// The scanner is transport-only: it parses SSE framing, hands each event's
// JSON-RPC payload to the Router, and writes the (possibly transformed) event
// onward. It does not own a network connection — the caller wires src/dst —
// which keeps it unit-testable with plain in-memory pipes.
type SSEScanner struct {
	ServerName string
	Router     *Router
	// OnVerdict mirrors HTTPProxy.OnVerdict: an optional persistence hook.
	OnVerdict func(m *pipeline.Message, verdict pipeline.Verdict, traces []pipeline.Trace)
}

// NewSSEScanner constructs an SSEScanner.
func NewSSEScanner(serverName string, router *Router) *SSEScanner {
	return &SSEScanner{ServerName: serverName, Router: router}
}

// Pump reads an SSE stream from src and writes the inspected stream to dst
// until src is exhausted or ctx is cancelled. SSE events are blocks of lines
// terminated by a blank line; we re-emit each event verbatim except that a
// blocked `data:` payload is dropped and a transformed one is rewritten.
//
// Per the SSE spec, a single event may carry multiple `data:` lines whose
// values are joined with "\n"; we inspect the joined payload and, on a clean
// pass, forward the original lines unchanged to avoid reframing surprises.
func (s *SSEScanner) Pump(ctx context.Context, src io.Reader, dst io.Writer) error {
	r := bufio.NewReaderSize(src, 1<<20)
	var event []string // raw lines of the current event (including "data:" prefixes)
	var data strings.Builder

	flush := func() error {
		if len(event) == 0 {
			return nil
		}
		defer func() { event = event[:0]; data.Reset() }()

		payload := strings.TrimRight(data.String(), "\n")
		if payload == "" {
			// Comment/heartbeat/empty event — forward verbatim.
			return writeEvent(dst, event)
		}
		msg := parseRPC([]byte(payload))
		pm := &pipeline.Message{
			ServerName: s.ServerName,
			ToolName:   msg.toolName(),
			Method:     msg.Method,
			Direction:  pipeline.Inbound, // SSE pushes server→agent
			Raw:        []byte(payload),
		}
		traces, verdict := s.Router.Inspect(ctx, TransportSSE, pm)
		if s.OnVerdict != nil {
			s.OnVerdict(pm, verdict, traces)
		}
		switch verdict {
		case pipeline.VerdictBlock, pipeline.VerdictError:
			// Drop the malicious event; the agent simply never receives it.
			return nil
		case pipeline.VerdictTransform:
			// Re-emit a single data: line with the redacted payload.
			_, err := io.WriteString(dst, "data: "+string(pm.Raw)+"\n\n")
			return err
		default:
			return writeEvent(dst, event)
		}
	}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line, err := r.ReadString('\n')
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" && line != "" {
			// Blank line terminates the event.
			if ferr := flush(); ferr != nil {
				return ferr
			}
		} else if trimmed != "" {
			event = append(event, trimmed)
			if v, ok := strings.CutPrefix(trimmed, "data:"); ok {
				data.WriteString(strings.TrimPrefix(v, " "))
				data.WriteByte('\n')
			}
		}
		if err != nil {
			if err == io.EOF {
				return flush()
			}
			return err
		}
	}
}

// writeEvent re-emits an event's raw lines followed by the terminating blank
// line.
func writeEvent(dst io.Writer, lines []string) error {
	for _, l := range lines {
		if _, err := io.WriteString(dst, l+"\n"); err != nil {
			return err
		}
	}
	_, err := io.WriteString(dst, "\n")
	return err
}
