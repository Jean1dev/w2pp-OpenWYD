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
// Castle gates delegate to the Castle/Zakum runtime. Periodic generic re-locking
// and GM lock commands remain deferred; a successfully opened gate stays open
// until restart or an event reset.
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
	// requirement. itemAbility includes catalog base effects because static gates
	// usually carry EF_KEYID in ItemList rather than in their instance slots.
	castleLevel := -1
	if g.State == world.StateLocked || body.State == world.StateLocked {
		if gateKey := d.itemAbility(g.Item, efKeyID); gateKey != 0 {
			slot := d.carryKeySlot(e, gateKey, -1)
			if gateKey >= 11 && gateKey <= 14 {
				level, _, _ := d.events.castle.State()
				slot = d.carryKeySlot(e, gateKey, level)
			}
			if slot < 0 {
				// sIndex 773 opens silently without a key message (legacy quirk).
				if g.Item.Index != 773 {
					d.notify(w, s, NoticeNoKey)
				}
				return
			}
			if gateKey == 10 {
				castleLevel = itemQuestID(e.Carry[slot])
				if castleLevel < 0 || castleLevel >= len(d.castleQuests) {
					return
				}
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
	if castleLevel >= 0 {
		d.openCastleQuest(w, s, e, castleLevel)
	}
}

// carryKeySlot returns the first carry slot holding a key whose EF_KEYID matches
// key, or -1. questLevel < 0 accepts any such key; questLevel >= 0 additionally
// requires the item's EF_QUEST stamp to name that castle quest level, which is
// what keeps a level-2 castle key from opening the level-3 gate. Loop-only
// (reads Entity.Carry directly).
func (d *Dispatcher) carryKeySlot(e *world.Entity, key, questLevel int) int {
	for i := 0; i < activeCarryLimit(e); i++ {
		it := e.Carry[i]
		if it.Empty() || d.itemAbility(it, efKeyID) != key {
			continue
		}
		if questLevel < 0 || itemQuestID(it) == questLevel {
			return i
		}
	}
	return -1
}

func itemQuestID(it world.Item) int {
	for _, effect := range it.Effects {
		if effect.Effect == efQuest {
			return int(effect.Value)
		}
	}
	return -1
}
