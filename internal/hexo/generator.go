package hexo

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Generator runs hexo generate against a shared blog workdir, serializing
// runs with a mutex and isolating each draft's public output under PreviewDir/<id>/.
type Generator struct {
	BlogWorkdir    string
	PreviewDir     string
	PreviewBaseURL string // e.g. https://diary.ismyh.cn/preview — used for root/url override
	HexoBin        string // default "npx" → npx hexo …
	mu             sync.Mutex
}

func New(blogWorkdir, previewDir string) *Generator {
	return &Generator{
		BlogWorkdir: blogWorkdir,
		PreviewDir:  previewDir,
		HexoBin:     "npx",
	}
}

// Generate writes markdown into a per-draft staging post under source/_posts,
// runs hexo generate with public_dir overridden to PreviewDir/<id>, then
// removes the staging post. Concurrent calls are serialized.
//
// Hexo root/url are overridden to /preview/<id>/ so url_for() links resolve
// under the preview mount. ZenMind (and similar themes) also hardcode
// href="/css/…"; those are rewritten after generate.
func (g *Generator) Generate(id, filename, markdown string) (previewRel string, err error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.BlogWorkdir == "" {
		return "", fmt.Errorf("blog_workdir not configured")
	}
	postsDir := filepath.Join(g.BlogWorkdir, "source", "_posts")
	if err := os.MkdirAll(postsDir, 0o755); err != nil {
		return "", err
	}
	themeDir := filepath.Join(g.BlogWorkdir, "themes")
	entries, _ := os.ReadDir(themeDir)
	hasTheme := false
	for _, e := range entries {
		if e.Name() == "." || e.Name() == ".." || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		info, err := os.Stat(filepath.Join(themeDir, e.Name())) // follow symlinks
		if err == nil && info.IsDir() {
			hasTheme = true
			break
		}
	}
	if !hasTheme {
		return "", fmt.Errorf("no theme directory under %s — themes/hexo-theme-ZenMind is a gitlink without .gitmodules; copy a local blog tree that already contains the theme into blog_workdir", themeDir)
	}

	outDir := filepath.Join(g.PreviewDir, id)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	// Wipe previous preview so stale pages don't linger.
	if err := os.RemoveAll(outDir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}

	stagingName := "diary-relay-preview-" + id + "-" + filename
	stagingPath := filepath.Join(postsDir, stagingName)
	if err := os.WriteFile(stagingPath, []byte(markdown), 0o644); err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(stagingPath) }()

	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return "", err
	}
	root, siteURL := previewRootAndURL(g.PreviewBaseURL, id)
	overridePath := filepath.Join(g.BlogWorkdir, ".diary-relay-preview-"+id+".yml")
	override := fmt.Sprintf("public_dir: %q\nroot: %q\nurl: %q\n", absOut, root, siteURL)
	if err := os.WriteFile(overridePath, []byte(override), 0o644); err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(overridePath) }()

	cmd, err := g.hexoCmd("generate", "--config", "_config.yml,"+filepath.Base(overridePath))
	if err != nil {
		return "", err
	}
	cmd.Dir = g.BlogWorkdir
	cmd.Env = append(os.Environ(), "HEXO_HIDE_BANNER=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("hexo generate: %w\n%s", err, truncate(string(out), 4000))
	}
	// hexo-cli may exit 0 while only printing Usage if package.json lacks "hexo" key
	nFiles := 0
	_ = filepath.Walk(absOut, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			nFiles++
		}
		return nil
	})
	if nFiles == 0 {
		return "", fmt.Errorf("hexo generate produced no files in %s\n%s", absOut, truncate(string(out), 4000))
	}
	if err := rewriteHardcodedRootAssets(absOut, root); err != nil {
		return "", fmt.Errorf("rewrite preview assets: %w", err)
	}
	return id, nil
}

// previewRootAndURL derives Hexo root + url for a preview served under PreviewBaseURL/<id>/.
// Example: base https://diary.ismyh.cn/preview → root /preview/<id>/, url …/preview/<id>
func previewRootAndURL(baseURL, id string) (root, siteURL string) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = "http://127.0.0.1:8080/preview"
	}
	siteURL = base + "/" + id
	pathPrefix := "/preview"
	if u, err := url.Parse(base); err == nil && u.Path != "" && u.Path != "/" {
		pathPrefix = strings.TrimRight(u.Path, "/")
	}
	root = pathPrefix + "/" + id + "/"
	return root, siteURL
}

// hardcodedRootAsset rewrites theme-hardcoded site-root asset URLs
// (href="/css/…", src="/img/…") so they resolve under the preview root.
// Paths that already include the preview root (from url_for) are left alone.
var hardcodedRootAsset = regexp.MustCompile(`(?i)(\b(?:href|src)\s*=\s*["'])/(css|js|img|fonts|images)/`)

func rewriteHardcodedRootAssets(dir, root string) error {
	root = strings.TrimRight(root, "/") + "/"
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		name := strings.ToLower(info.Name())
		if !strings.HasSuffix(name, ".html") && !strings.HasSuffix(name, ".htm") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		updated := hardcodedRootAsset.ReplaceAllFunc(b, func(m []byte) []byte {
			// m is like href="/css/  — insert preview root after the leading slash of the path
			sub := hardcodedRootAsset.FindSubmatch(m)
			if len(sub) < 3 {
				return m
			}
			prefix, folder := sub[1], sub[2]
			return []byte(string(prefix) + root + string(folder) + "/")
		})
		if string(updated) == string(b) {
			return nil
		}
		return os.WriteFile(path, updated, info.Mode().Perm())
	})
}

// RemovePreview deletes PreviewDir/<id>.
func (g *Generator) RemovePreview(id string) error {
	return os.RemoveAll(filepath.Join(g.PreviewDir, id))
}

// hexoCmd resolves the local blog's node_modules/.bin/hexo when present,
// otherwise falls back to `npx hexo …`.
func (g *Generator) hexoCmd(args ...string) (*exec.Cmd, error) {
	local := filepath.Join(g.BlogWorkdir, "node_modules", ".bin", "hexo")
	if st, err := os.Stat(local); err == nil && !st.IsDir() {
		return exec.Command(local, args...), nil
	}
	bin := g.HexoBin
	if bin == "" {
		bin = "npx"
	}
	full := append([]string{"hexo"}, args...)
	return exec.Command(bin, full...), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
