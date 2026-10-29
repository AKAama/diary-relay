package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"diary-relay/internal/config"
	"diary-relay/internal/draft"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	store, err := draft.OpenStore(filepath.Join(dir, "drafts.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	cfg := &config.Config{
		AuthToken:      "test-token",
		PreviewDir:     filepath.Join(dir, "preview"),
		PreviewBaseURL: "http://example.test/preview",
		Location:       time.FixedZone("CST", 8*3600),
	}
	return &Server{Cfg: cfg, Store: store, Now: func() time.Time {
		return time.Date(2026, 10, 29, 15, 0, 0, 0, cfg.Location)
	}}
}

func TestListDraftsAndWebUI(t *testing.T) {
	s := testServer(t)
	h := s.Handler()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET / => %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "日记编辑器") {
		t.Fatalf("expected Chinese UI title in body")
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/app.js", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /app.js => %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/drafts", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauth list => %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/drafts", strings.NewReader(`{"content":"hello","tags":["a"]}`))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create => %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/drafts", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("list => %d %s", rr.Code, rr.Body.String())
	}
	var list []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 draft, got %d", len(list))
	}
	if list[0]["content"] != "hello" {
		t.Fatalf("unexpected list payload: %#v", list[0])
	}
}
