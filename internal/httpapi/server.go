// Package httpapi serves the JSON API, rendered pages, and the embedded web UI.
package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/stewartmckee/canvasbin/internal/analytics"
	"github.com/stewartmckee/canvasbin/internal/billing"
	"github.com/stewartmckee/canvasbin/internal/config"
	"github.com/stewartmckee/canvasbin/internal/keys"
	"github.com/stewartmckee/canvasbin/internal/links"
	"github.com/stewartmckee/canvasbin/internal/live"
	"github.com/stewartmckee/canvasbin/internal/mail"
	"github.com/stewartmckee/canvasbin/internal/oauth"
	"github.com/stewartmckee/canvasbin/internal/pdf"
	"github.com/stewartmckee/canvasbin/internal/ratelimit"
	"github.com/stewartmckee/canvasbin/internal/render"
	"github.com/stewartmckee/canvasbin/internal/store"
)

// viewCSP isolates user HTML in an opaque origin so its scripts cannot reach the
// app's storage or call the API as the app.
const viewCSP = "sandbox allow-scripts allow-forms allow-popups allow-popups-to-escape-sandbox allow-modals allow-downloads"

const maxBodyBytes = 3 * store.MaxContentBytes

type Server struct {
	Store         *store.Store
	PublicBaseURL string
	CreateLimit   *ratelimit.Limiter
	Web           fs.FS
	MCP           http.Handler
	PDF           *pdf.Renderer
	PDFLimit      *ratelimit.Limiter
	// SelfURL is how the PDF renderer reaches this server, e.g. http://127.0.0.1:8080.
	SelfURL string
	// Version is the build version; deploys stamp it as YYYYMMDD-HHMMSS.
	Version string
	// Live, when set, pushes page changes to open browsers (see RunLive).
	Live *live.Hub

	// Accounts (all optional; only used when Config.Accounts is on).
	Config    config.Config
	Mailer    mail.Sender
	Billing   *billing.Service
	AuthLimit *ratelimit.Limiter // per client IP, for login/register/reset
	MailLimit *ratelimit.Limiter // per recipient, to stop mail bombing
	// OAuth is the MCP authorization server. Handler creates one when accounts
	// are on and this is nil; main sets it to share it with the MCP gate.
	OAuth *oauth.Provider
	// Analytics, when set, records events the browser can't see.
	Analytics *analytics.Client
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("POST /api/pages", s.create)
	mux.HandleFunc("GET /api/pages/{key}", s.get)
	mux.HandleFunc("PUT /api/pages/{key}", s.update)
	mux.HandleFunc("DELETE /api/pages/{key}", s.remove)
	mux.HandleFunc("GET /api/pages/{key}/live", s.liveUpdates)
	mux.HandleFunc("POST /api/render", s.preview)
	mux.HandleFunc("GET /api/config", s.publicConfig)
	if s.accounts() {
		s.accountRoutes(mux)
		if s.OAuth == nil {
			s.OAuth = s.NewOAuth()
		}
		s.OAuth.Routes(mux)
	}
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})
	if s.accounts() {
		mux.HandleFunc("GET /robots.txt", s.robots)
		mux.HandleFunc("GET /sitemap.xml", s.sitemap)
	}
	if token := s.Config.OpenAIAppsChallenge; token != "" {
		mux.HandleFunc("GET /.well-known/openai-apps-challenge", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte(token))
		})
	}
	mux.HandleFunc("GET /raw/{key}", s.raw)
	mux.HandleFunc("GET /v/{key}", redirectTo(".html"))
	mux.HandleFunc("GET /e/{key}", redirectTo(""))
	if s.MCP != nil {
		for _, m := range []string{"GET", "POST", "DELETE"} {
			mux.Handle(m+" /mcp", s.MCP)
		}
	}
	spa := s.spa()
	mux.HandleFunc("GET /{key}", func(w http.ResponseWriter, r *http.Request) {
		seg, ext := keys.Split(r.PathValue("key"))
		key, err := keys.Parse(seg)
		if err != nil {
			spa.ServeHTTP(w, r)
			return
		}
		switch ext {
		case ".html":
			s.view(w, r, key)
		case ".md":
			s.markdown(w, r, key)
		case ".pdf":
			s.pdfDownload(w, r, key)
		default:
			s.serveIndex(w, r)
		}
	})
	mux.Handle("/", spa)
	return logRequests(s.commonHeaders(mux))
}

// marketingPaths are the public site pages search engines may index when
// accounts are on. Everything else (pages, app screens, API) stays noindex.
var marketingPaths = []string{
	"/", "/features", "/pricing", "/privacy", "/terms", "/contact",
	"/use-cases", "/use-cases/developers", "/use-cases/business", "/use-cases/family",
	"/use-cases/news-apps-games",
	"/connect", "/connect/claude", "/connect/chatgpt", "/connect/cursor",
}

func (s *Server) indexable(r *http.Request) bool {
	if !s.accounts() || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
		return false
	}
	for _, p := range marketingPaths {
		if r.URL.Path == p {
			return true
		}
	}
	return false
}

func (s *Server) robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "User-agent: *\nDisallow: /api/\nDisallow: /oauth/\nDisallow: /mcp\nDisallow: /raw/\n\nSitemap: %s/sitemap.xml\n",
		links.Base(s.PublicBaseURL, r))
}

func (s *Server) sitemap(w http.ResponseWriter, r *http.Request) {
	base := links.Base(s.PublicBaseURL, r)
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	lastmod := ""
	if d := releaseDate(s.Version); d != "" {
		lastmod = "<lastmod>" + d + "</lastmod>"
	}
	for _, p := range marketingPaths {
		fmt.Fprintf(&b, "  <url><loc>%s%s</loc>%s</url>\n", base, p, lastmod)
	}
	b.WriteString("</urlset>\n")
	_, _ = w.Write([]byte(b.String()))
}

var versionDate = regexp.MustCompile(`^(\d{4})(\d{2})(\d{2})-\d{6}$`)

// releaseDate turns a deploy version into a sitemap date (YYYY-MM-DD), or ""
// for local builds.
func releaseDate(version string) string {
	m := versionDate.FindStringSubmatch(version)
	if m == nil {
		return ""
	}
	return m[1] + "-" + m[2] + "-" + m[3]
}

// redirectTo keeps the original /v/{key} and /e/{key} links working.
func redirectTo(suffix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key, err := keys.Parse(r.PathValue("key"))
		if err != nil {
			http.Error(w, "page not found", http.StatusNotFound)
			return
		}
		http.Redirect(w, r, "/"+key+suffix, http.StatusMovedPermanently)
	}
}

type pageResponse struct {
	store.Page
	CanWrite bool   `json:"can_write"`
	Role     string `json:"role,omitempty"`
	Owned    bool   `json:"owned"`
	ReadKey  string `json:"read_key"`
	WriteKey string `json:"write_key,omitempty"`
	// Team details are only shown to members of the page's team.
	TeamID   *int64 `json:"team_id,omitempty"`
	TeamName string `json:"team_name,omitempty"`
	TeamRole string `json:"team_role,omitempty"`
	links.Links
}

func (s *Server) response(r *http.Request, a *store.Access) pageResponse {
	out := pageResponse{
		Page:     a.Page,
		CanWrite: a.CanWrite,
		Role:     a.Role,
		Owned:    a.OwnerID != nil,
		ReadKey:  a.ReadKey,
		WriteKey: a.WriteKey,
		Links:    links.For(links.Base(s.PublicBaseURL, r), a.ReadKey, a.WriteKey),
	}
	if a.TeamRole != "" {
		out.TeamID, out.TeamName, out.TeamRole = a.TeamID, a.TeamName, a.TeamRole
	}
	return out
}

type pageInput struct {
	Title   *string `json:"title"`
	Format  *string `json:"format"`
	Content *string `json:"content"`
	TeamID  *int64  `json:"team_id"`
}

// publicConfig tells the web UI which optional features are available.
func (s *Server) publicConfig(w http.ResponseWriter, r *http.Request) {
	cfg := map[string]any{
		"accounts":        s.accounts(),
		"billing_enabled": s.accounts() && s.Billing != nil && s.Billing.Enabled(),
	}
	if s.accounts() {
		cfg["free_quota"], cfg["pro_quota"] = s.Config.PlanQuotas()
		cfg["team_quota"] = s.Config.TeamPageQuota()
		cfg["team_billing_enabled"] = s.Billing != nil && s.Billing.TeamEnabled()
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	if s.CreateLimit != nil && !s.CreateLimit.Allow(links.ClientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "too many pages created, try again shortly")
		return
	}
	var in pageInput
	if !decode(w, r, &in) {
		return
	}
	var (
		a   *store.Access
		err error
	)
	u := s.SessionUser(r)
	if u != nil && in.TeamID != nil {
		if err = s.checkTeamCreate(r, u, *in.TeamID); err != nil {
			writeStoreError(w, err)
			return
		}
		a, err = s.Store.CreateInTeam(r.Context(), u.ID, *in.TeamID, deref(in.Title), deref(in.Format), deref(in.Content))
		if err == nil {
			a, err = s.Store.GetForUser(r.Context(), a.WriteKey, u.ID)
			s.logPage(r, u, a, store.ActivityPageCreated)
		}
	} else if u != nil {
		if err = s.checkQuota(r, u); err != nil {
			writeStoreError(w, err)
			return
		}
		a, err = s.Store.CreateOwned(r.Context(), u.ID, deref(in.Title), deref(in.Format), deref(in.Content))
	} else if s.accounts() {
		writeError(w, http.StatusUnauthorized, "sign in to create pages")
		return
	} else {
		a, err = s.Store.Create(r.Context(), deref(in.Title), deref(in.Format), deref(in.Content))
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if u != nil && s.Analytics.Enabled() {
		n, _ := s.Store.CountCreated(r.Context(), u.ID)
		s.Analytics.PageCreated("web", a.Format, a.TeamID != nil, n)
	}
	writeJSON(w, http.StatusCreated, s.response(r, a))
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	key, ok := pathKey(w, r)
	if !ok {
		return
	}
	var (
		a   *store.Access
		err error
	)
	if u := s.SessionUser(r); u != nil {
		a, err = s.Store.GetForUser(r.Context(), key, u.ID)
	} else {
		a, err = s.Store.Get(r.Context(), key)
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.response(r, a))
}

func (s *Server) update(w http.ResponseWriter, r *http.Request) {
	key, ok := pathKey(w, r)
	if !ok {
		return
	}
	var in pageInput
	if !decode(w, r, &in) {
		return
	}
	changes := store.Changes{Title: in.Title, Format: in.Format, Content: in.Content}
	var (
		a   *store.Access
		err error
	)
	if u := s.SessionUser(r); u != nil {
		a, err = s.Store.UpdateForUser(r.Context(), key, u.ID, changes)
		if err == nil {
			s.logPage(r, u, a, store.ActivityPageEdited)
		}
	} else {
		a, err = s.Store.Update(r.Context(), key, changes)
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.response(r, a))
}

func (s *Server) remove(w http.ResponseWriter, r *http.Request) {
	key, ok := pathKey(w, r)
	if !ok {
		return
	}
	var err error
	if u := s.SessionUser(r); u != nil {
		var a *store.Access
		a, err = s.Store.DeleteForUser(r.Context(), key, u.ID)
		if err == nil {
			s.logPage(r, u, a, store.ActivityPageDeleted)
		}
	} else {
		err = s.Store.Delete(r.Context(), key)
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// preview renders unsaved content exactly as /{key}.html would, for the editor's Preview tab.
func (s *Server) preview(w http.ResponseWriter, r *http.Request) {
	var in pageInput
	if !decode(w, r, &in) {
		return
	}
	format, err := store.NormalizeFormat(deref(in.Format))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	doc, err := render.Document(store.Page{Title: deref(in.Title), Format: format, Content: deref(in.Content)})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not render page")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"html": doc})
}

func (s *Server) view(w http.ResponseWriter, r *http.Request, key string) {
	a, ok := s.lookupForView(w, r, key)
	if !ok {
		return
	}
	doc, err := render.Document(a.Page)
	if err != nil {
		http.Error(w, "could not render page", http.StatusInternalServerError)
		return
	}
	if s.accounts() {
		if show, err := s.Store.ShowsBadge(r.Context(), a.ID); err != nil {
			slog.Error("badge lookup", "err", err)
		} else if show {
			doc = render.WithBadge(doc, links.Base(s.PublicBaseURL, r)+"/?utm_source=badge&utm_medium=referral&utm_campaign=made_with_canvasbin")
		}
	}
	if s.Live != nil && r.URL.Query().Get("live") != "0" {
		doc = render.WithLiveReload(doc, a.ReadKey, liveTime(a.UpdatedAt))
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", viewCSP)
	h.Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(doc))
}

func (s *Server) pdfDownload(w http.ResponseWriter, r *http.Request, key string) {
	if s.PDF == nil {
		http.Error(w, "PDF export is not available", http.StatusNotFound)
		return
	}
	a, ok := s.lookupForView(w, r, key)
	if !ok {
		return
	}
	if s.PDFLimit != nil && !s.PDFLimit.Allow(links.ClientIP(r)) {
		http.Error(w, "too many PDF requests, try again shortly", http.StatusTooManyRequests)
		return
	}
	out, err := s.PDF.Render(r.Context(), s.SelfURL+"/"+a.ReadKey+".html?live=0")
	if errors.Is(err, pdf.ErrBusy) {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	if err != nil {
		slog.Error("pdf", "err", err)
		http.Error(w, "could not render PDF", http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/pdf")
	h.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.pdf"`, filename(a.Title)))
	h.Set("Cache-Control", "no-cache")
	_, _ = w.Write(out)
}

var unsafeFilename = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func filename(title string) string {
	name := strings.Trim(unsafeFilename.ReplaceAllString(strings.TrimSpace(title), "-"), "-.")
	if name == "" {
		return "canvasbin"
	}
	if len(name) > 80 {
		name = name[:80]
	}
	return name
}

func (s *Server) markdown(w http.ResponseWriter, r *http.Request, key string) {
	a, ok := s.lookupForView(w, r, key)
	if !ok {
		return
	}
	if a.Format != store.FormatMarkdown {
		http.Error(w, "this page is HTML; use .html or /raw/ for its source", http.StatusNotFound)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/markdown; charset=utf-8")
	h.Set("Content-Security-Policy", viewCSP)
	h.Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(a.Content))
}

func (s *Server) raw(w http.ResponseWriter, r *http.Request) {
	key, err := keys.Parse(r.PathValue("key"))
	if err != nil {
		http.Error(w, "page not found", http.StatusNotFound)
		return
	}
	a, ok := s.lookupForView(w, r, key)
	if !ok {
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/plain; charset=utf-8")
	h.Set("Content-Security-Policy", viewCSP)
	h.Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(a.Content))
}

func (s *Server) lookupForView(w http.ResponseWriter, r *http.Request, key string) (*store.Access, bool) {
	a, err := s.Store.Get(r.Context(), key)
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "page not found", http.StatusNotFound)
		return nil, false
	}
	if err != nil {
		slog.Error("view lookup", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, false
	}
	return a, true
}

func (s *Server) spa() http.Handler {
	files := http.FileServerFS(s.Web)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if to := canonicalPath(r.URL.Path); to != "" {
			if r.URL.RawQuery != "" {
				to += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, to, http.StatusMovedPermanently)
			return
		}
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if strings.HasPrefix(p, "prerender/") {
			s.writeIndex(w, r, http.StatusNotFound)
			return
		}
		if p != "" && p != "index.html" {
			if st, err := fs.Stat(s.Web, p); err == nil && !st.IsDir() {
				if strings.HasPrefix(p, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
			if strings.HasPrefix(p, "assets/") {
				http.NotFound(w, r)
				return
			}
		}
		if !isAppPath(r.URL.Path) {
			s.writeIndex(w, r, http.StatusNotFound)
			return
		}
		s.serveIndex(w, r)
	})
}

// appPaths are the web app's routes besides /{key}; keep in step with
// web/src/router.js. Anything else gets the app's not-found screen with a 404.
var appPaths = map[string]bool{
	"/": true, "/index.html": true, "/new": true, "/login": true, "/register": true,
	"/forgot-password": true, "/reset-password": true, "/account": true, "/admin": true,
	"/teams/new": true,
}

var teamPath = regexp.MustCompile(`^/teams/[0-9]+$`)

// canonicalPath returns where a duplicate URL for an app route should
// redirect (trailing slash, /index.html), or "" if p is already canonical.
func canonicalPath(p string) string {
	if p == "/index.html" {
		return "/"
	}
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		if t := strings.TrimRight(p, "/"); t != "" && isAppPath(t) {
			return t
		}
	}
	return ""
}

func isAppPath(p string) bool {
	if p != "/" {
		p = strings.TrimSuffix(p, "/")
	}
	if appPaths[p] || teamPath.MatchString(p) {
		return true
	}
	for _, m := range marketingPaths {
		if p == m {
			return true
		}
	}
	return false
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	s.writeIndex(w, r, http.StatusOK)
}

func (s *Server) writeIndex(w http.ResponseWriter, r *http.Request, status int) {
	if status == http.StatusOK && s.indexable(r) && s.SessionUser(r) == nil {
		if page, err := fs.ReadFile(s.Web, prerenderFile(r.URL.Path)); err == nil {
			writeHTML(w, status, page)
			return
		}
	}
	index, err := fs.ReadFile(s.Web, "index.html")
	if err != nil {
		http.Error(w, "web UI not built", http.StatusNotFound)
		return
	}
	if status == http.StatusOK && s.indexable(r) {
		index = bytes.Replace(index, []byte(`content="noindex, nofollow"`), []byte(`content="index, follow"`), 1)
	}
	writeHTML(w, status, index)
}

// prerenderFile is where the web build writes a marketing page's static HTML.
func prerenderFile(p string) string {
	if p == "/" {
		return "prerender/index.html"
	}
	return "prerender" + p + ".html"
}

func writeHTML(w http.ResponseWriter, status int, b []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

func pathKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	key, err := keys.Parse(r.PathValue("key"))
	if err != nil {
		writeError(w, http.StatusNotFound, store.ErrNotFound.Error())
		return "", false
	}
	return key, true
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, store.ErrTooLarge.Error())
			return false
		}
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, store.ErrReadOnly):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, store.ErrTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, err.Error())
	case errors.Is(err, store.ErrBadFormat):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, store.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, store.ErrQuotaExceeded):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error(), "code": "quota_exceeded"})
	case errors.Is(err, store.ErrUnverified):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error(), "code": "unverified"})
	case errors.Is(err, store.ErrDisabled):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error(), "code": "disabled"})
	case errors.Is(err, store.ErrConflict), errors.Is(err, store.ErrEmailTaken):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, store.ErrInvalidToken), errors.Is(err, store.ErrBadTeamName),
		errors.Is(err, store.ErrBadTeamRole), errors.Is(err, store.ErrBadComment):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, store.ErrTeamInactive):
		writeJSON(w, http.StatusPaymentRequired, map[string]string{"error": err.Error(), "code": "team_inactive"})
	case errors.Is(err, store.ErrTeamOwner):
		writeError(w, http.StatusConflict, err.Error())
	default:
		slog.Error("store", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (s *Server) commonHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Referrer-Policy", "no-referrer")
		if !s.indexable(r) {
			h.Set("X-Robots-Tag", "noindex, nofollow")
		}
		h.Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// logRequests logs method, route shape and status. Keys are redacted from paths.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if r.URL.Path == "/healthz" {
			return
		}
		slog.Info("http", "method", r.Method, "path", redact(r.URL.Path), "status", rec.status, "ms", time.Since(start).Milliseconds())
	})
}

func redact(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if len(part) == keys.Length {
			parts[i] = "{key}"
		}
	}
	return strings.Join(parts, "/")
}
