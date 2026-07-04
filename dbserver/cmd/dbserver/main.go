// Command dbserver is the persistence service (DBSrv). It ships the one-shot
// account-file converter and the gRPC AccountService (api/db/v1) that tmServer
// consumes over gRPC+mTLS (migration-plan.md §3.5).
//
// Usage:
//
//	dbserver convert -accounts <dir> [-dsn <postgres-url>]
//	dbserver serve [-addr :7514] -dsn <postgres-url> [-tls-cert … -tls-key … -tls-ca …]
//	dbserver seed-account -name <login> -pass <password> [-dsn <postgres-url>]
//
// convert: without -dsn it is a dry run; with -dsn it applies migrations and
// persists each converted account. serve: applies migrations then serves gRPC.
// seed-account: creates a single password-only account (no characters) for local
// testing; the player creates characters in the client. Idempotent.
package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/convert"
	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/grpcsrv"
	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/savefmt"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/secret"
	"github.com/jeanluca/w2pp-openwyd/internal/secure"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
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
	case "seed-account":
		if err := runSeedAccount(os.Args[2:], logger); err != nil {
			logger.Error("seed-account failed", "err", err)
			os.Exit(1)
		}
	case "import-npcs":
		if err := runImportNPCs(os.Args[2:], logger); err != nil {
			logger.Error("import-npcs failed", "err", err)
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
	fmt.Fprintln(os.Stderr, "  dbserver seed-account -name <login> -pass <password> [-dsn <postgres-url>]")
	fmt.Fprintln(os.Stderr, "  dbserver import-npcs -content <Release/> [-dsn <postgres-url>]")
}

// runImportNPCs seeds npc_definition (+ npc_shop_item) from the NPCGener.txt
// spawn blocks (npc-editing-plan.md §4.1). It imports only MERCHANT blocks — the
// NPCs whose shop the moderator edits; monsters and non-shop NPCs keep spawning
// from NPCGener.txt. Idempotent by slug, so re-running only adds new blocks.
// Without -dsn it is a dry run that reports what it would import.
func runImportNPCs(args []string, logger *slog.Logger) error {
	fs := flag.NewFlagSet("import-npcs", flag.ExitOnError)
	contentDir := fs.String("content", "", "path to the Release/ content tree")
	dsn := fs.String("dsn", envOr("W2PP_DB_DSN", ""), "PostgreSQL DSN; if unset, dry run")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *contentDir == "" {
		return fmt.Errorf("-content is required")
	}

	defs, err := buildNPCDefinitions(*contentDir, logger)
	if err != nil {
		return err
	}
	logger.Info("scanned NPC generators", "merchant_definitions", len(defs))

	if *dsn == "" {
		logger.Info("dry run (no -dsn) — not persisting")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()
	if err := store.Migrate(ctx, pool); err != nil {
		return err
	}
	inserted, err := store.New(pool).SeedNPCDefinitions(ctx, defs)
	if err != nil {
		return fmt.Errorf("seed npc definitions: %w", err)
	}
	logger.Info("import complete", "inserted", inserted, "already_present", len(defs)-inserted)
	return nil
}

// buildNPCDefinitions parses NPCGener.txt and returns one domain.NPCDefinition
// per MERCHANT spawn block (leader template Merchant != 0), with its shop stock
// read from the template's Carry[]. The slug is stable across re-imports:
// "<template>-<blockIndex>". A block whose leader template is missing or is not a
// merchant is skipped.
func buildNPCDefinitions(contentDir string, logger *slog.Logger) ([]domain.NPCDefinition, error) {
	blocks, err := parseNPCGener(filepath.Join(contentDir, "TMsrv", "run", "NPCGener.txt"))
	if err != nil {
		return nil, err
	}
	templates := make(map[string]*savefmt.Mob)
	load := func(name string) *savefmt.Mob {
		if name == "" {
			return nil
		}
		if m, seen := templates[name]; seen {
			return m
		}
		var m *savefmt.Mob
		if b, terr := os.ReadFile(filepath.Join(contentDir, "TMsrv", "run", "npc", name)); terr == nil {
			if mob, derr := savefmt.DecodeMob(b); derr == nil {
				m = &mob
			} else {
				logger.Warn("npc template decode failed", "name", name, "err", derr)
			}
		}
		templates[name] = m
		return m
	}

	var out []domain.NPCDefinition
	for i, b := range blocks {
		mob := load(b.leader)
		// The shop flag the tmServer honours is CurrentScore.Merchant (STRUCT_MOB
		// offset 104), NOT the top-level Mob.Merchant (offset 17) — read the same
		// field here so importer and runtime agree on what is a merchant.
		if mob == nil || mob.CurrentScore.Merchant == 0 {
			continue // missing template, or a monster / non-shop NPC (stays in NPCGener.txt)
		}
		def := domain.NPCDefinition{
			Slug:         fmt.Sprintf("%s-%d", b.leader, i),
			TemplateName: b.leader,
			DisplayName:  cString(mob.Name[:]),
			Enabled:      true,
			PosX:         int32(b.startX),
			PosY:         int32(b.startY),
			RouteType:    int16(b.routeType),
			Merchant:     int16(mob.CurrentScore.Merchant),
		}
		// Shop slots are positional (MSG_ShopList sends slots 0..26 verbatim), so
		// keep each item at its template Carry index rather than compacting.
		for ci := 0; ci < 27; ci++ {
			c := mob.Carry[ci]
			if c.Index == 0 {
				continue
			}
			def.Shop = append(def.Shop, domain.NPCShopItem{
				Slot:      int16(ci),
				ItemIndex: int32(c.Index),
				Eff1:      c.Effects[0].Effect, EffV1: c.Effects[0].Value,
				Eff2: c.Effects[1].Effect, EffV2: c.Effects[1].Value,
				Eff3: c.Effects[2].Effect, EffV3: c.Effects[2].Value,
			})
		}
		out = append(out, def)
	}
	return out, nil
}

// npcGenerBlock is the subset of an NPCGener.txt spawn block the importer needs:
// the leader template name, the Start waypoint (spawn point) and the route type.
type npcGenerBlock struct {
	leader         string
	startX, startY int
	routeType      int
}

// parseNPCGener reads NPCGener.txt. Blocks begin with '#'; lines are "Key:\tvalue";
// '//' lines are comments. This is a minimal text parser for the import (the full
// spawn semantics live in tmserver/internal/content, which the tmServer uses at
// runtime; only the fields the seed needs are read here).
func parseNPCGener(path string) ([]npcGenerBlock, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("import-npcs: open NPCGener: %w", err)
	}
	defer f.Close()

	var out []npcGenerBlock
	var cur *npcGenerBlock
	flush := func() {
		if cur != nil && cur.leader != "" {
			out = append(out, *cur)
		}
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "#") {
			flush()
			cur = &npcGenerBlock{}
			continue
		}
		if cur == nil {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		switch key {
		case "Leader":
			cur.leader = val
		case "StartX":
			cur.startX = atoiSafe(val)
		case "StartY":
			cur.startY = atoiSafe(val)
		case "RouteType":
			cur.routeType = atoiSafe(val)
		}
	}
	flush()
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("import-npcs: scan NPCGener: %w", err)
	}
	return out, nil
}

// cString trims a fixed-size C string field at its first NUL.
func cString(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}

func atoiSafe(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// runSeedAccount creates one password-only account for local testing. The name
// is canonicalized to lowercase (matching the converter, convert.Account); the
// password is stored only as an argon2id hash (never plaintext). It is
// idempotent: an existing account is left untouched and reported, not an error,
// so the local bring-up script can run repeatedly.
func runSeedAccount(args []string, logger *slog.Logger) error {
	fs := flag.NewFlagSet("seed-account", flag.ExitOnError)
	name := fs.String("name", "", "account login (canonicalized to lowercase)")
	pass := fs.String("pass", "", "account password (hashed with argon2id)")
	dsn := fs.String("dsn", envOr("W2PP_DB_DSN", ""), "PostgreSQL DSN (or W2PP_DB_DSN)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" || *pass == "" {
		return fmt.Errorf("-name and -pass are required")
	}
	if *dsn == "" {
		return fmt.Errorf("-dsn (or W2PP_DB_DSN) is required")
	}
	canonical := strings.ToLower(*name)

	passHash, err := secret.HashSecret(*pass)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()
	if err := store.Migrate(ctx, pool); err != nil {
		return err
	}
	s := store.New(pool)

	if _, err := s.AccountByName(ctx, canonical); err == nil {
		logger.Info("account already exists — leaving untouched", "name", canonical)
		return nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("lookup: %w", err)
	}

	id, err := s.SaveAccount(ctx, domain.Account{Name: canonical, PassHash: passHash})
	if err != nil {
		return fmt.Errorf("save account: %w", err)
	}
	logger.Info("account seeded", "name", canonical, "id", id, "characters", 0)
	return nil
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
	st := store.New(pool)
	dbv1.RegisterAccountServiceServer(srv, grpcsrv.New(st))
	dbv1.RegisterNpcConfigServiceServer(srv, grpcsrv.NewNpcConfig(st))

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
