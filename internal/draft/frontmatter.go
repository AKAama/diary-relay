package draft

import (
	"fmt"
	"strings"
	"time"
)

// BuildMarkdown produces post body with front matter matching existing DailyJournal posts:
//
//	---
//	title: DailyJournal_1218
//	date: 2025-12-18 23:05:12
//	tags:
//	---
//
// When tags are provided they are emitted as a YAML list.
func BuildMarkdown(title string, content string, tags []string, t time.Time) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("title: ")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString("date: ")
	b.WriteString(t.Format("2006-01-02 15:04:05"))
	b.WriteString("\n")
	if len(tags) == 0 {
		b.WriteString("tags:\n")
	} else {
		b.WriteString("tags:\n")
		for _, tag := range tags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			b.WriteString("  - ")
			b.WriteString(escapeYAMLScalar(tag))
			b.WriteString("\n")
		}
	}
	b.WriteString("---\n")
	b.WriteString(content)
	if !strings.HasSuffix(content, "\n") {
		b.WriteString("\n")
	}
	return b.String()
}

func escapeYAMLScalar(s string) string {
	if strings.ContainsAny(s, ":#{}[],&*!|>'\"%@`") || strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") {
		return fmt.Sprintf("%q", s)
	}
	return s
}
