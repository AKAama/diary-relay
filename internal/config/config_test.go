package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_RefuseNonSourceBranch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	content := `
listen: "127.0.0.1:9"
auth_token: "t"
github:
  token: "x"
  branch: "main"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "source") {
		t.Fatalf("err=%v", err)
	}
}

func TestLoad_Defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(path, []byte("auth_token: secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.GitHub.Branch != "source" || c.GitHub.PostsPath != "source/_posts" {
		t.Fatalf("%+v", c.GitHub)
	}
	if c.Timezone != "Asia/Shanghai" {
		t.Fatalf("tz %s", c.Timezone)
	}
}
