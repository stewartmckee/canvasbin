package pdf

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestDialPinsResolvedAddress(t *testing.T) {
	p := newGuardProxy()
	p.lookup = func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("203.0.113.10"), netip.MustParseAddr("203.0.113.11")}, nil
	}
	var got []string
	p.dial = func(_ context.Context, _, addr string) (net.Conn, error) {
		got = append(got, addr)
		return nil, errors.New("stop")
	}
	_, err := p.dialContext(context.Background(), "tcp", "cdn.example:443")
	if err == nil {
		t.Fatal("expected dial error")
	}
	if len(got) != 2 || got[0] != "203.0.113.10:443" || got[1] != "203.0.113.11:443" {
		t.Fatalf("dialed %q, want the resolved addresses", got)
	}
}

func TestDialRejectsMixedAnswers(t *testing.T) {
	p := newGuardProxy()
	p.lookup = func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("203.0.113.10"), netip.MustParseAddr("10.0.0.8")}, nil
	}
	p.dial = func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("dialed a host that also has a private address")
		return nil, nil
	}
	if _, err := p.dialContext(context.Background(), "tcp", "rebind.example:443"); !errors.Is(err, errBlocked) {
		t.Fatalf("err = %v, want blocked", err)
	}
}

func TestProxyConnectRejectsPrivate(t *testing.T) {
	p := newGuardProxy()
	p.lookup = func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("10.1.2.3")}, nil
	}
	p.dial = func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("dialed a private address")
		return nil, nil
	}
	req := httptest.NewRequest(http.MethodConnect, "http://evil.test:443", nil)
	req.Host = "evil.test:443"
	rr := httptest.NewRecorder()
	p.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
}
