package api

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"diary-relay/internal/github"
)

func multipartBody(t *testing.T, data []byte) (*bytes.Buffer, string) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "x.png")
	_, _ = fw.Write(data)
	_ = mw.Close()
	return &buf, mw.FormDataContentType()
}

func TestUploadImage(t *testing.T) {
	s := testServer(t)
	s.Cfg.BlogWorkdir = t.TempDir()
	var putPath string
	gsrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.NotFound(w, r)
			return
		}
		putPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		var b map[string]any
		_ = json.Unmarshal(raw, &b)
		if b["branch"] != "source" || b["content"] == "" {
			http.Error(w, "bad", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"commit": map[string]string{"html_url": "https://github.com/c/img"}})
	}))
	defer gsrv.Close()
	gh := github.New("tok", "AKAama", "AKAama.github.io", "source", "source/_posts")
	gh.BaseURL = gsrv.URL
	s.GH = gh
	h := s.Handler()

	img := image.NewNRGBA(image.Rect(0, 0, 20, 10))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	img.Set(0, 0, color.NRGBA{0, 0, 0, 255})
	var pngBuf bytes.Buffer
	_ = png.Encode(&pngBuf, img)

	// unauth
	body, ct := multipartBody(t, pngBuf.Bytes())
	req := httptest.NewRequest(http.MethodPost, "/images", body)
	req.Header.Set("Content-Type", ct)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauth => %d", rr.Code)
	}

	body, ct = multipartBody(t, pngBuf.Bytes())
	req = httptest.NewRequest(http.MethodPost, "/images", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer test-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("upload => %d %s", rr.Code, rr.Body.String())
	}
	var resp imageResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if !regexp.MustCompile(`^/images/\d{4}/\d{2}/\d+-[0-9a-f]{8}\.jpg$`).MatchString(resp.Path) {
		t.Fatalf("path %q", resp.Path)
	}
	if resp.Markdown != "![]("+resp.Path+")" || resp.CommitURL == "" {
		t.Fatalf("resp %+v", resp)
	}
	if !strings.HasSuffix(putPath, "/contents/source"+resp.Path) {
		t.Fatalf("put path %q", putPath)
	}
	if _, err := os.Stat(filepath.Join(s.Cfg.BlogWorkdir, "source", resp.Path)); err != nil {
		t.Fatalf("local mirror: %v", err)
	}

	// unsupported type
	body, ct = multipartBody(t, []byte("not an image at all"))
	req = httptest.NewRequest(http.MethodPost, "/images?dry_run=1", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer test-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("bad type => %d", rr.Code)
	}
}
