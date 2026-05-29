package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return New(ts.URL)
}

func TestNewDefaultsBaseURL(t *testing.T) {
	if got := New("").BaseURL; got != DefaultBaseURL {
		t.Fatalf("BaseURL = %q, want %q", got, DefaultBaseURL)
	}
}

func TestOverview(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/overview" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"total_calls":42,"block_rate":0.25,"known_servers":3}`))
	})
	o, err := c.Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if o.TotalCalls != 42 || o.BlockRate != 0.25 || o.KnownServers != 3 {
		t.Fatalf("unexpected overview: %+v", o)
	}
}

func TestCallsAppliesQueryParams(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("limit") != "5" || q.Get("verdict") != "block" {
			t.Fatalf("query = %v", q)
		}
		_, _ = w.Write([]byte(`[{"id":"c1","verdict":"block","tool_name":"fs.read"}]`))
	})
	calls, err := c.Calls(context.Background(), 5, "block")
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].ID != "c1" || calls[0].Verdict != "block" {
		t.Fatalf("unexpected calls: %+v", calls)
	}
}

func TestCallDetail(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/calls/c-42" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"c-42","tool_name":"fs.read","stages":[{"stage":"policy","order":1,"outcome":"allow"}],"artifacts":[]}`))
	})
	d, err := c.Call(context.Background(), "c-42")
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != "c-42" || len(d.Stages) != 1 || d.Stages[0].Stage != "policy" {
		t.Fatalf("unexpected detail: %+v", d)
	}
}

func TestCallEmptyID(t *testing.T) {
	if _, err := New("").Call(context.Background(), ""); err == nil {
		t.Fatal("want error for empty id")
	}
}

func TestServers(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"s1","name":"files","transport":"stdio"}]`))
	})
	servers, err := c.Servers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 || servers[0].Name != "files" {
		t.Fatalf("unexpected servers: %+v", servers)
	}
}

func TestNon2xxIsError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	if _, err := c.Overview(context.Background()); err == nil {
		t.Fatal("want error for 500 response")
	}
}
