package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type GitHubConfig struct {
	Token     string `yaml:"token"`
	Owner     string `yaml:"owner"`
	Repo      string `yaml:"repo"`
	Branch    string `yaml:"branch"`
	PostsPath  string `yaml:"posts_path"` // e.g. "source/_posts"
	APIBaseURL string `yaml:"api_base_url"` // optional; default https://api.github.com (for smoke/mock)
}

type Config struct {
	Listen         string       `yaml:"listen"`
	AuthToken      string       `yaml:"auth_token"`
	GitHub         GitHubConfig `yaml:"github"`
	BlogWorkdir    string       `yaml:"blog_workdir"`
	PreviewDir     string       `yaml:"preview_dir"`
	PreviewBaseURL string       `yaml:"preview_base_url"`
	DBPath         string       `yaml:"db_path"`
	Timezone       string       `yaml:"timezone"`

	Location *time.Location `yaml:"-"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if c.Listen == "" {
		c.Listen = "127.0.0.1:8080"
	}
	if c.AuthToken == "" {
		return nil, fmt.Errorf("auth_token is required")
	}
	if c.GitHub.Owner == "" {
		c.GitHub.Owner = "AKAama"
	}
	if c.GitHub.Repo == "" {
		c.GitHub.Repo = "AKAama.github.io"
	}
	if c.GitHub.Branch == "" {
		c.GitHub.Branch = "source"
	}
	if c.GitHub.PostsPath == "" {
		c.GitHub.PostsPath = "source/_posts"
	}
	if c.PreviewDir == "" {
		c.PreviewDir = "./preview"
	}
	if c.PreviewBaseURL == "" {
		c.PreviewBaseURL = "http://127.0.0.1:8080/preview"
	}
	if c.DBPath == "" {
		c.DBPath = "./data/drafts.db"
	}
	if c.Timezone == "" {
		c.Timezone = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return nil, fmt.Errorf("timezone %q: %w", c.Timezone, err)
	}
	c.Location = loc
	if c.GitHub.Branch != "source" {
		return nil, fmt.Errorf("github.branch must be %q (refusing %q to protect main)", "source", c.GitHub.Branch)
	}
	return &c, nil
}
