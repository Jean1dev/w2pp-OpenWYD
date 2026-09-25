package handler

import (
	"math"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

var nightmareGenerators = [3][2]int{{2368, 2375}, {2377, 2384}, {2385, 2394}}
var nightmareOrigins = [3][2]int16{{19, 15}, {16, 16}, {19, 13}}
var nightmareDestinations = [3][2]int16{{1304, 335}, {1083, 308}, {1204, 152}}
var nightmareArcaneParty = [world.MaxParty][2]int16{
	{1204, 152}, {1217, 155}, {1195, 175}, {1182, 174},
	{1171, 190}, {1189, 196}, {1209, 182}, {1226, 190},
	{1230, 174}, {1247, 184}, {1224, 190}, {1211, 165},
}

// nightmareOpening identifies the latest local calendar occurrence, including
// Mystic/Arcane cycles that started in the previous hour.
func nightmareOpening(now time.Time, kind int) time.Time {
	now = now.In(time.Local)
	minute := now.Minute()/20*20 + kind*5
	opening := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), minute, 0, 0, time.Local)
	if opening.After(now) {
		opening = opening.Add(-20 * time.Minute)
	}
	return opening
}

func (d *Dispatcher) tickNightmare(w *world.World) {
	now := d.now()
	st := w.NightmareState()
	for kind := range st.Cycles {
		cycle := &st.Cycles[kind]
		opening := nightmareOpening(now, kind)
		// A backward wall-clock correction must not replay a completed cycle.
		if st.Initialized && opening.Unix() < cycle.OpeningUnix {
			continue
		}
		if !st.Initialized || opening.Unix() != cycle.OpeningUnix {
			d.clearNightmare(w, kind)
			// A restart cannot reconstruct a partially admitted round. Only the
			// next opening crossed by this process becomes available.
			*cycle = world.NightmareCycle{OpeningUnix: opening.Unix(), Available: st.Initialized || now.Equal(opening)}
		}
		elapsed := now.Unix() - cycle.OpeningUnix
		if elapsed >= 19*60 && !cycle.Finished {
			d.clearNightmare(w, kind)
			cycle.Finished, cycle.Available, cycle.Admissions = true, false, 0
			d.log.Info("nightmare ended", "kind", kind, "opening", cycle.OpeningUnix)
		}
		if cycle.Available && !cycle.Started && elapsed >= 4*60 && elapsed < 19*60 {
			cycle.Started = true
			participants := 0
			w.ForEachPlayer(func(s *world.Session, e *world.Entity) {
				if world.NightmareMap(e.X, e.Y) == kind {
					participants++
					sendNightmareTime(w, s, int32(19*60-elapsed))
				}
			})
			if participants > 0 {
				for idx := nightmareGenerators[kind][0]; idx <= nightmareGenerators[kind][1]; idx++ {
					ids := w.GenerateMob(idx)
					if len(ids) == 0 {
						d.log.Warn("nightmare generator produced no mobs", "generator", idx)
					}
					d.revealSpawned(w, ids)
				}
			}
			d.log.Info("nightmare started", "kind", kind, "players", participants)
		}
	}
	st.Initialized = true
	w.SetNightmareState(st)
}

func (d *Dispatcher) clearNightmare(w *world.World, kind int) {
	for idx := nightmareGenerators[kind][0]; idx <= nightmareGenerators[kind][1]; idx++ {
		w.ClearGenerator(idx)
	}
	w.ForEachMob(func(id int, e *world.Entity) {
		if world.NightmareMap(e.X, e.Y) == kind {
			w.DespawnMob(id, 0)
		}
	})
	w.ForEachPlayer(func(s *world.Session, e *world.Entity) {
		if world.NightmareMap(e.X, e.Y) != kind {
			return
		}
		if e.HP <= 0 {
			e.HP = 2
			s.ReqHp = 2
			d.sendScore(w, s, e)
			d.sendSetHpMp(w, s, e)
		}
		d.recall(w, s, e)
	})
}

func sendNightmareTime(w *world.World, s *world.Session, seconds int32) {
	w.SendTo(s, protocol.Header{Type: protocol.MsgStartTime, ID: protocol.IDScene}, protocol.EncodeStandardParm(seconds))
}

func nightmareClass(kind int, class uint8) bool {
	switch kind {
	case 0:
		return class == classMasterMortal
	case 1:
		return class == classMasterArch
	case 2:
		return class == classMasterCelestial || class == classMasterCelestialCS || class == classMasterSCelestial
	default:
		return false
	}
}

func (d *Dispatcher) useNightmareScroll(w *world.World, s *world.Session, e *world.Entity, src, kind int) {
	d.tickNightmare(w)
	reject := func(message string) {
		w.Send(s, protocol.MsgMessagePanel, protocol.EncodeMessagePanelBody(message))
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	}
	if s.Trade.Active || s.TradeMode != 0 {
		reject("Encerre a troca antes de entrar no Pesadelo.")
		return
	}
	if e.X/128 != nightmareOrigins[kind][0] || e.Y/128 != nightmareOrigins[kind][1] {
		reject("Não é possível usar este item aqui.")
		return
	}
	if e.Leader != 0 && e.Leader != -1 {
		reject("Somente o líder do grupo pode usar este item.")
		return
	}
	if !nightmareClass(kind, e.ClassMaster) {
		reject([3]string{"Entrada permitida somente à Mortais", "Entrada permitida somente à Archs", "Entrada permitida somente à Celestiais"}[kind])
		return
	}
	now := d.now()
	st := w.NightmareState()
	cycle := &st.Cycles[kind]
	remaining := cycle.OpeningUnix + 240 - now.Unix()
	if !cycle.Available || cycle.Started || cycle.Finished || remaining <= 0 || now.Unix() < cycle.OpeningUnix {
		reject("Horário não permitido.")
		return
	}
	if kind == 2 && e.NightmareEntries <= 0 {
		reject("Saldo de entradas insuficiente; consulte /nt.")
		return
	}
	group := e.Carry[src].Index == int16(3324+kind)
	counted := group || kind == 2
	limit := d.cfg.MaxNightmare
	if limit <= 0 {
		limit = 3
	}
	if counted && cycle.Admissions >= limit {
		reject("Limite de entradas do Pesadelo atingido.")
		return
	}
	// Refuse unavailable content before charging anyone for an empty event.
	for idx := nightmareGenerators[kind][0]; idx <= nightmareGenerators[kind][1]; idx++ {
		if g := w.GeneratorAt(idx); g == nil || len(g.LeaderTmpl) == 0 || (g.MaxGroup > 0 && len(g.FollowerTmpl) == 0) {
			reject("Pesadelo indisponível. Tente novamente mais tarde.")
			return
		}
	}
	if !d.enterNightmare(w, s, e, kind, 0, int32(remaining)) {
		reject("Não há espaço no destino.")
		return
	}
	if counted {
		cycle.Admissions++
	}
	consumeOneItem(&e.Carry[src])
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	w.SetNightmareState(st)
	if !group {
		return
	}
	seen := map[int]bool{s.Conn: true}
	for i, id := range e.PartyList {
		if !world.IsPlayer(id) || seen[id] {
			continue
		}
		seen[id] = true
		member, ms := w.Entity(id), w.Session(id)
		if member == nil || ms == nil || ms.Mode != world.UserPlay || ms.Trade.Active || ms.TradeMode != 0 || member.HP <= 0 || !nightmareClass(kind, member.ClassMaster) {
			continue
		}
		if kind == 2 && member.NightmareEntries <= 0 {
			continue
		}
		d.enterNightmare(w, ms, member, kind, i, int32(remaining))
	}
}

func (d *Dispatcher) enterNightmare(w *world.World, s *world.Session, e *world.Entity, kind, index int, seconds int32) bool {
	x, y := nightmareDestinations[kind][0], nightmareDestinations[kind][1]
	if kind == 2 {
		x, y = nightmareArcaneParty[index][0], nightmareArcaneParty[index][1]
	}
	if kind != 0 && world.NightmareMap(e.SaveX, e.SaveY) == kind {
		x, y = e.SaveX, e.SaveY
	}
	// Legacy DoTeleport resolves occupied cells. Reserve sequentially within the
	// owner loop so group members cannot overwrite each other's grid entries.
	x, y, ok := w.EmptyCellNear(x, y)
	if !ok || world.NightmareMap(x, y) != kind {
		return false
	}
	if kind == 2 {
		// rand()%1 is zero, but both draws still advance the legacy LCG.
		w.Rand().Intn(1)
		w.Rand().Intn(1)
		e.NightmareEntries--
	}
	d.doTeleport(w, s, x, y)
	sendNightmareTime(w, s, seconds)
	return true
}

func (d *Dispatcher) useNightmareDeed(w *world.World, s *world.Session, e *world.Entity, src int) {
	if s.Trade.Active || s.TradeMode != 0 || e.NightmareEntries > math.MaxInt32-13 {
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}
	e.NightmareEntries += 13
	e.LastNightmareUse = d.now().Unix()
	consumeOneItem(&e.Carry[src])
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	sendKefraBalance(w, s, e.NightmareEntries)
}

func (d *Dispatcher) nightmareMobKilled(w *world.World, mob *world.Entity) {
	kind := world.NightmareGenerator(int(mob.GenIndex))
	if kind < 0 || mob.Summoner != 0 {
		return
	}
	st := w.NightmareState()
	cycle := st.Cycles[kind]
	now := d.now().Unix()
	if !cycle.Available || !cycle.Started || now < cycle.OpeningUnix+240 || now >= cycle.OpeningUnix+19*60 {
		return
	}
	d.revealSpawned(w, w.GenerateMob(int(mob.GenIndex)))
}
