package draft

import (
	"strings"
	"testing"
	"time"
)

func TestBuildMarkdown_EmptyTags(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	tm := time.Date(2025, 12, 18, 23, 5, 12, 0, loc)
	md := BuildMarkdown("DailyJournal_1218", "牙好疼好疼", nil, tm)
	want := "---\ntitle: DailyJournal_1218\ndate: 2025-12-18 23:05:12\ntags:\n---\n牙好疼好疼\n"
	if md != want {
		t.Fatalf("mismatch:\n got: %q\nwant: %q", md, want)
	}
}

func TestBuildMarkdown_WithTags(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	tm := time.Date(2026, 10, 23, 8, 0, 0, 0, loc)
	md := BuildMarkdown("DailyJournal_1023", "hello", []string{"骑行", "工作"}, tm)
	if !strings.Contains(md, "tags:\n  - 骑行\n  - 工作\n") {
		t.Fatalf("tags block missing:\n%s", md)
	}
	if !strings.HasPrefix(md, "---\ntitle: DailyJournal_1023\ndate: 2026-10-23 08:00:00\n") {
		t.Fatalf("header:\n%s", md)
	}
	if !strings.HasSuffix(md, "hello\n") {
		t.Fatalf("body suffix:\n%s", md)
	}
}

func TestParseMarkdown(t *testing.T) {
	md := "---\ntitle: DailyJournal_1218\ndate: 2025-12-18 23:05:12\ntags:\n  - 日常\n  - 骑行\n---\n牙好疼\n"
	p := ParseMarkdown(md)
	if p.Title != "DailyJournal_1218" || p.Date != "2025-12-18 23:05:12" {
		t.Fatalf("meta %#v", p)
	}
	if len(p.Tags) != 2 || p.Tags[0] != "日常" || p.Tags[1] != "骑行" {
		t.Fatalf("tags %#v", p.Tags)
	}
	if p.Body != "牙好疼\n" {
		t.Fatalf("body %q", p.Body)
	}
	round := BuildMarkdownWithDate(p.Title, strings.TrimSuffix(p.Body, "\n"), p.Tags, p.Date)
	if !strings.Contains(round, "date: 2025-12-18 23:05:12\n") {
		t.Fatalf("round trip date:\n%s", round)
	}
}

func TestParseMarkdown_NoFrontMatter(t *testing.T) {
	p := ParseMarkdown("plain\n")
	if p.Title != "" || p.Body != "plain\n" {
		t.Fatalf("%#v", p)
	}
}
