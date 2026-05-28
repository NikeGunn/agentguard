// Package dashboard serves the single-page dashboard plus the JSON API and
// SSE event stream the page consumes.
//
// For Milestone 4 the dashboard is a hand-written HTML + vanilla JS + CSS
// trio embedded via embed.FS so the binary stays distribution-friendly.
// The SvelteKit + Bun build replaces these files in a later milestone
// without changing the API surface.
package dashboard

import (
	"embed"
	"io/fs"
)

//go:embed assets/*
var assetsFS embed.FS

// Assets returns the dashboard's static asset tree, rooted at "assets".
func Assets() fs.FS {
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		// embed.FS guarantees this never fails for a literal path that
		// exists at build time; panic so the linker error surfaces.
		panic(err)
	}
	return sub
}
