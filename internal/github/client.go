package github

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// Client talks to GitHub Contents API. BaseURL defaults to https://api.github.com
// and is overridable for tests (httptest).
type Client struct {
	Token      string
	Owner      string
	Repo       string
	Branch     string
	PostsPath  string // e.g. "source/_posts"
	HTTPClient *http.Client
	BaseURL    string
}

func New(token, owner, repo, branch, postsPath string) *Client {
	return &Client{
		Token:      token,
		Owner:      owner,
		Repo:       repo,
		Branch:     branch,
		PostsPath:  strings.Trim(postsPath, "/"),
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		BaseURL:    "https://api.github.com",
	}
}

type contentFile struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	SHA         string `json:"sha"`
	Type        string `json:"type"`
	DownloadURL string `json:"download_url"`
}

type putResponse struct {
	Content struct {
		Name string `json:"name"`
		Path string `json:"path"`
		SHA  string `json:"sha"`
		HTML string `json:"html_url"`
	} `json:"content"`
	Commit struct {
		SHA     string `json:"sha"`
		HTMLURL string `json:"html_url"`
		Message string `json:"message"`
	} `json:"commit"`
}

type apiError struct {
	Message string `json:"message"`
}

// ListPostFilenames lists bare .md filenames under PostsPath on Branch.
func (c *Client) ListPostFilenames() ([]string, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s",
		c.BaseURL, c.Owner, c.Repo, c.PostsPath, url.QueryEscape(c.Branch))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list contents: %s: %s", resp.Status, parseAPIError(body))
	}
	var files []contentFile
	if err := json.Unmarshal(body, &files); err != nil {
		return nil, fmt.Errorf("decode list: %w", err)
	}
	var names []string
	for _, f := range files {
		if f.Type == "file" && strings.HasSuffix(f.Name, ".md") {
			names = append(names, f.Name)
		}
	}
	return names, nil
}

// CreateOrUpdateFile writes content to PostsPath/filename on Branch.
// Returns the commit HTML URL.
func (c *Client) CreateOrUpdateFile(filename, content, message string) (commitURL string, err error) {
	if c.Branch != "source" {
		return "", fmt.Errorf("refusing to write branch %q (only source allowed)", c.Branch)
	}
	filePath := path.Join(c.PostsPath, filename)
	sha, err := c.getFileSHA(filePath)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"message": message,
		"content": base64.StdEncoding.EncodeToString([]byte(content)),
		"branch":  c.Branch,
	}
	if sha != "" {
		payload["sha"] = sha
	}
	raw, _ := json.Marshal(payload)
	u := fmt.Sprintf("%s/repos/%s/%s/contents/%s", c.BaseURL, c.Owner, c.Repo, filePath)
	req, err := http.NewRequest(http.MethodPut, u, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	c.setHeaders(req)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("put contents: %s: %s", resp.Status, parseAPIError(body))
	}
	var pr putResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return "", fmt.Errorf("decode put: %w", err)
	}
	if pr.Commit.HTMLURL != "" {
		return pr.Commit.HTMLURL, nil
	}
	return pr.Content.HTML, nil
}

func (c *Client) getFileSHA(filePath string) (string, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s",
		c.BaseURL, c.Owner, c.Repo, filePath, url.QueryEscape(c.Branch))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	c.setHeaders(req)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("get file: %s: %s", resp.Status, parseAPIError(body))
	}
	var f contentFile
	if err := json.Unmarshal(body, &f); err != nil {
		return "", err
	}
	return f.SHA, nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Authorization", "Bearer "+c.Token)
	if req.Method == http.MethodPut || req.Method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
}

func parseAPIError(body []byte) string {
	var e apiError
	if json.Unmarshal(body, &e) == nil && e.Message != "" {
		return e.Message
	}
	s := string(body)
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
