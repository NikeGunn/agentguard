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

type pypiResp struct {
	Info pypiInfo                    `json:"info"`
	URLs []pypiURL                   `json:"urls"`
	Rel  map[string][]map[string]any `json:"releases"`
}

type pypiInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Summary     string `json:"summary"`
	HomePage    string `json:"home_page"`
	ProjectURL  string `json:"project_url"`
	License     string `json:"license"`
	Author      string `json:"author"`
	AuthorEmail string `json:"author_email"`
}

type pypiURL struct {
	UploadTime string `json:"upload_time_iso_8601"`
}

// PyPI fetches metadata for a PyPI package by exact name.
// Returns (nil, nil) on 404.
func (c *Client) PyPI(ctx context.Context, name string) (*Metadata, error) {
	if name == "" {
		return nil, errors.New("pypi: empty name")
	}
	u := "https://pypi.org/pypi/" + url.PathEscape(name) + "/json"
	resp, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pypi: status %d", resp.StatusCode)
	}
	var doc pypiResp
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, err
	}
	m := &Metadata{
		Source:      "pypi",
		Name:        doc.Info.Name,
		Version:     doc.Info.Version,
		Description: doc.Info.Summary,
		Homepage:    coalesce(doc.Info.HomePage, doc.Info.ProjectURL),
		License:     doc.Info.License,
		Author:      coalesce(doc.Info.Author, doc.Info.AuthorEmail),
		FoundAt:     time.Now(),
	}
	if len(doc.URLs) > 0 {
		if t, err := time.Parse(time.RFC3339, doc.URLs[0].UploadTime); err == nil {
			m.PublishedAt = t
		}
	}
	return m, nil
}

func coalesce(parts ...string) string {
	for _, p := range parts {
		if p != "" {
			return p
		}
	}
	return ""
}
