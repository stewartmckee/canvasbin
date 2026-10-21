package render

import (
	"bytes"
	"html/template"
)

// Notice is a small standalone page shown after following a link from an
// email, such as confirming or cancelling an updates subscription.
type Notice struct {
	Kind      string // "success", "neutral" or "error"; picks the icon and colour
	Title     string // browser tab title
	Heading   string
	Message   string
	PageTitle string // shown in bold inside the message when set
	Email     string
	LinkURL   string
	LinkText  string
	Product   string
}

var noticeTmpl = template.Must(template.New("notice").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>{{.Title}}</title>
<style>
:root{--bg:#f4f5f8;--card:#fff;--ink:#16181d;--muted:#61666f;--line:rgba(16,24,40,.09);--btn:#16181d;--btn-ink:#fff;--accent:#16a34a;--accent-soft:rgba(22,163,74,.12)}
@media (prefers-color-scheme:dark){:root{--bg:#111214;--card:#1b1c1f;--ink:#f3f4f6;--muted:#a0a4ab;--line:rgba(255,255,255,.09);--btn:#f3f4f6;--btn-ink:#16181d}}
.neutral{--accent:#61666f;--accent-soft:rgba(97,102,111,.14)}
.error{--accent:#dc2626;--accent-soft:rgba(220,38,38,.12)}
*{box-sizing:border-box}
html,body{height:100%}
body{margin:0;display:grid;place-items:center;padding:24px;background:radial-gradient(1200px 600px at 50% -10%,var(--accent-soft),transparent 70%),var(--bg);color:var(--ink);font:16px/1.55 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;-webkit-font-smoothing:antialiased}
main{width:100%;max-width:440px;padding:40px 32px 32px;background:var(--card);border:1px solid var(--line);border-radius:20px;box-shadow:0 1px 2px rgba(16,24,40,.04),0 24px 60px -24px rgba(16,24,40,.28);text-align:center;animation:rise .35s ease-out}
@keyframes rise{from{transform:translateY(8px)}to{transform:none}}
.icon{display:grid;place-items:center;width:64px;height:64px;margin:0 auto 20px;border-radius:50%;background:var(--accent-soft);color:var(--accent)}
.icon svg{width:30px;height:30px}
h1{margin:0 0 8px;font-size:24px;line-height:1.25;letter-spacing:-.015em}
p{margin:0;color:var(--muted)}
p strong{color:var(--ink);font-weight:600}
.email{display:inline-block;margin-top:14px;padding:4px 12px;border-radius:999px;background:var(--bg);color:var(--ink);font-size:14px;word-break:break-all}
.btn{display:inline-flex;align-items:center;justify-content:center;gap:8px;width:100%;height:46px;margin-top:28px;border-radius:12px;background:var(--btn);color:var(--btn-ink);font-weight:600;font-size:15px;text-decoration:none;transition:opacity .15s}
.btn:hover{opacity:.9}
.btn svg{width:16px;height:16px}
footer{margin-top:24px;font-size:13px;color:var(--muted)}
footer a{color:inherit}
@media (max-width:480px){main{padding:32px 22px 24px}h1{font-size:22px}}
</style>
</head>
<body class="{{.Kind}}">
<main>
  <div class="icon" aria-hidden="true">
    {{- if eq .Kind "success"}}<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
    {{- else if eq .Kind "error"}}<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 8v5"/><path d="M12 16.5h.01"/><circle cx="12" cy="12" r="9.5"/></svg>
    {{- else}}<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"/><path d="M10.3 21a1.94 1.94 0 0 0 3.4 0"/><path d="M3 3l18 18"/></svg>
    {{- end}}
  </div>
  <h1>{{.Heading}}</h1>
  <p>{{.Message}}{{if .PageTitle}} <strong>{{.PageTitle}}</strong>.{{end}}</p>
  {{- if .Email}}<span class="email">{{.Email}}</span>{{end}}
  {{- if .LinkURL}}
  <a class="btn" href="{{.LinkURL}}">{{.LinkText}}<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 12h14"/><path d="m13 6 6 6-6 6"/></svg></a>
  {{- end}}
  <footer>{{.Product}}</footer>
</main>
</body>
</html>
`))

// NoticePage renders n as a complete HTML document.
func NoticePage(n Notice) string {
	var b bytes.Buffer
	if err := noticeTmpl.Execute(&b, n); err != nil {
		return "<!doctype html><title>" + template.HTMLEscapeString(n.Title) + "</title><p>" + template.HTMLEscapeString(n.Message) + "</p>"
	}
	return b.String()
}
