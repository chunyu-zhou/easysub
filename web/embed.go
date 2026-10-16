// Package web exposes the embedded frontend build output.
package web

import "embed"

// Files contains the frontend build output. The all: prefix keeps the placeholder
// file embedded too, so development builds without generated assets still compile.
//
//go:embed all:dist
var Files embed.FS
