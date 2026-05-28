package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type npmDoc struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	License     any                   `json:"license"`
	Homepage    string                `json:"homepage"`
	DistTags    map[string]string     `json:"dist-tags"`
	Versions    map[string]npmVersion `json:"versions"`
	Time        map[string]string     `json:"time"`
	Repository  any                   `json:"repository"`
	Author      any                   `json:"author"`
}

type npmVersion struct {
	Version    string `json:"version"`
	Repository any    `json:"repository"`
}

// NPM fetches metadata for an npm package by exact name (incl. scoped).
// Returns (nil, nil) on a clean 404.
func (c *Client) NPM(ctx context.Context, name string) (*Metadata, error) {
	if name == "" {
		return nil, errors.New("npm: empty name")
	}
	u := "https://registry.npmjs.org/" + url.PathEscape(name)
	resp, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("npm: status %d", resp.StatusCode)
	}
	var doc npmDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, err
	}
	latest := doc.DistTags["latest"]
	m := &Metadata{
		Source:      "npm",
		Name:        doc.Name,
		Version:     latest,
		Description: doc.Description,
		Homepage:    doc.Homepage,
		License:     licenseString(doc.License),
		Author:      authorString(doc.Author),
		Repository:  repoString(doc.Repository),
		FoundAt:     time.Now(),
	}
	if latest != "" {
		if ts, ok := doc.Time[latest]; ok {
			if t, err := time.Parse(time.RFC3339, ts); err == nil {
				m.PublishedAt = t
			}
		}
		if v, ok := doc.Versions[latest]; ok && m.Repository == "" {
			m.Repository = repoString(v.Repository)
		}
	}
	if dl, err := c.npmDownloads(ctx, doc.Name); err == nil {
		m.Downloads = dl
	}
	return m, nil
}

type npmDownloadsResp struct {
	Downloads int64 `json:"downloads"`
}

func (c *Client) npmDownloads(ctx context.Context, name string) (int64, error) {
	u := "https://api.npmjs.org/downloads/point/last-month/" + url.PathEscape(name)
	resp, err := c.get(ctx, u)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("status %d", resp.StatusCode)
	}
	var d npmDownloadsResp
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return 0, err
	}
	return d.Downloads, nil
}

func licenseString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case map[string]any:
		if t, ok := s["type"].(string); ok {
			return t
		}
	}
	return ""
}

func authorString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case map[string]any:
		if n, ok := s["name"].(string); ok {
			return n
		}
	}
	return ""
}

func repoString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case map[string]any:
		if u, ok := s["url"].(string); ok {
			return u
		}
	}
	return ""
}
