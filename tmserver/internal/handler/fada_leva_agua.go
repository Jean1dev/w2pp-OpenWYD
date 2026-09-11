package handler

import "github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"

// The fairy that walks a party through the Água (a server rule decided
// 2026-09-12; the legacy has nothing like it).
//
// Clearing a room hands the leader the next scroll and stops there: somebody has
// to open the bag and use it, once per room, eight times a run. With a Fada Verde
// (the XP one) or a Fada Vermelha in the leader's fairy slot the chain walks
// itself — a few seconds after the room falls the whole party is moved into the
// next one, and on to the boss after the last numbered room.
//
// The scroll that ride replaces is NOT minted: it is the same reward, spent as a
// ride instead of handed over, and printing it too would turn one fairy into an
// endless supply of entries. It is handed over on every path that fails, so a
// party that cannot be moved is never left with nothing.
//
// The Fada Azul (3901, 3904, 3907) is deliberately out — it was not asked for.

const (
	// fadaEsperaNaAgua is the pause between the last monster dying and the party
	// being moved, in 1s ticks: long enough to pick up what fell, short enough to
	// feel automatic.
	fadaEsperaNaAgua = 5
)

// fadaLevaNaAgua reports whether a fairy carries the party through the chain:
// the Verde family (which includes the Suprema, 3913) and the Vermelha. The
// indices are the ones fairyExpBonus already pays experience for, minus the Azul
// and the Verde-Azul.
func fadaLevaNaAgua(idx int16) bool {
	switch idx {
	case 3900, 3903, 3906, 3911, 3912, 3913: // Fada Verde, incl. a Suprema
		return true
	case 3902, 3905, 3908: // Fada Vermelha
		return true
	}
	return false
}

// avancoDaFada is one party waiting to be moved on.
type avancoDaFada struct {
	variant int // which chain (N, M or A)
	room    int // the room just cleared — also the room whose scroll is owed
	leader  int // entity id of the party leader
	espera  int // ticks left before the first attempt
	prazo   int // ticks left to keep retrying while the next room is busy
}

// proximaSalaDaAgua is the room the chain moves to. The last numbered room (7,
// the one whose reward is the Evocação Neses) leads to the boss; the dead room 8
// is never entered.
func proximaSalaDaAgua(room int) int {
	if room == waterDeadRoom-1 {
		return waterBossRoom
	}
	return room + 1
}

// agendarAvancoDaFada queues the ride, and reports whether it took it. False
// means the caller keeps the old behaviour and hands out the scroll.
func (d *Dispatcher) agendarAvancoDaFada(w *world.World, leader *world.Entity, variant, room int) bool {
	if leader == nil || !fadaLevaNaAgua(leader.Equip[fairyEquipSlot].Index) {
		return false
	}
	s := w.Session(leader.ID)
	if s == nil || s.Mode != world.UserPlay {
		return false
	}
	// Retry for as long as the cleared room lasts. When its countdown runs out
	// everyone standing in it is thrown out of the dungeon, and by then the scroll
	// has to be in the bag — that is what lets the party walk back in.
	return d.enfileirarAvancoDaFada(avancoDaFada{
		variant: variant,
		room:    room,
		leader:  leader.ID,
		espera:  fadaEsperaNaAgua,
		prazo:   int(d.events.water[variant][room]) * waterTickPeriod,
	})
}

// enfileirarAvancoDaFada adds one ride to the queue, refusing a second one for
// the same room. The clear hook can fire more than once per run (a respawn, a
// second GenerateMob), and claimWaterReward only guards the payout — without
// this, two rides would queue and the party would be moved twice.
func (d *Dispatcher) enfileirarAvancoDaFada(a avancoDaFada) bool {
	for _, j := range d.events.aguaFada {
		if j.variant == a.variant && j.room == a.room {
			return true
		}
	}
	d.events.aguaFada = append(d.events.aguaFada, a)
	return true
}

// tickFadaDaAgua moves the queued parties. It runs every tick (1s), not on the
// water's 2s cadence, because the wait is counted in seconds.
func (d *Dispatcher) tickFadaDaAgua(w *world.World) {
	if len(d.events.aguaFada) == 0 {
		return
	}
	// Filter in place: every entry either survives to the next tick or is resolved
	// here, so the queue is rewritten from its own head.
	restantes := d.events.aguaFada[:0]
	for _, a := range d.events.aguaFada {
		if a.espera > 0 {
			a.espera--
			restantes = append(restantes, a)
			continue
		}
		leader := w.Entity(a.leader)
		s := w.Session(a.leader)
		// Logged out, no longer the leader, or the fairy came off: whatever earned
		// the ride is gone, but the room was cleared and the scroll is still owed.
		if leader == nil || s == nil || s.Mode != world.UserPlay || leader.Leader != 0 ||
			!fadaLevaNaAgua(leader.Equip[fairyEquipSlot].Index) {
			d.entregarPergaminhoDaFada(w, leader, a, "lider saiu ou tirou a fada")
			continue
		}
		// Walked out of the dungeon on foot: moving the party from outside would
		// teleport people who already left the run.
		if !insideAnyWaterRoom(a.variant, leader.X, leader.Y) {
			d.entregarPergaminhoDaFada(w, leader, a, "lider fora da agua")
			continue
		}
		proxima := proximaSalaDaAgua(a.room)
		if ocupante, ocupada := d.waterRoomBusy(w, a.variant, proxima); ocupada {
			if a.prazo--; a.prazo > 0 {
				restantes = append(restantes, a)
				continue
			}
			d.log.Info("fairy advance gave up: next room busy",
				"variant", a.variant, "room", proxima, "occupant", ocupante)
			d.entregarPergaminhoDaFada(w, leader, a, "proxima sala ocupada")
			continue
		}
		d.abrirSalaDaAgua(w, s, leader, a.variant, proxima)
		d.announceWaterRoom(w, leader, "A fada levou o grupo: "+waterRoomLabel(proxima)+".")
		d.log.Info("fairy advanced the party",
			"leader", leader.Name, "variant", a.variant, "from", a.room, "to", proxima)
	}
	d.events.aguaFada = restantes
}

// entregarPergaminhoDaFada is every failed path: the party gets the scroll it
// would have got without a fairy, and the announcement that goes with it.
func (d *Dispatcher) entregarPergaminhoDaFada(w *world.World, leader *world.Entity, a avancoDaFada, motivo string) {
	if leader == nil {
		// Nobody to hand it to. The reward dies with the run, exactly as it does
		// when a leader disconnects mid-room today.
		d.log.Info("fairy advance dropped: leader gone",
			"variant", a.variant, "room", a.room, "motivo", motivo)
		return
	}
	d.grantNextWaterScroll(w, leader, a.variant, a.room)
	d.announceWaterRoom(w, leader, waterRoomLabel(a.room)+" limpa! Use o proximo pergaminho.")
	d.log.Info("fairy advance fell back to the scroll",
		"leader", leader.Name, "variant", a.variant, "room", a.room, "motivo", motivo)
}
