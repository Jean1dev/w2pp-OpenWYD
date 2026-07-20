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
// _MSG_UseItem.cpp:4958-5012): dragging a Classe item onto an equipped piece
// of gear at the matching EF_ITEMLEVEL grade rerolls its sanc (capped at +6)
// and its two bonus stats (SetItemBonus2). The Classe item is always consumed
// on a successful match; a gate failure resyncs the dragged slot instead.
//
// Deviates from Source in exactly one place: the DestType gate below is
// `!= Equip` here, not the literal `!m->DestType` (which rejects DestType==0
// == ITEM_PLACE_EQUIP, i.e. every legitimate target — the feature could never
// fire as literally written). Every analogous gate elsewhere in this same
// file (_MSG_UseItem.cpp:3779,3901,3979,4057) uses the correct polarity;
// reproduced as a corrected typo, not a literal port.
func (d *Dispatcher) useClasseItem(w *world.World, s *world.Session, e *world.Entity, body protocol.MsgUseItemBody, src int) {
	dst := d.itemSlot(w, s, e, int(body.DestType), int(body.DestPos))
	if dst == nil || dst.Empty() {
		return // no such target slot — the legacy just logs and drops the message
	}

	if int(body.DestType) != world.ItemPlaceEquip {
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

	refine.ClasseBonus(dst, d.itemPos[int(dst.Index)], func(n int) int { return w.Rand().Intn(n) }, d.itemAbility)

	d.sendSlot(w, s, int(body.DestType), int(body.DestPos), *dst)
	d.log.Info("classe item bonus reroll", "conn", s.Conn, "item", dst.Index, "effects", dst.Effects)
	consumeOneItem(&e.Carry[src])
	// The Classe slot is deliberately NOT re-sent on success, mirroring
	// refineSucceed: the client already removed the item it dragged.
}
