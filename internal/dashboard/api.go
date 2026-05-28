package dashboard

import (
	"encoding/json"
	"net/http"
	"time"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.Overview(r.Context(), parseWindow(r, time.Hour))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleTimeseries(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.CallsByMinute(r.Context(), parseWindow(r, time.Hour))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, nilToEmpty(rows))
}

func (s *Server) handleTopTools(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.TopTools(r.Context(), parseWindow(r, 24*time.Hour), parseLimit(r, 10, 50))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, nilToEmpty(rows))
}

func (s *Server) handleServers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.Servers(r.Context(), parseLimit(r, 100, 500))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, nilToEmpty(rows))
}

func (s *Server) handleCalls(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.RecentCalls(r.Context(), parseLimit(r, 100, 500), r.URL.Query().Get("verdict"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, nilToEmpty(rows))
}

func (s *Server) handleStats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"sse_subscribers": s.store.Broadcaster().Count(),
	})
}

// nilToEmpty turns a nil slice into an empty slice so the JSON encoder
// renders "[]" rather than "null" — the frontend treats null as an error.
func nilToEmpty[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}
