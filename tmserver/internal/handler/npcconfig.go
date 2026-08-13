package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/npccfg"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Moderator-edited NPC overlay (npc-editing-plan.md). The web writes cold config
// to Postgres; the tmServer reads it (via dbServer) and MATERIALIZES the live
// entities inside the single-owner loop — the web never touches live state.
//
// DB-managed NPCs are the MERCHANT subset: at boot the NPCGener.txt merchant
// blocks are skipped (main.spawnNPCs) and these definitions own them instead, so
// there is no double-spawn. Monsters and non-shop NPCs keep coming from
// NPCGener.txt untouched.

const (
	// npcPollPeriod is how many mob-AI ticks (1s each) between config version
	// polls. Moderator edits are rare, so ~15s keeps the DB load negligible while
	// still feeling live.
	npcPollPeriod = 15
	// npcFetchTimeout bounds the off-loop version/snapshot gRPC calls.
	npcFetchTimeout = 5 * time.Second
)

// Boot retry budget. Vars, not consts, so tests can shrink them.
var (
	// npcBootRetryBudget bounds how long boot waits for the dbServer to become
	// reachable and finish reconciling the content catalog. Both services are
	// deployed together and the dbServer seeds the catalog BEFORE it listens, so
	// the tmServer legitimately wins the race; without this wait the deploy
	// crash-loops (production incident, 2026-08-13).
	npcBootRetryBudget = 90 * time.Second
	// npcBootRetryInitial and npcBootRetryMax bracket the exponential backoff.
	npcBootRetryInitial = 1 * time.Second
	npcBootRetryMax     = 5 * time.Second
)

// ApplyNPCConfigBoot fetches the definition snapshot synchronously and applies it
// once at boot (no reveal — no players are connected yet). It runs before the
// loop starts, so it is single-threaded like spawnNPCs. Loading is fail-closed:
// starting with an incomplete content catalog would silently remove legacy NPCs.
// Transient failures are retried until npcBootRetryBudget expires — the dbServer
// may still be unreachable, or still mid-seed and answering with a short catalog.
// Structural failures (duplicate or non-DB-managed generator index) never heal on
// their own, so they fail immediately instead of burning the budget.
func (d *Dispatcher) ApplyNPCConfigBoot(ctx context.Context, w *world.World) error {
	if d.npcSource == nil {
		return nil
	}
	deadline := time.Now().Add(npcBootRetryBudget)
	delay := npcBootRetryInitial
	for attempt := 1; ; attempt++ {
		retriable, err := d.loadNPCConfigBoot(ctx, w)
		if err == nil {
			return nil
		}
		if !retriable {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("gave up after %d attempts in %s: %w", attempt, npcBootRetryBudget, err)
		}
		d.log.Warn("npc config boot load failed, retrying", "attempt", attempt, "retry_in", delay, "err", err)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("npc config boot load: %w", ctx.Err())
		case <-timer.C:
		}
		delay = min(delay*2, npcBootRetryMax)
	}
}

// loadNPCConfigBoot runs one boot-load attempt. retriable reports whether the
// failure is the kind that clears once the dbServer finishes coming up.
func (d *Dispatcher) loadNPCConfigBoot(ctx context.Context, w *world.World) (retriable bool, err error) {
	fetchCtx, cancel := context.WithTimeout(ctx, npcFetchTimeout)
	defer cancel()
	snap, err := d.npcSource.Snapshot(fetchCtx)
	if err != nil {
		return true, fmt.Errorf("npc config boot load: %w", err)
	}
	expected := w.DBManagedGeneratorCount()
	seen := make(map[int]struct{}, expected)
	for _, def := range snap.Defs {
		if def.Origin != "content" || def.GeneratorIndex < 0 {
			continue
		}
		if _, exists := seen[def.GeneratorIndex]; exists {
			return false, fmt.Errorf("npc config duplicate generator index %d", def.GeneratorIndex)
		}
		g := w.GeneratorAt(def.GeneratorIndex)
		if g == nil || !g.DBManaged {
			return false, fmt.Errorf("npc config content generator index %d is not DB-managed", def.GeneratorIndex)
		}
		seen[def.GeneratorIndex] = struct{}{}
	}
	if len(seen) != expected {
		// The dbServer seeds the whole catalog in one transaction, so a short
		// count means we read before it committed — not that content is broken.
		return true, fmt.Errorf("npc config incomplete: content generators=%d expected=%d", len(seen), expected)
	}
	d.applyNPCConfig(w, snap, false)
	d.log.Info("npc config applied at boot", "version", snap.Version, "npcs", len(d.managedNPCs))
	return false, nil
}

// pollNPCConfig checks the config version off the loop each npcPollPeriod ticks
// and, when it changed, reloads the snapshot and applies it inside the loop. Only
// one poll is in flight at a time (npcPolling). Called from the mob-AI Tick.
func (d *Dispatcher) pollNPCConfig(w *world.World) {
	if d.npcSource == nil || d.npcPolling {
		return
	}
	d.npcPollTick++
	if d.npcPollTick%npcPollPeriod != 0 {
		return
	}
	known := d.npcVersion // captured in-loop; the off-loop goroutine never reads loop state
	d.npcPolling = true
	w.GoDetached(func() func(*world.World) {
		ctx, cancel := context.WithTimeout(context.Background(), npcFetchTimeout)
		defer cancel()
		version, err := d.npcSource.Version(ctx)
		if err != nil {
			return func(*world.World) { d.npcPolling = false; d.log.Warn("npc config version poll failed", "err", err) }
		}
		if version == known {
			return func(*world.World) { d.npcPolling = false }
		}
		snap, err := d.npcSource.Snapshot(ctx)
		if err != nil {
			return func(*world.World) { d.npcPolling = false; d.log.Warn("npc config reload failed", "err", err) }
		}
		return func(w *world.World) {
			d.applyNPCConfig(w, snap, true)
			d.npcPolling = false
			d.log.Info("npc config reloaded", "version", snap.Version, "npcs", len(d.managedNPCs))
		}
	})
}

// applyNPCConfig reconciles the live world with a config snapshot, inside the
// loop. It fully re-materializes the managed set: despawn every currently managed
// NPC, then spawn each enabled definition. Merchant count is small and reloads
// are rare (moderator-driven), so a full reconcile is simplest and obviously
// correct. It also refreshes the global item-price overlay. reveal announces new
// spawns to in-view players (true on hot-reload, false at boot).
func (d *Dispatcher) applyNPCConfig(w *world.World, snap npccfg.Snapshot, reveal bool) {
	d.rebuildItemPrices(snap.PriceOverrides)

	for slug, id := range d.managedNPCs {
		w.DespawnMob(id, 0) // removeType 0 = out-of-view removal, never queues a respawn
		delete(d.managedNPCs, slug)
	}

	for _, def := range snap.Defs {
		if def.Origin == "content" && def.GeneratorIndex >= 0 {
			g := w.GeneratorAt(def.GeneratorIndex)
			if g == nil || !g.DBManaged {
				d.log.Warn("npc content generator index is not DB-managed", "slug", def.Slug, "index", def.GeneratorIndex)
				continue
			}
			w.ClearGenerator(def.GeneratorIndex)
			if !def.Enabled {
				g.LeaderTmpl = nil
				g.FollowerTmpl = nil
				continue
			}
			if def.Template == nil {
				d.log.Warn("npc definition has no template — skipped", "slug", def.Slug)
				continue
			}
			g.MinuteGenerate, g.MinGroup, g.MaxGroup, g.MaxNumMob = def.MinuteGenerate, def.MinGroup, def.MaxGroup, def.MaxNumMob
			g.RouteType, g.Formation = def.RouteType, def.Formation
			g.SegX, g.SegY, g.SegRange, g.SegWait = def.SegX, def.SegY, def.SegRange, def.SegWait
			g.FightAction, g.DieAction = def.FightAction, def.DieAction
			g.LeaderTmpl = npcTemplateWithDisplayName(def.Template, def.DisplayName)
			g.FollowerTmpl = def.FollowerTemplate
			ids := w.GenerateMob(def.GeneratorIndex)
			if len(ids) == 0 {
				d.log.Warn("npc content generator produced no entities", "slug", def.Slug, "index", def.GeneratorIndex)
				continue
			}
			if def.Merchant != 0 {
				applyShop(w.Entity(ids[0]), def.Shop)
			}
			d.managedNPCs[def.Slug] = ids[0]
			if reveal {
				d.revealSpawned(w, ids)
			}
			continue
		}
		if !def.Enabled {
			continue
		}
		if def.Template == nil {
			// Template failed to resolve (missing/corrupt file) — logged in the
			// dbclient source too, but warn here so the skip is visible alongside
			// the other skip branches (bad position, no free slot).
			d.log.Warn("npc definition has no template — skipped", "slug", def.Slug)
			continue
		}
		if def.X <= 0 || def.Y <= 0 {
			d.log.Warn("npc definition has no valid position — skipped", "slug", def.Slug, "x", def.X, "y", def.Y)
			continue
		}
		template := npcTemplateWithDisplayName(def.Template, def.DisplayName)
		id := w.SpawnMobAt(world.MobSpawn{
			Template:  template,
			X:         def.X,
			Y:         def.Y,
			RouteType: def.RouteType,
			GenIndex:  -1, // not owned by an NPCGener block (no CurrentNumMob accounting)
		})
		if id < 0 {
			d.log.Warn("npc spawn failed (no free slot)", "slug", def.Slug)
			continue
		}
		if def.Merchant != 0 {
			applyShop(w.Entity(id), def.Shop)
		}
		d.managedNPCs[def.Slug] = id
		if reveal {
			d.revealSpawned(w, []int{id})
		}
	}
	d.npcVersion = snap.Version
}

// npcTemplateWithDisplayName overlays the moderator display name onto a copy of
// the raw STRUCT_MOB. Templates are shared by the dbclient cache, so mutating the
// input would leak one NPC's translated name into every NPC using that template.
func npcTemplateWithDisplayName(template []byte, displayName string) []byte {
	if strings.TrimSpace(displayName) == "" {
		return template
	}
	out := append([]byte(nil), template...)
	for i := 0; i < 16 && i < len(out); i++ {
		out[i] = 0
	}
	copy(out[0:min(16, len(out))], displayName)
	return out
}

// applyShop overwrites a merchant entity's shop stock (its Carry[]) with the
// moderator-defined slots. Slot is the MSG_ShopList index (0..26); it is mapped
// to the real Carry index via protocol.ShopSlot (3 tabs of 9: Carry[0..8],
// [27..35], [54..62]) — the same mapping reqShopList reads back, so an item in
// tab 2/3 (slot >= 9) actually reaches the client. The shop is authoritative:
// slots not listed are cleared, so emptying the shop in the web sells nothing.
func applyShop(e *world.Entity, shop []npccfg.ShopItem) {
	if e == nil {
		return
	}
	for i := range e.Carry {
		e.Carry[i] = world.Item{}
	}
	for _, it := range shop {
		if it.Slot < 0 || it.Slot >= maxShopSlots {
			continue
		}
		e.Carry[protocol.ShopSlot(it.Slot)] = world.Item{
			Index:   int16(it.Index),
			Effects: shopEffects(it),
		}
	}
}

const amountEffect = 61

func shopEffects(it npccfg.ShopItem) [3]world.Effect {
	eff := [3]world.Effect{
		{Effect: it.Eff[0][0], Value: it.Eff[0][1]},
		{Effect: it.Eff[1][0], Value: it.Eff[1][1]},
		{Effect: it.Eff[2][0], Value: it.Eff[2][1]},
	}
	if it.Quantity <= 1 {
		return eff
	}
	for i := range eff {
		if eff[i].Effect == 0 {
			eff[i] = world.Effect{Effect: amountEffect, Value: it.Quantity}
			return eff
		}
	}
	return eff
}

// maxShopSlots is the number of MSG_ShopList slots (3 tabs of 9); a shop item's
// Slot is a display index in [0, maxShopSlots).
const maxShopSlots = 27

// rebuildItemPrices resets the effective price map to the content catalog, then
// applies the moderator's global overrides on top. Loop-only, so handlers reading
// d.itemPrices never race it.
func (d *Dispatcher) rebuildItemPrices(overrides map[int]int32) {
	if len(overrides) == 0 && d.baseItemPrices == nil {
		return
	}
	d.itemPrices = cloneInt32Map(d.baseItemPrices)
	if d.itemPrices == nil {
		d.itemPrices = make(map[int]int32, len(overrides))
	}
	for idx, price := range overrides {
		d.itemPrices[idx] = price
	}
}

// cloneInt32Map returns a shallow copy of m (nil for a nil input).
func cloneInt32Map(m map[int]int32) map[int]int32 {
	if m == nil {
		return nil
	}
	out := make(map[int]int32, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
