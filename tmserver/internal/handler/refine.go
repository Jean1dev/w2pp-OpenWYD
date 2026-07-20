package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// The refine dusts, classified (like every _MSG_UseItem action) by the source
// item's EF_VOLATILE rather than by its index. OriLacto = Vol-4
// (_MSG_UseItem.cpp:849).
const (
	volDustOri = 4 // Poeira de Oriharucon (412)
	volDustLac = 5 // Poeira de Lactolerium (413)
)

const (
	// itemLacto100 (Lactolerium_100) carries EF_VOLATILE 5 like an ordinary Lac
	// dust but forces the Âmago row, a flat 100% (_MSG_UseItem.cpp:850).
	itemLacto100 = 4141
	// item769 is capped at +9 by name (_MSG_UseItem.cpp:802).
	item769 = 769

	// Ori stops at +6, Lac at +9; the dust path never goes past +9 except for the
	// one +10→+11 step.
	oriMaxSanc  = 6
	lacMaxSanc  = 9
	gemSancLvl  = 10 // the +10 special case: goes to +11 and preserves the gem
	sancHardCap = 11 // at +11 and above the dust path refuses outright

	// refineRollModulo is the dust roll: a plain rand()%100
	// (_MSG_UseItem.cpp:851). NOT combine.Roll's rand()%115 with the -15 fold —
	// the two paths use different distributions.
	refineRollModulo = 100
	// pityRollModulo: rand()%4 <= 2, a 3-in-4 chance to accrue pity on a failure
	// (_MSG_UseItem.cpp:943).
	pityRollModulo = 4
	pityRollMax    = 2
)

// Mount eggs (2300..2329) hatch into their mount at sIndex+30 when refined.
const (
	eggLo       = 2300
	eggHi       = 2330
	eggHatchAdd = 30
	// eggIncuDelayBase is the cooldown stamped on a successful egg refine:
	// rand()%4 + 6 (_MSG_UseItem.cpp:899).
	eggIncuDelayBase = 6
	// incuLevelExempt: the incubation gate only applies below level 999
	// (_MSG_UseItem.cpp:842).
	incuLevelExempt = 999

	// A freshly hatched mount reinterprets its effect slots as mount data
	// (BASE_GetItemAbility, Basedef.cpp:1758-1770): stEffect[0].sValue = MOUNTHP,
	// [1].cEffect = MOUNTSANC, [1].cValue = MOUNTLIFE, [2].cEffect = MOUNTFEED,
	// [2].cValue = MOUNTKILL.
	hatchMountHP   = 20000
	hatchMountSanc = 1
	hatchMountFeed = 30
	hatchMountKill = 1
	hatchLifeBase  = 10 // MOUNTLIFE = rand()%20 + 10
	hatchLifeSpan  = 20
)

// Effect ids the refine path gates on (ItemEffect.h).
const (
	efNoSanc    = 126 // EF_NOSANC: this item can never be refined
	efIncubate  = 78  // EF_INCUBATE: the refine level an egg hatches at
	efIncuDelay = 84  // EF_INCUDELAY: an egg's cooldown between refine attempts
)

func isEgg(it world.Item) bool { return it.Index >= eggLo && it.Index < eggHi }

// Tintura items (3397-3406) don't refine — a dust converts them straight into
// the matching Feijão Mágico (3407-3416 = tintura + 10, mirroring the color
// order useMagicBean's color math already relies on, item.go:1072). This must
// run before refine.Bootstrap would plant a bogus EF_SANC pair into a fresh
// tintura's zeroed effect slots (issue #130).
const (
	tinturaLo = 3397 // Tintura_Azul
	tinturaHi = 3406 // Tintura_Azul_Claro
)

func isTintura(it world.Item) bool { return it.Index >= tinturaLo && it.Index <= tinturaHi }

// itemInstanceAbility sums ONLY an item's instance effects, which is what
// BASE_GetBonusItemAbility does (Basedef.cpp:2048) — unlike itemAbility (=
// BASE_GetItemAbility) it never reads the catalog.
//
// Only valid for the effects BASE_GetBonusItemAbility excludes from its
// `value *= sanc+10` refine scaling (Basedef.cpp:2084). EF_INCUBATE and
// EF_INCUDELAY, the two this path needs, are both on that exclusion list.
//
// The catalog blindness is load-bearing, not an oversight: an egg's EF_INCUBATE
// lives in ItemList.csv, so this returns 0 for a fresh egg and the hatch gate
// `level >= incubate` passes on the first successful refine.
func itemInstanceAbility(it world.Item, eff uint8) int {
	var total int
	for _, ef := range it.Effects {
		if ef.Effect == eff {
			total += int(ef.Value)
		}
	}
	return total
}

// putShort writes a raw 16-bit value across an effect pair's two bytes, the way
// the legacy assigns STRUCT_ITEM's `stEffect[i].sValue` union member on
// little-endian x86.
func putShort(ef *world.Effect, v uint16) {
	ef.Effect = uint8(v)
	ef.Value = uint8(v >> 8)
}

// refineItem is the dust-refine path of _MSG_UseItem (_MSG_UseItem.cpp:153-208
// and :802-976): dragging a Poeira de Ori (Vol 4) or de Lac (Vol 5) onto an item
// raises its refine level. The dust is consumed either way; on failure the item
// itself is never destroyed nor downgraded, it only accrues "pity" that raises
// the next attempt's odds.
//
// Only the standard-equipment path is ported (issue #103). The legacy's four
// other dust branches — sealed items (:224), celestial/HC (:385), arch stones
// (:514) and +10..+14 earrings (:716) — all roll rand()%115 with the -15 fold and
// destroy or reset the item on failure; they are deferred to their own issues.
func (d *Dispatcher) refineItem(w *world.World, s *world.Session, e *world.Entity, body protocol.MsgUseItemBody, src, vol int) {
	dst := d.itemSlot(w, s, e, int(body.DestType), int(body.DestPos))
	if dst == nil || dst.Empty() {
		return // no such target slot — the legacy just logs and drops the message
	}

	if isTintura(*dst) {
		d.refineTintura(w, s, e, dst, body, src)
		return
	}

	// A dust only refines equipment: a target that is itself a consumable (which
	// includes dragging a dust onto another dust) is refused (:178).
	//
	// The legacy reads EF_VOLATILE with BASE_GetItemAbility (catalog + instance);
	// this reads the catalog map useItem already classifies the source dust with.
	// EF_VOLATILE is a catalog property — no minted item carries it as an instance
	// effect — so the two agree in practice.
	if d.itemVolatiles[int(dst.Index)] != 0 {
		d.refineReject(w, s, e, src, NoticeOnlyToEquips)
		return
	}
	if d.itemAbility(*dst, efNoSanc) != 0 {
		d.refineReject(w, s, e, src, NoticeCantRefineMore)
		return
	}

	level := refine.Level(*dst)
	// Ori cannot touch an item already at +6 or above (:203).
	if vol == volDustOri && level >= oriMaxSanc {
		d.refineReject(w, s, e, src, NoticeCantRefineMore)
		return
	}
	// +9 is the dust path's wall. +10 is allowed through as the one special case
	// (it becomes +11); +11 and up are refused (:802, where the legacy compares
	// against the REF_11 sentinel).
	if level == lacMaxSanc || level >= sancHardCap || (level >= lacMaxSanc && dst.Index == item769) {
		d.refineReject(w, s, e, src, NoticeCantRefineMore)
		return
	}

	// Read the pity BEFORE the bootstrap: planting the pair zeroes the slot, and
	// the legacy reads it first too (:809 before :814), so a failed refine keeps
	// accruing onto the previous count.
	pity := refine.Pity(*dst)

	// A never-refined item has nowhere to store a level yet. When all three effect
	// slots hold real effects the legacy refuses to refine the item at all (:814).
	if level == 0 && !refine.Bootstrap(dst) {
		d.refineReject(w, s, e, src, NoticeCantRefineMore)
		return
	}

	// An egg that failed a previous refine has to sit out its incubation timer,
	// which RegenMob ticks down while the egg is equipped (:842, regenPlayers).
	//
	// UNVERIFIED: the legacy reads BaseScore.Level here; the Entity only carries
	// CurrentScore.Level. They differ only via EF_LEVEL gear, never near 999.
	if isEgg(*dst) && e.Level < incuLevelExempt && itemInstanceAbility(*dst, efIncuDelay) > 0 {
		d.refineReject(w, s, e, src, NoticeIncuWaitMore)
		return
	}

	anvil := vol - volDustOri // 0 = Ori, 1 = Lac — also indexes g_pSancGrade
	// The rate anvil can differ from the grade anvil: Lactolerium_100 forces the
	// Âmago row for a guaranteed 100%, but still steps the level as a Lac (:850).
	rateAnvil := anvil
	if e.Carry[src].Index == itemLacto100 {
		rateAnvil = refine.Amago
	}

	// Order is parity-critical: the rate is computed BEFORE the roll, and the
	// grade read after it (:850-853). One rand() on the hot path.
	rate := refine.SuccessRate(d.sancRate, *dst, rateAnvil)
	roll := w.Rand().Intn(refineRollModulo)
	grade := d.itemAbility(*dst, efItemLevel)

	t := refineTarget{item: dst, place: int(body.DestType), slot: int(body.DestPos)}
	// `<=` is inclusive, and a 0 rate always fails despite roll 0 passing it (:855).
	if roll <= rate && rate != 0 {
		d.refineSucceed(w, s, e, t, src, vol, anvil, level, grade)
		return
	}
	d.refineFail(w, s, e, t, src, anvil, level, pity)
}

// refineTintura converts a tintura into its matching Feijão Mágico. This is a
// deterministic conversion, not a success-rate roll — tinturas carry no
// EF_SANC to roll against, and no legacy source for this branch exists (see
// game-rules.md §3.6.1); it's a migration design decision, not a port.
func (d *Dispatcher) refineTintura(w *world.World, s *world.Session, e *world.Entity, dst *world.Item, body protocol.MsgUseItemBody, src int) {
	dst.Index = int16(magicBeanBase + (int(dst.Index) - tinturaLo))
	d.notify(w, s, NoticeRefineSuccess)
	d.sendSlot(w, s, int(body.DestType), int(body.DestPos), *dst)
	consumeOneItem(&e.Carry[src])
}

// refineTarget is the item being refined, together with where it lives so the
// result can be pushed back to the client.
type refineTarget struct {
	item        *world.Item
	place, slot int
}

// refineSucceed applies a won roll (_MSG_UseItem.cpp:856-927).
func (d *Dispatcher) refineSucceed(w *world.World, s *world.Session, e *world.Entity, t refineTarget, src, vol, anvil, level, grade int) {
	dst := t.item

	if level == gemSancLvl {
		// +10 → +11, carrying the gem index across (:857).
		refine.Set(dst, gemSancLvl+1, refine.Gem(*dst))
	} else {
		if grade >= 1 && grade <= 5 {
			level += refine.Step(anvil, grade)
			if level >= oriMaxSanc && vol == volDustOri {
				level = oriMaxSanc
			} else if level > lacMaxSanc {
				level = lacMaxSanc
			}
		} else {
			// No usable grade → a plain +1. Note the legacy skips BOTH caps on this
			// branch (:874), so it is the one way past them; reproduced.
			level++
		}
		refine.Set(dst, level, 0) // a success clears the pity
	}

	d.refreshScore(e)
	d.sendScore(w, s, e)
	d.notify(w, s, NoticeRefineSuccess)

	if isEgg(*dst) {
		d.hatchEgg(w, s, t, level)
	}

	d.sendSlot(w, s, t.place, t.slot, *dst)
	// SendEmotion(conn, 14, 3) — cosmetic, and no emotion packet exists in this
	// port yet (_MSG_UseItem.cpp:920).
	consumeOneItem(&e.Carry[src])
	// The dust slot is deliberately NOT re-sent: the client already removed the
	// item it dragged, and the legacy only echoes the source back on the refusal
	// paths, to undo that removal.
}

// refineFail applies a lost roll (_MSG_UseItem.cpp:929-974). The item survives
// untouched — only the dust is spent and the pity counter grows.
func (d *Dispatcher) refineFail(w *world.World, s *world.Session, e *world.Entity, t refineTarget, src, anvil, level, pity int) {
	d.notify(w, s, NoticeFailToRefine)
	consumeOneItem(&e.Carry[src])

	if w.Rand().Intn(pityRollModulo) <= pityRollMax {
		// Lac always accrues pity; Ori only up to +5 (:943-949).
		if anvil == refine.Lac || (level <= 5 && anvil == refine.Ori) {
			pity++
		}
	}

	dst := t.item
	if isEgg(*dst) {
		// A failed egg refine starts the incubation cooldown (:951).
		dst.Effects[2] = world.Effect{Effect: efIncuDelay, Value: uint8(w.Rand().Intn(pityRollModulo))}
	}
	if level != gemSancLvl {
		refine.Set(dst, level, pity)
	}
	d.sendSlot(w, s, t.place, t.slot, *dst)
	// SendEmotion(conn, 15|20, 0) — cosmetic, not ported (:967).
}

// hatchEgg is the mount-egg branch inlined in the refine success
// (_MSG_UseItem.cpp:890-917). It stamps a fresh incubation cooldown and, once the
// egg's level reaches its EF_INCUBATE threshold, turns it into the mount at
// sIndex+30.
func (d *Dispatcher) hatchEgg(w *world.World, s *world.Session, t refineTarget, level int) {
	dst := t.item
	dst.Effects[2] = world.Effect{
		Effect: efIncuDelay,
		Value:  uint8(w.Rand().Intn(pityRollModulo) + eggIncuDelayBase),
	}

	// Reads the INSTANCE EF_INCUBATE, which a fresh egg does not have (its
	// threshold is a catalog effect) — so this is 0 and the egg hatches on its
	// first successful refine. That is the legacy's behaviour, not a shortcut:
	// see itemInstanceAbility.
	if level < itemInstanceAbility(*dst, efIncubate) {
		return
	}

	dst.Index += eggHatchAdd
	putShort(&dst.Effects[0], hatchMountHP)
	dst.Effects[1] = world.Effect{
		Effect: hatchMountSanc,
		Value:  uint8(w.Rand().Intn(hatchLifeSpan) + hatchLifeBase),
	}
	dst.Effects[2] = world.Effect{Effect: hatchMountFeed, Value: hatchMountKill}
	d.notify(w, s, NoticeIncubated)
	// MountProcess(conn, 0) is a no-op: with a NULL mount its IsEqual stays 1 and
	// it returns immediately (Server.cpp:4639-4643). Nothing to port.
	//
	// The legacy sends the slot here AND again after the egg block (:916, :919);
	// the duplicate is harmless and refineSucceed does the second send.
	d.sendSlot(w, s, t.place, t.slot, *dst)
}

// refineReject refuses a refine and re-sends the DUST slot, so the client puts
// the item it dragged back where it was (_MSG_UseItem.cpp:805).
func (d *Dispatcher) refineReject(w *world.World, s *world.Session, e *world.Entity, src int, n Notice) {
	d.notify(w, s, n)
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
}

// sendSlot pushes one item slot to the client (SendItem).
func (d *Dispatcher) sendSlot(w *world.World, s *world.Session, place, slot int, it world.Item) {
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(place, slot, itemToSel(it)))
}
