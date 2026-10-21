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

// updatesCSS lives inside the block's shadow root, so page styles can't reach
// it and it can't restyle the page.
const updatesCSS = `:host{display:block;box-sizing:border-box;max-width:800px;margin:32px auto;padding:0 16px;font-family:inherit;color-scheme:light;--cb-bg:#fff;--cb-ink:#16181d;--cb-muted:#61666f;--cb-line:rgba(16,24,40,.1);--cb-soft:#f6f7f9;--cb-btn:#16181d;--cb-btn-ink:#fff;--cb-live:#16a34a}
:host([data-theme=dark]){color-scheme:dark;--cb-bg:#1b1c1f;--cb-ink:#f3f4f6;--cb-muted:#a0a4ab;--cb-line:rgba(255,255,255,.1);--cb-soft:#232428;--cb-btn:#f3f4f6;--cb-btn-ink:#16181d}
:host([data-fit]){max-width:none;margin:28px 0;padding:0}
*{box-sizing:border-box}
.card{background:var(--cb-bg);color:var(--cb-ink);border:1px solid var(--cb-line);border-radius:16px;box-shadow:0 1px 2px rgba(16,24,40,.04),0 16px 40px -16px rgba(16,24,40,.22);overflow:hidden;font-size:15px;line-height:1.55;text-align:left}
.latest{padding:22px 24px 18px}
.eyebrow{display:flex;align-items:center;gap:8px;margin:0;font-size:12px;font-weight:600;letter-spacing:.08em;text-transform:uppercase;color:var(--cb-muted)}
.dot{flex:none;width:8px;height:8px;border-radius:50%;background:var(--cb-live);box-shadow:0 0 0 4px rgba(22,163,74,.18)}
.eyebrow time{font-weight:500;letter-spacing:.02em;text-transform:none}
h3{margin:8px 0 6px;font-size:21px;line-height:1.3;font-weight:700;letter-spacing:-.01em}
h4{margin:2px 0 4px;font-size:16px;line-height:1.35;font-weight:700}
.body>:first-child{margin-top:0}
.body>:last-child{margin-bottom:0}
.body p{margin:.5em 0}
.body a{color:inherit}
.body img{max-width:100%;height:auto}
.earlier{border-top:1px solid var(--cb-line)}
.earlier summary{display:flex;align-items:center;gap:8px;padding:12px 24px;list-style:none;cursor:pointer;font-size:14px;font-weight:600;color:var(--cb-muted)}
.earlier summary::-webkit-details-marker{display:none}
.earlier summary::after{content:"";width:7px;height:7px;margin-left:auto;border-right:2px solid currentColor;border-bottom:2px solid currentColor;transform:translateY(-2px) rotate(45deg);transition:transform .15s}
.earlier[open] summary::after{transform:translateY(2px) rotate(-135deg)}
.count{display:inline-grid;place-items:center;min-width:20px;height:20px;padding:0 6px;border-radius:999px;background:var(--cb-soft);color:var(--cb-ink);font-size:12px}
.earlier ol{margin:0;padding:0 24px 8px;list-style:none}
.earlier li{padding:14px 0;border-top:1px solid var(--cb-line)}
.earlier li:first-child{padding-top:4px;border-top:0}
.earlier time{display:block;font-size:12px;color:var(--cb-muted)}
.subscribe{display:flex;flex-wrap:wrap;align-items:center;gap:12px 20px;margin:0;padding:16px 24px;background:var(--cb-soft);border-top:1px solid var(--cb-line)}
.card>.subscribe:first-child{border-top:0}
.copy{display:flex;flex:1 1 220px;flex-direction:column;gap:2px}
.copy strong{font-size:15px}
.copy span{font-size:13px;color:var(--cb-muted)}
.fields{display:flex;flex:1 1 300px;gap:8px;max-width:420px}
input{flex:1;min-width:0;height:40px;padding:0 12px;border:1px solid var(--cb-line);border-radius:10px;background:var(--cb-bg);color:var(--cb-ink);font:inherit;font-size:14px}
input:focus{outline:2px solid var(--cb-ink);outline-offset:1px}
button{height:40px;padding:0 16px;border:0;border-radius:10px;background:var(--cb-btn);color:var(--cb-btn-ink);font:inherit;font-size:14px;font-weight:600;white-space:nowrap;cursor:pointer}
button:hover{opacity:.9}
button[disabled]{opacity:.6;cursor:default}
.msg{flex-basis:100%;margin:0;font-size:13px;color:var(--cb-muted)}
@media (max-width:520px){.latest{padding:18px}.subscribe{padding:16px 18px}.earlier summary{padding:12px 18px}.earlier ol{padding:0 18px 8px}.fields{max-width:none}}`

// UpdatesBlock renders the updates card: the newest update in full, older ones
// folded away, and the email sign-up. It uses declarative shadow DOM (with a
// script fallback) so it looks the same on any page.
func UpdatesBlock(readKey string, updates []store.PageUpdate, productName string) (string, error) {
	var b strings.Builder
	b.WriteString(`<canvasbin-updates id="canvasbin-updates" aria-label="Page updates"><template shadowrootmode="open">`)
	b.WriteString(`<style>` + updatesCSS + `</style><div class="card">`)
	for i, u := range updates {
		body, err := Markdown(u.Body)
		if err != nil {
			return "", err
		}
		title := html.EscapeString(u.Title)
		when := fmt.Sprintf(`<time datetime="%s">%s</time>`,
			html.EscapeString(u.CreatedAt.UTC().Format(time.RFC3339)),
			html.EscapeString(u.CreatedAt.UTC().Format("2 Jan 2006")))
		switch i {
		case 0:
			fmt.Fprintf(&b, `<article class="latest" id="update-%d"><p class="eyebrow"><span class="dot"></span>Latest update · %s</p><h3>%s</h3><div class="body">%s</div></article>`,
				u.ID, when, title, body)
			if len(updates) > 1 {
				fmt.Fprintf(&b, `<details class="earlier"><summary>Earlier updates <span class="count">%d</span></summary><ol>`, len(updates)-1)
			}
		default:
			fmt.Fprintf(&b, `<li id="update-%d">%s<h4>%s</h4><div class="body">%s</div></li>`, u.ID, when, title, body)
		}
	}
	if len(updates) > 1 {
		b.WriteString(`</ol></details>`)
	}
	b.WriteString(`<form class="subscribe"><div class="copy"><strong>Get updates by email</strong>`)
	b.WriteString(`<span>We’ll email you when this page posts an update. Unsubscribe any time.</span></div>`)
	b.WriteString(`<div class="fields"><input type="email" name="email" required autocomplete="email" placeholder="you@example.com" aria-label="Email address">`)
	b.WriteString(`<button type="submit">Subscribe</button></div><p class="msg" role="status" hidden></p></form>`)
	b.WriteString(`</div></template></canvasbin-updates>`)
	b.WriteString(updatesScript(readKey))
	return b.String(), nil
}

// updatesScript attaches the shadow root where declarative shadow DOM isn't
// supported, matches the card to a dark page, fits it to narrow columns, opens
// #update-N links, and handles the sign-up form.
func updatesScript(readKey string) string {
	key := html.EscapeString(readKey)
	return `<script data-canvasbin-updates>(function(){var h=document.getElementById("canvasbin-updates");if(!h)return;var r=h.shadowRoot;if(!r){var t=h.querySelector("template");if(!t)return;r=h.attachShadow({mode:"open"});r.appendChild(t.content);t.remove();}` +
		`function lum(c){var m=(c||"").match(/[\d.]+/g);if(!m||m.length<3||(m.length>3&&+m[3]===0))return null;return(0.2126*m[0]+0.7152*m[1]+0.0722*m[2])/255}` +
		`var el=h.parentElement,l=null;while(el&&l===null){l=lum(getComputedStyle(el).backgroundColor);el=el.parentElement}if(l!==null&&l<0.4)h.setAttribute("data-theme","dark");` +
		`if(h.parentElement!==document.body||getComputedStyle(document.body).maxWidth!=="none")h.setAttribute("data-fit","");` +
		`function go(){var m=location.hash.match(/^#update-(\d+)$/);if(!m)return;var e=r.getElementById("update-"+m[1]);if(!e)return;var d=e.closest("details");if(d)d.open=true;e.scrollIntoView({block:"start"})}go();window.addEventListener("hashchange",go);` +
		`var f=r.querySelector("form"),i=r.querySelector("input"),b=r.querySelector("button"),g=r.querySelector(".msg");f.addEventListener("submit",function(e){e.preventDefault();b.disabled=true;` +
		`fetch("/api/pages/"+encodeURIComponent("` + key + `")+"/subscribe",{method:"POST",credentials:"omit",headers:{"Content-Type":"application/json"},body:JSON.stringify({email:i.value})}).then(function(x){return x.json().then(function(j){return{ok:x.ok,body:j}})})` +
		`.then(function(x){g.hidden=false;g.textContent=x.ok?(x.body.message||"Check your email to confirm."):((x.body&&x.body.error)||"Could not subscribe.");if(x.ok)i.value=""})` +
		`.catch(function(){g.hidden=false;g.textContent="Could not subscribe."}).then(function(){b.disabled=false})})})();</script>`
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
