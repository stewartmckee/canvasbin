package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/stewartmckee/canvasbin/internal/billing"
	"github.com/stewartmckee/canvasbin/internal/config"
	"github.com/stewartmckee/canvasbin/internal/httpapi"
	"github.com/stewartmckee/canvasbin/internal/live"
	"github.com/stewartmckee/canvasbin/internal/mail"
	"github.com/stewartmckee/canvasbin/internal/mcpserver"
	"github.com/stewartmckee/canvasbin/internal/pdf"
	"github.com/stewartmckee/canvasbin/internal/ratelimit"
	"github.com/stewartmckee/canvasbin/internal/store"
	"github.com/stewartmckee/canvasbin/web"
)

var version = "dev"

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbURL := env("DATABASE_URL", "postgres://canvasbin:canvasbin@localhost:5432/canvasbin?sslmode=disable")
	addr := env("HTTP_ADDR", ":8080")
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return err
	}
	base := cfg.PublicBaseURL

	st, err := store.Open(ctx, dbURL)
	if err != nil {
		return err
	}
	defer st.Close()

	limiter := ratelimit.New(60, 10*time.Minute)
	srv := &httpapi.Server{
		Store:         st,
		PublicBaseURL: base,
		CreateLimit:   limiter,
		Web:           web.Dist(),
		PDF:           pdf.New(2),
		PDFLimit:      ratelimit.New(20, 10*time.Minute),
		SelfURL:       selfURL(addr),
		Version:       version,
		Config:        cfg,
		Live:          live.New(10000),
	}
	go srv.RunLive(ctx)
	mcpOpts := mcpserver.Options{
		Store: st, PublicBaseURL: base, CreateLimit: limiter, Version: version, Config: cfg,
	}
	if cfg.Accounts {
		srv.Mailer = mail.New(cfg.MailgunKey, cfg.MailgunDomain, cfg.MailFrom)
		srv.Billing = billing.New(cfg, st)
		srv.AuthLimit = ratelimit.New(30, 10*time.Minute)
		srv.MailLimit = ratelimit.New(5, time.Hour)
		// MCP OAuth: the provider serves the authorization server routes and
		// resolves bearer tokens for the /mcp gate and tools.
		srv.OAuth = srv.NewOAuth()
		mcpOpts.UserFromRequest = srv.OAuth.UserFromRequest
		mcpOpts.ScopeFromRequest = srv.OAuth.ScopeFromRequest
		mcpOpts.Mailer = srv.Mailer
		mcpOpts.MailLimit = srv.MailLimit
		go pruneExpired(ctx, st)
		slog.Info("accounts enabled", "billing", srv.Billing.Enabled())
	}
	srv.MCP = mcpserver.Handler(mcpOpts)
	hs := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", addr, "version", version)
		errc <- hs.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return hs.Shutdown(shutdownCtx)
}

// pruneExpired periodically removes expired sessions and stale email tokens.
func pruneExpired(ctx context.Context, st *store.Store) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := st.DeleteExpiredSessions(ctx); err != nil {
				slog.Error("prune sessions", "err", err)
			}
		}
	}
}

// selfURL is the loopback address the PDF renderer uses to load pages.
func selfURL(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		port = "8080"
	}
	return "http://127.0.0.1:" + port
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
