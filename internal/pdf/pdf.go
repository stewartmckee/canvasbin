// Package pdf prints rendered pages to PDF with headless Chromium.
package pdf

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

var ErrBusy = errors.New("too many PDFs rendering, try again shortly")

type Renderer struct {
	ExecPath string
	Timeout  time.Duration
	slots    chan struct{}
}

func New(concurrency int) *Renderer {
	return &Renderer{
		ExecPath: os.Getenv("CHROME_BIN"),
		Timeout:  30 * time.Second,
		slots:    make(chan struct{}, concurrency),
	}
}

// Render loads pageURL in a fresh headless browser and prints it to A4.
// Only pageURL itself may be fetched from a private address; every other
// request to loopback, private or link-local networks is blocked so page
// content cannot reach internal services.
func (r *Renderer) Render(ctx context.Context, pageURL string) ([]byte, error) {
	select {
	case r.slots <- struct{}{}:
		defer func() { <-r.slots }()
	case <-time.After(5 * time.Second):
		return nil, ErrBusy
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("hide-scrollbars", true),
	)
	if r.ExecPath != "" {
		opts = append(opts, chromedp.ExecPath(r.ExecPath))
	}
	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()
	tabCtx, cancelTab := chromedp.NewContext(allocCtx)
	defer cancelTab()

	paused := chromedp.Events(tabCtx, fetch.RequestPaused)
	c := chromedp.FromContext(tabCtx)
	if c == nil || c.Target == nil {
		return nil, errors.New("render pdf: browser did not start")
	}
	target := c.Target
	go func() {
		for ev, err := range paused {
			if err != nil {
				return
			}
			go func() {
				if ev.Request != nil && (ev.Request.URL == pageURL || allowed(ev.Request.URL)) {
					_, _ = cdp.Call(tabCtx, target, fetch.ContinueRequest, fetch.ContinueRequestParams{RequestID: ev.RequestID})
					return
				}
				_, _ = cdp.Call(tabCtx, target, fetch.FailRequest, fetch.FailRequestParams{
					RequestID: ev.RequestID, ErrorReason: network.ErrorReasonBlockedByClient,
				})
			}()
		}
	}()

	err := chromedp.Do(tabCtx,
		chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
			_, err := cdp.Call(ctx, t, fetch.Enable, fetch.EnableParams{})
			return err
		}),
		chromedp.Navigate(pageURL),
		chromedp.Sleep(500*time.Millisecond),
	)
	if err != nil {
		return nil, fmt.Errorf("render pdf: %w", err)
	}
	out, err := chromedp.Run(tabCtx, chromedp.PrintToPDF(
		chromedp.PDFPaper(chromedp.PaperA4),
		chromedp.PDFMargin(0.4),
		chromedp.PDFPrintBackground(),
		chromedp.PDFPreferCSSPageSize(),
	))
	if err != nil {
		return nil, fmt.Errorf("render pdf: %w", err)
	}
	return out, nil
}

// allowed reports whether a subresource URL may be fetched: data/blob URLs, or
// http(s) to hosts that resolve only to public addresses.
func allowed(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	switch u.Scheme {
	case "data", "blob":
		return true
	case "http", "https":
	default:
		return false
	}
	host := u.Hostname()
	var addrs []netip.Addr
	if a, err := netip.ParseAddr(host); err == nil {
		addrs = []netip.Addr{a}
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil || len(ips) == 0 {
			return false
		}
		addrs = ips
	}
	for _, a := range addrs {
		a = a.Unmap()
		if a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() ||
			a.IsUnspecified() || a.IsMulticast() || isCGNAT(a) {
			return false
		}
	}
	return true
}

var cgnat = netip.MustParsePrefix("100.64.0.0/10")

func isCGNAT(a netip.Addr) bool { return cgnat.Contains(a) }
