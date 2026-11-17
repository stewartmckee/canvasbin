package render

import (
	"html"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/stewartmckee/canvasbin/internal/store"
)

// LinkPreview is the title and description chat apps show when a page link is pasted.
type LinkPreview struct {
	Title       string
	Description string
	URL         string
	SiteName    string
}

const (
	maxPreviewTitle       = 200
	maxPreviewDescription = 300
)

var (
	headOpenRe  = regexp.MustCompile(`(?i)<head(?:\s[^>]*)?>`)
	headCloseRe = regexp.MustCompile(`(?i)</head\s*>`)
	htmlOpenRe  = regexp.MustCompile(`(?i)<html(?:\s[^>]*)?>`)
	titleTagRe  = regexp.MustCompile(`(?is)<title[^>]*>[\s\S]*?</title>`)
	metaTagRe   = regexp.MustCompile(`(?is)<meta\s[^>]*>`)
	canonRe     = regexp.MustCompile(`(?is)<link\s[^>]*rel\s*=\s*["']canonical["'][^>]*>`)
	docTitleRe  = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	h1Re        = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)
	tagsRe      = regexp.MustCompile(`(?s)<[^>]*>`)
)

// PreviewTitle is the name used for the browser tab and link cards.
// A stored title wins. Otherwise the first heading in the page is used.
func PreviewTitle(stored, format, content string) string {
	if t := oneLine(stored); t != "" {
		return clip(t, maxPreviewTitle)
	}
	var found []string
	if format == store.FormatMarkdown {
		if h := markdownHeading(content); h != "" {
			found = append(found, h)
		}
	} else {
		if t := textOf(docTitleRe, content); t != "" {
			found = append(found, t)
		}
		if h := textOf(h1Re, content); h != "" {
			found = append(found, h)
		}
	}
	for _, c := range found {
		c = oneLine(c)
		if c != "" && !genericTitle(c) {
			return clip(c, maxPreviewTitle)
		}
	}
	return "Canvasbin"
}

// PreviewDescription is the link-card summary. A stored description wins.
// Otherwise a description meta tag in the page source is used.
func PreviewDescription(stored, content string) string {
	if d := oneLine(stored); d != "" {
		return clip(d, maxPreviewDescription)
	}
	if d := oneLine(namedMeta(content, "description")); d != "" {
		return clip(d, maxPreviewDescription)
	}
	return ""
}

func namedMeta(doc, name string) string {
	for _, tag := range metaTagRe.FindAllString(doc, -1) {
		if strings.EqualFold(metaAttr(tag, "name"), name) {
			return metaAttr(tag, "content")
		}
	}
	return ""
}

// ApplyLinkPreview writes title, description and Open Graph tags into the
// document head. The body is left as it is. Existing preview tags in the head
// are replaced so a shell page cannot keep advertising the site itself.
func ApplyLinkPreview(doc string, p LinkPreview) string {
	title := oneLine(p.Title)
	if title == "" {
		title = "Canvasbin"
	}
	title = clip(title, maxPreviewTitle)
	desc := clip(oneLine(p.Description), maxPreviewDescription)
	site := oneLine(p.SiteName)
	if site == "" {
		site = "Canvasbin"
	}
	block := previewBlock(title, desc, oneLine(p.URL), site)

	open := headOpenRe.FindStringIndex(doc)
	if open == nil {
		return insertHead(doc, block)
	}
	rest := doc[open[1]:]
	close := headCloseRe.FindStringIndex(rest)
	if close == nil {
		return doc[:open[1]] + block + doc[open[1]:]
	}
	inner := titleTagRe.ReplaceAllString(rest[:close[0]], "")
	inner = metaTagRe.ReplaceAllStringFunc(inner, dropPreviewMeta)
	inner = canonRe.ReplaceAllString(inner, "")
	return doc[:open[1]] + block + inner + rest[close[0]:]
}

func previewBlock(title, desc, url, site string) string {
	var b strings.Builder
	b.WriteString("\n<title>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</title>\n")
	if desc != "" {
		writeMeta(&b, "name", "description", desc)
	}
	writeMeta(&b, "property", "og:type", "article")
	writeMeta(&b, "property", "og:site_name", site)
	writeMeta(&b, "property", "og:title", title)
	if desc != "" {
		writeMeta(&b, "property", "og:description", desc)
	}
	if url != "" {
		writeMeta(&b, "property", "og:url", url)
		b.WriteString(`<link rel="canonical" href="`)
		b.WriteString(html.EscapeString(url))
		b.WriteString("\">\n")
	}
	writeMeta(&b, "name", "twitter:card", "summary")
	writeMeta(&b, "name", "twitter:title", title)
	if desc != "" {
		writeMeta(&b, "name", "twitter:description", desc)
	}
	return b.String()
}

func writeMeta(b *strings.Builder, attr, name, content string) {
	b.WriteString("<meta ")
	b.WriteString(attr)
	b.WriteString(`="`)
	b.WriteString(html.EscapeString(name))
	b.WriteString(`" content="`)
	b.WriteString(html.EscapeString(content))
	b.WriteString("\">\n")
}

func insertHead(doc, block string) string {
	head := "<head>" + block + "</head>"
	if loc := htmlOpenRe.FindStringIndex(doc); loc != nil {
		return doc[:loc[1]] + head + doc[loc[1]:]
	}
	return head + doc
}

func dropPreviewMeta(tag string) string {
	name := strings.ToLower(metaAttr(tag, "name"))
	prop := strings.ToLower(metaAttr(tag, "property"))
	switch name {
	case "description", "twitter:card", "twitter:title", "twitter:description", "twitter:image":
		return ""
	}
	switch prop {
	case "og:title", "og:description", "og:url", "og:type", "og:site_name",
		"og:image", "og:image:width", "og:image:height", "og:image:alt":
		return ""
	}
	return tag
}

func metaAttr(tag, attr string) string {
	re := regexp.MustCompile(`(?i)(?:^|\s)` + regexp.QuoteMeta(attr) + `\s*=\s*("([^"]*)"|'([^']*)')`)
	m := re.FindStringSubmatch(tag)
	if m == nil {
		return ""
	}
	if m[2] != "" {
		return html.UnescapeString(m[2])
	}
	return html.UnescapeString(m[3])
}

func textOf(re *regexp.Regexp, src string) string {
	m := re.FindStringSubmatch(src)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(tagsRe.ReplaceAllString(m[1], " "))
}

func markdownHeading(src string) string {
	fence := false
	for _, line := range strings.Split(src, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "```") {
			fence = !fence
			continue
		}
		if fence || !strings.HasPrefix(trim, "#") {
			continue
		}
		i := 0
		for i < len(trim) && trim[i] == '#' {
			i++
		}
		if i > 0 && i <= 3 && i < len(trim) && trim[i] == ' ' {
			return strings.TrimSpace(trim[i:])
		}
	}
	return ""
}

func genericTitle(t string) bool {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "canvasbin", "canvasbin.dev", "untitled", "untitled page":
		return true
	}
	return false
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
