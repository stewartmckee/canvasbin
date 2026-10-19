package mail

import (
	"bytes"
	"html/template"
)

// htmlEmail is the content of an HTML email. Mail clients ignore <style>
// blocks and modern CSS unevenly, so the layout uses tables and inline styles,
// and links are rendered as table-cell buttons that work in Outlook.
type htmlEmail struct {
	Preheader    string // inbox preview text, hidden in the body
	Heading      string
	Intro        []string
	Button       *emailButton
	PromptsIntro string
	Prompts      []string
	After        []string
	Secondary    []emailButton
	Footer       string
	FooterLink   *emailButton
	Base         string
}

type emailButton struct {
	Label string
	URL   string
}

var emailTmpl = template.Must(template.New("email").Parse(`<!doctype html>
<html lang="en" xmlns="http://www.w3.org/1999/xhtml">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="color-scheme" content="light">
<meta name="supported-color-schemes" content="light">
<title>{{.Heading}}</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f4f7;">
<div style="display:none;max-height:0;overflow:hidden;opacity:0;color:#f4f4f7;font-size:1px;line-height:1px;">{{.Preheader}}</div>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background-color:#f4f4f7;">
<tr><td align="center" style="padding:32px 12px;">
<table role="presentation" width="600" cellpadding="0" cellspacing="0" border="0" style="width:100%;max-width:600px;">
<tr><td style="padding:0 8px 16px 8px;font-family:Arial,Helvetica,sans-serif;font-size:20px;font-weight:bold;color:#4f46e5;">
<a href="{{.Base}}" style="color:#4f46e5;text-decoration:none;">Canvasbin</a>
</td></tr>
<tr><td style="background-color:#ffffff;border:1px solid #e5e7eb;border-radius:12px;padding:32px 32px 28px 32px;font-family:Arial,Helvetica,sans-serif;color:#1f2937;">
<h1 style="margin:0 0 16px 0;font-size:24px;line-height:32px;color:#111827;font-weight:bold;">{{.Heading}}</h1>
{{range .Intro}}<p style="margin:0 0 16px 0;font-size:16px;line-height:24px;color:#374151;">{{.}}</p>
{{end}}{{with .Button}}<table role="presentation" cellpadding="0" cellspacing="0" border="0" style="margin:8px 0 24px 0;">
<tr><td align="center" bgcolor="#4f46e5" style="border-radius:8px;background-color:#4f46e5;">
<a href="{{.URL}}" target="_blank" style="display:inline-block;padding:14px 28px;font-family:Arial,Helvetica,sans-serif;font-size:16px;font-weight:bold;line-height:20px;color:#ffffff;text-decoration:none;border-radius:8px;border:1px solid #4f46e5;">{{.Label}}</a>
</td></tr></table>
{{end}}{{if .Prompts}}<p style="margin:8px 0 12px 0;font-size:16px;line-height:24px;color:#374151;font-weight:bold;">{{.PromptsIntro}}</p>
{{range .Prompts}}<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:0 0 12px 0;">
<tr><td style="background-color:#eef2ff;border-left:4px solid #4f46e5;border-radius:6px;padding:14px 16px;font-family:Arial,Helvetica,sans-serif;font-size:15px;line-height:22px;color:#1e1b4b;">
<span style="font-size:12px;line-height:16px;letter-spacing:1px;text-transform:uppercase;color:#4f46e5;font-weight:bold;">Try asking</span><br>
&ldquo;{{.}}&rdquo;
</td></tr></table>
{{end}}{{end}}{{range .After}}<p style="margin:16px 0 0 0;font-size:16px;line-height:24px;color:#374151;">{{.}}</p>
{{end}}{{if .Secondary}}<table role="presentation" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0 0 0;">
<tr>{{range .Secondary}}<td style="padding:0 8px 8px 0;">
<table role="presentation" cellpadding="0" cellspacing="0" border="0"><tr><td align="center" bgcolor="#ffffff" style="border-radius:8px;border:1px solid #c7d2fe;">
<a href="{{.URL}}" target="_blank" style="display:inline-block;padding:10px 18px;font-family:Arial,Helvetica,sans-serif;font-size:14px;font-weight:bold;line-height:18px;color:#4f46e5;text-decoration:none;">{{.Label}}</a>
</td></tr></table>
</td>{{end}}</tr></table>
{{end}}</td></tr>
<tr><td style="padding:20px 8px 0 8px;font-family:Arial,Helvetica,sans-serif;font-size:13px;line-height:20px;color:#6b7280;">{{.Footer}}{{with .FooterLink}} <a href="{{.URL}}" style="color:#4f46e5;text-decoration:underline;">{{.Label}}</a>{{end}}</td></tr>
</table>
</td></tr>
</table>
</body>
</html>
`))

func (e htmlEmail) render() (string, error) {
	var b bytes.Buffer
	if err := emailTmpl.Execute(&b, e); err != nil {
		return "", err
	}
	return b.String(), nil
}
