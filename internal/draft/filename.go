package draft

import (
	"fmt"
	"strings"
	"time"
)

// FilenameForDay returns DailyJournal-MMDD.md, or DailyJournal-MMDD-N.md
// when base (and lower suffixes) already exist in existingNames.
// existingNames should be bare filenames (with .md), case-sensitive as stored in git.
func FilenameForDay(t time.Time, existingNames []string) string {
	mmdd := t.Format("0102")
	base := fmt.Sprintf("DailyJournal-%s.md", mmdd)
	set := make(map[string]struct{}, len(existingNames))
	for _, n := range existingNames {
		set[n] = struct{}{}
	}
	if _, ok := set[base]; !ok {
		return base
	}
	for i := 2; ; i++ {
		cand := fmt.Sprintf("DailyJournal-%s-%d.md", mmdd, i)
		if _, ok := set[cand]; !ok {
			return cand
		}
	}
}

// DefaultTitle returns DailyJournal_MMDD for the given local time.
func DefaultTitle(t time.Time) string {
	return "DailyJournal_" + t.Format("0102")
}

// TitleFromFilename derives DailyJournal_MMDD[_N] style title from a filename
// like DailyJournal-1218.md or DailyJournal-1218-2.md. Falls back to DefaultTitle.
func TitleFromFilename(filename string, t time.Time) string {
	name := strings.TrimSuffix(filename, ".md")
	if strings.HasPrefix(name, "DailyJournal-") {
		return "DailyJournal_" + strings.TrimPrefix(name, "DailyJournal-")
	}
	return DefaultTitle(t)
}
