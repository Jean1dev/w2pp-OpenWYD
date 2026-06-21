// Command dbserver is the persistence service (DBSrv). It ships the one-shot
// account-file converter and the gRPC AccountService (api/db/v1) that tmServer
// consumes over gRPC+mTLS (migration-plan.md §3.5).
//
// Usage:
//
//	dbserver convert -accounts <dir> [-dsn <postgres-url>]
//	dbserver serve [-addr :7514] -dsn <postgres-url> [-tls-cert … -tls-key … -tls-ca …]
//
// convert: without -dsn it is a dry run; with -dsn it applies migrations and
// persists each converted account. serve: applies migrations then serves gRPC.
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
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/convert"
	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/grpcsrv"
	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/store"
	"github.com/jeanluca/w2pp-openwyd/internal/secure"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "convert":
		if err := runConvert(os.Args[2:], logger); err != nil {
			logger.Error("convert failed", "err", err)
			os.Exit(1)
		}
	case "serve":
		if err := runServe(os.Args[2:], logger); err != nil {
			logger.Error("serve failed", "err", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  dbserver convert -accounts <dir> [-dsn <postgres-url>]")
	fmt.Fprintln(os.Stderr, "  dbserver serve [-addr :7514] -dsn <postgres-url> [-tls-cert -tls-key -tls-ca]")
}

// runServe applies migrations and serves the gRPC AccountService until the
// process receives SIGINT/SIGTERM, then stops gracefully.
func runServe(args []string, logger *slog.Logger) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", ":7514", "gRPC listen address")
	dsn := fs.String("dsn", envOr("W2PP_DB_DSN", ""), "PostgreSQL DSN (or W2PP_DB_DSN)")
	tlsCert := fs.String("tls-cert", os.Getenv("W2PP_TLS_CERT"), "server certificate (PEM)")
	tlsKey := fs.String("tls-key", os.Getenv("W2PP_TLS_KEY"), "server private key (PEM)")
	tlsCA := fs.String("tls-ca", os.Getenv("W2PP_TLS_CA"), "client CA (PEM) for mTLS")
	if err := fs.Parse(args); err != nil {
		return err
	}
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
	dbv1.RegisterAccountServiceServer(srv, grpcsrv.New(store.New(pool)))

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", *addr, err)
	}
	logger.Info("dbserver serving", "addr", *addr, "mtls", *tlsCert != "")

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

func runConvert(args []string, logger *slog.Logger) error {
	fs := flag.NewFlagSet("convert", flag.ExitOnError)
	accounts := fs.String("accounts", "", "path to the legacy account/ directory")
	dsn := fs.String("dsn", "", "PostgreSQL DSN; if set, persist (else dry run)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *accounts == "" {
		return fmt.Errorf("-accounts is required")
	}

	rep, err := convert.WalkAccounts(*accounts)
	if err != nil {
		return err
	}
	logger.Info("conversion scan complete",
		"total", rep.Total, "converted", rep.Converted, "skipped", rep.Skipped, "failed", rep.Failed)
	for _, r := range rep.Results {
		switch {
		case r.Err != nil:
			logger.Warn("failed", "path", r.Path, "err", r.Err)
		case r.Skipped != "":
			logger.Warn("skipped", "path", r.Path, "reason", r.Skipped)
		}
	}

	if *dsn == "" {
		logger.Info("dry run (no -dsn) — not persisting")
		if rep.Failed > 0 {
			return fmt.Errorf("%d file(s) failed to convert", rep.Failed)
		}
		return nil
	}
	return persist(rep, *dsn, logger)
}

func persist(rep *convert.Report, dsn string, logger *slog.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	if err := store.Migrate(ctx, pool); err != nil {
		return err
	}
	s := store.New(pool)

	imported := 0
	for _, r := range rep.Results {
		if r.Account == nil {
			continue
		}
		if _, err := s.SaveAccount(ctx, *r.Account); err != nil {
			return fmt.Errorf("persist %q: %w", r.Account.Name, err)
		}
		imported++
	}
	logger.Info("import complete", "imported", imported)
	return nil
}
