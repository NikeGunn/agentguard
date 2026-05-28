package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type ghRepo struct {
	FullName    string    `json:"full_name"`
	Description string    `json:"description"`
	Homepage    string    `json:"homepage"`
	HTMLURL     string    `json:"html_url"`
	License     ghLicense `json:"license"`
	Stargazers  int64     `json:"stargazers_count"`
	Forks       int64     `json:"forks_count"`
	OpenIssues  int64     `json:"open_issues_count"`
	PushedAt    time.Time `json:"pushed_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Archived    bool      `json:"archived"`
	Owner       ghOwner   `json:"owner"`
}

type ghLicense struct {
	SPDXID string `json:"spdx_id"`
}

type ghOwner struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}

// GitHub fetches metadata for "owner/repo". Returns (nil, nil) on 404.
func (c *Client) GitHub(ctx context.Context, ownerRepo string) (*Metadata, error) {
	if ownerRepo == "" {
		return nil, errors.New("github: empty repo")
	}
	parts := strings.SplitN(ownerRepo, "/", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("github: want owner/repo, got %q", ownerRepo)
	}
	u := "https://api.github.com/repos/" + parts[0] + "/" + parts[1]
	resp, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: status %d", resp.StatusCode)
	}
	var r ghRepo
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	published := r.PushedAt
	if published.IsZero() {
		published = r.UpdatedAt
	}
	return &Metadata{
		Source:      "github",
		Name:        r.FullName,
		Description: r.Description,
		Homepage:    coalesce(r.Homepage, r.HTMLURL),
		Repository:  r.HTMLURL,
		License:     r.License.SPDXID,
		Author:      r.Owner.Login,
		Downloads:   r.Stargazers,
		PublishedAt: published,
		FoundAt:     time.Now(),
	}, nil
}
