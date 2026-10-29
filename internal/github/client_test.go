package github

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListPostFilenames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/AKAama/AKAama.github.io/contents/source/_posts" {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.URL.Query().Get("ref") != "source" {
			t.Errorf("ref %s", r.URL.Query().Get("ref"))
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Errorf("auth header missing")
		}
		_ = json.NewEncoder(w).Encode([]map[string]string{
			{"name": "DailyJournal-1218.md", "path": "source/_posts/DailyJournal-1218.md", "type": "file"},
			{"name": "notes", "path": "source/_posts/notes", "type": "dir"},
			{"name": "readme.txt", "path": "source/_posts/readme.txt", "type": "file"},
		})
	}))
	defer srv.Close()

	c := New("tok", "AKAama", "AKAama.github.io", "source", "source/_posts")
	c.BaseURL = srv.URL
	c.HTTPClient = srv.Client()
	names, err := c.ListPostFilenames()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "DailyJournal-1218.md" {
		t.Fatalf("names %#v", names)
	}
}

func TestCreateOrUpdateFile_Create(t *testing.T) {
	var putBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			http.NotFound(w, r)
		case r.Method == http.MethodPut:
			raw, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(raw, &putBody); err != nil {
				t.Fatal(err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"content": map[string]string{"html_url": "https://github.com/x"},
				"commit":  map[string]string{"html_url": "https://github.com/AKAama/AKAama.github.io/commit/abc", "sha": "abc"},
			})
		default:
			http.Error(w, "bad", 405)
		}
	}))
	defer srv.Close()

	c := New("tok", "AKAama", "AKAama.github.io", "source", "source/_posts")
	c.BaseURL = srv.URL
	c.HTTPClient = srv.Client()
	url, err := c.CreateOrUpdateFile("DailyJournal-1023.md", "---\ntitle: x\n---\nbody\n", "diary: add DailyJournal-1023.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(url, "commit/abc") {
		t.Fatalf("url %s", url)
	}
	if putBody["branch"] != "source" {
		t.Fatalf("branch %#v", putBody["branch"])
	}
	if _, ok := putBody["sha"]; ok {
		t.Fatalf("create should not send sha")
	}
	decoded, err := base64.StdEncoding.DecodeString(putBody["content"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(decoded), "body") {
		t.Fatalf("content %s", decoded)
	}
}

func TestCreateOrUpdateFile_UpdateUsesSHA(t *testing.T) {
	var putBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]string{"sha": "existing-sha", "name": "DailyJournal-1023.md", "type": "file"})
		case http.MethodPut:
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &putBody)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"commit": map[string]string{"html_url": "https://github.com/c/def"},
			})
		}
	}))
	defer srv.Close()

	c := New("tok", "AKAama", "AKAama.github.io", "source", "source/_posts")
	c.BaseURL = srv.URL
	c.HTTPClient = srv.Client()
	_, err := c.CreateOrUpdateFile("DailyJournal-1023.md", "x", "msg")
	if err != nil {
		t.Fatal(err)
	}
	if putBody["sha"] != "existing-sha" {
		t.Fatalf("sha %#v", putBody["sha"])
	}
}

func TestRefuseNonSourceBranch(t *testing.T) {
	c := New("tok", "AKAama", "AKAama.github.io", "main", "source/_posts")
	_, err := c.CreateOrUpdateFile("x.md", "y", "z")
	if err == nil || !strings.Contains(err.Error(), "only source") {
		t.Fatalf("err %v", err)
	}
}

func TestGetPostAndListPosts(t *testing.T) {
	md := "---\ntitle: t\n---\nbody\n"
	b64 := base64.StdEncoding.EncodeToString([]byte(md))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/contents/source/_posts") {
			_ = json.NewEncoder(w).Encode([]map[string]string{
				{"name": "a.md", "path": "source/_posts/a.md", "type": "file", "sha": "s"},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"name": "a.md", "path": "source/_posts/a.md", "type": "file", "sha": "s",
			"encoding": "base64", "content": b64 + "\n",
		})
	}))
	defer srv.Close()
	c := New("tok", "AKAama", "AKAama.github.io", "source", "source/_posts")
	c.BaseURL = srv.URL
	c.HTTPClient = srv.Client()
	list, err := c.ListPosts()
	if err != nil || len(list) != 1 || list[0].Name != "a.md" {
		t.Fatalf("%v %#v", err, list)
	}
	f, err := c.GetPost("a.md")
	if err != nil {
		t.Fatal(err)
	}
	if f.Content != md || f.SHA != "s" {
		t.Fatalf("%#v", f)
	}
	if _, err := c.GetPost("../a.md"); err == nil {
		t.Fatal("expected path error")
	}
}

func TestUpdateFileRequiresSHA(t *testing.T) {
	c := New("tok", "AKAama", "AKAama.github.io", "source", "source/_posts")
	_, err := c.UpdateFile("a.md", "x", "m", "")
	if err == nil || !strings.Contains(err.Error(), "sha") {
		t.Fatalf("err %v", err)
	}
}
