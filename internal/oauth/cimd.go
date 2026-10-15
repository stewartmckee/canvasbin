package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	errBadRedirect  = errors.New("redirect_uri must be https or a loopback http URL")
	errBadClientID  = errors.New("client_id must be an https URL with a path and no fragment or credentials")
	errBlockedAddr  = errors.New("address is not allowed")
	errBadClientDoc = errors.New("invalid client metadata document")
)

const (
	cimdTimeout     = 5 * time.Second
	cimdMaxBytes    = 64 << 10
	cimdMaxRedirect = 3
	cimdCacheTTL    = 5 * time.Minute
	cimdCacheMax    = 512
)

// Client is what the authorize endpoint needs to know about an OAuth client,
// whether it was registered dynamically or described by a metadata document.
type Client struct {
	ID           string
	Name         string
	RedirectURIs []string
}

// IsCIMDClientID reports whether a client_id is an https URL (a Client ID
// Metadata Document) rather than an id we issued.
func IsCIMDClientID(id string) bool { return strings.HasPrefix(id, "https://") }

// ValidateClientIDURL checks the shape of a metadata-document client_id and
// rejects obviously internal hosts before any network access.
func ValidateClientIDURL(raw string) (*url.URL, error) {
	if len(raw) > 2048 || strings.Contains(raw, "#") {
		return nil, errBadClientID
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return nil, errBadClientID
	}
	if u.Path == "" || u.Path == "/" || u.Fragment != "" {
		return nil, errBadClientID
	}
	for _, seg := range strings.Split(u.Path, "/") {
		if seg == "." || seg == ".." {
			return nil, errBadClientID
		}
	}
	host := strings.ToLower(u.Hostname())
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return nil, errBlockedAddr
	}
	if ip, err := netip.ParseAddr(host); err == nil && blockedIP(ip) {
		return nil, errBlockedAddr
	}
	return u, nil
}

// blockedIP reports whether ip is a loopback, private, link-local, multicast,
// reserved or otherwise non-public address that a server-side fetch must never reach.
func blockedIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

var blockedPrefixes = func() []netip.Prefix {
	var out []netip.Prefix
	for _, s := range []string{
		"0.0.0.0/8",       // "this" network
		"100.64.0.0/10",   // carrier-grade NAT
		"192.0.0.0/24",    // IETF protocol assignments
		"192.0.2.0/24",    // TEST-NET-1
		"192.88.99.0/24",  // 6to4 relay
		"198.18.0.0/15",   // benchmarking
		"198.51.100.0/24", // TEST-NET-2
		"203.0.113.0/24",  // TEST-NET-3
		"240.0.0.0/4",     // reserved + broadcast
		"64:ff9b::/96",    // NAT64 (can embed private v4)
		"64:ff9b:1::/48",  // local-use NAT64
		"100::/64",        // discard-only
		"2001::/32",       // Teredo
		"2001:db8::/32",   // documentation
		"2002::/16",       // 6to4
	} {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}()

// safeControl runs on every outbound connection attempt, after DNS resolution,
// so a hostname that resolves (or is rebound) to an internal address is refused.
func safeControl(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return errBlockedAddr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || blockedIP(ip) {
		return errBlockedAddr
	}
	return nil
}

func safeDialer() *net.Dialer {
	return &net.Dialer{Timeout: cimdTimeout, Control: safeControl}
}

func newSafeHTTPClient() *http.Client {
	d := safeDialer()
	return &http.Client{
		Timeout: 2 * cimdTimeout,
		Transport: &http.Transport{
			Proxy:                 nil, // a proxy would bypass the address check
			DialContext:           d.DialContext,
			TLSHandshakeTimeout:   cimdTimeout,
			ResponseHeaderTimeout: cimdTimeout,
			DisableKeepAlives:     true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > cimdMaxRedirect {
				return errors.New("too many redirects")
			}
			// Every hop must stay on https; the dialer rejects internal addresses.
			if req.URL.Scheme != "https" {
				return errBlockedAddr
			}
			return nil
		},
	}
}

type cimdEntry struct {
	client  *Client
	expires time.Time
}

type cimdFetcher struct {
	http *http.Client
	now  func() time.Time

	mu    sync.Mutex
	cache map[string]cimdEntry
}

func newCIMDFetcher() *cimdFetcher {
	return &cimdFetcher{http: newSafeHTTPClient(), now: time.Now, cache: map[string]cimdEntry{}}
}

func (f *cimdFetcher) cached(id string) (*Client, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.cache[id]
	if !ok || !f.now().Before(e.expires) {
		delete(f.cache, id)
		return nil, false
	}
	return e.client, true
}

func (f *cimdFetcher) put(c *Client) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.cache) >= cimdCacheMax {
		now := f.now()
		for k, e := range f.cache {
			if !now.Before(e.expires) {
				delete(f.cache, k)
			}
		}
		if len(f.cache) >= cimdCacheMax {
			f.cache = map[string]cimdEntry{}
		}
	}
	f.cache[c.ID] = cimdEntry{client: c, expires: f.now().Add(cimdCacheTTL)}
}

// Get returns the client described by the document at clientID, fetching it
// (SSRF-safe, size-capped, briefly cached) when needed.
func (f *cimdFetcher) Get(ctx context.Context, clientID string) (*Client, error) {
	u, err := ValidateClientIDURL(clientID)
	if err != nil {
		return nil, err
	}
	if c, ok := f.cached(clientID); ok {
		return c, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*cimdTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, errBadClientID
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "canvasbin-oauth")
	resp, err := f.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch client metadata: %w", unwrapURLError(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch client metadata: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, cimdMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("fetch client metadata: %w", unwrapURLError(err))
	}
	if len(body) > cimdMaxBytes {
		return nil, fmt.Errorf("%w: document too large", errBadClientDoc)
	}
	c, err := ParseClientDocument(clientID, body)
	if err != nil {
		return nil, err
	}
	f.put(c)
	return c, nil
}

func unwrapURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

// ParseClientDocument validates a fetched metadata document for clientID.
func ParseClientDocument(clientID string, body []byte) (*Client, error) {
	var doc struct {
		ClientID                string   `json:"client_id"`
		ClientName              string   `json:"client_name"`
		RedirectURIs            []string `json:"redirect_uris"`
		TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
		ClientSecret            string   `json:"client_secret"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("%w: not valid JSON", errBadClientDoc)
	}
	if doc.ClientID != clientID {
		return nil, fmt.Errorf("%w: client_id does not match its URL", errBadClientDoc)
	}
	if doc.ClientSecret != "" || (doc.TokenEndpointAuthMethod != "" && doc.TokenEndpointAuthMethod != "none") {
		return nil, fmt.Errorf("%w: only public clients (token_endpoint_auth_method none) are supported", errBadClientDoc)
	}
	if len(doc.RedirectURIs) == 0 || len(doc.RedirectURIs) > maxRedirectURIs {
		return nil, fmt.Errorf("%w: redirect_uris is required", errBadClientDoc)
	}
	for _, ru := range doc.RedirectURIs {
		if err := ValidateRedirectURI(ru); err != nil {
			return nil, fmt.Errorf("%w: %v", errBadClientDoc, err)
		}
	}
	return &Client{ID: clientID, Name: cleanName(doc.ClientName), RedirectURIs: doc.RedirectURIs}, nil
}

const maxRedirectURIs = 10

// cleanName makes a client-supplied display name safe to show: no control
// characters, bounded length.
func cleanName(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
	if r := []rune(s); len(r) > 100 {
		s = string(r[:100])
	}
	return s
}
