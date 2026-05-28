package dashboard

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agentguard/agentguard/internal/store"
)

func tempStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(context.Background(), filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestDashboardAPIRoutes(t *testing.T) {
	s := tempStore(t)
	srv := New(Config{Store: s})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	for _, path := range []string{
		"/api/overview",
		"/api/timeseries",
		"/api/top-tools",
		"/api/servers",
		"/api/calls",
		"/api/stats",
	} {
		t.Run(path, func(t *testing.T) {
			resp, err := ts.Client().Get(ts.URL + path)
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, 200, resp.StatusCode)
			var v any
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&v))
		})
	}
}

func TestDashboardServesIndex(t *testing.T) {
	s := tempStore(t)
	srv := New(Config{Store: s})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
}
