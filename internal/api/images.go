package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

	"diary-relay/internal/imageproc"
)

type imageResponse struct {
	Path      string `json:"path"`     // site path, e.g. /images/2026/10/xxx.jpg
	Markdown  string `json:"markdown"` // ![](/images/...)
	Size      int    `json:"size"`
	CommitURL string `json:"commit_url,omitempty"`
	DryRun    bool   `json:"dry_run,omitempty"`
	Warning   string `json:"warning,omitempty"`
}

// imageName builds images/YYYY/MM/<unix-ms>-<rand>.<ext> (no leading slash).
func imageName(now time.Time, ext string) string {
	var rb [4]byte
	_, _ = rand.Read(rb[:])
	return fmt.Sprintf("images/%04d/%02d/%d-%s.%s",
		now.Year(), int(now.Month()), now.UnixMilli(), hex.EncodeToString(rb[:]), ext)
}

// handleUploadImage: multipart field "file". ?dry_run=1 skips the GitHub commit
// (image is still written to the local preview blog source).
func (s *Server) handleUploadImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, imageproc.MaxInputBytes+(1<<20))
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeErr(w, http.StatusRequestEntityTooLarge, imageproc.ErrTooLarge.Error())
			return
		}
		writeErr(w, http.StatusBadRequest, "需要 multipart/form-data，字段名 file")
		return
	}
	defer r.MultipartForm.RemoveAll()
	f, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "缺少文件字段 file")
		return
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, imageproc.MaxInputBytes+1))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "读取文件失败")
		return
	}
	res, err := imageproc.Process(raw)
	if err != nil {
		code := http.StatusUnsupportedMediaType
		if errors.Is(err, imageproc.ErrTooLarge) {
			code = http.StatusRequestEntityTooLarge
		} else if !errors.Is(err, imageproc.ErrUnsupported) && !errors.Is(err, imageproc.ErrHEIC) {
			code = http.StatusBadRequest
		}
		writeErr(w, code, err.Error())
		return
	}

	dry := r.URL.Query().Get("dry_run") == "1"
	if !dry && s.GH == nil {
		writeErr(w, http.StatusServiceUnavailable, "github token not configured")
		return
	}
	rel := imageName(s.now(), res.Ext)
	out := imageResponse{Path: "/" + rel, Markdown: "![](/" + rel + ")", Size: len(res.Data), DryRun: dry}

	if !dry {
		url, err := s.GH.PutImage(path.Join("source", rel), res.Data, "diary-relay: upload "+path.Base(rel))
		if err != nil {
			writeErr(w, http.StatusBadGateway, "上传到 GitHub 失败: "+err.Error())
			return
		}
		out.CommitURL = url
	}
	// Mirror into local blog source so hexo previews include it.
	if s.Cfg.BlogWorkdir != "" {
		local := filepath.Join(s.Cfg.BlogWorkdir, "source", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(local), 0o755); err == nil {
			err = os.WriteFile(local, res.Data, 0o644)
		}
		if err != nil {
			log.Printf("image local mirror: %v", err)
			out.Warning = "预览目录写入失败: " + err.Error()
		}
	}
	writeJSON(w, http.StatusOK, out)
}
