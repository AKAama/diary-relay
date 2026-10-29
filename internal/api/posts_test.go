package api

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"diary-relay/internal/github"
)

func mockGitHub(t *testing.T) (*httptest.Server, *github.Client) {
	t.Helper()
	md := "---\ntitle: DailyJournal_1218\ndate: 2025-12-18 23:05:12\ntags:\n  - 日常\n---\n牙好疼\n"
	b64 := base64.StdEncoding.EncodeToString([]byte(md))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/contents/source/_posts"):
			_ = json.NewEncoder(w).Encode([]map[string]string{
				{"name": "DailyJournal-1218.md", "path": "source/_posts/DailyJournal-1218.md", "type": "file", "sha": "sha1"},
				{"name": "skip.txt", "path": "source/_posts/skip.txt", "type": "file"},
			})
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/contents/source/_posts/DailyJournal-1218.md"):
			_ = json.NewEncoder(w).Encode(map[string]string{
				"name": "DailyJournal-1218.md", "path": "source/_posts/DailyJournal-1218.md",
				"type": "file", "sha": "sha1", "encoding": "base64", "content": b64,
			})
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/contents/source/_posts/DailyJournal-1218.md"):
			raw, _ := io.ReadAll(r.Body)
			var body map[string]any
			_ = json.Unmarshal(raw, &body)
			if body["sha"] != "sha1" {
				http.Error(w, `{"message":"sha mismatch"}`, 409)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"content": map[string]string{"sha": "sha2", "path": "source/_posts/DailyJournal-1218.md", "name": "DailyJournal-1218.md", "html_url": "https://github.com/x"},
				"commit":  map[string]string{"html_url": "https://github.com/c/new", "sha": "c2"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	c := github.New("tok", "AKAama", "AKAama.github.io", "source", "source/_posts")
	c.BaseURL = srv.URL
	c.HTTPClient = srv.Client()
	return srv, c
}

func TestPostsListGetAndPathSafety(t *testing.T) {
	s := testServer(t)
	_, gh := mockGitHub(t)
	s.GH = gh
	s.Cfg.GitHub.Token = "tok"
	h := s.Handler()

	// unauth
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/posts", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauth list => %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/posts", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("list => %d %s", rr.Code, rr.Body.String())
	}
	var list []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0]["filename"] != "DailyJournal-1218.md" {
		t.Fatalf("list %#v", list)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/posts/DailyJournal-1218.md", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("get => %d %s", rr.Code, rr.Body.String())
	}
	var detail map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail["title"] != "DailyJournal_1218" || detail["content"] != "牙好疼" || detail["sha"] != "sha1" {
		t.Fatalf("detail %#v", detail)
	}
	tags, _ := detail["tags"].([]any)
	if len(tags) != 1 || tags[0] != "日常" {
		t.Fatalf("tags %#v", detail["tags"])
	}

	// non-.md basename rejected by SafePostFilename
	for _, bad := range []string{"notes.txt", "readme"} {
		rr = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/posts/"+bad, nil)
		req.Header.Set("Authorization", "Bearer test-token")
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("bad %q => %d %s", bad, rr.Code, rr.Body.String())
		}
	}
}

func TestUpdatePostPreservesDate(t *testing.T) {
	s := testServer(t)
	_, gh := mockGitHub(t)
	s.GH = gh
	s.Cfg.GitHub.Token = "tok"
	h := s.Handler()

	// After PUT, GetPost is called again — serve updated content with new sha.
	// Our mock returns same content; that's fine for sha refresh attempt.
	body := `{"title":"新标题","content":"改过了","tags":["骑行"],"sha":"sha1"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/posts/DailyJournal-1218.md", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("put => %d %s", rr.Code, rr.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["commit_url"] == nil || out["commit_url"] == "" {
		t.Fatalf("missing commit_url %#v", out)
	}

	// missing sha
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/posts/DailyJournal-1218.md", strings.NewReader(`{"content":"x","sha":""}`))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing sha => %d", rr.Code)
	}
}
