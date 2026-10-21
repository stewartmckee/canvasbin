package render

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/stewartmckee/canvasbin/internal/store"
)

const updatesInjectMarker = "<!-- canvasbin-updates -->"

// UpdatesBlock renders the subscribe form and feed for injected pages.
func UpdatesBlock(readKey string, updates []store.PageUpdate, productName string) (string, error) {
	var b strings.Builder
	b.WriteString(`<section id="canvasbin-updates" class="canvasbin-updates" aria-label="Page updates">`)
	b.WriteString(`<div id="canvasbin-subscribe" class="canvasbin-subscribe">`)
	b.WriteString(`<form id="canvasbin-subscribe-form">`)
	b.WriteString(`<label for="canvasbin-subscribe-email">Email updates</label>`)
	b.WriteString(`<input type="email" id="canvasbin-subscribe-email" name="email" required autocomplete="email" placeholder="you@example.com">`)
	b.WriteString(`<button type="submit">Subscribe</button>`)
	b.WriteString(`<p id="canvasbin-subscribe-msg" hidden></p>`)
	b.WriteString(`</form></div>`)
	b.WriteString(`<div class="canvasbin-update-list">`)
	for _, u := range updates {
		body, err := Markdown(u.Body)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, `<article id="update-%d" class="canvasbin-update">`, u.ID)
		fmt.Fprintf(&b, `<h3 class="canvasbin-update-title">%s</h3>`, html.EscapeString(u.Title))
		fmt.Fprintf(&b, `<time class="canvasbin-update-time" datetime="%s">%s</time>`,
			html.EscapeString(u.CreatedAt.UTC().Format(time.RFC3339)),
			html.EscapeString(u.CreatedAt.Format("d MMM yyyy")))
		b.WriteString(`<div class="canvasbin-update-body">`)
		b.WriteString(body)
		b.WriteString(`</div></article>`)
	}
	b.WriteString(`</div></section>`)
	b.WriteString(subscribeScript(readKey))
	return b.String(), nil
}

func subscribeScript(readKey string) string {
	key := html.EscapeString(readKey)
	return `<script data-canvasbin-subscribe>(function(){var k="` + key + `";var f=document.getElementById("canvasbin-subscribe-form");if(!f)return;f.addEventListener("submit",function(e){e.preventDefault();var em=document.getElementById("canvasbin-subscribe-email").value;var m=document.getElementById("canvasbin-subscribe-msg");fetch("/api/pages/"+encodeURIComponent(k)+"/subscribe",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({email:em})}).then(function(r){return r.json().then(function(j){return{ok:r.ok,body:j}})}).then(function(x){m.hidden=false;m.textContent=x.ok?(x.body.message||"Check your email to confirm."):((x.body&&x.body.error)||"Could not subscribe.");}).catch(function(){m.hidden=false;m.textContent="Could not subscribe.";});});})();</script>`
}

// WithUpdates injects the updates block before </body> or after the marker.
func WithUpdates(doc, block string) string {
	if strings.Contains(doc, updatesInjectMarker) {
		return strings.Replace(doc, updatesInjectMarker, block, 1)
	}
	return beforeBodyEnd(doc, block)
}
