// Command adminserver is the staff admin panel: account search, role, VIP and
// audit, served as plain HTTP with an embedded UI.
//
// It is deliberately a SEPARATE service, not a surface bolted onto webServer.
// The game services run in production with players connected; a panel that lives
// inside one of them cannot be rolled back without rolling back the game too.
// Standalone, the whole feature is deletable: nothing in tmServer, dbServer,
// binServer or webServer imports it or knows it exists, and the only shared
// resource is the database — where every change this feature needs is additive.
//
// Plain HTTP rather than gRPC for the same reason. gRPC is right for the internal
// links, but this endpoint is opened in a browser, and browsers do not speak it —
// gRPC here would mean grpc-web plus a proxy, or a second service whose only job
// is translation. Both cost more than they return for a staff-only panel.
//
// It does NOT run migrations. dbServer and webServer share store.Migrate and
// whichever boots first brings the schema up; the panel only reads a schema it
// does not own, which is what keeps deleting the service a complete undo.
//
// Usage:
//
//	adminserver [-addr :8080] -dsn <postgres-url> [-session-ttl 2h] [-insecure-cookies]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/accounts"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/donate"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/entrega"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/gamedata"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/jogo"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/panel"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/personagem"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/plataforma"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/session"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

// Shutdown budget. Long enough to finish an in-flight request, short enough that
// the platform's own stop timeout never fires first and turns a clean stop into a
// kill.
const shutdownTimeout = 10 * time.Second

// HTTP server timeouts. Go's defaults are unlimited, which on a public endpoint
// lets a slow or abandoned client hold a connection open indefinitely — and this
// service, unlike the internal ones, is reachable from the internet.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("adminserver failed", "err", err)
		os.Exit(1)
	}
}

// run serves HTTP until the process receives SIGINT/SIGTERM, then drains.
func run(logger *slog.Logger) error {
	addr := flag.String("addr", defaultAddr(), "HTTP listen address")
	dsn := flag.String("dsn", envOr("DATABASE_URL", os.Getenv("W2PP_DB_DSN")), "PostgreSQL DSN (or DATABASE_URL)")
	sessionTTL := flag.Duration("session-ttl", 2*time.Hour, "how long a staff session stays valid")
	// Browsers drop a Secure cookie sent over plain HTTP, so local development
	// on http://localhost cannot log in without this. Off by default: the flag
	// has to be asked for, never assumed.
	insecureCookies := flag.Bool("insecure-cookies", false, "omit the Secure flag on the session cookie (local HTTP only)")
	// Optional. Without it the item pages are hidden rather than broken, so the
	// panel still runs against nothing but the database.
	webAddr := flag.String("webserver", os.Getenv("W2PP_WEBSERVER"), "webServer gRPC address for the item pages (empty = hide them)")
	jogoAddr := flag.String("tmserver", os.Getenv("W2PP_TMSERVER_CONTROL"), "tmServer control address for the live pages: who is online, kick, notice (empty = hide them). Needs W2PP_CONTROL_TOKEN to match the tmServer's")
	flag.Parse()

	if *dsn == "" {
		return fmt.Errorf("-dsn (or DATABASE_URL) is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := store.Pool(ctx, *dsn)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	var game panel.GameData
	if *webAddr != "" {
		// Insecure credentials: this link stays on the platform's private
		// network, exactly like tmServer's to dbServer. Give it mTLS the day
		// that link gets it, not before — a lone service with certificates the
		// others lack is a maintenance trap, not a security gain.
		// The key the web-api checks (webserver/internal/authz). Empty is the
		// migration step and nothing more: the web-api lets an unkeyed caller
		// through only while IT has no key configured either, so both sides can
		// be updated before either starts enforcing.
		chave := os.Getenv("W2PP_WEB_TOKEN_PAINEL")
		conn, err := grpc.NewClient(*webAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithUnaryInterceptor(mandaChaveWeb(chave)))
		if err != nil {
			return fmt.Errorf("webserver dial: %w", err)
		}
		defer func() { _ = conn.Close() }()
		game = gamedata.New(conn)
		logger.Info("webServer wired", "addr", *webAddr, "chave", chave != "")
		if chave == "" {
			logger.Warn("sem W2PP_WEB_TOKEN_PAINEL: as páginas de item e NPC vão " +
				"parar de funcionar assim que o web-api passar a exigir a chave")
		}
	} else {
		logger.Warn("no webServer configured; item pages are hidden",
			"configuration", "W2PP_WEBSERVER")
	}

	// The link to the running game. Off unless an address is given, and refused
	// without a token rather than dialled and failing on every call: the panel
	// would show a Servidor tab that only ever reports a rejection.
	var live panel.Live
	if *jogoAddr != "" {
		token := os.Getenv("W2PP_CONTROL_TOKEN")
		if token == "" {
			return fmt.Errorf("-tmserver is set but W2PP_CONTROL_TOKEN is empty; " +
				"the game server refuses every call without it")
		}
		conn, cerr := grpc.NewClient(*jogoAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if cerr != nil {
			return fmt.Errorf("tmserver dial: %w", cerr)
		}
		defer func() { _ = conn.Close() }()
		live = jogo.New(conn, token)
		logger.Info("live game link enabled", "addr", *jogoAddr)
	} else {
		logger.Info("live game pages disabled",
			"configuration", "W2PP_TMSERVER_CONTROL + W2PP_CONTROL_TOKEN")
	}

	// Hosting API, for the game-server status card and its restart button. The
	// project and environment ids are injected into every service by the
	// platform; only the token and the game service's id have to be set by hand.
	var plat panel.Platform
	platCfg := plataforma.Config{
		// Prefer RAILWAY_PROJECT_TOKEN: it reaches this project only, while an
		// account token reaches every project its owner has — and this value
		// lives in the environment of a service published on the internet.
		ProjectToken:  os.Getenv("RAILWAY_PROJECT_TOKEN"),
		Token:         os.Getenv("RAILWAY_API_TOKEN"),
		ProjectID:     os.Getenv("RAILWAY_PROJECT_ID"),
		EnvironmentID: os.Getenv("RAILWAY_ENVIRONMENT_ID"),
		ServiceID:     os.Getenv("W2PP_TMSERVER_SERVICE_ID"),
	}
	if platCfg.Ready() {
		plat = plataforma.New(platCfg)
		logger.Info("hosting API wired", "service", platCfg.ServiceID)
	} else {
		logger.Warn("no hosting API configured; the restart card is hidden",
			"configuration", "RAILWAY_API_TOKEN + W2PP_TMSERVER_SERVICE_ID")
	}

	handler, err := panel.New(panel.Config{
		Platform:    plat,
		Accounts:    store.New(pool),
		GameData:    game,
		Writer:      accounts.New(pool),
		Entregas:    entrega.New(pool),
		Personagens: personagem.New(pool),
		Eventos:     store.New(pool),
		MesaXP:      store.New(pool),
		Masmorras:   store.New(pool),
		Quests:      store.New(pool),
		Spawn:       store.New(pool),
		Combate:     store.New(pool),
		BonusDrop:   store.New(pool),
		Maquinas:    store.New(pool),
		Denuncias:   store.New(pool),
		Guildas:     store.New(pool),
		Carteira:    donate.New(pool),
		Trocas:      store.New(pool),
		Censo:       store.New(pool),
		Chat:        store.New(pool),
		Jogo:        live,
		Audit:       audit.New(pool),
		Sessions:    session.New(*sessionTTL),
		Logger:      logger,
		SecureOnly:  !*insecureCookies,
	})
	if err != nil {
		return fmt.Errorf("build panel: %w", err)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("adminserver listening", "addr", *addr, "session_ttl", *sessionTTL,
			"secure_cookies", !*insecureCookies)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("serve: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return <-errCh
	}
}

// defaultAddr honours the port the platform assigns. Railway (and most PaaS)
// inject PORT and route to it; binding a hardcoded port there means the health
// probe hits a closed socket and the deploy is marked failed with the process
// running fine.
func defaultAddr() string {
	if p := os.Getenv("PORT"); p != "" {
		return ":" + p
	}
	return ":8080"
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// mandaChaveWeb attaches the panel's key to every call to the web-api.
//
// An empty key sends no header at all, rather than an empty one: the web-api
// refuses a blank key on purpose, and sending one would turn "not configured
// yet" into a confusing authentication failure instead of the pass-through the
// migration step needs.
func mandaChaveWeb(chave string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any,
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
	) error {
		if chave != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, webv1.TokenHeader, chave)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
