package pdf

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"time"
)

// errBlocked is returned when a host resolves to a non-public address.
var errBlocked = errors.New("blocked address")

// guardProxy is an HTTP proxy that dials the IP it just resolved. Chromium
// sends every non-loopback request here, so a later DNS answer cannot move
// the connection onto a private network.
type guardProxy struct {
	addr   string
	srv    *http.Server
	lookup func(ctx context.Context, host string) ([]netip.Addr, error)
	dial   func(ctx context.Context, network, addr string) (net.Conn, error)
	client *http.Client
}

func startGuardProxy() (*guardProxy, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	p := newGuardProxy()
	p.addr = ln.Addr().String()
	p.srv = &http.Server{
		Handler:           p,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { _ = p.srv.Serve(ln) }()
	return p, nil
}

func newGuardProxy() *guardProxy {
	p := &guardProxy{
		lookup: func(ctx context.Context, host string) ([]netip.Addr, error) {
			return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		},
		dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, addr)
		},
	}
	p.client = &http.Client{
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           p.dialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 20 * time.Second,
			MaxIdleConns:          8,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return p
}

func (p *guardProxy) Close() {
	if p.srv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = p.srv.Shutdown(ctx)
}

func (p *guardProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.connect(w, r)
		return
	}
	if r.URL == nil || (r.URL.Scheme != "http" && r.URL.Scheme != "https") || r.URL.Host == "" {
		http.Error(w, "blocked", http.StatusForbidden)
		return
	}
	out, err := http.NewRequestWithContext(r.Context(), r.Method, r.URL.String(), r.Body)
	if err != nil {
		http.Error(w, "blocked", http.StatusForbidden)
		return
	}
	out.Header = r.Header.Clone()
	for _, h := range []string{"Proxy-Connection", "Proxy-Authorization", "Connection", "Keep-Alive", "Te", "Trailer", "Transfer-Encoding", "Upgrade"} {
		out.Header.Del(h)
	}
	resp, err := p.client.Do(out)
	if err != nil {
		http.Error(w, "blocked", http.StatusForbidden)
		return
	}
	defer resp.Body.Close()
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 32<<20))
}

func (p *guardProxy) connect(w http.ResponseWriter, r *http.Request) {
	dest := r.Host
	if _, _, err := net.SplitHostPort(dest); err != nil {
		dest = net.JoinHostPort(dest, "443")
	}
	upstream, err := p.dialContext(r.Context(), "tcp", dest)
	if err != nil {
		http.Error(w, "blocked", http.StatusForbidden)
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		upstream.Close()
		http.Error(w, "blocked", http.StatusInternalServerError)
		return
	}
	client, _, err := hj.Hijack()
	if err != nil {
		upstream.Close()
		return
	}
	defer client.Close()
	defer upstream.Close()
	if _, err := fmt.Fprint(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(upstream, client)
		close(done)
	}()
	_, _ = io.Copy(client, upstream)
	<-done
}

// dialContext connects to a resolved public address, never to the hostname.
func (p *guardProxy) dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, errBlocked
	}
	ips, err := p.publicIPs(ctx, host)
	if err != nil {
		return nil, err
	}
	var last error
	for _, ip := range ips {
		c, err := p.dial(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return c, nil
		}
		last = err
	}
	if last == nil {
		last = errBlocked
	}
	return nil, last
}

func (p *guardProxy) publicIPs(ctx context.Context, host string) ([]netip.Addr, error) {
	var addrs []netip.Addr
	if a, err := netip.ParseAddr(host); err == nil {
		addrs = []netip.Addr{a}
	} else {
		var err error
		addrs, err = p.lookup(ctx, host)
		if err != nil || len(addrs) == 0 {
			return nil, errBlocked
		}
	}
	out := make([]netip.Addr, 0, len(addrs))
	for _, a := range addrs {
		a = a.Unmap()
		if blocked(a) {
			return nil, errBlocked
		}
		out = append(out, a)
	}
	return out, nil
}

func blocked(a netip.Addr) bool {
	a = a.Unmap()
	if !a.IsValid() || !a.IsGlobalUnicast() || a.IsPrivate() || isCGNAT(a) {
		return true
	}
	return a.Is4() && v4zero.Contains(a)
}

var v4zero = netip.MustParsePrefix("0.0.0.0/8")
