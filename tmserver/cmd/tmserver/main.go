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
	_ "net/http/pprof" // registers /debug/pprof on DefaultServeMux; only reachable via -pprof-addr
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"syscall"
	"time"

	"google.golang.org/grpc"

	gamev1 "github.com/jeanluca/w2pp-openwyd/api/game/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/buildinfo"
	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/internal/npctemplate"
	"github.com/jeanluca/w2pp-openwyd/internal/secure"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/binclient"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/control"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/dbclient"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/handler"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/itemstat"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/mobstat"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/mountrate"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/npccfg"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/route"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/worldcfg"
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
	idleTimeoutSec := flag.Int("idle-timeout-sec", envInt("W2PP_IDLE_TIMEOUT_SEC", 0), "drop a connection that sends nothing for this many seconds (0 = disabled). An authenticated socket that goes silent otherwise holds one of the 1000 session slots forever. Off by default because the real client's idle cadence is unconfirmed — enable once a capture shows it, or a legitimate idle player gets disconnected")
	contentDir := flag.String("content", os.Getenv("W2PP_CONTENT"), "path to the Release/ content tree (empty = skip; validates rates/catalogs/maps at boot)")
	npcEditing := flag.Bool("npc-editing", envBool("W2PP_NPC_EDITING", false), "enable the moderator NPC-editing overlay (npc-editing-plan.md); needs -dbserver and -content. OFF by default: turn it on only after `dbserver import-npcs` has seeded npc_definition, else DB-managed merchant NPCs would be skipped from NPCGener.txt with nothing to replace them")
	mobStatEditing := flag.Bool("mob-stat-editing", envBool("W2PP_MOB_STAT_EDITING", false), "enable the moderator mob/NPC template stat overlay (mob-template-editing-plan.md, the equivalent-tool successor to the legacy EDITAPPMOB); needs -dbserver and -content. Applied ONCE at boot, like every other content load — a moderator edit needs a tmServer restart to take effect (EDITAPPMOB itself required a server restart too), independent of -npc-editing")
	itemStatEditing := flag.Bool("item-stat-editing", envBool("W2PP_ITEM_STAT_EDITING", false), "enable the moderator item base stat overlay (0023_item_stats): what a catalog item requires to equip and the effects it grants. Needs -dbserver and -content. Applied ONCE at boot like -mob-stat-editing, and for a sharper reason — these numbers feed the equip score model, which is recomputed per character, so a live swap would leave two players wearing the same item with different stats. Independent of -mob-stat-editing")
	controlAddr := flag.String("control-addr", os.Getenv("W2PP_CONTROL_ADDR"), "listen address for the admin control API (kick, broadcast, who is online). Empty disables it. Requires W2PP_CONTROL_TOKEN: the tmServer has no database and cannot check a moderator role, so it authenticates the caller as the panel by shared secret and trusts the panel's own role check and audit trail")
	defStatusAddr := os.Getenv("W2PP_STATUS_ADDR")
	if defStatusAddr == "" {
		defStatusAddr = ":80"
	}
	statusAddr := flag.String("status-addr", defStatusAddr, "HTTP channel-status listen address (serv00.htm); real WYD serves status on :80, separate from the game port. Empty disables")
	clientVersion := flag.Int("client-version", envInt("W2PP_CLIENT_VERSION", 7640), "MSG_AccountLogin.ClientVersion the client must send (protocol-spec says 7640; this 7662 'Cavaleiros de Kersef' build sends 12000)")
	doubleExp := flag.Bool("double-exp", envBool("W2PP_DOUBLE_EXP", false), "DOUBLEMODE: double PvE experience (gameconfig double)")
	newbieEvent := flag.Bool("newbie-event", envBool("W2PP_NEWBIE_EVENT", false), "NewbieEventServer: +15% exp and newbie under-100 bonus (gameconfig)")
	kefraLive := flag.Bool("kefra-live", envBool("W2PP_KEFRA_LIVE", false), "KefraLive: when false, PvE exp is halved (default legacy KefraLive=0)")
	maxNightmare := flag.Int("max-nightmare", envInt("W2PP_MAX_NIGHTMARE", 0), "maxNightmare: Pesadelo runs allowed per 4-minute window per tier, server-wide (0 = the legacy default of 3)")
	logSends := flag.Bool("log-sends", envBool("W2PP_LOG_SENDS", false), "log every S→C frame (conn/type/id/len) — client-freeze diagnostics (investigacao-freeze-cliente.md); high volume, enable only while reproducing an incident")
	pprofAddr := flag.String("pprof-addr", os.Getenv("W2PP_PPROF_ADDR"), "expose net/http/pprof on this address (empty disables). Bind to loopback or a private network only — these endpoints dump memory and let any caller start a CPU profile. Without it nothing on the box can answer whether the process is leaking goroutines")
	// Cast-buff duration tuning (issue #229). The legacy formula is
	// (AffectTime+1)*(100+Special)/100 ticks of 8s, which puts an endgame character
	// at 30-100 min per buff — reported as far too long three times over (#92, #202,
	// #229). 100/0/0 restores the legacy durations exactly.
	affectScalePct := flag.Int("affect-scale-pct", envInt("W2PP_AFFECT_SCALE_PCT", 15), "percent of the legacy cast-affect duration to keep, applied only OUTSIDE the target band: a non-aggressive affect whose BASE (unmastered) duration is already under -affect-max-minutes keeps its legacy mastery curve (issue #236). 100 = legacy")
	affectMinSeconds := flag.Int("affect-min-seconds", envInt("W2PP_AFFECT_MIN_SECONDS", 60), "floor for NON-aggressive cast affects, in seconds; keeps the shortest buffs usable (0 = no floor)")
	affectMaxMinutes := flag.Int("affect-max-minutes", envInt("W2PP_AFFECT_MAX_MINUTES", 10), "cap for cast affects, in minutes; cuts the mastery tail (0 = no cap)")
	flag.Parse()

	// Echo the effective wiring at boot: the client-version and the resolved
	// dbServer/binServer addresses are the knobs most often misconfigured in a
	// container deploy (version-mismatch drops, or "produced zero addresses" when
	// an internal hostname is wrong), so surface them before anything connects.
	// Which build is serving. Reading a bug report against the wrong binary has
	// cost several full test rounds: a fix that looks ineffective is often just
	// an older container still running. This line makes that checkable.
	logger.Info("tmserver build", "revision", buildinfo.Revision(), "built", buildinfo.Built())
	logger.Info("tmserver config",
		"client_version", *clientVersion,
		"dbserver", addrOrNone(*dbAddr),
		"binserver", addrOrNone(*binAddr),
		"content", addrOrNone(*contentDir),
		"npc_editing", *npcEditing,
		"mob_stat_editing", *mobStatEditing)

	// When -content is set, load and validate the content tree up front so a
	// missing/corrupt mount fails fast instead of surfacing mid-session. The
	// recipe→combine-family and AttributeMap-bit semantics remain UNVERIFIED
	// (PROGRESS Fase 5), so this validates and exposes the data; it does not
	// rewire gameplay on unverified mappings.
	var itemPrices map[int]int32
	var itemNames map[int]string
	var itemEffects map[int][]content.BaseEffect
	var itemReqs map[int]content.ItemReq
	var itemVolatiles, itemPos, itemUnique, itemGrades, itemExtra, itemDurations map[int]int
	var itemRanges map[int]int16
	var combineFamilies map[protocol.Type]handler.CombineFamily
	var odinCatalog combine.Catalog
	var spells *content.SkillData
	var heights *content.Grid
	var sancRate *content.SancRate
	var compRate *content.CompRate
	var questRates *content.QuestRates
	var language *content.Language
	if *contentDir != "" {
		c, err := loadContent(*contentDir, logger)
		if err != nil {
			return err
		}
		items := c.items
		itemPrices, itemEffects, itemReqs = items.Prices(), items.BaseEffects(), items.Requirements()
		itemNames = items.Names()
		itemVolatiles, itemPos, itemUnique = items.Volatiles(), items.Positions(), items.Uniques()
		// Lifetime of the temporary items, read from the "(30dias)" in their name.
		// The clock only starts when one is first equipped (handler.startTimedItem).
		itemDurations = items.Durations()
		itemGrades = items.Grades()
		itemExtra = items.Extras()
		itemRanges = items.Ranges()
		odinCatalog = handler.NewCombineCatalog(items, c.comp)
		combineFamilies = handler.DefaultCombineFamilies(odinCatalog)
		spells = c.skills
		heights = c.heights
		sancRate = c.sanc
		compRate = c.comp
		questRates = c.quests
		language = c.language
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
	var worldEvents worldcfg.Source
	var dungeonGates handler.DungeonGateSource
	var spawnRates handler.SpawnRateSource
	if *dbAddr != "" {
		conn, err := grpc.NewClient(*dbAddr, grpc.WithTransportCredentials(clientCreds))
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		dbConn = conn
		persist = dbclient.New(conn)
		worldEvents = dbclient.NewWorldEventConfig(conn)
		dungeonGates = dbclient.NewDungeonGateSource(conn)
		spawnRates = dbclient.NewSpawnRateSource(conn)
		logger.Info("dbServer wired", "addr", *dbAddr)
	} else {
		logger.Warn("no -dbserver: using no-op persistence (logins report no account)")
	}

	// The client fetches a channel-status page over HTTP before the CPSock
	// connect; serve it from the content tree when available.
	var statusFile string
	var baseMobs map[int][]byte
	var summonMobs [][]byte
	var vineMob []byte
	var castleQuests []content.CastleQuest
	if *contentDir != "" {
		statusFile = filepath.Join(*contentDir, "Common", "serv00.htm")
		if bm, err := content.LoadBaseMobs(*contentDir); err != nil {
			logger.Warn("base mob templates not loaded", "err", err)
		} else {
			baseMobs = bm
			logger.Info("base mob templates loaded", "classes", len(baseMobs))
		}
		if sm, missing, err := content.LoadBaseSummons(*contentDir); err != nil {
			logger.Warn("base summon templates not loaded (BM evocations disabled)", "err", err)
		} else {
			summonMobs = sm
			if len(missing) > 0 {
				logger.Warn("optional summon templates missing", "names", missing)
			}
			logger.Info("base summon templates loaded", "count", len(summonMobs)-len(missing))
		}
		// Muro de Espinhos is CreateMob("Vinha", ..., "Boss", 3) in the C++,
		// but the shipped Linux content tree carries the wall template as npc/Vine.
		if vm, err := content.LoadNPCTemplate(*contentDir, "Vine"); err != nil {
			logger.Warn("vine template not loaded (skill 98 cannot create Muro de Espinhos)", "err", err)
		} else {
			vineMob = vm
			logger.Info("vine template loaded")
		}
		if cq, err := content.LoadCastleQuests(filepath.Join(*contentDir, "Common", "Settings", "CastleQuest.txt")); err != nil {
			logger.Warn("castle quests not loaded (Castle/Zakum disabled)", "err", err)
		} else {
			castleQuests = cq
			logger.Info("castle quests loaded", "count", len(castleQuests))
		}
	}

	// Moderator mob/NPC template stat overlay (mob-template-editing-plan.md, the
	// equivalent-tool successor to the legacy EDITAPPMOB): the single switch is
	// -mob-stat-editing (W2PP_MOB_STAT_EDITING), off by default. Unlike
	// -npc-editing this applies to ANY npc/<template_name> file — monsters
	// included, the tool's primary rebalancing use case — not just the
	// DB-managed merchant subset, so it is deliberately independent of
	// -npc-editing (its own flag, its own dependency check). Fetched ONCE here,
	// synchronously, before any template gets loaded below: there is no
	// hot-reload for this feature, matching EDITAPPMOB's own restart-to-apply
	// behavior (it wrote the file; the server only read it at boot too).
	var mobStatOverrides map[string]mobstat.Override
	if *mobStatEditing {
		if dbConn == nil || *contentDir == "" {
			return fmt.Errorf("-mob-stat-editing requires both -dbserver (config source) and -content (mob templates)")
		}
		fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		overrides, ferr := dbclient.NewMobStatSource(dbConn).Fetch(fetchCtx)
		cancel()
		if ferr != nil {
			return fmt.Errorf("fetch mob template stat overrides: %w", ferr)
		}
		mobStatOverrides = overrides
		logger.Info("mob template stat overlay enabled (moderator editing)", "overrides", len(mobStatOverrides))
	}

	// Moderator item base stat overlay (0023_item_stats), the item-side sibling
	// of the block above and applied at the same moment and for the same kind of
	// reason: an item's effects feed the equip score model, which is recomputed
	// per character, so swapping them under a running server would leave two
	// players wearing the same item with different stats until each happened to
	// recompute. Item PRICE is the deliberate contrast — it rides the ~15s NPC
	// config poll and hot-reloads safely, because a price is only read at the
	// moment of a shop transaction.
	//
	// Applied here, over the maps loadContent just built, before the dispatcher
	// is constructed from them and before anything else can read them.
	if *itemStatEditing {
		if dbConn == nil || *contentDir == "" {
			return fmt.Errorf("-item-stat-editing requires both -dbserver (config source) and -content (the item catalog to override)")
		}
		fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		overrides, ferr := dbclient.NewItemStatSource(dbConn).Fetch(fetchCtx)
		cancel()
		if ferr != nil {
			return fmt.Errorf("fetch item stat overrides: %w", ferr)
		}
		itemstat.Apply(itemEffects, itemReqs, itemRanges, itemVolatiles, overrides)
		logger.Info("item base stat overlay enabled (moderator editing)", "overrides", len(overrides))
	}

	// Mount growth curves (0030_mount_growth_rate). No flag of its own: unlike the
	// stat overlays, an empty table changes nothing — every lineage simply keeps
	// the compiled default — so there is no unseeded-database hazard to guard
	// against, and a switch would only be one more thing to forget to turn on.
	var mountRates mountrate.Table
	if dbConn != nil {
		fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		rates, ferr := dbclient.NewMountRateSource(dbConn).Fetch(fetchCtx)
		cancel()
		if ferr != nil {
			// Warn rather than fail: a missing curve costs balance, not correctness,
			// and refusing to boot the game server over it would be the worse trade.
			logger.Warn("mount growth curves not loaded; every lineage uses the default", "err", ferr)
		} else {
			mountRates = rates
			logger.Info("mount growth curves loaded", "lineages", len(rates))
		}
	}

	// Mount absorption pairs (0035_mount_absorb), read beside the curves and for
	// the same reason: an empty table changes nothing — every lineage falls back
	// to the legacy's flat 25% on both axes — so there is no unseeded-database
	// hazard and no switch to forget to turn on.
	var mountAbsorb mountrate.AbsorbTable
	var mountConfigVersion int64
	if dbConn != nil {
		fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		absorb, ferr := dbclient.NewMountAbsorbSource(dbConn).Fetch(fetchCtx)
		cancel()
		if ferr != nil {
			// Warn rather than fail, like the curves: what is lost is balance, and
			// refusing to boot the game over it would be the worse trade.
			logger.Warn("mount absorption not loaded; every lineage uses the legacy share", "err", ferr)
		} else {
			mountAbsorb = absorb
			logger.Info("mount absorption loaded", "lineages", len(absorb))
		}
		// The version is read in the same breath as the tables, so what this
		// process reports is exactly what it is playing. Read separately it could
		// drift by a save landing between the two calls, and the panel would then
		// show a green light over a curve nobody is using.
		verCtx, verCancel := context.WithTimeout(ctx, 10*time.Second)
		ver, verr := dbclient.NewMountAbsorbSource(dbConn).Version(verCtx)
		verCancel()
		if verr != nil {
			logger.Warn("mount config version not read; the panel cannot tell if a saved curve is live", "err", verr)
		} else {
			mountConfigVersion = ver
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
			b, terr := content.LoadNPCTemplate(*contentDir, name)
			if terr != nil {
				return nil, terr
			}
			return mobstat.ApplyOverride(b, name, mobStatOverrides), nil
		}, logger)
		logger.Info("npc config overlay enabled (moderator editing)")
	}

	// Mesa de XP: the panel-managed reward tables (0030_xp_table). Read ONCE
	// here, for the same reason the mob and item overlays are: the tables shape a
	// grind, and swapping them under a running server would pay two players
	// different experience for the same mob depending on when their kill landed.
	//
	// No flag guards it, because an unedited table IS the legacy behaviour — a
	// server whose panel nobody touched runs exactly as before. A failed read is
	// logged loudly and does not stop the boot: an older database that has not
	// run the migration yet must still be able to start the game, and the legacy
	// tables are the honest fallback.
	var xpConfig level.Config
	if dbConn != nil {
		fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		cfg, ferr := dbclient.NewXPConfigSource(dbConn).Fetch(fetchCtx)
		cancel()
		switch {
		case ferr != nil:
			logger.Error("mesa de XP unreadable; running the legacy tables", "err", ferr)
		case len(cfg.Overrides) > 0:
			xpConfig = cfg
			logger.Info("mesa de XP loaded", "version", cfg.Version, "branches", len(cfg.Overrides))
		default:
			xpConfig = cfg
			logger.Info("mesa de XP empty; running the legacy tables", "version", cfg.Version)
		}
	}

	// Quest-trophy payouts (0036_quest_reward): the panel's rows over whatever
	// Common/Settings/QuestsRate.txt gave. Applied here, before the dispatcher is
	// built from questRates, so nothing reads the table while it is being
	// changed — the same moment the mob and item overlays land.
	//
	// A tier the panel never touched keeps the content file's numbers, so a
	// database with no rows behaves exactly as before. A failed read is logged
	// and does NOT stop the boot: the content file is a complete, working answer
	// on its own, and refusing to start over an unreachable override would turn
	// a balance edit into an outage.
	if dbConn != nil && questRates != nil {
		fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		cfg, ferr := dbclient.NewQuestRewardSource(dbConn).Fetch(fetchCtx)
		cancel()
		switch {
		case ferr != nil:
			logger.Error("quest rewards unreadable; using the content file", "err", ferr)
		case len(cfg.Tiers) == 0:
			logger.Info("quest rewards empty; using the content file", "version", cfg.Version)
		default:
			applied := 0
			for _, q := range cfg.Tiers {
				if questRates.SetTier(int(q.Tier), content.QuestTierRate{
					MortalExp: q.MortalExp, ArchExp: q.ArchExp, Coin: q.Coin,
					MortalMin: q.MortalMin, MortalMax: q.MortalMax,
					ArchMin: q.ArchMin, ArchMax: q.ArchMax,
				}) {
					applied++
					continue
				}
				// A row for a tier this build does not know about is skipped
				// rather than folded into a neighbour: a newer panel may have
				// written one, and guessing would pay the wrong quest.
				logger.Warn("quest reward for an unknown tier ignored", "tier", q.Tier)
			}
			logger.Info("quest rewards loaded", "version", cfg.Version, "tiers", applied)
		}
	}

	// Drop-bonus ladders (0037_drop_bonus): the panel's rows over the legacy
	// table. Read here, at boot, for the same reason as the quest payouts — it is
	// balance, not operations, and two players killing the same mob minutes apart
	// must not get items from different generations.
	//
	// It starts as the legacy ladder, so a database with no rows (or no dbServer
	// at all) rolls exactly what the original did. A failed read is logged and
	// does NOT stop the boot: refusing to start over an unreachable balance
	// override would turn a tuning edit into an outage.
	dropBonus := refine.TabelasPadrao()
	if dbConn != nil {
		fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		cfg, ferr := dbclient.NewDropBonusSource(dbConn).Fetch(fetchCtx)
		cancel()
		switch {
		case ferr != nil:
			logger.Error("drop bonus ladders unreadable; using the legacy table", "err", ferr)
		default:
			dropBonus.Ligado = cfg.Ligado
			applied := 0
			for _, f := range cfg.Faixas {
				if dropBonus.Aplica(f) {
					applied++
					continue
				}
				// A band this build does not know is skipped rather than folded
				// into a neighbour: a newer panel may have written it, and
				// guessing would roll the wrong ladder for every drop in it.
				logger.Warn("drop bonus band for an unknown distance ignored", "distancia", f.Distancia)
			}
			logger.Info("drop bonus ladders loaded",
				"version", cfg.Version, "ligado", cfg.Ligado, "faixas", applied)
			if !cfg.Ligado {
				// Said out loud because the symptom — every dropped item coming
				// out with empty effects — is exactly the bug this system was
				// built to fix, and somebody will hunt it for an hour otherwise.
				logger.Warn("drop bonus roll is OFF; dropped items keep the mob's own effects")
			}
		}
	}

	// Seed the world-event RNG from the wall clock so the weather sequence differs
	// between boots (handler.worldEventRNGSeed explains why the fixed seed is only
	// for tests). Zero means "use the fixed seed", so keep it out of range.
	eventSeed := uint32(time.Now().UnixNano())
	if eventSeed == 0 {
		eventSeed = 1
	}
	dispatch := handler.New(handler.Config{
		Log: logger, ClientVersion: int32(*clientVersion), BaseMobs: baseMobs, SummonMobs: summonMobs, VineMob: vineMob, ItemPrices: itemPrices, ItemNames: itemNames, ItemEffects: itemEffects, ItemReqs: itemReqs,
		ItemVolatiles: itemVolatiles, ItemDurations: itemDurations, MountRates: mountRates, MountAbsorb: mountAbsorb, ItemPos: itemPos, ItemUnique: itemUnique, ItemGrades: itemGrades, ItemExtra: itemExtra, Spells: spells, Heights: heights,
		SancRate:        sancRate,
		ExpEvents:       level.ExpEvents{DoubleMode: *doubleExp, NewbieEvent: *newbieEvent, KefraLive: *kefraLive},
		XPConfig:        xpConfig,
		CombineFamilies: combineFamilies,
		OdinCatalog:     odinCatalog,
		CombineCatalog:  odinCatalog,
		CompRate:        compRate,
		QuestRates:      questRates,
		DropBonus:       &dropBonus,
		Language:        language,
		NpcConfig:       npcConfig,
		WorldEvents:     worldEvents,
		DungeonGates:    dungeonGates,
		SpawnRates:      spawnRates,
		CastleQuests:    castleQuests,
		EventRNGSeed:    eventSeed,
		MaxNightmare:    *maxNightmare,
		AffectDuration: world.AffectDuration{
			ScalePct: *affectScalePct,
			MinTicks: handler.AffectTicksFromSeconds(*affectMinSeconds),
			MaxTicks: handler.AffectTicksFromMinutes(*affectMaxMinutes),
		},
	})
	w := world.New(world.Config{
		RejectChecksum: *rejectChecksum,
		MaxMsgPerSec:   *maxMsgPerSec,
		MsgBurst:       *msgBurst,
		IdleTimeout:    time.Duration(*idleTimeoutSec) * time.Second,
		// Long enough for the "server is restarting" frame to leave the socket,
		// short enough to leave the character saves their share of the SIGTERM →
		// SIGKILL window (Docker's default grace is 10s). Only production sets
		// this; tests leave it zero so they do not pay it on every world.
		ShutdownGrace: 2 * time.Second,
		StatusFile:    statusFile,
		ItemRanges:    itemRanges,
		LogSends:      *logSends,
		// Which items are worth an identity (0033_item_serial). The rule reads
		// the item catalog, which the dispatcher has and the world does not.
		Marcavel: dispatch.Marcavel,
	}, logger, persist, dispatch.Handle)
	// Mob-AI pulse: monsters acquire/chase/melee nearby players each tick (mobai.go).
	w.SetTickHandler(world.DefaultMobTick, dispatch.Tick)
	// Party teardown on disconnect: unlink the party bond and reap the summons
	// before the slot is freed (party.go SessionEnd).
	w.SetSessionEndHandler(dispatch.SessionEnd)
	// Nobody is in-play on a server that just started. Clearing the presence
	// marks here is what keeps an unclean shutdown from stranding characters
	// marked online — which would leave the staff panel refusing to edit them
	// forever. A non-zero count means the last shutdown was not clean.
	// The first block of item serials, taken before the world serves anybody.
	// Nothing is running yet, so this can block; without it the first saves of a
	// fresh boot would go out unmarked, which is a hole at exactly the moment
	// the server is writing every item it has for the first time. Not fatal: a
	// server with no database still runs, its items simply carry no identity.
	if err := w.PrimeSerials(ctx); err != nil {
		logger.Warn("could not reserve the first item serials; items stay unmarked until a later block lands", "err", err)
	}

	if n, err := persist.ClearAllPresence(ctx); err != nil {
		logger.Warn("could not clear character presence", "err", err)
	} else if n > 0 {
		logger.Info("cleared stale character presence", "characters", n)
	}
	// The newbie flag has two owners: the dispatcher's ExpEvents (the EXP bonus)
	// and the world (the sub-120 spawn HP handicap). Set here, BEFORE spawnNPCs
	// below, so the boot population is handicapped too — the portal config may
	// override it later via ApplyWorldEventConfigBoot.
	w.SetNewbieEvent(*newbieEvent)

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

	// Profiling endpoint, opt-in and on its own listener (see -pprof-addr).
	if *pprofAddr != "" {
		go servePprof(ctx, *pprofAddr, logger)
	}

	// Populate the world with NPCs/monsters from NPCGener.txt (before Serve starts
	// the loop, so spawning is single-threaded). Capped to fit the mob slots. When
	// the DB overlay is active, merchant blocks are skipped here (owned by
	// npc_definition) and applied from the config snapshot instead.
	if *contentDir != "" {
		spawnNPCs(w, *contentDir, npcConfig != nil, mobStatOverrides, logger)
		seedWorldItems(w, *contentDir, logger)
	}
	if npcConfig != nil {
		// Retries while the dbServer is still coming up; ctx aborts the wait on
		// SIGTERM instead of holding the process for the whole retry budget.
		if err := dispatch.ApplyNPCConfigBoot(ctx, w); err != nil {
			return err
		}
	}
	if worldEvents != nil {
		dispatch.ApplyWorldEventConfigBoot(w)
		dispatch.ApplyDungeonGatesBoot()
		dispatch.ApplySpawnRatesBoot()
	}
	// The individual respawn queue takes its delay from the same area dial the
	// minute timer does, so the desert's dozen blocks without a minute period
	// are not left running at 15s while everything around them slows down. It is
	// installed unconditionally: with no source the hook reads 100% and returns
	// exactly DefaultRespawnDelay.
	dispatch.InstallRespawnDelay(w)
	dispatch.ApplyGuildStateBoot(w)

	// Admin control API (kick, broadcast, who is online). Off unless an address
	// is given, and it refuses to start without a token rather than falling back
	// to an open endpoint: secure.ServerCreds returns insecure credentials when
	// TLS is unconfigured, so "wire it like the other services" would have
	// shipped an unauthenticated way to kick every player off the server.
	if *controlAddr != "" {
		ctl, cerr := control.NewServer(w, os.Getenv("W2PP_CONTROL_TOKEN"), logger, dispatch.Teleport,
			// What the panel cannot see from the database: whether this server
			// was booted to read the moderator overlays at all.
			control.Overlays{
				ItemStats: *itemStatEditing,
				MobStats:  *mobStatEditing,
				NPCs:      *npcEditing,
				// The version this process actually booted with, not whatever
				// the database holds now. That gap is the whole point: it is
				// what tells the panel a save is still waiting for a restart.
				XPConfigVersion:    xpConfig.Version,
				MountConfigVersion: mountConfigVersion,
			})
		if cerr != nil {
			return fmt.Errorf("-control-addr is set but the API cannot start: %w", cerr)
		}
		cln, lerr := net.Listen("tcp", *controlAddr)
		if lerr != nil {
			return fmt.Errorf("listen on control address: %w", lerr)
		}
		gsrv := grpc.NewServer(grpc.UnaryInterceptor(ctl.Interceptor()))
		gamev1.RegisterGameControlServiceServer(gsrv, ctl)
		go func() {
			<-ctx.Done()
			gsrv.GracefulStop()
		}()
		go func() {
			if serr := gsrv.Serve(cln); serr != nil {
				logger.Error("control api stopped", "err", serr)
			}
		}()
		logger.Info("control api listening", "addr", *controlAddr)
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
func spawnNPCs(w *world.World, dir string, skipMerchants bool, mobStatOverrides map[string]mobstat.Override, logger *slog.Logger) {
	gens, err := content.LoadNPCGenerators(filepath.Join(dir, "TMsrv", "run", "NPCGener.txt"))
	if err != nil {
		logger.Warn("NPC generators not loaded", "err", err)
		return
	}
	// A real monster whose kill reward is zero or beyond the legacy award gate
	// (10M, MobKilled.cpp:1284) means the content tree wasn't restamped with
	// cmd/exptool — players would gain nothing from those kills (issue #43).
	const maxSaneMobExp = 10_000_000
	// loadedTemplate keeps the raw (pre-override) Merchant classification
	// alongside the override-applied bytes: rawMerchant drives the
	// DB-managed-merchant skip below, so a moderator's stat override (which
	// is documented as informational-only for STRUCT_MOB.Merchant, see
	// mob-template-editing-frontend.md §6.3) can't turn a monster into a
	// "merchant" that vanishes from NPCGener.txt, or vice versa.
	type loadedTemplate struct {
		bytes       []byte
		rawMerchant uint8
	}
	templates := make(map[string]loadedTemplate)
	// stats records the on-disk layout of every referenced template so the boot
	// log says how much of the catalog needed widening from the legacy 756/920
	// forms, and how many blocks lost their template and why (issue #244 — the
	// load error used to be discarded here, leaving a rejected template visible
	// only as a name in the "NPC templates missing" sample).
	var stats npctemplate.ScanStats
	load := func(name string) loadedTemplate {
		if name == "" {
			return loadedTemplate{}
		}
		t, seen := templates[name]
		if !seen {
			b, res, terr := npctemplate.Load(dir, name)
			if terr != nil {
				logger.Warn("npc template load failed", "npc", name, "err", terr)
				stats.CountUnreadable()
			} else {
				stats.Count(res.Version)
				t.rawMerchant = protocol.ParseMobBasics(b).Merchant
				// Apply the moderator stat override (if any) BEFORE the exp sanity
				// check below, so a fix made via the web tool clears the warning too.
				t.bytes = mobstat.ApplyOverride(b, name, mobStatOverrides)
				if mb := protocol.ParseMobBasics(t.bytes); mb.Merchant == 0 && mb.Level >= 1 &&
					(mb.Exp <= 0 || mb.Exp > maxSaneMobExp) {
					logger.Warn("monster template has unbalanced Exp (run cmd/exptool)",
						"npc", name, "level", mb.Level, "exp", mb.Exp)
				}
			}
			templates[name] = t
		}
		return t
	}

	wgens := make([]*world.Generator, len(gens))
	dbOwned := make([]bool, len(gens))
	skipped := 0
	missingLeaderBlocks, missingFollowerBlocks := 0, 0
	missingLeaderNames := make(map[string]struct{})
	missingFollowerNames := make(map[string]struct{})
	for i, g := range gens {
		leader := load(g.Leader)
		if leader.bytes == nil {
			missingLeaderBlocks++
			if g.Leader != "" {
				missingLeaderNames[g.Leader] = struct{}{}
			}
			continue // block unusable without its Leader template
		}
		// When DB-managed NPC config is active, merchant blocks are owned by
		// npc_definition (materialized by the dispatcher overlay), so skip them here
		// to avoid double-spawning. Monsters / non-shop NPCs stay on NPCGener.txt.
		// Uses the RAW (pre-override) classification — a moderator's stat
		// override must not flip a block between "spawn from NPCGener.txt"
		// and "owned by npc_definition".
		if skipMerchants && leader.rawMerchant != 0 {
			skipped++
			dbOwned[i] = true
		}
		follower := load(g.Follower)
		if g.Follower != "" && follower.bytes == nil {
			missingFollowerBlocks++
			missingFollowerNames[g.Follower] = struct{}{}
		}
		wg := &world.Generator{
			DBManaged:      dbOwned[i],
			MinuteGenerate: g.MinuteGenerate,
			MinGroup:       g.MinGroup,
			MaxGroup:       g.MaxGroup,
			MaxNumMob:      g.MaxNumMob,
			RouteType:      uint8(g.RouteType),
			Formation:      g.Formation,
			SegX:           g.SegX,
			SegY:           g.SegY,
			LeaderTmpl:     leader.bytes,
			FollowerTmpl:   follower.bytes,
			FightAction:    g.FightAction,
			DieAction:      g.DieAction,
		}
		for s := 0; s < 5; s++ {
			wg.SegRange[s] = int16(g.SegRange[s])
			wg.SegWait[s] = int16(g.SegWait[s])
		}
		wgens[i] = wg
		if dbOwned[i] {
			wg.LeaderTmpl = nil
			wg.FollowerTmpl = nil
		}
	}
	w.RegisterGenerators(wgens)

	total := 0
	for i := range wgens {
		if wgens[i] != nil && !dbOwned[i] && !world.IsWaterDungeonGenerator(i) &&
			!world.IsEventOwnedGenerator(i) {
			total += len(w.GenerateMob(i))
		}
	}
	if missingLeaderBlocks > 0 || missingFollowerBlocks > 0 {
		logger.Warn("NPC templates missing",
			"leader_blocks_skipped", missingLeaderBlocks,
			"missing_leader_templates", len(missingLeaderNames),
			"missing_leader_sample", sampleNames(missingLeaderNames, 10),
			"follower_blocks_degraded", missingFollowerBlocks,
			"missing_follower_templates", len(missingFollowerNames),
			"missing_follower_sample", sampleNames(missingFollowerNames, 10))
	}
	logger.Info("npc template catalog", "layouts", stats)
	logger.Info("NPCs spawned", "generators", len(gens), "mobs", total, "templates", len(templates),
		"merchant_blocks_skipped", skipped)
}

// seedWorldItems spawns the static world objects (gates/doors) from
// TMsrv/run/InitItem.csv before Serve starts the loop, so id assignment is
// single-threaded and follows file order (matching the legacy CreateItem sequence
// the client expects). Missing/unreadable is a warning, not fatal — like the maps.
func seedWorldItems(w *world.World, dir string, logger *slog.Logger) {
	items, err := content.LoadInitItems(filepath.Join(dir, "TMsrv", "run", "InitItem.csv"))
	if err != nil {
		logger.Warn("world items not loaded", "err", err)
		return
	}
	seeded := 0
	for _, it := range items {
		// Gates seed open (parity with CreateItem); locking arrives with the deferred
		// event/timer systems. SeedWorldItem returns -1 only when the id table is full.
		if id := w.SeedWorldItem(world.Item{Index: it.Index}, it.PosX, it.PosY, world.StateOpen); id >= 0 {
			seeded++
		}
	}
	logger.Info("world items seeded", "count", seeded)
}

func sampleNames(names map[string]struct{}, limit int) []string {
	if len(names) == 0 || limit <= 0 {
		return nil
	}
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	if len(out) > limit {
		return out[:limit]
	}
	return out
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

// servePprof exposes net/http/pprof on its own listener, mirroring the status
// server's shape (own address, closed with the context).
//
// It is opt-in because these endpoints dump the heap and let any caller start a
// CPU profile — they belong on loopback or a private network, never beside the
// game port. It exists at all because the process otherwise ships with no
// profiling surface whatsoever, which makes "is it leaking goroutines?"
// unanswerable on a running server, and a leak is what turns a silent bug into
// a rising bill on usage-priced hosting.
func servePprof(ctx context.Context, addr string, logger *slog.Logger) {
	srv := &http.Server{Addr: addr, Handler: http.DefaultServeMux}
	go func() { <-ctx.Done(); _ = srv.Close() }()
	logger.Info("pprof listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Warn("pprof server stopped", "err", err)
	}
}

// loadContent loads and validates the Release/ content tree (Fase 5 loaders).
// The rates and catalogs are required (a broken mount is a hard error); the maps
// are large and optional (a warning when absent). It logs what was loaded so the
// operator can confirm the mount is correct.
func loadContent(dir string, logger *slog.Logger) (*loadedContent, error) {
	comp, err := content.LoadCompRate(filepath.Join(dir, "Common", "Settings", "CompRate.txt"))
	if err != nil {
		return nil, err
	}
	sanc, err := content.LoadSancRate(filepath.Join(dir, "Common", "Settings", "SancRate.txt"))
	if err != nil {
		return nil, err
	}
	quests, err := content.LoadQuestRates(filepath.Join(dir, "Common", "Settings", "QuestsRate.txt"))
	if err != nil {
		return nil, err
	}
	items, err := content.LoadItemList(filepath.Join(dir, "Common", "ItemList.csv"))
	if err != nil {
		return nil, err
	}
	skills, err := content.LoadSkillData(filepath.Join(dir, "Common", "SkillData.csv"))
	if err != nil {
		return nil, err
	}
	// The string table is optional but load-bearing for anything the server says:
	// without it only the notices with a compiled fallback reach the player, and
	// every other refusal is silent (handler/notice.go). Warn loudly rather than
	// fail, because a server with no text still runs.
	language, err := content.LoadLanguage(filepath.Join(dir, "TMsrv", "run", "Language.txt"))
	if err != nil {
		logger.Warn("language table not loaded: most notifications will be silent", "err", err)
	}

	// The Ori/Lac rates are worth logging outright: SancRate.txt only overrides the
	// indices it lists, so a broken mount silently leaves the compiled defaults and
	// the operator would otherwise not notice.
	logger.Info("content loaded",
		"comprate_families", comp.Families(),
		"sancrate_ori", sancRow(sanc, 0), "sancrate_lac", sancRow(sanc, 1),
		"items", items.Len(), "skills", skills.Len(), "language_lines", language.Len())

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
	return &loadedContent{items: items, comp: comp, sanc: sanc, quests: quests, skills: skills, heights: heights, language: language}, nil
}

// loadedContent is what a mounted Release/ tree yields. It is a struct rather
// than a return list only because the list had grown past readability.
type loadedContent struct {
	items    *content.ItemList
	comp     *content.CompRate
	sanc     *content.SancRate
	quests   *content.QuestRates
	skills   *content.SkillData
	heights  *content.Grid
	language *content.Language
}

// sancRow renders one anvil's rate row for the boot log.
func sancRow(s *content.SancRate, anvil int) []int {
	row := make([]int, 0, 12)
	for i := 0; i < 12; i++ {
		row = append(row, s.Rate(anvil, i))
	}
	return row
}
