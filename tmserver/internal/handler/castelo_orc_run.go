package handler

import (
	"fmt"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/internal/droprule"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// The Castelo Orc run: a new rule, not the legacy's. A party leader hands the
// Xamã Orc the Chave Portão Orc Sul; the castle is emptied of its open-world
// orcs and of anyone outside the party, the quest's own monsters rise (world blocks
// CasteloOrcGenFirst..Last) and the party is dropped at the south-west wall with
// fifteen minutes on the clock. One party at a time, server-wide, like the Sala
// Secreta: the castle, its blocks and the sweep are shared.
//
// It ends on the clock, two minutes after the Grão-Lorde falls (the loot
// window), or a minute after the last member left the castle. Then the quest's
// monsters go, whoever is still inside is sent back to the /erion landing, and
// the open-world orcs refill on their own generator timers.
//
// Not modeled yet: the gates relocking behind the party (the server never sends
// a gate to the client), a completion prize, and any level or tier gate.
// Nothing survives a restart — a boot mid-run simply ends it, as with the Água
// and the Carta.
const (
	// itemChaveCasteloOrc is the entry key: Chave_Portão_Orc_Sul, the first of the
	// legacy castle's four gate keys. Where it drops is the Mesa de Drops' call
	// (migration 0051): the Quest 256 Hydra and Elf arenas and the Desert.
	itemChaveCasteloOrc = 465
	gradeCasteloOrc     = 40 // the Xamã Orc's EF_GRADE0 (Merchant 100); no shipped template uses 40

	casteloOrcRunSeconds  = 15 * 60
	casteloOrcLootSeconds = 2 * 60
	// casteloOrcFollowerEvery is how often the Guarda do Lorde block is topped
	// back up while a run is on: the last room is meant to be farmed.
	casteloOrcFollowerEvery = 30
	// casteloOrcAbandonSeconds ends a run nobody is in any more, so a party that
	// walked away does not hold the castle for the rest of its fifteen minutes.
	casteloOrcAbandonSeconds = 60
	// casteloOrcNPCCheckEvery is how often the Xamã is looked for and raised
	// again if something removed it.
	casteloOrcNPCCheckEvery = 10
	// casteloOrcResyncEvery re-pushes the clock to the party, the way the water
	// rooms re-push theirs on every change: a member who relogged or died and
	// walked back in gets the counter again within a minute.
	casteloOrcResyncEvery = 60

	casteloOrcNPCTemplate = "COrc_Xama"
	casteloOrcBossGen     = world.CasteloOrcGenFirst
	casteloOrcFollowerGen = world.CasteloOrcGenFirst + 1
)

// casteloOrcBox is the castle the run owns: every spawn of the old castle blocks
// (x 2438-2553, y 2053-2158) plus a margin.
var casteloOrcBox = areaBox{2430, 2045, 2562, 2166}

var (
	// casteloOrcEntry is where the party lands: an Orc_Arqueiro_ spawn by the
	// south-west wall, a few steps from the Sentinela, the first guardian.
	casteloOrcEntry = [2]int16{2446, 2134}
	// casteloOrcExit is the /erion landing (chat.go), where the Xamã stands and
	// where a finished run sends everyone back.
	casteloOrcExit = [2]int16{2461, 2003}
	casteloOrcNPC  = [2]int16{2464, 2006}
)

// casteloOrcWorldBlocks are the open-world castle blocks ([373-394], [402-497];
// 395-400 are the Kefra blocks, 401 is the one typed at StartX 2). A run clears
// them and keeps them from regenerating until it ends.
func isCasteloOrcWorldBlock(idx int) bool {
	return (idx >= 373 && idx <= 394) || (idx >= 402 && idx <= 497)
}

// casteloOrcRun is the one run in progress. Loop-owned.
type casteloOrcRun struct {
	active        bool
	secondsLeft   int
	party         []int // player conns admitted at the start
	leaderName    string
	bossDown      bool
	emptyFor      int
	sinceFollower int
}

func (r *casteloOrcRun) inParty(conn int) bool {
	for _, c := range r.party {
		if c == conn {
			return true
		}
	}
	return false
}

// casteloOrcMoveAllowed refuses a step into the castle to anyone outside the
// party while a run is on.
func (d *Dispatcher) casteloOrcMoveAllowed(conn int, x, y int16) bool {
	if !d.casteloOrc.active || !casteloOrcBox.contains(x, y) {
		return true
	}
	return d.casteloOrc.inParty(conn)
}

// casteloOrcSuppresses keeps the open-world castle blocks from regenerating
// under the party.
func (d *Dispatcher) casteloOrcSuppresses(idx int) bool {
	return d.casteloOrc.active && isCasteloOrcWorldBlock(idx)
}

// casteloOrcQuestNPC is the Xamã Orc (Merchant 100, grade 40).
func (d *Dispatcher) casteloOrcQuestNPC(w *world.World, s *world.Session, e, npc *world.Entity) {
	if d.casteloOrc.active {
		sendSay(w, npc, fmt.Sprintf("Um grupo já está no castelo. Volte em %d min.", (d.casteloOrc.secondsLeft+59)/60))
		return
	}
	// Members carry their leader's conn; only a leader or a soloist opens a run.
	if e.Leader != 0 {
		sendSay(w, npc, "Só o líder do grupo pode abrir o castelo.")
		return
	}
	slot := -1
	for i := 0; i < activeCarryLimit(e); i++ {
		if e.Carry[i].Index == itemChaveCasteloOrc {
			slot = i
			break
		}
	}
	if slot < 0 {
		// The catalog spells names with underscores; the NPC says them with spaces.
		key := strings.ReplaceAll(d.itemName(itemChaveCasteloOrc), "_", " ")
		sendSay(w, npc, fmt.Sprintf("Traga a %s para abrir o castelo.", key))
		return
	}
	consumeOneItem(&e.Carry[slot])
	d.sendSlot(w, s, world.ItemPlaceCarry, slot, e.Carry[slot])
	d.openCasteloOrc(w, e)
	sendSay(w, npc, "O castelo é de vocês por 15 minutos.")
}

// openCasteloOrc starts a run for e's party.
func (d *Dispatcher) openCasteloOrc(w *world.World, e *world.Entity) {
	party := []int{e.ID}
	for _, id := range e.PartyList {
		// Pets live in the PartyList too; only players are admitted.
		if id <= 0 || id == e.ID || !world.IsPlayer(id) {
			continue
		}
		if ms := w.Session(id); ms != nil && ms.Mode == world.UserPlay {
			party = append(party, id)
		}
	}
	d.casteloOrc = casteloOrcRun{active: true, secondsLeft: casteloOrcRunSeconds, party: party, leaderName: e.Name}

	// The castle, emptied: the open-world orcs and the quest's own leftovers.
	// ClearGenerator rather than a despawn sweep, so each block's population
	// counter drops with its mobs and nothing is left on the 15s respawn queue.
	for idx := 0; idx < w.GeneratorCount(); idx++ {
		if isCasteloOrcWorldBlock(idx) || world.IsCasteloOrcGenerator(idx) {
			w.ClearGenerator(idx)
		}
	}
	d.casteloOrcSweep(w, true)

	spawned := 0
	for idx := world.CasteloOrcGenFirst; idx <= world.CasteloOrcGenLast; idx++ {
		// A troop block is filled to its cap group by group; GenerateMob returns
		// nothing once the cap is reached. The bound is only a backstop.
		for range 10 {
			ids := w.GenerateMob(idx)
			if len(ids) == 0 {
				break
			}
			d.revealSpawned(w, ids)
			spawned += len(ids)
		}
	}

	for _, conn := range party {
		if s := w.Session(conn); s != nil {
			d.doTeleport(w, s, casteloOrcEntry[0], casteloOrcEntry[1])
			d.sendCasteloOrcCountdown(w, s)
			sendClientMessage(w, s, "Castelo Orc: 15 minutos. Derrube o Grão-Lorde.")
		}
	}
	d.log.Info("castelo orc started", "leader", e.Name, "party", len(party), "mobs", spawned)
}

// casteloOrcSweep sends every player standing in the castle back to the /erion
// landing — all of them when the run ends, only those outside the party when it
// starts. Staff are left where they stand, as the castle move gate leaves them.
func (d *Dispatcher) casteloOrcSweep(w *world.World, strangersOnly bool) {
	var out []*world.Session
	w.ForEachPlaying(-1, func(s *world.Session, e *world.Entity) {
		if !casteloOrcBox.contains(e.X, e.Y) || s.AccessLevel >= world.AccessModerator {
			return
		}
		if strangersOnly && d.casteloOrc.inParty(s.Conn) {
			return
		}
		out = append(out, s)
	})
	// Teleport outside the walk: doTeleport mutates position and view.
	for _, s := range out {
		if e := w.Entity(s.Conn); e != nil && e.HP <= 0 {
			e.HP = 2 // the destination flow expects a living entity (clearArea)
		}
		d.doTeleport(w, s, casteloOrcExit[0], casteloOrcExit[1])
		if strangersOnly {
			sendClientMessage(w, s, "Um grupo abriu o Castelo Orc. Volte em 15 minutos.")
		} else {
			sendClientMessage(w, s, "A corrida do Castelo Orc terminou.")
		}
	}
}

// sendCasteloOrcCountdown shows the run's clock: the same MsgStartTime, in the
// same unit, as the water rooms' counter the players already know
// (sendWaterCountdown sends its 2-second units ×2, i.e. seconds) and the
// Pesadelo's. The client counts it down by itself; the resync only corrects it.
// It cannot reuse sendWaterCountdown: that one takes a uint8, and 900 does not
// fit.
func (d *Dispatcher) sendCasteloOrcCountdown(w *world.World, s *world.Session) {
	body := protocol.EncodeStandardParm(int32(d.casteloOrc.secondsLeft))
	w.SendTo(s, protocol.Header{Type: protocol.MsgStartTime, ID: protocol.IDScene}, body)
}

// broadcastCasteloOrcCountdown pushes the clock to every party member in play,
// with a line on the message panel when there is one.
func (d *Dispatcher) broadcastCasteloOrcCountdown(w *world.World, text string) {
	for _, conn := range d.casteloOrc.party {
		if s := w.Session(conn); s != nil && s.Mode == world.UserPlay {
			d.sendCasteloOrcCountdown(w, s)
			sendClientMessage(w, s, text)
		}
	}
}

// casteloOrcResyncDue reports whether the clock is re-pushed at this second.
func casteloOrcResyncDue(secondsLeft int) bool {
	return secondsLeft > 0 && secondsLeft%casteloOrcResyncEvery == 0
}

// tickCasteloOrc is the per-second clock, and the Xamã's keeper.
func (d *Dispatcher) tickCasteloOrc(w *world.World) {
	if d.tickCount%casteloOrcNPCCheckEvery == 0 {
		d.ensureCasteloOrcNPC(w)
	}
	r := &d.casteloOrc
	if !r.active {
		return
	}
	r.secondsLeft--
	if r.secondsLeft <= 0 {
		d.endCasteloOrc(w, "tempo")
		return
	}
	if casteloOrcResyncDue(r.secondsLeft) {
		d.broadcastCasteloOrcCountdown(w, "")
	}

	r.sinceFollower++
	if r.sinceFollower >= casteloOrcFollowerEvery {
		r.sinceFollower = 0
		d.revealSpawned(w, w.GenerateMob(casteloOrcFollowerGen))
	}

	inside := false
	for _, conn := range r.party {
		if e := w.Entity(conn); e != nil && e.Mode == world.MobUser && casteloOrcBox.contains(e.X, e.Y) {
			if s := w.Session(conn); s != nil && s.Mode == world.UserPlay {
				inside = true
				break
			}
		}
	}
	if inside {
		r.emptyFor = 0
		return
	}
	r.emptyFor++
	if r.emptyFor >= casteloOrcAbandonSeconds {
		d.endCasteloOrc(w, "abandono")
	}
}

// casteloOrcBossKilled cuts the clock to the loot window. It runs before the
// despawn, like the other boss hooks.
func (d *Dispatcher) casteloOrcBossKilled(w *world.World, mob *world.Entity) {
	r := &d.casteloOrc
	if !r.active || r.bossDown || mob == nil || int(mob.GenIndex) != casteloOrcBossGen {
		return
	}
	r.bossDown = true
	if r.secondsLeft > casteloOrcLootSeconds {
		r.secondsLeft = casteloOrcLootSeconds
	}
	d.broadcastCasteloOrcCountdown(w, "O Grão-Lorde caiu! 2 minutos para o saque.")
	d.log.Info("castelo orc boss down", "leader", r.leaderName)
}

// endCasteloOrc takes the quest's monsters away and empties the castle.
func (d *Dispatcher) endCasteloOrc(w *world.World, why string) {
	for idx := world.CasteloOrcGenFirst; idx <= world.CasteloOrcGenLast; idx++ {
		w.ClearGenerator(idx)
	}
	d.casteloOrcSweep(w, false)
	d.log.Info("castelo orc finished", "leader", d.casteloOrc.leaderName, "why", why, "boss_down", d.casteloOrc.bossDown)
	d.casteloOrc = casteloOrcRun{}
}

// ensureCasteloOrcNPC raises the Xamã at the /erion landing when it is not
// standing. It is spawned here and not from NPCGener on purpose: a Merchant NPC
// in NPCGener belongs to the NPC overlay when W2PP_NPC_EDITING is on, and would
// then need a `dbserver import-npcs` run to appear at all.
func (d *Dispatcher) ensureCasteloOrcNPC(w *world.World) {
	if d.casteloOrcNPCTmpl == nil {
		return
	}
	if e := w.Entity(d.casteloOrcNPCID); d.casteloOrcNPCID >= world.MaxUser && e != nil && e.Mode != world.MobEmpty &&
		droprule.Canonical(e.TemplateName) == droprule.Canonical(casteloOrcNPCTemplate) {
		return
	}
	x, y, ok := w.EmptyCellNear(casteloOrcNPC[0], casteloOrcNPC[1])
	if !ok {
		return
	}
	id := w.SpawnMobAt(world.MobSpawn{Template: d.casteloOrcNPCTmpl, TemplateName: casteloOrcNPCTemplate, X: x, Y: y, GenIndex: -1})
	if id < 0 {
		return
	}
	// Off the respawn queue: the keeper, not the queue, brings it back.
	w.Entity(id).Template = nil
	d.casteloOrcNPCID = id
	d.revealSpawned(w, []int{id})
	d.log.Info("castelo orc npc raised", "id", id, "x", x, "y", y)
}
