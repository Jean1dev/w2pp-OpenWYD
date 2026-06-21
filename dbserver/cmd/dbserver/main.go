// Command dbserver is the persistence service (DBSrv). In Phase 2 it ships the
// one-shot account-file converter; the gRPC server (api/db/v1) is wired in
// Phase 3 when tmServer becomes the consumer (migration-plan.md §4).
//
// Usage:
//
//	dbserver convert -accounts <dir> [-dsn <postgres-url>]
//
// Without -dsn the converter is a dry run that reports what it would import.
// With -dsn it applies migrations and persists each converted account.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/convert"
	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/store"
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
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: dbserver convert -accounts <dir> [-dsn <postgres-url>]")
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
