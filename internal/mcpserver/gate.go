package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/stewartmckee/canvasbin/internal/links"
	"github.com/stewartmckee/canvasbin/internal/store"
)

// OAuth scopes the gate checks. They mirror internal/oauth (not imported to
// keep this package free of the authorization server).
const (
	scopeRead  = "pages:read"
	scopeWrite = "pages:write"
)

// readTools need only pages:read; every other tool needs pages:write.
var readTools = map[string]bool{"list_pages": true, "get_page": true}

// maxGateBody caps the JSON-RPC body the gate buffers (same cap as the SDK).
const maxGateBody = 3*store.MaxContentBytes + 64<<10

type rpcMessage struct {
	Method string `json:"method"`
	Params struct {
		Name string `json:"name"`
	} `json:"params"`
}

// ToolCalls returns the tool names invoked by a JSON-RPC body (one message or a
// batch). Anything that isn't a tools/call (initialize, tools/list,
// notifications) contributes nothing; unparseable bodies yield nil and are left
// for the SDK to reject.
func ToolCalls(body []byte) []string {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil
	}
	var msgs []rpcMessage
	if body[0] == '[' {
		if json.Unmarshal(body, &msgs) != nil {
			return nil
		}
	} else {
		var m rpcMessage
		if json.Unmarshal(body, &m) != nil {
			return nil
		}
		msgs = []rpcMessage{m}
	}
	var names []string
	for _, m := range msgs {
		if m.Method == "tools/call" {
			names = append(names, m.Params.Name)
		}
	}
	return names
}

func hasBearer(r *http.Request) bool {
	h := r.Header.Get("Authorization")
	return len(h) > 7 && strings.EqualFold(h[:7], "bearer ")
}

type userCtxKey struct{}

// gate runs before the MCP SDK when accounts are on. Every request, initialize
// included, needs a valid bearer token; without one it answers 401 with the
// discovery pointer MCP clients follow to start OAuth, so clients prompt for
// sign-in as soon as they connect. Tool calls are then checked against the
// token's scope.
func (o Options) gate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := links.Base(o.PublicBaseURL, r)
		user := o.UserFromRequest(r)
		if user == nil {
			errAttr := ""
			if hasBearer(r) {
				errAttr = `error="invalid_token"`
			}
			challenge(w, base, errAttr, scopeWrite)
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey{}, user)
		if r.Method != http.MethodPost || o.ScopeFromRequest == nil {
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxGateBody))
		if err != nil {
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				gateError(w, http.StatusRequestEntityTooLarge, "request too large")
			} else {
				gateError(w, http.StatusBadRequest, "could not read request body")
			}
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		scope := o.ScopeFromRequest(r)
		for _, n := range ToolCalls(body) {
			need := scopeWrite
			if readTools[n] {
				need = scopeRead
			}
			if !scopeAllows(scope, need) {
				challenge(w, base, `error="insufficient_scope"`, need)
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func scopeAllows(scope, need string) bool {
	for _, s := range strings.Fields(scope) {
		if s == need || (need == scopeRead && s == scopeWrite) {
			return true
		}
	}
	return false
}

// challenge writes the RFC 9728 / MCP authorization challenge. errAttr is empty
// when no credentials were sent (RFC 6750 §3.1). insufficient_scope is a 403;
// everything else a 401.
func challenge(w http.ResponseWriter, base, errAttr, scope string) {
	status, msg := http.StatusUnauthorized, "authorization required"
	if strings.Contains(errAttr, "insufficient_scope") {
		status, msg = http.StatusForbidden, "insufficient scope"
	}
	attrs := `resource_metadata="` + base + `/.well-known/oauth-protected-resource/mcp", scope="` + scope + `"`
	if errAttr != "" {
		attrs = errAttr + ", " + attrs
	}
	w.Header().Set("WWW-Authenticate", "Bearer "+attrs)
	gateError(w, status, msg)
}

func gateError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
