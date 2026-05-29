package dashboard

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/agentguard/agentguard/internal/store"
)

// Server holds the dashboard's HTTP handlers and dependencies. Construct
// via New() and start via Run().
type Server struct {
	store *store.Store
	addr  string
	mux   *chi.Mux
}

// Config configures a new Server.
type Config struct {
	Store *store.Store
	// Addr is the listen address. Default: 127.0.0.1:7878.
	Addr string
}

// New returns a configured but unstarted Server.
func New(cfg Config) *Server {
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:7878"
	}
	s := &Server{store: cfg.Store, addr: cfg.Addr}
	s.mux = s.routes()
	return s
}

// Addr returns the listen address (useful for tests via httptest).
func (s *Server) Addr() string { return s.addr }

// Handler returns the underlying http.Handler so tests can bind it to an
// httptest.Server instead of opening a real socket.
func (s *Server) Handler() http.Handler { return s.mux }

// Run starts the dashboard HTTP server and blocks until ctx is cancelled
// or the server errors. Returns http.ErrServerClosed on a clean shutdown.
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		err := <-errCh
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (s *Server) routes() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	// All requests bind to localhost only (the listener already enforces
	// 127.0.0.1) — no CSRF or origin checks needed at this stage.

	// The 30s timeout applies ONLY to the snapshot JSON API, which must be
	// fast. It must NOT wrap /events: SSE is a long-lived stream that has
	// already written its 200 headers, so when the timeout fired it tried to
	// write a 504 on top of the open stream — the "superfluous WriteHeader"
	// log that repeated every ~31s. Static assets are likewise left unwrapped.
	r.Group(func(api chi.Router) {
		api.Use(middleware.Timeout(30 * time.Second))
		api.Get("/api/overview", s.handleOverview)
		api.Get("/api/timeseries", s.handleTimeseries)
		api.Get("/api/top-tools", s.handleTopTools)
		api.Get("/api/servers", s.handleServers)
		api.Get("/api/calls", s.handleCalls)
		api.Get("/api/calls/{id}", s.handleCallDetail)
		api.Get("/api/stats", s.handleStats)
	})

	r.Get("/events", s.handleSSE)

	// Static asset handler — serves embedded dashboard at "/".
	r.Handle("/*", http.FileServer(http.FS(Assets())))
	return r
}

// parseWindow reads ?window= into a time.Duration, defaulting to one hour.
func parseWindow(r *http.Request, def time.Duration) time.Duration {
	if v := r.URL.Query().Get("window"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return def
}

func parseLimit(r *http.Request, def, max int) int {
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > max {
				return max
			}
			return n
		}
	}
	return def
}
