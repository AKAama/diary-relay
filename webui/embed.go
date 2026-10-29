package webui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed index.html app.js style.css
var assets embed.FS

// Handler serves the same-origin Markdown editor at /, /app.js, /style.css.
func Handler() http.Handler {
	return http.FileServer(http.FS(assets))
}

// Open opens a named asset (for tests).
func Open(name string) (fs.File, error) {
	return assets.Open(name)
}
