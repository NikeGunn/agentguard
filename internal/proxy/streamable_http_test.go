package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/agentguard/agentguard/internal/pipeline"
)

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestHTTPProxy_AllowForwardsToUpstream(t *testing.T) {
	var gotBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`))
	}))
	defer upstream.Close()

	chain := pipeline.NewChain(testStage{name: "ok", verdict: pipeline.VerdictPass})
	p := NewHTTPProxy("github", mustURL(t, upstream.URL), NewRouter(chain))

	var captured pipeline.Verdict
	p.OnVerdict = func(_ *pipeline.Message, v pipeline.Verdict, _ []pipeline.Trace) { captured = v }

	reqBody := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"create_issue"}}`
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp/github", strings.NewReader(reqBody)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Fatalf("upstream result not forwarded: %s", rec.Body.String())
	}
	if gotBody != reqBody {
		t.Fatalf("upstream got %q, want %q", gotBody, reqBody)
	}
	if captured != pipeline.VerdictPass {
		t.Fatalf("OnVerdict got %s", captured)
	}
}

func TestHTTPProxy_BlockReturnsRPCErrorAndSkipsUpstream(t *testing.T) {
	upstreamHit := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	chain := pipeline.NewChain(testStage{name: "policy", verdict: pipeline.VerdictBlock, reason: "create_issue denied"})
	p := NewHTTPProxy("github", mustURL(t, upstream.URL), NewRouter(chain))

	reqBody := `{"jsonrpc":"2.0","id":42,"method":"tools/call","params":{"name":"create_issue"}}`
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp/github", strings.NewReader(reqBody)))

	if upstreamHit {
		t.Fatal("blocked call must NOT reach upstream")
	}
	var resp struct {
		ID    json.RawMessage `json:"id"`
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not JSON-RPC: %v (%s)", err, rec.Body.String())
	}
	if resp.Error.Code != -32099 {
		t.Errorf("error code = %d, want -32099", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "Blocked by AgentGuard") {
		t.Errorf("message = %q", resp.Error.Message)
	}
	if string(resp.ID) != "42" {
		t.Errorf("response id = %s, want 42 (correlation)", resp.ID)
	}
}

func TestHTTPProxy_TransformForwardsRewrittenBody(t *testing.T) {
	var gotBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer upstream.Close()

	redacted := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x","token":"[REDACTED]"}}`)
	chain := pipeline.NewChain(testStage{name: "scan", verdict: pipeline.VerdictTransform, xform: redacted})
	p := NewHTTPProxy("github", mustURL(t, upstream.URL), NewRouter(chain))

	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp/github",
		bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x","token":"sk-secret"}}`))))

	if !strings.Contains(gotBody, "[REDACTED]") || strings.Contains(gotBody, "sk-secret") {
		t.Fatalf("transform not applied before forward: %s", gotBody)
	}
}
