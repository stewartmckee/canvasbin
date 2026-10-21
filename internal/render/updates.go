package render

import (
	"fmt"
	"html"
	"strings"
	"time"

	nethtml "golang.org/x/net/html"

	"github.com/stewartmckee/canvasbin/internal/store"
)

const updatesInjectMarker = "<!-- canvasbin-updates -->"

// updatesCSS is scoped to the block and inherits the page's fonts and colours,
// so it sits naturally under any hero.
const updatesCSS = `<style>
#canvasbin-updates{box-sizing:border-box;margin:1.25rem 0;padding:.9rem 1rem;border:1px solid rgba(127,127,127,.28);border-radius:12px;font:inherit;color:inherit;text-align:left}
#canvasbin-updates *{box-sizing:border-box}
#canvasbin-updates .canvasbin-updates-head{display:flex;flex-wrap:wrap;align-items:center;justify-content:space-between;gap:.6rem}
#canvasbin-updates .canvasbin-updates-label{margin:0;font-size:.8rem;font-weight:700;letter-spacing:.06em;text-transform:uppercase;opacity:.7}
#canvasbin-updates form{display:flex;flex-wrap:wrap;gap:.4rem;margin:0}
#canvasbin-updates input{min-width:0;width:14rem;max-width:100%;padding:.4rem .6rem;border:1px solid rgba(127,127,127,.45);border-radius:8px;font:inherit;font-size:.9rem;color:inherit;background:transparent}
#canvasbin-updates button{padding:.4rem .8rem;border:0;border-radius:8px;font:inherit;font-size:.9rem;font-weight:600;color:#fff;background:#3b5bdb;cursor:pointer}
#canvasbin-updates #canvasbin-subscribe-msg{flex-basis:100%;margin:.2rem 0 0;font-size:.85rem}
#canvasbin-updates .canvasbin-update{margin-top:.75rem}
#canvasbin-updates .canvasbin-update-title{margin:0;font-size:1.05rem;line-height:1.3}
#canvasbin-updates .canvasbin-update-time{display:block;margin:.15rem 0 .3rem;font-size:.8rem;opacity:.65}
#canvasbin-updates .canvasbin-update-body>:first-child{margin-top:0}
#canvasbin-updates .canvasbin-update-body>:last-child{margin-bottom:0}
#canvasbin-updates details{margin-top:.75rem}
#canvasbin-updates summary{cursor:pointer;font-size:.9rem;opacity:.8}
</style>`

// UpdatesBlock renders the subscribe form and feed for injected pages. The
// newest update is shown in full; older ones fold away so the block stays
// short enough to sit above the fold.
func UpdatesBlock(readKey string, updates []store.PageUpdate, productName string) (string, error) {
	var b strings.Builder
	b.WriteString(updatesCSS)
	b.WriteString(`<section id="canvasbin-updates" class="canvasbin-updates" aria-label="Page updates">`)
	b.WriteString(`<div class="canvasbin-updates-head">`)
	b.WriteString(`<p class="canvasbin-updates-label">Updates</p>`)
	b.WriteString(`<form id="canvasbin-subscribe-form">`)
	b.WriteString(`<label for="canvasbin-subscribe-email" hidden>Email address</label>`)
	b.WriteString(`<input type="email" id="canvasbin-subscribe-email" name="email" required autocomplete="email" placeholder="you@example.com">`)
	b.WriteString(`<button type="submit">Get updates by email</button>`)
	b.WriteString(`<p id="canvasbin-subscribe-msg" hidden></p>`)
	b.WriteString(`</form></div>`)
	for i, u := range updates {
		if i == 1 {
			fmt.Fprintf(&b, `<details><summary>Earlier updates (%d)</summary>`, len(updates)-1)
		}
		if err := writeUpdate(&b, u); err != nil {
			return "", err
		}
	}
	if len(updates) > 1 {
		b.WriteString(`</details>`)
	}
	b.WriteString(`</section>`)
	b.WriteString(subscribeScript(readKey))
	return b.String(), nil
}

func writeUpdate(b *strings.Builder, u store.PageUpdate) error {
	body, err := Markdown(u.Body)
	if err != nil {
		return err
	}
	fmt.Fprintf(b, `<article id="update-%d" class="canvasbin-update">`, u.ID)
	fmt.Fprintf(b, `<h3 class="canvasbin-update-title">%s</h3>`, html.EscapeString(u.Title))
	fmt.Fprintf(b, `<time class="canvasbin-update-time" datetime="%s">%s</time>`,
		html.EscapeString(u.CreatedAt.UTC().Format(time.RFC3339)),
		html.EscapeString(u.CreatedAt.UTC().Format("2 Jan 2006")))
	b.WriteString(`<div class="canvasbin-update-body">`)
	b.WriteString(body)
	b.WriteString(`</div></article>`)
	return nil
}

func subscribeScript(readKey string) string {
	key := html.EscapeString(readKey)
	return `<script data-canvasbin-subscribe>(function(){var k="` + key + `";var f=document.getElementById("canvasbin-subscribe-form");if(!f)return;f.addEventListener("submit",function(e){e.preventDefault();var em=document.getElementById("canvasbin-subscribe-email").value;var m=document.getElementById("canvasbin-subscribe-msg");fetch("/api/pages/"+encodeURIComponent(k)+"/subscribe",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({email:em})}).then(function(r){return r.json().then(function(j){return{ok:r.ok,body:j}})}).then(function(x){m.hidden=false;m.textContent=x.ok?(x.body.message||"Check your email to confirm."):((x.body&&x.body.error)||"Could not subscribe.");}).catch(function(){m.hidden=false;m.textContent="Could not subscribe.";});});})();</script>`
}

// WithUpdates places the updates block at the marker when the page has one,
// otherwise straight after the page's hero so it shows above the fold.
func WithUpdates(doc, block string) string {
	if strings.Contains(doc, updatesInjectMarker) {
		return strings.Replace(doc, updatesInjectMarker, block, 1)
	}
	if at := heroEnd(doc); at >= 0 {
		return doc[:at] + "\n" + block + "\n" + doc[at:]
	}
	return beforeBodyEnd(doc, block)
}

// heroEnd returns the offset just past the page's hero, in order of preference:
// an element with a hero-like class, a <header> holding the first <h1>, the
// first <h1> plus the paragraph right after it, the first <header>, or the
// start of <body>. It returns -1 when none of those exist.
func heroEnd(doc string) int {
	z := nethtml.NewTokenizer(strings.NewReader(doc))
	off, bodyAt, header, h1 := 0, -1, -1, -1
	for {
		tt := z.Next()
		if tt == nethtml.ErrorToken {
			break
		}
		start := off
		off += len(z.Raw())
		if tt != nethtml.StartTagToken {
			continue
		}
		name, hasAttr := z.TagName()
		switch tag := string(name); {
		case tag == "body":
			bodyAt = off
		case hasAttr && heroClass(z):
			if end := elementEnd(doc, start); end > 0 {
				return end
			}
		case tag == "header" && header < 0:
			header = start
		case tag == "h1" && h1 < 0:
			h1 = start
		}
	}
	headerEnd := -1
	if header >= 0 {
		headerEnd = elementEnd(doc, header)
	}
	if headerEnd > 0 && h1 > header && h1 < headerEnd {
		return headerEnd
	}
	if h1 >= 0 {
		if end := elementEnd(doc, h1); end > 0 {
			if lede := ledeEnd(doc, end); lede > 0 {
				return lede
			}
			return end
		}
	}
	if headerEnd > 0 {
		return headerEnd
	}
	return bodyAt
}

var heroWords = []string{"hero", "banner", "masthead", "jumbotron"}

func heroClass(z *nethtml.Tokenizer) bool {
	for {
		key, val, more := z.TagAttr()
		if string(key) == "class" {
			for _, c := range strings.Fields(strings.ToLower(string(val))) {
				for _, w := range heroWords {
					if strings.Contains(c, w) {
						return true
					}
				}
			}
		}
		if !more {
			return false
		}
	}
}

// elementEnd returns the offset just past the element whose start tag begins at
// start, or -1 if it isn't closed.
func elementEnd(doc string, start int) int {
	z := nethtml.NewTokenizer(strings.NewReader(doc[start:]))
	tt := z.Next()
	off := start + len(z.Raw())
	if tt == nethtml.SelfClosingTagToken {
		return off
	}
	name, _ := z.TagName()
	tag, depth := string(name), 1
	for {
		tt := z.Next()
		if tt == nethtml.ErrorToken {
			return -1
		}
		off += len(z.Raw())
		if tt != nethtml.StartTagToken && tt != nethtml.EndTagToken {
			continue
		}
		if n, _ := z.TagName(); string(n) != tag {
			continue
		}
		if tt == nethtml.StartTagToken {
			depth++
		} else if depth--; depth == 0 {
			return off
		}
	}
}

// ledeEnd returns the end of a <p> that directly follows offset at (ignoring
// whitespace and comments), or -1.
func ledeEnd(doc string, at int) int {
	z := nethtml.NewTokenizer(strings.NewReader(doc[at:]))
	off := at
	for {
		tt := z.Next()
		if tt == nethtml.ErrorToken {
			return -1
		}
		raw := z.Raw()
		start := off
		off += len(raw)
		switch tt {
		case nethtml.CommentToken:
			continue
		case nethtml.TextToken:
			if strings.TrimSpace(string(raw)) == "" {
				continue
			}
		case nethtml.StartTagToken:
			if n, _ := z.TagName(); string(n) == "p" {
				return elementEnd(doc, start)
			}
		}
		return -1
	}
}
