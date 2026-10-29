package github

import "testing"

func TestSafePostFilename(t *testing.T) {
	ok, err := SafePostFilename("DailyJournal-1218.md")
	if err != nil || ok != "DailyJournal-1218.md" {
		t.Fatalf("ok case: %q %v", ok, err)
	}
	cases := []string{
		"",
		"..",
		"../x.md",
		"foo/bar.md",
		"/etc/passwd.md",
		"x.md/../y.md",
		"notes.txt",
		"dir\\file.md",
	}
	for _, c := range cases {
		if _, err := SafePostFilename(c); err == nil {
			t.Fatalf("expected error for %q", c)
		}
	}
}
