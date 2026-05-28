// Package registry queries public package and source registries (npm,
// PyPI, GitHub) to enrich MCP server metadata. Used by Stage-2
// attestation and the dashboard's "trust" column.
//
// All fetchers share an *http.Client, time-bound every request with a
// context, and return zero-value rich types on a clean "not found".
package registry

import (
	"context"
	"net/http"
	"time"
)

// Metadata is the registry-agnostic shape every fetcher returns.
type Metadata struct {
	Source      string    // "npm", "pypi", "github"
	Name        string    // canonical identifier
	Version     string    // latest published / default branch SHA prefix
	Description string
	Homepage    string
	Repository  string
	License     string
	Author      string
	Downloads   int64     // monthly downloads (npm/pypi) or stargazers (github)
	PublishedAt time.Time // most recent publish or commit time
	FoundAt     time.Time
}

// Client wraps a single shared http.Client and exposes the per-source
// fetchers. Construct via NewClient — the zero value is unusable.
type Client struct {
	HTTP    *http.Client
	UA      string
	Timeout time.Duration
}

// NewClient returns a Client with sensible defaults.
func NewClient() *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: 8 * time.Second},
		UA:      "AgentGuard/1.0 (+https://github.com/agentguard/agentguard)",
		Timeout: 8 * time.Second,
	}
}

func (c *Client) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UA)
	req.Header.Set("Accept", "application/json")
	return c.HTTP.Do(req)
}
