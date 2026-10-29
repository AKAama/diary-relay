package api

import (
	"net/http"
	"path/filepath"
	"strings"

	"diary-relay/webui"
)

// Handler builds the HTTP mux. Healthz, /preview/, and the web editor are public;
// /drafts* and /posts* require Bearer auth.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)

	auth := BearerAuth(s.Cfg.AuthToken)
	mux.Handle("GET /drafts", auth(http.HandlerFunc(s.handleListDrafts)))
	mux.Handle("POST /drafts", auth(http.HandlerFunc(s.handleCreateDraft)))
	mux.Handle("GET /drafts/{id}", auth(http.HandlerFunc(s.handleGetDraft)))
	mux.Handle("PUT /drafts/{id}", auth(http.HandlerFunc(s.handleUpdateDraft)))
	mux.Handle("DELETE /drafts/{id}", auth(http.HandlerFunc(s.handleDeleteDraft)))
	mux.Handle("POST /drafts/{id}/publish", auth(http.HandlerFunc(s.handlePublish)))

	mux.Handle("GET /posts", auth(http.HandlerFunc(s.handleListPosts)))
	mux.Handle("GET /posts/{name}", auth(http.HandlerFunc(s.handleGetPost)))
	mux.Handle("PUT /posts/{name}", auth(http.HandlerFunc(s.handleUpdatePost)))

	mux.Handle("POST /images", auth(http.HandlerFunc(s.handleUploadImage)))

	previewRoot, _ := filepath.Abs(s.Cfg.PreviewDir)
	fs := http.FileServer(http.Dir(previewRoot))
	mux.Handle("/preview/", http.StripPrefix("/preview/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent path escape; FileServer already cleans, but refuse ".."
		if strings.Contains(r.URL.Path, "..") {
			http.NotFound(w, r)
			return
		}
		fs.ServeHTTP(w, r)
	})))

	// Same-origin editor UI (no CORS needed).
	ui := webui.Handler()
	mux.Handle("GET /{$}", ui)
	mux.Handle("GET /index.html", ui)
	mux.Handle("GET /app.js", ui)
	mux.Handle("GET /style.css", ui)

	return mux
}
