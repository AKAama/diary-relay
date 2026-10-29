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

// ParsedPost is markdown split into Hexo-style front matter fields and body.
type ParsedPost struct {
	Title string
	Date  string // raw value as in file, e.g. "2025-12-18 23:05:12"
	Tags  []string
	Body  string
}

// ParseMarkdown extracts title/date/tags and body from a Hexo post.
// Missing front matter yields Body = whole input and empty meta fields.
func ParseMarkdown(md string) ParsedPost {
	md = strings.ReplaceAll(md, "\r\n", "\n")
	if !strings.HasPrefix(md, "---\n") && md != "---" {
		body := md
		if body != "" && !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		return ParsedPost{Body: body}
	}
	rest := strings.TrimPrefix(md, "---\n")
	end := strings.Index(rest, "\n---\n")
	var fm, body string
	if end < 0 {
		if strings.HasSuffix(rest, "\n---") {
			fm = strings.TrimSuffix(rest, "\n---")
			body = ""
		} else if rest == "---" {
			fm, body = "", ""
		} else {
			b := md
			if b != "" && !strings.HasSuffix(b, "\n") {
				b += "\n"
			}
			return ParsedPost{Body: b}
		}
	} else {
		fm = rest[:end]
		body = rest[end+len("\n---\n"):]
	}
	out := ParsedPost{Body: body}
	if out.Body != "" && !strings.HasSuffix(out.Body, "\n") {
		out.Body += "\n"
	}
	inTags := false
	for _, line := range strings.Split(fm, "\n") {
		trim := strings.TrimSpace(line)
		if inTags {
			if strings.HasPrefix(trim, "- ") {
				tag := strings.TrimSpace(strings.TrimPrefix(trim, "- "))
				tag = strings.Trim(tag, `"'`)
				if tag != "" {
					out.Tags = append(out.Tags, tag)
				}
				continue
			}
			inTags = false
		}
		if trim == "tags:" || trim == "tags: []" || trim == "tags:[]" {
			inTags = true
			continue
		}
		if strings.HasPrefix(trim, "tags:") {
			continue
		}
		if strings.HasPrefix(line, "title:") {
			out.Title = strings.TrimSpace(strings.TrimPrefix(line, "title:"))
			out.Title = strings.Trim(out.Title, `"'`)
			continue
		}
		if strings.HasPrefix(line, "date:") {
			out.Date = strings.TrimSpace(strings.TrimPrefix(line, "date:"))
			out.Date = strings.Trim(out.Date, `"'`)
			continue
		}
	}
	return out
}

// BuildMarkdownWithDate is like BuildMarkdown but writes dateFormatted verbatim
// (e.g. preserved original "2025-12-18 23:05:12").
func BuildMarkdownWithDate(title, content string, tags []string, dateFormatted string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("title: ")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString("date: ")
	b.WriteString(dateFormatted)
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
