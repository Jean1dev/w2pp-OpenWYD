package handler

import (
	"context"
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

// ApplyNPCConfigBoot fetches the definition snapshot synchronously and applies it
// once at boot (no reveal — no players are connected yet). It runs before the
// loop starts, so it is single-threaded like spawnNPCs. A fetch error is logged
// and left for the periodic poll to retry once the loop is running.
func (d *Dispatcher) ApplyNPCConfigBoot(w *world.World) {
	if d.npcSource == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), npcFetchTimeout)
	defer cancel()
	snap, err := d.npcSource.Snapshot(ctx)
	if err != nil {
		d.log.Warn("npc config boot load failed (will retry via poll)", "err", err)
		return
	}
	d.applyNPCConfig(w, snap, false)
	d.log.Info("npc config applied at boot", "version", snap.Version, "npcs", len(d.managedNPCs))
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
		id := w.SpawnMobAt(world.MobSpawn{
			Template:  def.Template,
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
			Index: int16(it.Index),
			Effects: [3]world.Effect{
				{Effect: it.Eff[0][0], Value: it.Eff[0][1]},
				{Effect: it.Eff[1][0], Value: it.Eff[1][1]},
				{Effect: it.Eff[2][0], Value: it.Eff[2][1]},
			},
		}
	}
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
