package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// The Portão Orc Sul — InitItem 462, EF_KEYID 5 like the Chave do Rei Orc,
// standing in the arch at (2487,2129) — is the Castelo Orc run's door. It is the
// only world object this server draws: the gates the legacy seeds from
// InitItem.csv were never sent to the client, and turning them all on at once
// would shut doors across the map that nobody asked for.
//
// This one never opens. An open door would let a second party in behind the
// first (team rule, 11/09/2026): the leader's key on it — or handed to the Xamã —
// starts a run and takes the party through, and the gate stays locked. Its key
// requirement is the legacy's (EF_KEYID 5, the Chave do Rei Orc's); the
// generic key path in gate.go never sees it.
//
// The wire is the legacy's: MSG_CreateItem when the gate enters a player's view
// (GridMulticast, SendFunc.cpp:862), MSG_DecayItem when it leaves
// (SendFunc.cpp:845). Confirmed in game on 11/09/2026 that the 7662 client draws
// it from these; that a closed one also stops a player walking into it is the
// client's doing (it raises the ground under the gate) and is still UNVERIFIED —
// the server has no height check on player movement.
const (
	itemPortaoOrcSul = 462
	// gateHeightClosed is GetCreateItem's Height for a closed gate: -204 stored in
	// an unsigned char (GetFunc.cpp:1339).
	gateHeightClosed = 0x34
)

var casteloOrcGatePos = [2]int16{2487, 2129}

// gateSeenKey keys a world object in the session's seen set, which otherwise
// holds entity ids (all >= 0).
func gateSeenKey(id int) int { return -(world.GroundItemIDOffset + id) }

// casteloOrcGate returns the Portão Orc Sul, looking it up on first use; nil when
// InitItem.csv did not seed it (tests, a broken mount).
func (d *Dispatcher) casteloOrcGate(w *world.World) *world.GroundItem {
	if d.casteloOrcGateID == 0 {
		d.casteloOrcGateID = -1
		w.ForEachStaticItem(func(g *world.GroundItem) {
			if d.casteloOrcGateID < 0 && g.Item.Index == itemPortaoOrcSul &&
				g.X == casteloOrcGatePos[0] && g.Y == casteloOrcGatePos[1] {
				d.casteloOrcGateID = g.ID
			}
		})
		// Every gate is seeded open (CreateItem, Server.cpp:8075); this one is
		// shut for good.
		if g := w.GroundItem(d.casteloOrcGateID); g != nil {
			g.State = world.StateLocked
		}
	}
	if d.casteloOrcGateID < 0 {
		return nil
	}
	return w.GroundItem(d.casteloOrcGateID)
}

// gateCreateBody is GetCreateItem for the gate.
func (d *Dispatcher) gateCreateBody(g *world.GroundItem) []byte {
	height := uint8(gateHeightClosed)
	if g.State == world.StateOpen {
		height = 0
		if d.heights != nil {
			height = d.heights.At(int(g.X), int(g.Y))
		}
	}
	it := protocol.WireItem{Index: g.Item.Index}
	for i, ef := range g.Item.Effects {
		it.Effects[i] = protocol.WireEffect{Effect: ef.Effect, Value: ef.Value}
	}
	return protocol.EncodeCreateItemBody(protocol.CreateItemData{
		GridX: uint16(g.X), GridY: uint16(g.Y), ItemID: uint16(world.GroundItemIDOffset + g.ID),
		Item: it, Rotate: uint8(g.Rotate), State: uint8(g.State), Height: height,
	})
}

// syncCasteloOrcGate draws the gate for s when (x,y) brings it into view and
// takes it away when it falls out: the item half of GridMulticast.
func (d *Dispatcher) syncCasteloOrcGate(w *world.World, s *world.Session, x, y int16) {
	g := d.casteloOrcGate(w)
	if g == nil || s == nil {
		return
	}
	key := gateSeenKey(g.ID)
	if chebyshev(x, y, g.X, g.Y) <= world.ViewRange {
		if w.MarkSeen(s, key) {
			w.SendTo(s, protocol.Header{Type: protocol.MsgCreateItem, ID: protocol.IDScene}, d.gateCreateBody(g))
		}
		return
	}
	if w.Seen(s, key) {
		w.UnmarkSeen(s, key)
		w.SendTo(s, protocol.Header{Type: protocol.MsgDecayItem, ID: protocol.IDScene},
			protocol.EncodeDecayItemBody(uint16(world.GroundItemIDOffset+g.ID)))
	}
}

// casteloOrcGateRequest is a click on the Portão Orc Sul: the Xamã's opening,
// answered in the message panel. The gate itself does not move.
func (d *Dispatcher) casteloOrcGateRequest(w *world.World, s *world.Session, e *world.Entity) {
	d.casteloOrcTryOpen(w, s, e, func(text string) { sendClientMessage(w, s, text) })
}
