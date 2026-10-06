// Package webui holds the single page frontend, compiled into the binary so
// that the server needs nothing beside it on disk.
//
// The frontend in ui/ builds straight into dist/ here ("npm run build", or
// "make build" from mangle-go, which does both). The directory is committed
// holding only a placeholder, so the Go code builds before the frontend has
// been; a binary built that way serves the sign-in pages and the API, and
// says the interface is missing where the application would be.
package webui

import (
	"embed"
	"io/fs"
	"regexp"
)

//go:embed all:dist
var dist embed.FS

// FS returns the frontend build: index.html at its root and every asset
// under static/.
func FS() fs.FS {
	build, err := fs.Sub(dist, "dist")
	if err != nil {
		// fs.Sub only fails on an invalid path, and "dist" is a constant.
		panic(err)
	}
	return build
}

// Built reports whether a frontend build was embedded.
func Built() bool {
	_, err := fs.Stat(FS(), "index.html")
	return err == nil
}

// fingerprinted matches the file names the build gives its bundles, which
// carry a hash of their content: "UsersView-BMYrZSJz.js".
var fingerprinted = regexp.MustCompile(`-[A-Za-z0-9_-]{8}\.(?:js|css|woff2?|svg|png|jpg|webp)$`)

// Immutable reports whether an asset's name changes whenever its content
// does, so that a browser may cache it indefinitely.
func Immutable(name string) bool {
	return fingerprinted.MatchString(name)
}
