package dashboard

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/agentguard/agentguard/internal/store"
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

// handleCallDetail serves GET /api/calls/{id} — the full single-call record
// the investigation page renders: header, inline request/response (for the
// diff view), the ordered pipeline stage waterfall, and stored artifacts.
func (s *Server) handleCallDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing call id"})
		return
	}
	detail, err := s.store.CallDetail(r.Context(), id)
	if errors.Is(err, store.ErrCallNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "call not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, detail)
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
