package render

import (
	"strings"
	"testing"

	"github.com/stewartmckee/canvasbin/internal/store"
)

func TestPreviewTitleFallsBackToHeading(t *testing.T) {
	if got := PreviewTitle("  Weekly  ", store.FormatHTML, "<h1>Ignored</h1>"); got != "Weekly" {
		t.Fatalf("stored title: %q", got)
	}
	if got := PreviewTitle("", store.FormatHTML, "<title>Canvasbin</title><h1>Launch notes</h1>"); got != "Launch notes" {
		t.Fatalf("html heading: %q", got)
	}
	if got := PreviewTitle("", store.FormatMarkdown, "```\n# not this\n```\n## Status\n"); got != "Status" {
		t.Fatalf("markdown heading: %q", got)
	}
	if got := PreviewTitle("", store.FormatHTML, "<p>no heading</p>"); got != "Canvasbin" {
		t.Fatalf("empty: %q", got)
	}
}

func TestApplyLinkPreviewReplacesShellTags(t *testing.T) {
	shell := `<!doctype html><html><head>
<title>Canvasbin</title>
<meta
  name="description"
  content="Share HTML and Markdown pages with a link."
>
<meta property="og:title" content="Canvasbin">
<meta property="og:description" content="Share HTML and Markdown pages with a link.">
<meta property="og:url" content="https://canvasbin.dev/">
<meta property="og:image" content="https://canvasbin.dev/og.png">
<link rel="canonical" href="https://canvasbin.dev/">
<meta name="theme-color" content="#4f46e5">
</head><body><div id="app"></div></body></html>`
	got := ApplyLinkPreview(shell, LinkPreview{
		Title:       "Launch notes",
		Description: `A <week> of "shipping"`,
		URL:         "https://canvasbin.dev/abc.html",
		SiteName:    "Canvasbin",
	})
	for _, want := range []string{
		"<title>Launch notes</title>",
		`property="og:title" content="Launch notes"`,
		`property="og:description" content="A &lt;week&gt; of &#34;shipping&#34;"`,
		`property="og:url" content="https://canvasbin.dev/abc.html"`,
		`rel="canonical" href="https://canvasbin.dev/abc.html"`,
		`name="theme-color"`,
		`<div id="app"></div>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s\n%s", want, got)
		}
	}
	for _, gone := range []string{
		"Share HTML and Markdown pages",
		`content="https://canvasbin.dev/"`,
		`href="https://canvasbin.dev/"`,
		"og.png",
		"<title>Canvasbin</title>",
	} {
		if strings.Contains(got, gone) {
			t.Errorf("still has %s\n%s", gone, got)
		}
	}
}

func TestApplyLinkPreviewKeepsBody(t *testing.T) {
	src := "<!doctype html><html><head><title>Old</title></head><body><p>keep</p></body></html>"
	got := ApplyLinkPreview(src, LinkPreview{Title: "New", URL: "https://canvasbin.dev/k.html"})
	if !strings.Contains(got, "<title>New</title>") || !strings.Contains(got, "<p>keep</p>") {
		t.Fatalf("%s", got)
	}
	if strings.Contains(got, "og:description") {
		t.Fatalf("empty description should be omitted: %s", got)
	}
}
