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
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"google.golang.org/grpc"

	"github.com/jeanluca/w2pp-openwyd/internal/secure"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/binclient"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/dbclient"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/handler"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/npccfg"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/route"
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

// envInt reads an integer flag default from the environment so container
// deployments (Railway) can set it as a variable like the other W2PP_* knobs;
// a missing or malformed value falls back to def.
func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// envBool reads a boolean flag default from the environment (Railway-style knob),
// accepting the usual truthy spellings; a missing or malformed value falls back to
// def.
func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

// addrOrNone renders an empty flag value as "(none)" so the boot log reads
// clearly when an optional address (dbServer/binServer/content) is unset.
func addrOrNone(v string) string {
	if v == "" {
		return "(none)"
	}
	return v
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
	npcEditing := flag.Bool("npc-editing", envBool("W2PP_NPC_EDITING", false), "enable the moderator NPC-editing overlay (npc-editing-plan.md); needs -dbserver and -content. OFF by default: turn it on only after `dbserver import-npcs` has seeded npc_definition, else DB-managed merchant NPCs would be skipped from NPCGener.txt with nothing to replace them")
	defStatusAddr := os.Getenv("W2PP_STATUS_ADDR")
	if defStatusAddr == "" {
		defStatusAddr = ":80"
	}
	statusAddr := flag.String("status-addr", defStatusAddr, "HTTP channel-status listen address (serv00.htm); real WYD serves status on :80, separate from the game port. Empty disables")
	clientVersion := flag.Int("client-version", envInt("W2PP_CLIENT_VERSION", 7640), "MSG_AccountLogin.ClientVersion the client must send (protocol-spec says 7640; this 7662 'Cavaleiros de Kersef' build sends 12000)")
	flag.Parse()

	// Echo the effective wiring at boot: the client-version and the resolved
	// dbServer/binServer addresses are the knobs most often misconfigured in a
	// container deploy (version-mismatch drops, or "produced zero addresses" when
	// an internal hostname is wrong), so surface them before anything connects.
	logger.Info("tmserver config",
		"client_version", *clientVersion,
		"dbserver", addrOrNone(*dbAddr),
		"binserver", addrOrNone(*binAddr),
		"content", addrOrNone(*contentDir),
		"npc_editing", *npcEditing)

	// When -content is set, load and validate the content tree up front so a
	// missing/corrupt mount fails fast instead of surfacing mid-session. The
	// recipe→combine-family and AttributeMap-bit semantics remain UNVERIFIED
	// (PROGRESS Fase 5), so this validates and exposes the data; it does not
	// rewire gameplay on unverified mappings.
	var itemPrices map[int]int32
	var itemEffects map[int][]content.BaseEffect
	var itemReqs map[int]content.ItemReq
	var itemVolatiles, itemPos, itemUnique map[int]int
	var itemRanges map[int]int16
	var spells *content.SkillData
	var heights *content.Grid
	if *contentDir != "" {
		items, skills, hm, err := loadContent(*contentDir, logger)
		if err != nil {
			return err
		}
		itemPrices, itemEffects, itemReqs = items.Prices(), items.BaseEffects(), items.Requirements()
		itemVolatiles, itemPos, itemUnique = items.Volatiles(), items.Positions(), items.Uniques()
		itemRanges = items.Ranges()
		spells = skills
		heights = hm
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	clientCreds, err := secure.ClientCreds(secure.Config{
		CertFile: *tlsCert, KeyFile: *tlsKey, CAFile: *tlsCA, ServerName: *tlsServerName,
	})
	if err != nil {
		return err
	}

	// Persistence: real dbServer adapter when -dbserver is set, else no-op. The
	// connection is retained (dbConn) so the NPC-config overlay can share it.
	var persist world.Persistence = world.NopPersistence{}
	var dbConn *grpc.ClientConn
	if *dbAddr != "" {
		conn, err := grpc.NewClient(*dbAddr, grpc.WithTransportCredentials(clientCreds))
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		dbConn = conn
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

	// Moderator NPC-editing overlay (npc-editing-plan.md): the single switch is
	// -npc-editing (W2PP_NPC_EDITING), off by default so an unseeded DB never makes
	// the NPCGener.txt merchants vanish. When on, it MUST have a dbServer (the config
	// source) and a content tree (to resolve template bytes into the 816-byte
	// STRUCT_MOB) — both are hard dependencies, not optional, so fail fast with a
	// clear error rather than booting an overlay that can't read or spawn anything.
	var npcConfig npccfg.Source
	if *npcEditing {
		if dbConn == nil || *contentDir == "" {
			return fmt.Errorf("-npc-editing requires both -dbserver (config source) and -content (NPC templates)")
		}
		npcConfig = dbclient.NewNpcConfig(dbConn, func(name string) ([]byte, error) {
			return content.LoadNPCTemplate(*contentDir, name)
		}, logger)
		logger.Info("npc config overlay enabled (moderator editing)")
	}

	dispatch := handler.New(handler.Config{
		Log: logger, ClientVersion: int32(*clientVersion), BaseMobs: baseMobs, ItemPrices: itemPrices, ItemEffects: itemEffects, ItemReqs: itemReqs,
		ItemVolatiles: itemVolatiles, ItemPos: itemPos, ItemUnique: itemUnique, Spells: spells, Heights: heights,
		NpcConfig: npcConfig,
	})
	w := world.New(world.Config{
		RejectChecksum: *rejectChecksum,
		MaxMsgPerSec:   *maxMsgPerSec,
		MsgBurst:       *msgBurst,
		StatusFile:     statusFile,
		ItemRanges:     itemRanges,
	}, logger, persist, dispatch.Handle)
	// Mob-AI pulse: monsters acquire/chase/melee nearby players each tick (mobai.go).
	w.SetTickHandler(world.DefaultMobTick, dispatch.Tick)

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
	// the loop, so spawning is single-threaded). Capped to fit the mob slots. When
	// the DB overlay is active, merchant blocks are skipped here (owned by
	// npc_definition) and applied from the config snapshot instead.
	if *contentDir != "" {
		spawnNPCs(w, *contentDir, npcConfig != nil, logger)
	}
	if npcConfig != nil {
		dispatch.ApplyNPCConfigBoot(w)
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	logger.Info("tmserver listening", "addr", *addr, "mtls", *tlsCert != "")

	return w.Serve(ctx, ln)
}

// spawnNPCs parses NPCGener.txt, registers every block as a world.Generator
// (spawn recipe + live population accounting) and fires one GenerateMob per
// block to populate the world: leader + rolled followers per group, instance
// waypoints randomized per mob, respecting each block's MaxNumMob. From then on
// the AI tick regenerates MinuteGenerate>0 blocks on their minute phase and the
// 15s respawn queue covers the rest.
//
// Boot divergence (deliberate): the original starts EMPTY and fills over time
// via the minute timer — blocks with MinuteGenerate<=0 (~45% of the file, e.g.
// the Coliseum) only ever spawn through event code. We populate everything up
// front so the world is playable immediately. This burns the LCG at boot (one
// stream for all spawns, like the original's global rand()); there is no legacy
// boot rand order to diverge from.
func spawnNPCs(w *world.World, dir string, skipMerchants bool, logger *slog.Logger) {
	gens, err := content.LoadNPCGenerators(filepath.Join(dir, "TMsrv", "run", "NPCGener.txt"))
	if err != nil {
		logger.Warn("NPC generators not loaded", "err", err)
		return
	}
	templates := make(map[string][]byte)
	load := func(name string) []byte {
		if name == "" {
			return nil
		}
		tmpl, seen := templates[name]
		if !seen {
			if b, terr := content.LoadNPCTemplate(dir, name); terr == nil {
				tmpl = b
			}
			templates[name] = tmpl
		}
		return tmpl
	}

	wgens := make([]*world.Generator, len(gens))
	skipped := 0
	for i, g := range gens {
		leader := load(g.Leader)
		if leader == nil {
			continue // block unusable without its Leader template (~1400 miss files)
		}
		// When DB-managed NPC config is active, merchant blocks are owned by
		// npc_definition (materialized by the dispatcher overlay), so skip them here
		// to avoid double-spawning. Monsters / non-shop NPCs stay on NPCGener.txt.
		if skipMerchants && protocol.ParseMobBasics(leader).Merchant != 0 {
			skipped++
			continue
		}
		wg := &world.Generator{
			MinuteGenerate: g.MinuteGenerate,
			MinGroup:       g.MinGroup,
			MaxGroup:       g.MaxGroup,
			MaxNumMob:      g.MaxNumMob,
			RouteType:      uint8(g.RouteType),
			SegX:           g.SegX,
			SegY:           g.SegY,
			LeaderTmpl:     leader,
			FollowerTmpl:   load(g.Follower),
		}
		for s := 0; s < 5; s++ {
			wg.SegRange[s] = int16(g.SegRange[s])
			wg.SegWait[s] = int16(g.SegWait[s])
		}
		wgens[i] = wg
	}
	w.RegisterGenerators(wgens)

	total := 0
	for i := range wgens {
		if wgens[i] != nil {
			total += len(w.GenerateMob(i))
		}
	}
	logger.Info("NPCs spawned", "generators", len(gens), "mobs", total, "templates", len(templates),
		"merchant_blocks_skipped", skipped)
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
func loadContent(dir string, logger *slog.Logger) (*content.ItemList, *content.SkillData, *content.Grid, error) {
	comp, err := content.LoadCompRate(filepath.Join(dir, "Common", "Settings", "CompRate.txt"))
	if err != nil {
		return nil, nil, nil, err
	}
	sanc, err := content.LoadSancRate(filepath.Join(dir, "Common", "Settings", "SancRate.txt"))
	if err != nil {
		return nil, nil, nil, err
	}
	items, err := content.LoadItemList(filepath.Join(dir, "Common", "ItemList.csv"))
	if err != nil {
		return nil, nil, nil, err
	}
	skills, err := content.LoadSkillData(filepath.Join(dir, "Common", "SkillData.csv"))
	if err != nil {
		return nil, nil, nil, err
	}
	logger.Info("content loaded",
		"comprate_families", comp.Families(), "sancrate_anvils", sanc.Anvils(),
		"items", items.Len(), "skills", skills.Len())

	// Maps are optional: 17 MiB HeightMap + 1 MiB AttributeMap aren't required to
	// accept logins; warn rather than fail when they aren't mounted. When both
	// load, bake the attribute blocks into the height grid once (the boot-time
	// BASE_ApplyAttribute) — the result drives mob pathfinding (route.Next).
	var heights *content.Grid
	attr, err := content.LoadGrid(filepath.Join(dir, "TMsrv", "run", "AttributeMap.dat"), content.AttributeMapDim)
	if err != nil {
		logger.Warn("attribute map not loaded", "err", err)
	}
	hm, err := content.LoadHeightMap(filepath.Join(dir, "TMsrv", "run", "HeightMap.dat"))
	if err != nil {
		logger.Warn("height map not loaded", "err", err)
	}
	if hm != nil && attr != nil {
		route.Bake(hm, attr)
		heights = hm
		logger.Info("walkability grid baked", "dim", hm.Dim)
	} else if hm != nil || attr != nil {
		logger.Warn("mob pathfinding disabled: need BOTH HeightMap.dat and AttributeMap.dat")
	}
	return items, skills, heights, nil
}
