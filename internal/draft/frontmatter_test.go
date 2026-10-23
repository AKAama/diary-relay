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
