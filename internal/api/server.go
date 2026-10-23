package api

import (
	"net/http"
	"path/filepath"
	"strings"
)

// Handler builds the HTTP mux. Healthz and /preview/ are public;
// /drafts* require Bearer auth.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)

	auth := BearerAuth(s.Cfg.AuthToken)
	mux.Handle("POST /drafts", auth(http.HandlerFunc(s.handleCreateDraft)))
	mux.Handle("GET /drafts/{id}", auth(http.HandlerFunc(s.handleGetDraft)))
	mux.Handle("PUT /drafts/{id}", auth(http.HandlerFunc(s.handleUpdateDraft)))
	mux.Handle("DELETE /drafts/{id}", auth(http.HandlerFunc(s.handleDeleteDraft)))
	mux.Handle("POST /drafts/{id}/publish", auth(http.HandlerFunc(s.handlePublish)))

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

	return mux
}
