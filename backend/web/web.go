// Package web embeds the built frontend (frontend/dist) for serving as a
// single deployable binary in production.
//
// go:embed can only reach files inside this module's directory tree, so it
// cannot embed ../frontend/dist directly. The real prod build therefore
// copies frontend/dist into backend/web/dist before `go build` (see
// CLAUDE.md's "Prod: single binary" section for the exact commands).
// backend/web/dist is a gitignored build artifact, like the backend binary
// itself; only a placeholder backend/web/dist/.gitkeep is committed so that
// `go build ./...` and `go test ./...` still compile from a clean checkout
// without requiring a frontend build first. The "all:" prefix on the embed
// pattern is required so the directory embeds successfully even when it
// contains only the dotfile placeholder (a plain `//go:embed dist` fails to
// compile with "contains no embeddable files" in that case).
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// DistFS returns the embedded frontend build output rooted at its own
// contents (i.e. with the "dist" prefix stripped), suitable for serving
// directly with http.FileServer(http.FS(...)).
func DistFS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		// Only fails for a malformed path, which "dist" never is — the
		// directory is always embedded at compile time.
		panic(err)
	}
	return sub
}
