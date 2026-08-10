package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Classe A-E item range (_MSG_UseItem.cpp:4989): 4016-4020 are the plain
// tiers, 4021-4025 the "(P)" variant — both contiguous 5-item ranges mapping
// to the same tier 1..5 (A..E), matched against the target's EF_ITEMLEVEL.
const classeALo = 4016

// classeTier maps a Classe item's sIndex to its tier 1..5 (A..E)
// (_MSG_UseItem.cpp:4989). Only meaningful for items already classified as
// volClasses by the catalog; callers don't need to range-check idx first —
// same as the legacy, which computes this arithmetically with no bounds check.
func classeTier(idx int16) int {
	return int(idx-classeALo)%5 + 1
}

// useClasseItem is the Classe A-E path of _MSG_UseItem (Vol==190,
// _MSG_UseItem.cpp:4958-5012): dragging a Classe item onto a piece of gear
// sitting in the INVENTORY at the matching EF_ITEMLEVEL grade rerolls its sanc
// (capped at +6) and its two bonus stats (SetItemBonus2). The Classe item is
// consumed on a successful reroll; a gate failure resyncs the dragged slot.
//
// The target must NOT be equipped (issue #228). The legacy gate reads
// `if (!m->DestType || m->DestPos >= 60)` (:4970) — it rejects
// DestType==ITEM_PLACE_EQUIP(0) and bounds DestPos to the carry range. Two
// things confirm that polarity is intentional rather than the typo an earlier
// port took it for: the bound is 60 (maxUnlockedCarry), not 16 (MaxEquip) as in
// the genuinely equip-only gates at :3779/:3901/:3979/:4057; and SetItemBonus2
// is followed by SendItem alone, with no score recompute (:4997-5000), which is
// only safe because the rerolled item is not worn.
func (d *Dispatcher) useClasseItem(w *world.World, s *world.Session, e *world.Entity, body protocol.MsgUseItemBody, src int) {
	dst := d.itemSlot(w, s, e, int(body.DestType), int(body.DestPos))
	if dst == nil || dst.Empty() {
		return // no such target slot — the legacy just logs and drops the message
	}

	// The legacy's companion `m->DestPos >= 60` bound needs no explicit port:
	// itemSlot routes carry through carrySlotAccessible, which already caps at
	// activeCarryLimit(e) <= maxUnlockedCarry == 60 and yields nil above it —
	// caught by the dst == nil return above.
	if int(body.DestType) == world.ItemPlaceEquip {
		// _NN_Only_To_Equips is the string Source sends here (:4972). It reads
		// backwards for this gate — one of the legacy's many misfiled message
		// ids — but the client shows it, so it is kept for parity.
		d.notify(w, s, NoticeOnlyToEquips)
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}

	mobType := d.itemAbility(*dst, efMobType)
	if itemSanc(*dst) >= 10 || (mobType != 0 && mobType != 2) {
		// Source sends no notice on this gate — just resyncs the dragged slot.
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}

	if d.itemAbility(*dst, efItemLevel) != classeTier(e.Carry[src].Index) {
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}

	// Deliberate deviation from SetItemBonus2 (Server.cpp:2719-2861), which has
	// no branch for an nPos outside helm/chest/legs/glove/boot and so falls
	// through to consume the Classe item for no effect — the known legacy bug
	// recorded in "Source/Lista de bugs.txt:6". Refuse and resync instead of
	// destroying the item.
	if !refine.ClasseBonus(dst, d.itemPos[int(dst.Index)], func(n int) int { return w.Rand().Intn(n) }, d.itemAbility) {
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}

	d.sendSlot(w, s, int(body.DestType), int(body.DestPos), *dst)
	d.log.Info("classe item bonus reroll", "conn", s.Conn, "item", dst.Index, "effects", dst.Effects)
	consumeOneItem(&e.Carry[src])
	// The Classe slot is deliberately NOT re-sent on success, mirroring
	// refineSucceed: the client already removed the item it dragged.
}
