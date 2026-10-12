// Package httpapi serves the JSON API, rendered pages, and the embedded web UI.
package httpapi

import (
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

	"github.com/stewartmckee/canvasbin/internal/keys"
	"github.com/stewartmckee/canvasbin/internal/links"
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

	PDF      *pdf.Renderer
	PDFLimit *ratelimit.Limiter
	// SelfURL is how the PDF renderer reaches this server, e.g. http://127.0.0.1:8080.
	SelfURL string
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("POST /api/pages", s.create)
	mux.HandleFunc("GET /api/pages/{key}", s.get)
	mux.HandleFunc("PUT /api/pages/{key}", s.update)
	mux.HandleFunc("DELETE /api/pages/{key}", s.remove)
	mux.HandleFunc("POST /api/render", s.preview)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})
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
			s.serveIndex(w)
		}
	})
	mux.Handle("/", spa)
	return logRequests(commonHeaders(mux))
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
	ReadKey  string `json:"read_key"`
	WriteKey string `json:"write_key,omitempty"`
	links.Links
}

func (s *Server) response(r *http.Request, a *store.Access) pageResponse {
	return pageResponse{
		Page:     a.Page,
		CanWrite: a.CanWrite,
		ReadKey:  a.ReadKey,
		WriteKey: a.WriteKey,
		Links:    links.For(links.Base(s.PublicBaseURL, r), a.ReadKey, a.WriteKey),
	}
}

type pageInput struct {
	Title   *string `json:"title"`
	Format  *string `json:"format"`
	Content *string `json:"content"`
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
	a, err := s.Store.Create(r.Context(), deref(in.Title), deref(in.Format), deref(in.Content))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, s.response(r, a))
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	key, ok := pathKey(w, r)
	if !ok {
		return
	}
	a, err := s.Store.Get(r.Context(), key)
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
	a, err := s.Store.Update(r.Context(), key, store.Changes{Title: in.Title, Format: in.Format, Content: in.Content})
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
	if err := s.Store.Delete(r.Context(), key); err != nil {
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
	out, err := s.PDF.Render(r.Context(), s.SelfURL+"/"+a.ReadKey+".html")
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
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
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
		s.serveIndex(w)
	})
}

func (s *Server) serveIndex(w http.ResponseWriter) {
	index, err := fs.ReadFile(s.Web, "index.html")
	if err != nil {
		http.Error(w, "web UI not built", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(index)
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

func commonHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Robots-Tag", "noindex, nofollow")
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
