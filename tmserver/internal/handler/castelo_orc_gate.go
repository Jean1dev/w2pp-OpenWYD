package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// The Portão Orc Sul — InitItem 462, EF_KEYID 5 like the Chave Portão Orc Sul,
// standing in the arch at (2487,2129) — is the Castelo Orc run's door. It is the
// only world object this server draws: the gates the legacy seeds from
// InitItem.csv were never sent to the client, and turning them all on at once
// would shut doors across the map that nobody asked for. This one stays locked,
// opens when a run starts (the leader's key on it, or handed to the Xamã) and
// locks again when the run ends.
//
// The wire is the legacy's: MSG_CreateItem when the gate enters a player's view
// (GridMulticast, SendFunc.cpp:862) and when it relocks (the minute timer,
// ProcessSecMinTimer.cpp:2696), MSG_UpdateItem when it opens
// (_MSG_UpdateItem.cpp:102), MSG_DecayItem when it leaves the view
// (SendFunc.cpp:845). What stops a player at a closed gate is the client, which
// raises the ground under it; the server has no height check on player movement.
// UNVERIFIED in game: that the 7662 client draws and blocks this gate from these
// packets alone.
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
		// Every gate is seeded open (CreateItem, Server.cpp:8075); the run's door
		// starts shut.
		if g := w.GroundItem(d.casteloOrcGateID); g != nil && !d.casteloOrc.active {
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

// setCasteloOrcGate opens or locks the gate and tells everyone in view: an open
// goes out as MSG_UpdateItem to a client that already draws the gate, anything
// else as a fresh MSG_CreateItem.
func (d *Dispatcher) setCasteloOrcGate(w *world.World, state int16) {
	g := d.casteloOrcGate(w)
	if g == nil || g.State == state {
		return
	}
	g.State = state
	key := gateSeenKey(g.ID)
	w.ForEachInViewAt(g.X, g.Y, -1, func(s *world.Session, _ *world.Entity) {
		if state == world.StateOpen && w.Seen(s, key) {
			body := (&protocol.MsgUpdateItemBody{ItemID: int32(world.GroundItemIDOffset + g.ID), State: world.StateOpen}).Encode()
			w.SendTo(s, protocol.Header{Type: protocol.MsgUpdateItem, ID: protocol.IDScene}, body)
			return
		}
		w.MarkSeen(s, key)
		w.SendTo(s, protocol.Header{Type: protocol.MsgCreateItem, ID: protocol.IDScene}, d.gateCreateBody(g))
	})
	d.log.Info("castelo orc gate", "id", g.ID, "state", state)
}

// casteloOrcGateRequest is a click on the Portão Orc Sul: the Xamã's opening,
// answered in the message panel.
func (d *Dispatcher) casteloOrcGateRequest(w *world.World, s *world.Session, e *world.Entity) {
	d.casteloOrcTryOpen(w, s, e, func(text string) { sendClientMessage(w, s, text) })
}
