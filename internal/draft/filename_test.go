package draft

import (
	"testing"
	"time"
)

func TestFilenameForDay(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	tm := time.Date(2025, 12, 18, 23, 5, 12, 0, loc)

	got := FilenameForDay(tm, nil)
	if got != "DailyJournal-1218.md" {
		t.Fatalf("empty existing: got %q", got)
	}

	got = FilenameForDay(tm, []string{"DailyJournal-1218.md"})
	if got != "DailyJournal-1218-2.md" {
		t.Fatalf("one exists: got %q", got)
	}

	got = FilenameForDay(tm, []string{
		"DailyJournal-1218.md",
		"DailyJournal-1218-2.md",
		"other.md",
	})
	if got != "DailyJournal-1218-3.md" {
		t.Fatalf("two exist: got %q", got)
	}

	// Gap in suffixes: -2 missing → still pick -2 (first free)
	got = FilenameForDay(tm, []string{"DailyJournal-1218.md", "DailyJournal-1218-3.md"})
	if got != "DailyJournal-1218-2.md" {
		t.Fatalf("gap: got %q", got)
	}
}

func TestDefaultTitle(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	tm := time.Date(2025, 12, 18, 23, 5, 12, 0, loc)
	if got := DefaultTitle(tm); got != "DailyJournal_1218" {
		t.Fatalf("got %q", got)
	}
}

func TestTitleFromFilename(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	tm := time.Date(2025, 12, 18, 0, 0, 0, 0, loc)
	if got := TitleFromFilename("DailyJournal-1218-2.md", tm); got != "DailyJournal_1218-2" {
		t.Fatalf("got %q", got)
	}
}
