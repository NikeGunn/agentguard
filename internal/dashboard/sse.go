package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/agentguard/agentguard/internal/store"
)

// handleSSE streams live tool_call inserts to the dashboard. On connect we
// send a `hello` event with the current overview snapshot, subscribe to
// the store broadcaster, then forward each ToolCall as a `call` event
// until the client disconnects.
//
// A 25-second heartbeat keeps intermediaries from idle-closing.
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	if stats, err := s.store.Overview(r.Context(), time.Hour); err == nil {
		writeEvent(w, flusher, "hello", stats)
	}

	ch := s.store.Broadcaster().Subscribe(128)
	defer s.store.Broadcaster().Unsubscribe(ch)

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case tc, ok := <-ch:
			if !ok {
				return
			}
			writeEvent(w, flusher, "call", sseCall(tc))
		case t := <-heartbeat.C:
			fmt.Fprintf(w, ": ping %d\n\n", t.Unix())
			flusher.Flush()
		}
	}
}

func sseCall(tc store.ToolCall) map[string]any {
	out := map[string]any{
		"id":         tc.ID,
		"server_id":  tc.ServerID,
		"tool_name":  tc.ToolName,
		"direction":  tc.Direction,
		"verdict":    tc.Verdict,
		"started_at": tc.StartedAt,
	}
	if tc.VerdictReason != nil {
		out["reason"] = *tc.VerdictReason
	}
	if tc.LatencyMSProxy != nil {
		out["latency_ms"] = *tc.LatencyMSProxy
	}
	return out
}

func writeEvent(w http.ResponseWriter, f http.Flusher, name string, payload any) {
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, b)
	f.Flush()
}
