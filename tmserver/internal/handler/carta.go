package handler

import (
	"github.com/jeanluca/w2pp-openwyd/internal/dungeon"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// The Sala Secreta, opened by a Carta de Duelo (EF_VOLATILE 20). The name is
// misleading: it is not a 1v1 duel but a four-room party instance
// (_MSG_UseItem.cpp:2873-2977 for the ticket, MobKilled.cpp:1890-2100 for the
// room progression, ProcessSecMinTimer.cpp:1597-1636 for the clock).
//
// Shape of a run: the party leader burns a card while standing on the tower
// altar; the whole party is dropped into sala 1 with sixty seconds on the clock,
// and every mob of the dungeon is spawned up front — the salas are not populated
// one at a time. Clearing a sala's blocks advances early; running out of time
// advances anyway, pushing whoever is still inside into the next room. Sala 4 is
// the boss; when it falls the clock is cut to three seconds and the next tick
// empties the room and ends the run.
//
// The original keeps exactly one run server-wide in two globals (CartaTime and
// CartaSala, Server.cpp:531-532) and that is reproduced here rather than made
// per-party: the room rectangles, the generator blocks and the eviction sweep
// are all shared, so two concurrent runs would evict each other. The "someone is
// already inside" gate is what enforces it.
const (
	volCartaDuelo = 20

	// cartaSalaSeconds is the per-sala clock; cartaBossSeconds is the short fuse
	// set when the boss dies, so the run ends on the next tick rather than
	// instantly (MobKilled.cpp:1975).
	cartaSalaSeconds uint8 = 60
	cartaBossSeconds uint8 = 3

	// cartaLastSala is the boss room.
	cartaLastSala uint8 = 4
)

// cartaAltarBox is the tile the ticket must be used on. The legacy compares the
// quarter-grid — TargetX/4 == 261 && TargetY/4 == 422 — which is this 4x4 block.
var cartaAltarBox = areaBox{1044, 1688, 1047, 1691}

// cartaEnter is CartaPos[4] (Server.cpp:432): where each sala drops the party.
var cartaEnter = [4][2]int16{
	{786, 3688}, // sala 1
	{843, 3688}, // sala 2
	{843, 3632}, // sala 3
	{786, 3640}, // sala 4 — the boss
}

// cartaSalaBox is the rectangle swept when a sala ends: everyone inside is moved
// to the NEXT sala's entry point. Only salas 1-3 have one — sala 4 ends the run
// through cartaFinalBox instead.
var cartaSalaBox = [3]areaBox{
	{778, 3652, 832, 3698},
	{836, 3652, 890, 3698},
	{834, 3595, 889, 3645},
}

// cartaFinalBox is ClearArea(776, 3595, 834, 3648): the boss room, emptied when
// the run ends. clearArea recalls whoever is standing in it.
var cartaFinalBox = areaBox{776, 3595, 834, 3648}

// cartaOccupiedBox is GetUserInArea(774, 3593, 892, 3702): the whole dungeon,
// scanned to refuse a second party while one is inside.
var cartaOccupiedBox = areaBox{774, 3593, 892, 3702}

// cartaWipeBox is the mob sweep the ticket runs before spawning, so a previous
// run's leftovers cannot be inherited (_MSG_UseItem.cpp:2921-2932, which walks
// pMobGrid over 767..896 x 3582..3711).
var cartaWipeBox = areaBox{767, 3582, 896, 3711}

// cartaBases are the generator block bases of the three card sets
// (Basedef.h:374-420). Within a set the layout is fixed: base+0..base+7 are the
// four salas' mobs, two blocks each, and base+8 is the boss. base+9 exists in the
// header and is counted when checking whether sala 4 is clear, but the ticket
// never spawns it — reproduced rather than tidied.
var cartaBases = [3]int{2395, 2405, 2415}

// cartaBaseForCard maps a ticket item to its generator base. The legacy switches
// on sIndex-3171 with the A card as the default branch, which is how item 1731
// reaches it (_MSG_UseItem.cpp:2937-2967).
func cartaBaseForCard(index int16) (int, bool) {
	switch index {
	case 3172:
		return cartaBases[0], true // Carta de Duelo (N)
	case 3171:
		return cartaBases[1], true // Carta de Duelo (M)
	case 1731:
		return cartaBases[2], true // Carta de Duelo (A)
	}
	return 0, false
}

// cartaGateForCard is the staff door each card answers to: one per tier, like
// the Pesadelo and the Água, so the M and A runs can be shut while N stays open.
func cartaGateForCard(index int16) dungeon.Gate {
	switch index {
	case 3171:
		return dungeon.CartaM
	case 1731:
		return dungeon.CartaA
	}
	return dungeon.CartaN
}

// cartaBlockSala reports which sala a generator block belongs to, and the blocks
// that must ALL be down for that sala to count as cleared.
func cartaBlockSala(idx int) (sala uint8, blocks []int, ok bool) {
	for _, base := range cartaBases {
		off := idx - base
		if off < 0 || off > 9 {
			continue
		}
		switch {
		case off <= 1:
			return 1, []int{base, base + 1}, true
		case off <= 3:
			return 2, []int{base + 2, base + 3}, true
		case off <= 5:
			return 3, []int{base + 4, base + 5}, true
		default:
			return 4, []int{base + 6, base + 7, base + 8, base + 9}, true
		}
	}
	return 0, nil, false
}

// cartaRunning reports whether a run is in progress.
func (d *Dispatcher) cartaRunning() bool { return d.events.cartaSala != 0 }

// useCartaDuelo is the ticket (_MSG_UseItem.cpp:2873-2977).
func (d *Dispatcher) useCartaDuelo(w *world.World, s *world.Session, e *world.Entity, src int) {
	card := e.Carry[src].Index
	base, ok := cartaBaseForCard(card)
	if !ok {
		d.rejectUnimplementedConsumable(w, s, e, src)
		return
	}
	// Logged on every attempt, before the gates: notify() is a bare numeric code
	// the client does not render, so without this a refused card is
	// indistinguishable from a dead handler.
	d.log.Info("carta attempt", "account", s.AccountName, "card", card,
		"x", e.X, "y", e.Y, "leader", e.Leader, "sala", d.events.cartaSala)

	// The staff door first: telling somebody off the altar to go stand on it,
	// when the dungeon is shut anyway, only wastes their walk.
	if gate := cartaGateForCard(card); !d.gateOpen(gate) {
		d.log.Info("carta refused: gate closed by staff", "account", s.AccountName, "gate", gate.Name())
		sendClientMessage(w, s, "A "+gate.Name()+" está fechada pela administração.")
		d.refuseCarta(w, s, e, src, NoticeCantUseHere)
		return
	}
	if !cartaAltarBox.contains(e.X, e.Y) {
		d.log.Info("carta refused: not on the altar",
			"account", s.AccountName, "x", e.X, "y", e.Y)
		d.refuseCarta(w, s, e, src, NoticeCantUseHere)
		return
	}
	// Members carry their leader's conn; only a leader (or a soloist) opens a run.
	if e.Leader != 0 {
		d.log.Info("carta refused: not the party leader",
			"account", s.AccountName, "leader", e.Leader)
		d.refuseCarta(w, s, e, src, NoticePartyLeaderOnly)
		return
	}
	if occupant, busy := d.cartaBusy(w); busy {
		d.log.Info("carta refused: someone is inside",
			"account", s.AccountName, "occupant", occupant)
		d.refuseCarta(w, s, e, src, NoticeSomeoneOnQuest)
		return
	}

	consumeOneItem(&e.Carry[src])
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])

	d.events.cartaTime, d.events.cartaSala = cartaSalaSeconds, 1
	d.wipeCartaMobs(w)
	spawned := d.populateCarta(w, base)
	d.enterCarta(w, s, e)
	d.log.Info("carta started", "account", s.AccountName, "card", card,
		"base", base, "mobs", spawned)
}

// refuseCarta answers a gate and re-syncs the slot, so the card visibly stays in
// the bag rather than appearing to vanish.
func (d *Dispatcher) refuseCarta(w *world.World, s *world.Session, e *world.Entity, src int, n Notice) {
	d.notify(w, s, n)
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
}

// cartaBusy reports whether any player stands inside the dungeon, naming the
// first one found (GetUserInArea, _MSG_UseItem.cpp:2895).
func (d *Dispatcher) cartaBusy(w *world.World) (string, bool) {
	name, busy := "", false
	w.ForEachPlaying(-1, func(_ *world.Session, e *world.Entity) {
		if busy || !cartaOccupiedBox.contains(e.X, e.Y) {
			return
		}
		name, busy = e.Name, true
	})
	return name, busy
}

// wipeCartaMobs removes every mob standing in the dungeon before a run spawns its
// own. Despawn type 3 is used deliberately: type 1 is a combat death and would
// push each mob onto the 15s respawn queue, refilling the rooms behind the party.
func (d *Dispatcher) wipeCartaMobs(w *world.World) {
	var ids []int
	w.ForEachMob(func(id int, e *world.Entity) {
		if cartaWipeBox.contains(e.X, e.Y) {
			ids = append(ids, id)
		}
	})
	for _, id := range ids {
		w.DespawnMob(id, 3)
	}
}

// populateCarta spawns the whole dungeon up front: two groups from each of the
// eight sala blocks, then one boss (_MSG_UseItem.cpp:2937-2967). ClearGenerator
// first so a previous run's population counter cannot leave the blocks saturated.
func (d *Dispatcher) populateCarta(w *world.World, base int) int {
	n := 0
	for off := 0; off <= 7; off++ {
		block := base + off
		w.ClearGenerator(block)
		n += len(w.GenerateMob(block))
		n += len(w.GenerateMob(block))
	}
	boss := base + 8
	w.ClearGenerator(boss)
	n += len(w.GenerateMob(boss))
	return n
}

// enterCarta drops the leader and every party member into sala 1 with the clock
// showing.
func (d *Dispatcher) enterCarta(w *world.World, s *world.Session, e *world.Entity) {
	dest := cartaEnter[0]
	d.doTeleport(w, s, dest[0], dest[1])
	d.sendCartaCountdown(w, s)
	for i := 0; i < world.MaxParty; i++ {
		member := e.PartyList[i]
		if member <= 0 || member == e.ID {
			continue
		}
		ms := w.Session(member)
		if ms == nil || ms.Mode != world.UserPlay {
			continue
		}
		d.doTeleport(w, ms, dest[0], dest[1])
		d.sendCartaCountdown(w, ms)
	}
}

// sendCartaCountdown pushes the sala clock to one client. The legacy doubles the
// value on the wire (MSG_StartTime, Parm = CartaTime*2), the same encoding the
// water rooms and the castle quest use.
func (d *Dispatcher) sendCartaCountdown(w *world.World, s *world.Session) {
	body := protocol.EncodeStandardParm(int32(d.events.cartaTime) * 2)
	w.SendTo(s, protocol.Header{Type: protocol.MsgStartTime, ID: protocol.IDScene}, body)
}

// broadcastCartaCountdown refreshes the clock for everyone inside.
//
// DIVERGENCE: the legacy sends this with MapaMulticast(6, 28) — every client on
// that map block, whether or not it is in the dungeon. This port has no
// map-scoped multicast, and the dungeon rectangle is the set that actually cares,
// so the frame goes to whoever stands in it.
func (d *Dispatcher) broadcastCartaCountdown(w *world.World) {
	w.ForEachPlaying(-1, func(s *world.Session, e *world.Entity) {
		if cartaOccupiedBox.contains(e.X, e.Y) {
			d.sendCartaCountdown(w, s)
		}
	})
}

// tickCarta is the per-second clock (ProcessSecMinTimer.cpp:1597-1636). Reaching
// 1 advances the run; the legacy never lets it reach 0 except by ending.
func (d *Dispatcher) tickCarta(w *world.World) {
	switch t := d.events.cartaTime; {
	case t > 1:
		d.events.cartaTime--
	case t == 1:
		d.advanceCarta(w)
	}
}

// advanceCarta moves the run on one sala, or ends it after the boss. Shared by
// the clock and the room-cleared hook, which differ only in what triggers them.
func (d *Dispatcher) advanceCarta(w *world.World) {
	sala := d.events.cartaSala
	if sala >= cartaLastSala {
		d.clearArea(w, cartaFinalBox)
		d.events.cartaTime, d.events.cartaSala = 0, 0
		d.log.Info("carta finished")
		return
	}
	d.events.cartaTime = cartaSalaSeconds
	// Push whoever is still in the sala that just ended into the next one. A
	// party that already walked ahead is left where it is, as in the original.
	if sala >= 1 && int(sala) <= len(cartaSalaBox) {
		d.evictCartaSala(w, sala)
	}
	d.events.cartaSala++
	d.broadcastCartaCountdown(w)
	d.log.Info("carta advanced", "sala", d.events.cartaSala)
}

// evictCartaSala is ClearAreaTeleport for one sala: everyone inside is moved to
// the next sala's entry point.
func (d *Dispatcher) evictCartaSala(w *world.World, sala uint8) {
	box := cartaSalaBox[sala-1]
	dest := cartaEnter[sala] // sala is 1-based, so this is the NEXT room
	var moved []*world.Session
	w.ForEachPlaying(-1, func(s *world.Session, e *world.Entity) {
		if box.contains(e.X, e.Y) {
			moved = append(moved, s)
		}
	})
	// Teleport outside the walk: doTeleport mutates position and view.
	for _, s := range moved {
		d.doTeleport(w, s, dest[0], dest[1])
	}
}

// cartaRoomCleared is the mob-death hook (MobKilled.cpp:1890-2100). It runs
// BEFORE DespawnMob, so the dying mob is still counted: a sala is clear when its
// blocks sum to exactly one.
//
// Killing the boss does not end the run here — it only cuts the clock to three
// seconds, and the next tick ends it (MobKilled.cpp:1975). That is the legacy's
// own sequencing and it matters: it gives the drop and the death animation a
// moment before the room is emptied.
func (d *Dispatcher) cartaRoomCleared(w *world.World, mob *world.Entity) {
	if mob == nil || !d.cartaRunning() {
		return
	}
	sala, blocks, ok := cartaBlockSala(int(mob.GenIndex))
	if !ok {
		return
	}
	// Only the sala the run is actually in may advance it. The legacy reads the
	// block population alone, which a leftover mob from an earlier run could
	// satisfy — this port refuses to be advanced by a room nobody is in.
	if sala != d.events.cartaSala {
		return
	}
	live := 0
	for _, block := range blocks {
		if g := w.GeneratorAt(block); g != nil {
			live += g.CurrentNumMob
		}
	}
	if live != 1 {
		return
	}
	if sala >= cartaLastSala {
		d.events.cartaTime, d.events.cartaSala = cartaBossSeconds, cartaLastSala
		d.log.Info("carta boss down; ending the run")
		return
	}
	d.log.Info("carta sala cleared", "sala", sala)
	d.advanceCarta(w)
}
