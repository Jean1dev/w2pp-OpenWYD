// Command tmserver is the WYD game server (TMSrv): it speaks the legacy CPSock
// wire protocol to the unmodified WYD.exe 7662 client (tmserver/internal/protocol)
// and owns the in-memory world state through a single game-loop goroutine
// (tmserver/internal/world).
//
// This entrypoint only does wiring (guidelines §3): flags, logging, the gRPC
// client connections to dbServer/binServer, the listener and graceful shutdown.
// Without -dbserver the persistence falls back to a no-op (local bring-up).
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"google.golang.org/grpc"

	"github.com/jeanluca/w2pp-openwyd/internal/secure"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/binclient"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/dbclient"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/handler"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("tmserver stopped with error", "err", err)
		os.Exit(1)
	}
	logger.Info("tmserver stopped")
}

func run(logger *slog.Logger) error {
	addr := flag.String("addr", ":8281", "CPSock listen address for the client edge")
	dbAddr := flag.String("dbserver", os.Getenv("W2PP_DBSERVER"), "dbServer gRPC address (empty = no-op persistence)")
	binAddr := flag.String("binserver", os.Getenv("W2PP_BINSERVER"), "binServer gRPC address (empty = allow-all billing)")
	tlsCert := flag.String("tls-cert", os.Getenv("W2PP_TLS_CERT"), "client certificate (PEM) for internal mTLS")
	tlsKey := flag.String("tls-key", os.Getenv("W2PP_TLS_KEY"), "client private key (PEM)")
	tlsCA := flag.String("tls-ca", os.Getenv("W2PP_TLS_CA"), "CA (PEM) verifying dbServer/binServer")
	tlsServerName := flag.String("tls-server-name", os.Getenv("W2PP_TLS_SERVER_NAME"), "expected server name in internal certs")
	rejectChecksum := flag.Bool("reject-checksum", false, "drop connections on CPSock checksum mismatch (Fase 7; off by default)")
	maxMsgPerSec := flag.Float64("max-msg-per-sec", 200, "per-connection inbound message rate limit (0 = disabled)")
	msgBurst := flag.Int("msg-burst", 400, "per-connection message burst depth")
	contentDir := flag.String("content", os.Getenv("W2PP_CONTENT"), "path to the Release/ content tree (empty = skip; validates rates/catalogs/maps at boot)")
	defStatusAddr := os.Getenv("W2PP_STATUS_ADDR")
	if defStatusAddr == "" {
		defStatusAddr = ":80"
	}
	statusAddr := flag.String("status-addr", defStatusAddr, "HTTP channel-status listen address (serv00.htm); real WYD serves status on :80, separate from the game port. Empty disables")
	clientVersion := flag.Int("client-version", 7640, "MSG_AccountLogin.ClientVersion the client must send (protocol-spec says 7640; this 7662 build's Config.bin reports 7649)")
	flag.Parse()

	// When -content is set, load and validate the content tree up front so a
	// missing/corrupt mount fails fast instead of surfacing mid-session. The
	// recipe→combine-family and AttributeMap-bit semantics remain UNVERIFIED
	// (PROGRESS Fase 5), so this validates and exposes the data; it does not
	// rewire gameplay on unverified mappings.
	if *contentDir != "" {
		if err := loadContent(*contentDir, logger); err != nil {
			return err
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	clientCreds, err := secure.ClientCreds(secure.Config{
		CertFile: *tlsCert, KeyFile: *tlsKey, CAFile: *tlsCA, ServerName: *tlsServerName,
	})
	if err != nil {
		return err
	}

	// Persistence: real dbServer adapter when -dbserver is set, else no-op.
	var persist world.Persistence = world.NopPersistence{}
	if *dbAddr != "" {
		conn, err := grpc.NewClient(*dbAddr, grpc.WithTransportCredentials(clientCreds))
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		persist = dbclient.New(conn)
		logger.Info("dbServer wired", "addr", *dbAddr)
	} else {
		logger.Warn("no -dbserver: using no-op persistence (logins report no account)")
	}

	// The client fetches a channel-status page over HTTP before the CPSock
	// connect; serve it from the content tree when available.
	var statusFile string
	var baseMobs map[int][]byte
	if *contentDir != "" {
		statusFile = filepath.Join(*contentDir, "Common", "serv00.htm")
		if bm, err := content.LoadBaseMobs(*contentDir); err != nil {
			logger.Warn("base mob templates not loaded", "err", err)
		} else {
			baseMobs = bm
			logger.Info("base mob templates loaded", "classes", len(baseMobs))
		}
	}

	dispatch := handler.New(handler.Config{
		Log: logger, ClientVersion: int32(*clientVersion), BaseMobs: baseMobs,
	})
	w := world.New(world.Config{
		RejectChecksum: *rejectChecksum,
		MaxMsgPerSec:   *maxMsgPerSec,
		MsgBurst:       *msgBurst,
		StatusFile:     statusFile,
	}, logger, persist, dispatch.Handle)

	// Billing gate: real binServer adapter when -binserver is set, else allow-all.
	if *binAddr != "" {
		conn, err := grpc.NewClient(*binAddr, grpc.WithTransportCredentials(clientCreds))
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		w.SetBilling(binclient.New(conn))
		logger.Info("binServer wired", "addr", *binAddr)
	}

	// Channel-status HTTP server on its own port (real WYD serves serv00.htm on
	// :80, separate from the game's :8281 — general-config.h of the snalmir
	// reference). The client probes status here, then opens the CPSock game
	// connection to the game port; keeping them apart avoids the client seeing an
	// HTTP server on the game port.
	if *statusAddr != "" {
		go serveStatusHTTP(ctx, *statusAddr, statusFile, logger)
	}

	// Populate the world with NPCs/monsters from NPCGener.txt (before Serve starts
	// the loop, so spawning is single-threaded). Capped to fit the mob slots.
	if *contentDir != "" {
		spawnNPCs(w, *contentDir, logger)
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	logger.Info("tmserver listening", "addr", *addr, "mtls", *tlsCert != "")

	return w.Serve(ctx, ln)
}

// spawnNPCs parses NPCGener.txt and spawns each generator's group (MinGroup,
// capped) of its Leader mob around the start point, up to a global cap that fits
// the mob slots. Templates are cached by name.
func spawnNPCs(w *world.World, dir string, logger *slog.Logger) {
	gens, err := content.LoadNPCGenerators(filepath.Join(dir, "TMsrv", "run", "NPCGener.txt"))
	if err != nil {
		logger.Warn("NPC generators not loaded", "err", err)
		return
	}
	const totalCap = 20000
	const perGenCap = 6
	templates := make(map[string][]byte)
	total := 0
	for _, g := range gens {
		if total >= totalCap || g.Leader == "" {
			continue
		}
		tmpl, seen := templates[g.Leader]
		if !seen {
			if b, terr := content.LoadNPCTemplate(dir, g.Leader); terr == nil {
				tmpl = b
			}
			templates[g.Leader] = tmpl
		}
		if tmpl == nil {
			continue
		}
		n := g.MinGroup
		if n < 1 {
			n = 1
		}
		if n > perGenCap {
			n = perGenCap
		}
		for i := 0; i < n && total < totalCap; i++ {
			x, y := g.StartX, g.StartY
			if g.StartRange > 0 {
				x += int16(rand.Intn(2*g.StartRange+1) - g.StartRange)
				y += int16(rand.Intn(2*g.StartRange+1) - g.StartRange)
			}
			if w.SpawnMob(tmpl, x, y) >= 0 {
				total++
			}
		}
	}
	logger.Info("NPCs spawned", "generators", len(gens), "mobs", total, "templates", len(templates))
}

// serveStatusHTTP runs the channel-status web server (serv00.htm). It answers any
// path with the status page so the client's GET succeeds regardless of the exact
// file it asks for. The body is read per request so it can be edited live.
func serveStatusHTTP(ctx context.Context, addr, statusFile string, logger *slog.Logger) {
	defaultBody := []byte("4\r\n-1\r\n-1\r\n-1\r\n-1\r\n-1\r\n-1\r\n-1\r\n-1\r\n-1\r\n")
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body := defaultBody
		if statusFile != "" {
			if b, err := os.ReadFile(statusFile); err == nil {
				body = b
			}
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write(body)
		logger.Info("served status (http)", "ip", r.RemoteAddr, "req", r.Method+" "+r.URL.Path)
	})
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() { <-ctx.Done(); _ = srv.Close() }()
	logger.Info("status server listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Warn("status server stopped", "err", err)
	}
}

// loadContent loads and validates the Release/ content tree (Fase 5 loaders).
// The rates and catalogs are required (a broken mount is a hard error); the maps
// are large and optional (a warning when absent). It logs what was loaded so the
// operator can confirm the mount is correct.
func loadContent(dir string, logger *slog.Logger) error {
	comp, err := content.LoadCompRate(filepath.Join(dir, "Common", "Settings", "CompRate.txt"))
	if err != nil {
		return err
	}
	sanc, err := content.LoadSancRate(filepath.Join(dir, "Common", "Settings", "SancRate.txt"))
	if err != nil {
		return err
	}
	items, err := content.LoadItemList(filepath.Join(dir, "Common", "ItemList.csv"))
	if err != nil {
		return err
	}
	skills, err := content.LoadSkillData(filepath.Join(dir, "Common", "SkillData.csv"))
	if err != nil {
		return err
	}
	logger.Info("content loaded",
		"comprate_families", comp.Families(), "sancrate_anvils", sanc.Anvils(),
		"items", items.Len(), "skills", skills.Len())

	// Maps are optional: 17 MiB HeightMap + 1 MiB AttributeMap aren't required to
	// accept logins; warn rather than fail when they aren't mounted.
	if _, err := content.LoadGrid(filepath.Join(dir, "TMsrv", "run", "AttributeMap.dat"), content.AttributeMapDim); err != nil {
		logger.Warn("attribute map not loaded", "err", err)
	}
	if _, err := content.LoadHeightMap(filepath.Join(dir, "TMsrv", "run", "HeightMap.dat")); err != nil {
		logger.Warn("height map not loaded", "err", err)
	}
	return nil
}
