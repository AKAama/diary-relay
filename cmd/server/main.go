package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"diary-relay/internal/api"
	"diary-relay/internal/config"
	"diary-relay/internal/draft"
	"diary-relay/internal/github"
	"diary-relay/internal/hexo"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to config.yaml")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := os.MkdirAll(cfg.PreviewDir, 0o755); err != nil {
		log.Fatalf("preview dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		log.Fatalf("db dir: %v", err)
	}

	store, err := draft.OpenStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer store.Close()

	gen := hexo.New(cfg.BlogWorkdir, cfg.PreviewDir)
	var gh *github.Client
	if cfg.GitHub.Token != "" {
		gh = github.New(cfg.GitHub.Token, cfg.GitHub.Owner, cfg.GitHub.Repo, cfg.GitHub.Branch, cfg.GitHub.PostsPath)
		if cfg.GitHub.APIBaseURL != "" {
			gh.BaseURL = strings.TrimRight(cfg.GitHub.APIBaseURL, "/")
		}
	}

	srv := &api.Server{Cfg: cfg, Store: store, Hexo: gen, GH: gh}
	handler := srv.Handler()

	log.Printf("diary-relay listening on %s (preview %s → %s)", cfg.Listen, cfg.PreviewBaseURL, cfg.PreviewDir)
	if cfg.BlogWorkdir == "" {
		log.Printf("WARNING: blog_workdir empty — preview generation disabled")
	}
	if cfg.GitHub.Token == "" {
		log.Printf("WARNING: github.token empty — publish disabled")
	}
	if err := http.ListenAndServe(cfg.Listen, handler); err != nil {
		log.Fatal(err)
	}
}
