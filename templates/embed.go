// Package templates embeds every HTML page and shared partial into the binary,
// so the server works from any working directory (and inside a container).
package templates

import "embed"

// FS holds the page templates (*.html) and the shared partials (partials/*.html).
//
//go:embed *.html partials/*.html
var FS embed.FS
