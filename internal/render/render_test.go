package render

import (
	"strings"
	"testing"

	"github.com/stewartmckee/canvasbin/internal/store"
)

func TestFullDocumentPassesThrough(t *testing.T) {
	src := "<!-- hi -->\n<!DOCTYPE html><html><body>x</body></html>"
	got, err := Document(store.Page{Format: store.FormatHTML, Content: src})
	if err != nil || got != src {
		t.Fatalf("full document should be unchanged, got %q", got)
	}
}

func TestFragmentIsWrapped(t *testing.T) {
	got, _ := Document(store.Page{Title: "A <b>", Format: store.FormatHTML, Content: "<p>hi</p>"})
	if !strings.Contains(got, "<title>A &lt;b&gt;</title>") || !strings.Contains(got, "<p>hi</p>") {
		t.Fatalf("unexpected wrap: %s", got)
	}
}

func TestMarkdown(t *testing.T) {
	got, _ := Document(store.Page{Format: store.FormatMarkdown, Content: "# Hello\n\n| a |\n|---|\n| 1 |"})
	if !strings.Contains(got, `<h1 id="hello">Hello</h1>`) || !strings.Contains(got, "<table>") {
		t.Fatalf("unexpected markdown: %s", got)
	}
}

func TestWithLiveReload(t *testing.T) {
	got := WithLiveReload("<html><body><p>x</p></BODY></html>", "k", "v")
	if !strings.Contains(got, `<p>x</p><script data-canvasbin-live>`) || !strings.HasSuffix(got, "</BODY></html>") {
		t.Fatalf("script should sit before the closing body tag: %s", got)
	}
	if got := WithLiveReload("<p>fragment</p>", "k", "v"); !strings.HasPrefix(got, "<p>fragment</p>\n<script data-canvasbin-live>") {
		t.Fatalf("script should be appended when there is no body tag: %s", got)
	}
}
