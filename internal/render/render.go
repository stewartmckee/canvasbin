// Package render turns stored page content into a standalone HTML document.
package render

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"

	"github.com/stewartmckee/canvasbin/internal/store"
)

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM, extension.Footnote),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(gmhtml.WithUnsafe()),
)

var fullDocRe = regexp.MustCompile(`(?is)^\s*(<!--.*?-->\s*)*(<!doctype\s|<html[\s>])`)

// IsFullDocument reports whether HTML content already carries its own document shell.
func IsFullDocument(content string) bool {
	return fullDocRe.MatchString(content)
}

// Markdown converts Markdown source to an HTML fragment.
func Markdown(src string) (string, error) {
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Document returns the page as a complete HTML document.
func Document(p store.Page) (string, error) {
	switch p.Format {
	case store.FormatMarkdown:
		body, err := Markdown(p.Content)
		if err != nil {
			return "", err
		}
		return wrap(p.Title, markdownCSS, `<article class="markdown-body">`+body+`</article>`), nil
	default:
		if IsFullDocument(p.Content) {
			return p.Content, nil
		}
		return wrap(p.Title, fragmentCSS, p.Content), nil
	}
}

var closeBodyRe = regexp.MustCompile(`(?i)</body\s*>`)

// liveReloadJS reconnects with backoff and reloads when the page changes, or
// when it changed while the socket was down (hello carries the current version).
const liveReloadJS = `(function(){var k=%s,v=%s,d=1000;function go(){var ws;try{ws=new WebSocket((location.protocol=="https:"?"wss://":"ws://")+location.host+"/api/pages/"+k+"/live")}catch(e){return}` +
	`ws.onopen=function(){d=1000};ws.onmessage=function(e){var m;try{m=JSON.parse(e.data)}catch(x){return}` +
	`if(m.type=="updated"||m.type=="deleted"||(m.type=="hello"&&m.updated_at!=v)){ws.onclose=null;location.reload()}};` +
	`ws.onclose=function(){setTimeout(go,d);d=Math.min(d*2,30000)}}go()})();`

// WithLiveReload adds a script that reloads the document when the page with
// readKey changes after version (the page's updated_at as sent by the server).
func WithLiveReload(doc, readKey, version string) string {
	tag := `<script data-canvasbin-live>` + fmt.Sprintf(liveReloadJS, strconv.Quote(readKey), strconv.Quote(version)) + `</script>`
	locs := closeBodyRe.FindAllStringIndex(doc, -1)
	if len(locs) == 0 {
		return doc + "\n" + tag + "\n"
	}
	at := locs[len(locs)-1][0]
	return doc[:at] + tag + "\n" + doc[at:]
}

func wrap(title, css, body string) string {
	if strings.TrimSpace(title) == "" {
		title = "Canvasbin"
	}
	return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + html.EscapeString(title) + `</title>
<style>` + css + `</style>
</head>
<body>
` + body + `
</body>
</html>
`
}

const fragmentCSS = `body{margin:0 auto;max-width:960px;padding:2rem 1.25rem;font-family:system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;line-height:1.55;color:#1f2328}
img,video{max-width:100%;height:auto}
table{border-collapse:collapse}td,th{border:1px solid #d0d7de;padding:.35rem .6rem}
pre{overflow:auto;background:#f6f8fa;padding:.8rem;border-radius:6px}`

const markdownCSS = `body{margin:0;background:#fff;color:#1f2328;font-family:system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
.markdown-body{max-width:860px;margin:0 auto;padding:2.5rem 1.25rem 4rem;line-height:1.6;font-size:16px}
.markdown-body h1,.markdown-body h2{border-bottom:1px solid #d8dee4;padding-bottom:.3em}
.markdown-body h1,.markdown-body h2,.markdown-body h3,.markdown-body h4{margin:1.5em 0 .6em;line-height:1.25}
.markdown-body a{color:#0969da}
.markdown-body code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:.88em;background:#eff1f3;padding:.15em .35em;border-radius:5px}
.markdown-body pre{background:#f6f8fa;padding:1rem;border-radius:6px;overflow:auto}
.markdown-body pre code{background:none;padding:0}
.markdown-body blockquote{margin:0;padding:0 1em;color:#59636e;border-left:.25em solid #d1d9e0}
.markdown-body table{border-collapse:collapse;display:block;overflow:auto}
.markdown-body td,.markdown-body th{border:1px solid #d1d9e0;padding:.4rem .8rem}
.markdown-body tr:nth-child(2n){background:#f6f8fa}
.markdown-body img{max-width:100%}
.markdown-body hr{border:0;border-top:1px solid #d1d9e0;margin:2em 0}`
