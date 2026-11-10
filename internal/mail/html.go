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

const secondaryButtonsPerRow = 3

func secondaryButtonRows(btns []emailButton, perRow int) [][]emailButton {
	if len(btns) == 0 || perRow <= 0 {
		return nil
	}
	var rows [][]emailButton
	for i := 0; i < len(btns); i += perRow {
		row := make([]emailButton, perRow)
		for j := 0; j < perRow; j++ {
			if i+j < len(btns) {
				row[j] = btns[i+j]
			}
		}
		rows = append(rows, row)
	}
	return rows
}

var emailTmpl = template.Must(template.New("email").Funcs(template.FuncMap{
	"chunkButtons": secondaryButtonRows,
}).Parse(`<!doctype html>
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
{{range .Prompts}}<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:0 0 10px 0;">
<tr><td align="center">
<table role="presentation" width="92%" cellpadding="0" cellspacing="0" border="0" style="width:92%;"><tr>
<td bgcolor="#e6e9ff" style="background-color:#e6e9ff;border:1px solid #c7cdfe;border-radius:14px;padding:14px 18px;font-family:Arial,Helvetica,sans-serif;font-size:16px;line-height:23px;font-weight:bold;color:#1e1b4b;text-align:left;">{{.}}</td>
</tr></table>
</td></tr></table>
{{end}}{{end}}{{range .After}}<p style="margin:16px 0 0 0;font-size:16px;line-height:24px;color:#374151;">{{.}}</p>
{{end}}{{if .Secondary}}
<!--[if mso]>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:16px 0 0 0;table-layout:fixed;">
{{range $row := chunkButtons .Secondary 3}}<tr>
{{range $row}}<td width="178" valign="top" align="center" style="width:33.33%;padding:6px 4px;">{{if .Label}}{{template "secondaryBtn" .}}{{end}}</td>
{{end}}</tr>
{{end}}
</table>
<![endif]-->
<!--[if !mso]><!-->
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:16px 0 0 0;width:100%;max-width:100%;">
<tr><td align="center" style="padding:0;">
<div style="width:100%;max-width:536px;margin:0 auto;">
{{range $row := chunkButtons .Secondary 3}}<div style="display:block;width:100%;text-align:center;font-size:0;line-height:0;margin:0 0 6px 0;">
{{range $row}}{{if .Label}}<div style="display:inline-block;width:32%;max-width:32%;vertical-align:top;padding:4px 2px;font-size:13px;line-height:17px;box-sizing:border-box;">
{{template "secondaryBtn" .}}
</div>{{end}}{{end}}
</div>{{end}}
</div>
</td></tr></table>
<!--<![endif]-->
{{end}}</td></tr>
<tr><td style="padding:20px 8px 0 8px;font-family:Arial,Helvetica,sans-serif;font-size:13px;line-height:20px;color:#6b7280;">{{.Footer}}{{with .FooterLink}} <a href="{{.URL}}" style="color:#4f46e5;text-decoration:underline;">{{.Label}}</a>{{end}}</td></tr>
</table>
</td></tr>
</table>
</body>
</html>
{{define "secondaryBtn"}}
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr><td align="center" bgcolor="#ffffff" style="border-radius:8px;border:1px solid #c7d2fe;background-color:#ffffff;">
<a href="{{.URL}}" target="_blank" style="display:block;padding:10px 6px;font-family:Arial,Helvetica,sans-serif;font-size:13px;font-weight:bold;line-height:17px;color:#4f46e5;text-decoration:none;text-align:center;word-wrap:break-word;overflow-wrap:break-word;">{{.Label}}</a>
</td></tr></table>
{{end}}
`))

func (e htmlEmail) render() (string, error) {
	var b bytes.Buffer
	if err := emailTmpl.Execute(&b, e); err != nil {
		return "", err
	}
	return b.String(), nil
}
