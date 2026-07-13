// Command webserver is the web-api: the gRPC edge the Next.js BFF calls
// server-side (web-platform-plan.md). It owns the web platform's account flows
// (sign-up, credential check) over the same `account` table and argon2id hashing
// as dbServer, but is a SEPARATE service from dbServer's legacy AccountService.
//
// Usage:
//
//	webserver [-addr :7600] -dsn <postgres-url> [-tls-cert … -tls-key … -tls-ca …] [-content <Release/>]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/secure"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/account"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/characters"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/dailyreward"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/donateshop"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/donatetopup"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/grpcsrv"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/itemcatalog"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/npcadmin"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/npctemplates"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/ranking"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("webserver failed", "err", err)
		os.Exit(1)
	}
}

// run applies migrations and serves the gRPC AccountWebService until the process
// receives SIGINT/SIGTERM, then stops gracefully. It shares store.Migrate with
// dbServer, so booting either service brings the schema up to date.
func run(logger *slog.Logger) error {
	addr := flag.String("addr", ":7600", "gRPC listen address")
	dsn := flag.String("dsn", envOr("W2PP_DB_DSN", ""), "PostgreSQL DSN (or W2PP_DB_DSN)")
	tlsCert := flag.String("tls-cert", os.Getenv("W2PP_TLS_CERT"), "server certificate (PEM)")
	tlsKey := flag.String("tls-key", os.Getenv("W2PP_TLS_KEY"), "server private key (PEM)")
	tlsCA := flag.String("tls-ca", os.Getenv("W2PP_TLS_CA"), "client CA (PEM) for mTLS")
	contentDir := flag.String("content", os.Getenv("W2PP_CONTENT"), "path to the Release/ content tree (empty = skip; ListMerchantTemplates/ListItemCatalog return empty lists and the UI falls back to manual entry)")
	flag.Parse()

	if *dsn == "" {
		return fmt.Errorf("-dsn (or W2PP_DB_DSN) is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()
	if err := store.Migrate(ctx, pool); err != nil {
		return err
	}

	creds, err := secure.ServerCreds(secure.Config{CertFile: *tlsCert, KeyFile: *tlsKey, CAFile: *tlsCA})
	if err != nil {
		return err
	}
	srv := grpc.NewServer(grpc.Creds(creds))
	st := store.New(pool)
	npcAdmin := npcadmin.New(st)
	donate := donateshop.New(st)
	dailyRwd := dailyreward.New(st)
	topup := donatetopup.New(st)
	if *contentDir != "" {
		templates, err := npctemplates.Scan(*contentDir, logger)
		if err != nil {
			logger.Warn("npc template scan failed; merchant picker will be empty", "content", *contentDir, "err", err)
		} else {
			logger.Info("scanned merchant npc templates", "count", len(templates))
			npcAdmin.SetTemplates(templates)
		}

		items, err := itemcatalog.Scan(*contentDir)
		if err != nil {
			logger.Warn("item catalog scan failed; item picker will be empty", "content", *contentDir, "err", err)
		} else {
			logger.Info("scanned item catalog", "count", len(items))
			npcAdmin.SetItems(items)
		}
	}
	webv1.RegisterAccountWebServiceServer(srv, grpcsrv.New(account.New(st)))
	webv1.RegisterRankingWebServiceServer(srv, grpcsrv.NewRanking(ranking.New(st)))
	webv1.RegisterCharacterWebServiceServer(srv, grpcsrv.NewCharacters(characters.New(st)))
	webv1.RegisterNpcAdminServiceServer(srv, grpcsrv.NewNpcAdmin(npcAdmin))
	webv1.RegisterDonateAdminServiceServer(srv, grpcsrv.NewDonateAdmin(donate))
	webv1.RegisterDonateShopServiceServer(srv, grpcsrv.NewDonateShop(donate))
	webv1.RegisterDailyRewardAdminServiceServer(srv, grpcsrv.NewDailyRewardAdmin(dailyRwd))
	webv1.RegisterDailyRewardServiceServer(srv, grpcsrv.NewDailyReward(dailyRwd))
	webv1.RegisterDonateTopupServiceServer(srv, grpcsrv.NewDonateTopup(topup))

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", *addr, err)
	}
	logger.Info("webserver serving", "addr", *addr, "mtls", *tlsCert != "")

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case <-ctx.Done():
		logger.Info("shutting down")
		srv.GracefulStop()
		return nil
	case err := <-errCh:
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	}
}

// envOr returns the environment value for key, or def when unset.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
