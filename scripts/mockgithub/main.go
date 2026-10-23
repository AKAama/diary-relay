// Tiny GitHub Contents API mock for smoke tests. LOCAL ONLY — not for production.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
)

func main() {
	addr := ":18080"
	if v := os.Getenv("MOCK_ADDR"); v != "" {
		addr = v
	}
	var mu sync.Mutex
	files := map[string]string{} // path -> sha

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		path := strings.TrimPrefix(r.URL.Path, "/")
		// /repos/{o}/{r}/contents/{path...}
		parts := strings.SplitN(path, "/", 5)
		if len(parts) < 5 || parts[0] != "repos" || parts[3] != "contents" {
			http.NotFound(w, r)
			return
		}
		contentPath := parts[4]

		switch r.Method {
		case http.MethodGet:
			if strings.HasSuffix(contentPath, "_posts") || strings.HasSuffix(contentPath, "_posts/") {
				list := []map[string]string{}
				for p, sha := range files {
					name := p[strings.LastIndex(p, "/")+1:]
					list = append(list, map[string]string{"name": name, "path": p, "type": "file", "sha": sha})
				}
				_ = json.NewEncoder(w).Encode(list)
				return
			}
			if sha, ok := files[contentPath]; ok {
				_ = json.NewEncoder(w).Encode(map[string]string{"name": contentPath, "path": contentPath, "type": "file", "sha": sha})
				return
			}
			http.NotFound(w, r)
		case http.MethodPut:
			raw, _ := io.ReadAll(r.Body)
			var body map[string]any
			_ = json.Unmarshal(raw, &body)
			if body["branch"] != "source" {
				http.Error(w, `{"message":"branch must be source"}`, 400)
				return
			}
			content, _ := base64.StdEncoding.DecodeString(fmt.Sprint(body["content"]))
			sha := fmt.Sprintf("sha-%d", len(content))
			files[contentPath] = sha
			log.Printf("PUT %s (%d bytes) branch=%v", contentPath, len(content), body["branch"])
			_ = json.NewEncoder(w).Encode(map[string]any{
				"content": map[string]string{"path": contentPath, "sha": sha, "html_url": "https://github.com/example/blob/" + contentPath},
				"commit":  map[string]string{"sha": "commit-" + sha, "html_url": "https://github.com/example/commit/" + sha, "message": fmt.Sprint(body["message"])},
			})
		default:
			http.Error(w, "method", 405)
		}
	})
	log.Printf("mock github on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
