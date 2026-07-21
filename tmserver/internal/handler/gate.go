package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// updateItem handles _MSG_UpdateItem (0x0374, _MSG_UpdateItem.cpp): a gate/door
// open request. ItemID is the world-item id (ground id + GroundItemIDOffset). If
// the gate is locked and carries an EF_KEYID, the player must hold a matching key
// (consumed on use); otherwise the gate just opens. On an actual state change the
// server re-broadcasts the request to everyone in view so their client reflects the
// open gate (the legacy GridMulticast).
//
// Deferred (documented, not built): the castle-gate path (CCastleZakum::
// OpenCastleGate) needs castle/RvR state that isn't modeled, and the periodic
// re-lock (ProcessSecMinTimer) / GM lock commands (imple.cpp) that would put a gate
// back into STATE_LOCKED. Seeded gates therefore start (and stay) open until those
// land, so the key path is only reachable for a gate a future system has locked.
// The wire↔client gate-id correspondence is UNVERIFIED without a capture; ids are
// assigned in InitItem.csv order to match the legacy CreateItem sequence.
func (d *Dispatcher) updateItem(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		w.AddCrackError(s, 1, 16)
		return
	}
	var body protocol.MsgUpdateItemBody
	if err := body.Decode(payload); err != nil {
		return
	}
	if body.State < 0 || body.State > 5 {
		w.AddCrackError(s, 50, 50)
		return
	}
	id := int(body.ItemID) - world.GroundItemIDOffset
	if id <= 0 || id >= world.MaxItem {
		w.AddCrackError(s, 50, 50)
		return
	}
	g := w.GroundItem(id)
	if g == nil || !g.Static {
		return // not a gate
	}

	// Key gate: only when the gate is (or is being set) locked AND it carries a key
	// requirement. A seeded gate has no baked effects, so gateKey is 0 and this is
	// skipped — parity with BASE_GetItemAbility over an all-zero STRUCT_ITEM.
	if g.State == world.StateLocked || body.State == world.StateLocked {
		if gateKey := itemKeyID(g.Item); gateKey != 0 {
			slot := carryKeySlot(e, gateKey)
			if slot < 0 {
				// sIndex 773 opens silently without a key message (legacy quirk).
				if g.Item.Index != 773 {
					d.notify(w, s, NoticeNoKey)
				}
				return
			}
			e.Carry[slot] = world.Item{} // consume the key
			d.sendSlot(w, s, world.ItemPlaceCarry, slot, e.Carry[slot])
		}
	}

	// Always open. Multicast only when the state actually changed (the legacy
	// UpdateItem returns FALSE for a no-op), echoing the client's own packet.
	if g.State == world.StateOpen {
		return
	}
	g.State = world.StateOpen
	w.SendTo(s, protocol.Header{Type: protocol.MsgUpdateItem, ID: protocol.IDScene}, payload)
	w.ForEachInViewAt(g.X, g.Y, s.Conn, func(os *world.Session, _ *world.Entity) {
		w.SendTo(os, protocol.Header{Type: protocol.MsgUpdateItem, ID: protocol.IDScene}, payload)
	})
	d.log.Info("gate opened", "conn", s.Conn, "gate", id, "x", g.X, "y", g.Y)
}

// itemKeyID returns the item's EF_KEYID value (0 when absent) — the Go analog of
// BASE_GetItemAbility(item, EF_KEYID) over the instance effects. A key/gate carries
// a single EF_KEYID, so the summed itemInstanceAbility equals that value.
func itemKeyID(it world.Item) int {
	return itemInstanceAbility(it, efKeyID)
}

// carryKeySlot returns the first carry slot holding an item whose EF_KEYID matches
// key, or -1. Loop-only (reads Entity.Carry directly).
func carryKeySlot(e *world.Entity, key int) int {
	for i := 0; i < activeCarryLimit(e); i++ {
		if !e.Carry[i].Empty() && itemKeyID(e.Carry[i]) == key {
			return i
		}
	}
	return -1
}
