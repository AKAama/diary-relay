package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"diary-relay/internal/config"
	"diary-relay/internal/draft"
	"diary-relay/internal/github"
	"diary-relay/internal/hexo"

	"github.com/google/uuid"
)

type Server struct {
	Cfg   *config.Config
	Store *draft.Store
	Hexo  *hexo.Generator
	GH    *github.Client
	Now   func() time.Time
}

type draftRequest struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
}

type draftResponse struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Tags       []string `json:"tags"`
	Filename   string   `json:"filename"`
	PreviewURL string   `json:"preview_url,omitempty"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
}

type publishResponse struct {
	ID        string `json:"id"`
	Filename  string `json:"filename"`
	CommitURL string `json:"commit_url"`
}

func (s *Server) now() time.Time {
	if s.Now != nil {
		return s.Now().In(s.Cfg.Location)
	}
	return time.Now().In(s.Cfg.Location)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (s *Server) handleCreateDraft(w http.ResponseWriter, r *http.Request) {
	var req draftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeErr(w, http.StatusBadRequest, "content is required")
		return
	}
	now := s.now()
	id := uuid.NewString()

	existing, err := s.collectExistingFilenames(now)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "list existing posts: "+err.Error())
		return
	}
	filename := draft.FilenameForDay(now, existing)
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = draft.DefaultTitle(now)
	}

	d := &draft.Draft{
		ID:        id,
		Title:     title,
		Content:   req.Content,
		Tags:      req.Tags,
		Filename:  filename,
		CreatedAt: now.UTC(),
		UpdatedAt: now.UTC(),
	}
	if err := s.Store.Create(d); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	previewURL, err := s.regenPreview(d, now)
	if err != nil {
		// Draft is saved; preview failure is reported but not fatal for create.
		writeJSON(w, http.StatusCreated, map[string]any{
			"id":          d.ID,
			"title":       d.Title,
			"filename":    d.Filename,
			"preview_url": "",
			"warning":     "preview failed: " + err.Error(),
			"created_at":  d.CreatedAt.Format(time.RFC3339),
			"updated_at":  d.UpdatedAt.Format(time.RFC3339),
		})
		return
	}
	writeJSON(w, http.StatusCreated, draftResponse{
		ID: d.ID, Title: d.Title, Content: d.Content, Tags: d.Tags,
		Filename: d.Filename, PreviewURL: previewURL,
		CreatedAt: d.CreatedAt.Format(time.RFC3339),
		UpdatedAt: d.UpdatedAt.Format(time.RFC3339),
	})
}

func (s *Server) handleGetDraft(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d, err := s.Store.Get(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, draftResponse{
		ID: d.ID, Title: d.Title, Content: d.Content, Tags: d.Tags,
		Filename: d.Filename,
		PreviewURL: strings.TrimRight(s.Cfg.PreviewBaseURL, "/") + "/" + d.ID + "/",
		CreatedAt: d.CreatedAt.Format(time.RFC3339),
		UpdatedAt: d.UpdatedAt.Format(time.RFC3339),
	})
}

func (s *Server) handleUpdateDraft(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d, err := s.Store.Get(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	var req draftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeErr(w, http.StatusBadRequest, "content is required")
		return
	}
	if t := strings.TrimSpace(req.Title); t != "" {
		d.Title = t
	}
	d.Content = req.Content
	if req.Tags != nil {
		d.Tags = req.Tags
	}
	now := s.now()
	d.UpdatedAt = now.UTC()
	if err := s.Store.Update(d); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	previewURL, err := s.regenPreview(d, now)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"id": d.ID, "title": d.Title, "filename": d.Filename,
			"preview_url": "", "warning": "preview failed: " + err.Error(),
			"updated_at": d.UpdatedAt.Format(time.RFC3339),
		})
		return
	}
	writeJSON(w, http.StatusOK, draftResponse{
		ID: d.ID, Title: d.Title, Content: d.Content, Tags: d.Tags,
		Filename: d.Filename, PreviewURL: previewURL,
		CreatedAt: d.CreatedAt.Format(time.RFC3339),
		UpdatedAt: d.UpdatedAt.Format(time.RFC3339),
	})
}

func (s *Server) handleDeleteDraft(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.Store.Delete(id); err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	_ = s.Hexo.RemovePreview(id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d, err := s.Store.Get(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	if s.GH == nil || s.Cfg.GitHub.Token == "" {
		writeErr(w, http.StatusServiceUnavailable, "github token not configured")
		return
	}
	now := s.now()
	// Re-resolve filename against live repo in case another post landed today.
	names, err := s.GH.ListPostFilenames()
	if err != nil {
		writeErr(w, http.StatusBadGateway, "list posts: "+err.Error())
		return
	}
	// Keep existing draft filename if still free; else allocate a new one.
	free := true
	for _, n := range names {
		if n == d.Filename {
			free = false
			break
		}
	}
	if !free {
		d.Filename = draft.FilenameForDay(now, names)
		d.UpdatedAt = now.UTC()
		_ = s.Store.Update(d)
	}

	md := draft.BuildMarkdown(d.Title, d.Content, d.Tags, now)
	msg := fmt.Sprintf("diary: add %s", d.Filename)
	commitURL, err := s.GH.CreateOrUpdateFile(d.Filename, md, msg)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "publish: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, publishResponse{
		ID: id, Filename: d.Filename, CommitURL: commitURL,
	})
}

func (s *Server) regenPreview(d *draft.Draft, now time.Time) (string, error) {
	if s.Hexo == nil || s.Cfg.BlogWorkdir == "" {
		return "", fmt.Errorf("hexo/blog_workdir not configured")
	}
	md := draft.BuildMarkdown(d.Title, d.Content, d.Tags, now)
	if _, err := s.Hexo.Generate(d.ID, d.Filename, md); err != nil {
		return "", err
	}
	return strings.TrimRight(s.Cfg.PreviewBaseURL, "/") + "/" + d.ID + "/", nil
}

func (s *Server) collectExistingFilenames(now time.Time) ([]string, error) {
	var names []string
	prefix := "DailyJournal-" + now.Format("0102")
	local, err := s.Store.FilenamesForPrefix(prefix)
	if err != nil {
		return nil, err
	}
	names = append(names, local...)
	if s.GH != nil && s.Cfg.GitHub.Token != "" {
		remote, err := s.GH.ListPostFilenames()
		if err != nil {
			return nil, err
		}
		names = append(names, remote...)
	}
	return names, nil
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
