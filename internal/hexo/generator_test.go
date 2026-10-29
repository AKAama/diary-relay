package hexo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreviewRootAndURL(t *testing.T) {
	root, site := previewRootAndURL("https://diary.ismyh.cn/preview", "abc")
	if root != "/preview/abc/" {
		t.Fatalf("root=%q", root)
	}
	if site != "https://diary.ismyh.cn/preview/abc" {
		t.Fatalf("site=%q", site)
	}
	root, site = previewRootAndURL("http://127.0.0.1:18081/preview/", "x")
	if root != "/preview/x/" || site != "http://127.0.0.1:18081/preview/x" {
		t.Fatalf("got root=%q site=%q", root, site)
	}
	root, _ = previewRootAndURL("", "z")
	if root != "/preview/z/" {
		t.Fatalf("default root=%q", root)
	}
}

func TestRewriteHardcodedRootAssets(t *testing.T) {
	dir := t.TempDir()
	html := `<link href="/css/style.css"><script src="/js/a.js"></script>` +
		`<img src="/img/x.png"><a href="/preview/abc/css/already.css">` +
		`<link href="https://cdn.example/x.css">`
	path := filepath.Join(dir, "index.html")
	if err := os.WriteFile(path, []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := rewriteHardcodedRootAssets(dir, "/preview/abc/"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	s := string(got)
	for _, want := range []string{
		`href="/preview/abc/css/style.css"`,
		`src="/preview/abc/js/a.js"`,
		`src="/preview/abc/img/x.png"`,
		`href="/preview/abc/css/already.css"`,
		`href="https://cdn.example/x.css"`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in:\n%s", want, s)
		}
	}
	if strings.Contains(s, `href="/css/`) || strings.Contains(s, `src="/js/`) {
		t.Fatalf("unrewritten root asset left in:\n%s", s)
	}
}
