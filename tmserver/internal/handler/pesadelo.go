package handler

import (
	"fmt"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Pesadelo (Nightmare) is a timed instanced dungeon in three tiers — Normal,
// Místico and Arcano — entered with a scroll while standing on a staging tile
// (_MSG_UseItem.cpp:2548/2644/2748).
//
// The shape the legacy encodes three times over, once per tier:
//
//   - Each tier opens for FOUR MINUTES, three times an hour, and the three tiers
//     are staggered five minutes apart so they never overlap: N at :00/:20/:40,
//     M at :05/:25/:45, A at :10/:30/:50.
//   - One minute before each opening the instance is wiped — everyone inside is
//     recalled out and the run counter resets (ProcessSecMinTimer.cpp:1006-1035).
//   - Only a party LEADER may use the scroll, and only in that tier's staging
//     area — one city per tier: N from Erion, M from Armia, A from Azran. The
//     party rides along, minus members who fail the gate.
//   - Entry is a ladder of class AND level, the same one the Pergaminho da Água
//     uses: N is Mortal, M is Arch (plus a Celestial still under 40), A is
//     Celestial up to 150 — past that the progression moves to the Água A chain.
//   - At most maxNightmare runs per window per tier, server-wide.
//
// The countdown the client shows is what remains of the four-minute window at
// the moment of entry, not a fresh four minutes — enter at :23:30 and you get
// thirty seconds.
const (
	pesaN = 0
	pesaM = 1
	pesaA = 2

	pesaTiers = 3
)

// EF_VOLATILE of the entry scrolls and of the ticket book.
const (
	volPesadeloN         = 173
	volPesadeloM         = 174
	volPesadeloA         = 175
	volEscrituraPesadelo = 212
)

// Group scrolls take the whole party; the individual ones (3390/3391/3392) carry
// the same EF_VOLATILE and are told apart by sIndex (_MSG_UseItem.cpp:2587).
const (
	itemPesadeloGrupoN = 3324
	itemPesadeloGrupoM = 3325
	itemPesadeloGrupoA = 3326

	// itemEscrituraPesadelo grants Celestials the entries the Arcano tier spends.
	itemEscrituraPesadelo = 5137
	escrituraTickets      = 13
)

const (
	// pesaWindowSeconds is NigthTime: the four-minute window, in seconds.
	pesaWindowSeconds = 240
	// pesaWindowMinutes is the same span in whole minutes, used to test whether
	// the current minute falls inside an open window.
	pesaWindowMinutes = 4
	// pesaWindowStride is the gap between a tier's three windows in an hour.
	pesaWindowStride = 20
	// defaultMaxNightmare is maxNightmare (Server.cpp:687) — runs allowed per
	// window per tier, server-wide. The legacy exposes it to the portal config.
	defaultMaxNightmare = 3
)

// pesaTier is one tier's fixed configuration.
type pesaTier struct {
	name string
	vol  int
	// groupItem is the sIndex of the party scroll; every other item carrying
	// this tier's EF_VOLATILE is the solo variant.
	groupItem int16
	// stage is the 128-unit segment where the scroll is accepted, the legacy
	// TargetX/128 gate. Each tier is entered from a different city: N from Erion,
	// M from Armia, A from Azran (the segments below are those cities' centres).
	stageX, stageY int16
	// seg is the dungeon's own 128-unit segment, used to wipe it and to notice
	// that a player's save point is already inside.
	segX, segY int16
	// openMinute is the first of the tier's three windows; the others follow at
	// +20 and +40. The wipe runs one minute before each.
	openMinute int
	// spawn is PesaNPosStandard/PesaMPosStandard/PesaAPosStandard: the landing
	// spot per party slot. N and M repeat one point; A spreads the party out.
	spawn [world.MaxParty][2]int16
	// classMsg is what the door says when the wrong tier knocks. The legacy sends
	// three different literals straight to SendClientMessage rather than going
	// through the string table (_MSG_UseItem.cpp:2571/2667/2772), so they are
	// copied verbatim here — including "somente à Mortais", which is the original
	// wording and not a typo introduced by the port.
	classMsg string
}

// pesaTierTable indexes the three tiers. Positions are Server.cpp:381/398/415.
var pesaTierTable = [pesaTiers]pesaTier{
	pesaN: {
		name: "N", vol: volPesadeloN, groupItem: itemPesadeloGrupoN,
		classMsg: "Entrada permitida somente à Mortais",
		stageX:   19, stageY: 15, segX: 10, segY: 2, openMinute: 0, // entra por Erion
		spawn: repeatSpawn(1304, 335),
	},
	pesaM: {
		name: "M", vol: volPesadeloM, groupItem: itemPesadeloGrupoM,
		classMsg: "Entrada permitida somente à Archs",
		stageX:   16, stageY: 16, segX: 8, segY: 2, openMinute: 5, // entra por Armia
		spawn: repeatSpawn(1083, 308),
	},
	pesaA: {
		name: "A", vol: volPesadeloA, groupItem: itemPesadeloGrupoA,
		classMsg: "Entrada permitida somente à Celestiais",
		stageX:   19, stageY: 13, segX: 9, segY: 1, openMinute: 10, // entra por Azran
		// The legacy table has thirteen rows but the loop only ever reads
		// PesaAPosStandard[0..MAX_PARTY-1], so the last row {1209, 174} is dead
		// and is not carried over.
		spawn: [world.MaxParty][2]int16{
			{1204, 152}, {1217, 155}, {1195, 175}, {1182, 174}, {1171, 190},
			{1189, 196}, {1209, 182}, {1226, 190}, {1230, 174}, {1247, 184},
			{1224, 190}, {1211, 165},
		},
	},
}

// repeatSpawn fills every party slot with the same point, which is how the
// legacy tables for N and M are written — thirteen copies of one coordinate.
func repeatSpawn(x, y int16) [world.MaxParty][2]int16 {
	var out [world.MaxParty][2]int16
	for i := range out {
		out[i] = [2]int16{x, y}
	}
	return out
}

// pesadeloTierForVolatile maps an EF_VOLATILE to its tier.
func pesadeloTierForVolatile(vol int) (int, bool) {
	for i := range pesaTierTable {
		if pesaTierTable[i].vol == vol {
			return i, true
		}
	}
	return 0, false
}

// isPesadeloVolatile reports whether an EF_VOLATILE is a Pesadelo entry scroll.
func isPesadeloVolatile(vol int) bool {
	_, ok := pesadeloTierForVolatile(vol)
	return ok
}

// segment reports whether (x, y) lies in the tier's dungeon segment.
func (t pesaTier) segment(x, y int16) bool {
	return x/128 == t.segX && y/128 == t.segY
}

// staging reports whether (x, y) is on the tier's staging tile, the only place
// its scroll may be used.
func (t pesaTier) staging(x, y int16) bool {
	return x/128 == t.stageX && y/128 == t.stageY
}

// box is the dungeon segment as a rectangle, for the periodic wipe.
func (t pesaTier) box() areaBox {
	return areaBox{
		x1: t.segX * 128, y1: t.segY * 128,
		x2: t.segX*128 + 127, y2: t.segY*128 + 127,
	}
}

// window returns the seconds left in the tier's currently open window, and
// whether one is open at all.
//
// Windows run for four minutes from openMinute, repeating every twenty:
// N :00/:20/:40, M :05/:25/:45, A :10/:30/:50. The legacy writes this as a long
// disjunction of rejected minute ranges and then subtracts the elapsed time
// three times over; the modulo says the same thing once.
func (t pesaTier) window(now time.Time) (secondsLeft int, open bool) {
	minute, sec := now.Minute(), now.Second()
	elapsedMin := ((minute - t.openMinute) + 60) % pesaWindowStride
	if elapsedMin >= pesaWindowMinutes {
		return 0, false
	}
	left := pesaWindowSeconds - (elapsedMin*60 + sec)
	if left <= 0 {
		return 0, false
	}
	return left, true
}

// nextWindow returns how long until the tier's next window opens, or 0 when one
// is open now. Backs the /nig command.
func (t pesaTier) nextWindow(now time.Time) time.Duration {
	if _, open := t.window(now); open {
		return 0
	}
	minute, sec := now.Minute(), now.Second()
	waitMin := ((t.openMinute - minute) + 60) % pesaWindowStride
	d := time.Duration(waitMin)*time.Minute - time.Duration(sec)*time.Second
	if d <= 0 {
		d += pesaWindowStride * time.Minute
	}
	return d
}

// wipeMinute reports whether the tier's instance is wiped at this minute: one
// minute before each of its three openings (ProcessSecMinTimer.cpp:1006-1035).
func (t pesaTier) wipeMinute(minute int) bool {
	return ((minute+1)-t.openMinute+60)%pesaWindowStride == 0
}

// Level caps per tier. The legacy has no active level gate at all — its only
// mention is a commented-out `Level >= 180` in the Arcano party loop — so these
// are a server rule, deliberately the same ladder the Pergaminho da Água already
// uses (waterClassAllowed): a tier you outgrow hands you over to the next one.
const (
	// pesaMaxLevel never rejects anybody: level.MaxLevel is 399. It is written
	// out rather than dropped so the rule reads in code the way it was
	// specified — "Mortal no N e Arch no M até 400", i.e. uncapped in practice.
	pesaMaxLevel = 400
	// pesaMCelestialMaxLevel lets a fresh Celestial run the Místico tier while it
	// is still low, the same 40 the Água M chain uses.
	pesaMCelestialMaxLevel = 40
	// pesaACelestialMaxLevel graduates a Celestial out of Arcano: past 150 the
	// progression continues in the Água A chain, which has no cap of its own.
	pesaACelestialMaxLevel = 150
)

// pesadeloTierClass reports whether a progression tier may enter this dungeon at
// all, ignoring level (_MSG_UseItem.cpp:2569/2664/2789). Kept separate from the
// level cap so a refusal can say WHICH gate closed: "wrong tier" and "you have
// outgrown this one" need different fixes.
func pesadeloTierClass(tier int, classMaster uint8) bool {
	switch tier {
	case pesaN:
		return classMaster == classMasterMortal
	case pesaM:
		// Arch owns the Místico tier; a plain Celestial is admitted too, while
		// still under pesaMCelestialMaxLevel. The sub-celestial tiers are not —
		// they are past this rung of the ladder.
		return classMaster == classMasterArch || classMaster == classMasterCelestial
	case pesaA:
		return isCelestialTier(classMaster)
	}
	return false
}

// pesadeloLevelCap is the highest level that class may enter that tier at.
func pesadeloLevelCap(tier int, classMaster uint8) int32 {
	if tier == pesaM && classMaster == classMasterCelestial {
		return pesaMCelestialMaxLevel
	}
	if tier == pesaA {
		return pesaACelestialMaxLevel
	}
	return pesaMaxLevel
}

// pesadeloAllowed is the whole gate — class and level. The party loop uses it to
// decide who rides along, where a member who fails is skipped silently rather
// than told why.
func pesadeloAllowed(tier int, classMaster uint8, level int32) bool {
	return pesadeloTierClass(tier, classMaster) && level <= pesadeloLevelCap(tier, classMaster)
}

// refusePesadelo answers a rejected use: notify, then resend the slot so the
// client never shows the scroll as consumed (the legacy SendItem on every
// refusal path).
func (d *Dispatcher) refusePesadelo(w *world.World, s *world.Session, e *world.Entity, src int, t pesaTier, n Notice) {
	d.notify(w, s, n)
	switch n {
	// The class gate is the one refusal whose line depends on the door: the tier
	// carries its own literal, because "wrong tier" is only useful advice when it
	// names the tier that said no.
	case NoticePesadeloClassNotAllowed:
		sendClientMessage(w, s, t.classMsg)

	// The closed gate needs a line because the notice it rides on lies. Its text
	// is the legacy's own _NN_CANT_USE_NIGHTMARE, "Pesadelo disponível entre 18h
	// e 24h." — an hour-of-day rule this server does not have. The real rule is
	// a four-minute window every twenty, staggered per tier, so a player who
	// reads the notice at 20h and finds the door shut concludes the server is
	// broken. Naming the tier and the countdown is the whole difference between
	// "está quebrado" and "volto em três minutos".
	case NoticePesadeloClosed:
		wait := t.nextWindow(d.now())
		sendClientMessage(w, s, fmt.Sprintf(
			"Pesadelo %s fechado. Abre em %dm%02ds. A janela dura %d min e volta a cada %d.",
			t.name, int(wait.Minutes()), int(wait.Seconds())%60,
			pesaWindowMinutes, pesaWindowStride))
	}
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
}

// usePesadeloScroll ports the three entry handlers. The gates run in the legacy
// order — area, party leader, class/ticket, schedule, run cap — and every
// refusal returns the scroll uneaten.
func (d *Dispatcher) usePesadeloScroll(w *world.World, s *world.Session, e *world.Entity, src, vol int) {
	tier, ok := pesadeloTierForVolatile(vol)
	if !ok {
		d.rejectUnimplementedConsumable(w, s, e, src)
		return
	}
	t := pesaTierTable[tier]

	// Logged on EVERY attempt, before the gates. notify()'s wire format is still
	// a placeholder (notice.go), so a refused scroll looks exactly like a dead
	// handler on the client: these lines are the only way to tell "the gate said
	// no" from "this code never ran".
	d.log.Info("pesadelo attempt",
		"account", s.AccountName, "tier", t.name, "vol", vol,
		"x", e.X, "y", e.Y, "leader", e.Leader, "classMaster", e.ClassMaster)

	if !t.staging(e.X, e.Y) {
		d.log.Info("pesadelo refused: wrong area",
			"account", s.AccountName, "tier", t.name, "x", e.X, "y", e.Y)
		d.refusePesadelo(w, s, e, src, t, NoticeCantUseHere)
		return
	}
	// Party members cannot open a run; only the leader (Leader == 0) may.
	if e.Leader != 0 {
		d.log.Info("pesadelo refused: not the party leader",
			"account", s.AccountName, "tier", t.name, "leader", e.Leader)
		d.refusePesadelo(w, s, e, src, t, NoticePartyLeaderOnly)
		return
	}
	if !pesadeloTierClass(tier, e.ClassMaster) {
		d.log.Info("pesadelo refused: wrong tier",
			"account", s.AccountName, "tier", t.name, "classMaster", e.ClassMaster)
		d.refusePesadelo(w, s, e, src, t, NoticePesadeloClassNotAllowed)
		return
	}
	// The level cap is checked after the class so someone in the wrong dungeon
	// entirely is told that, rather than being told they are too high for a tier
	// they could never enter anyway.
	if maxLevel := pesadeloLevelCap(tier, e.ClassMaster); e.Level > maxLevel {
		d.log.Info("pesadelo refused: outgrew this tier",
			"account", s.AccountName, "tier", t.name,
			"classMaster", e.ClassMaster, "level", e.Level, "cap", maxLevel)
		d.refusePesadelo(w, s, e, src, t, NoticePesadeloLevelTooHigh)
		return
	}
	// Arcano spends one of the Celestial's entries.
	if tier == pesaA && e.NightmareTickets <= 0 {
		d.log.Info("pesadelo refused: no entries left",
			"account", s.AccountName, "tickets", e.NightmareTickets)
		d.refusePesadelo(w, s, e, src, t, NoticePesadeloNoEntries)
		return
	}
	secondsLeft, open := t.window(d.now())
	if !open {
		d.log.Info("pesadelo refused: closed",
			"account", s.AccountName, "tier", t.name, "next_in", t.nextWindow(d.now()))
		d.refusePesadelo(w, s, e, src, t, NoticePesadeloClosed)
		return
	}
	if d.events.pesaRuns[tier] >= d.maxNightmare {
		d.log.Info("pesadelo refused: run cap reached",
			"account", s.AccountName, "tier", t.name, "runs", d.events.pesaRuns[tier], "cap", d.maxNightmare)
		d.refusePesadelo(w, s, e, src, t, NoticePesadeloLimited)
		return
	}

	// A solo scroll still counts against the cap in the legacy: the counter is
	// incremented before the party branch, for every tier.
	d.events.pesaRuns[tier]++
	withParty := e.Carry[src].Index == t.groupItem

	d.enterPesadelo(w, s, e, tier, secondsLeft, withParty)

	consumeOneItem(&e.Carry[src])
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	d.log.Info("pesadelo entered",
		"account", s.AccountName, "tier", t.name, "party", withParty,
		"seconds", secondsLeft, "runs", d.events.pesaRuns[tier])
}

// enterPesadelo moves the leader in, then any eligible party member, arming each
// client's countdown with the same remaining time.
func (d *Dispatcher) enterPesadelo(w *world.World, s *world.Session, e *world.Entity, tier, secondsLeft int, withParty bool) {
	// Arcano spends one entry per character admitted, the leader included. The
	// caller has already checked the leader has one.
	//
	// The spend is flushed immediately rather than left to the periodic save: an
	// entry is bought with gold, so a crash between here and the next save would
	// otherwise hand it back for free.
	if tier == pesaA {
		e.NightmareTickets--
		w.SaveCharacterAsync(s)
	}
	d.placeInPesadelo(w, s, e, tier, 0, secondsLeft)
	if !withParty {
		return
	}
	for i := 0; i < world.MaxParty; i++ {
		member := e.PartyList[i]
		if member <= 0 || member == e.ID {
			continue
		}
		ms := w.Session(member)
		if ms == nil || ms.Mode != world.UserPlay {
			continue
		}
		me := w.Entity(member)
		if me == nil {
			continue
		}
		// A member of the wrong progression tier is left behind rather than
		// refused — the run still happens for everyone else.
		if !pesadeloAllowed(tier, me.ClassMaster, me.Level) {
			continue
		}
		if tier == pesaA {
			if me.NightmareTickets <= 0 {
				continue
			}
			me.NightmareTickets--
			w.SaveCharacterAsync(ms)
		}
		d.placeInPesadelo(w, ms, me, tier, i, secondsLeft)
	}
}

// placeInPesadelo teleports one character in and starts their countdown.
//
// A character whose save point is already inside the instance lands on it
// instead of the standard slot — the legacy carve-out for someone who set a
// Gema Estelar in there (_MSG_UseItem.cpp:2715/2825).
func (d *Dispatcher) placeInPesadelo(w *world.World, s *world.Session, e *world.Entity, tier, slot, secondsLeft int) {
	t := pesaTierTable[tier]
	x, y := t.spawn[slot][0], t.spawn[slot][1]
	if t.segment(e.SaveX, e.SaveY) {
		x, y = e.SaveX, e.SaveY
	}
	d.doTeleport(w, s, x, y)
	d.sendPesadeloCountdown(w, s, secondsLeft)
}

// sendPesadeloCountdown pushes the window timer to one client (MSG_StartTime,
// the same signal the castle quest and the water dungeon use).
func (d *Dispatcher) sendPesadeloCountdown(w *world.World, s *world.Session, seconds int) {
	body := protocol.EncodeStandardParm(int32(seconds))
	w.SendTo(s, protocol.Header{Type: protocol.MsgStartTime, ID: protocol.IDScene}, body)
}

// useEscrituraPesadelo grants a Celestial thirteen Arcano entries
// (_MSG_UseItem.cpp:3291). The legacy's twenty-hour cooldown is commented out in
// the shipped source, so there is none.
func (d *Dispatcher) useEscrituraPesadelo(w *world.World, s *world.Session, e *world.Entity, src int) {
	e.NightmareTickets += escrituraTickets
	// Flushed immediately: the book costs 10M gold, and losing thirteen entries
	// to a crash before the next periodic save would be a real refund problem.
	w.SaveCharacterAsync(s)
	consumeOneItem(&e.Carry[src])
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	d.log.Info("nightmare entrance tickets granted",
		"account", s.AccountName, "granted", escrituraTickets, "total", e.NightmareTickets)
}

// showNightmareTickets answers "/nt" with the Arcano entries the character holds
// (_DN_CHANGE_COUNT, _MSG_MessageWhisper.cpp:513).
func (d *Dispatcher) showNightmareTickets(w *world.World, s *world.Session) {
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	sendClientMessage(w, s, fmt.Sprintf("Entradas de Pesadelo: %d", e.NightmareTickets))
}

// showNightmareTime answers "/nig" with each tier's schedule.
//
// DELIBERATE DIVERGENCE: the legacy prints the wall clock as "!!HHMMSS" and
// leaves the player to work out the windows from it — the client is what turns
// that into a timer. Since the schedule is server-side knowledge and the three
// tiers stagger, this reports the useful thing directly: what is open now and
// how long until the rest.
func (d *Dispatcher) showNightmareTime(w *world.World, s *world.Session) {
	now := d.now()
	for tier := range pesaTierTable {
		t := pesaTierTable[tier]
		if left, open := t.window(now); open {
			sendClientMessage(w, s, fmt.Sprintf("Pesadelo %s: ABERTO, %ds restantes", t.name, left))
			continue
		}
		wait := t.nextWindow(now)
		sendClientMessage(w, s, fmt.Sprintf("Pesadelo %s: abre em %dm%02ds",
			t.name, int(wait.Minutes()), int(wait.Seconds())%60))
	}
}

// despawnPesadeloMobs is DeleteMobMapa (Server.cpp:9655) for one tier's segment:
// the monsters left over from the run are removed so the next window starts on an
// empty instance instead of inheriting a half-cleared one. Returns how many went.
//
// DELIBERATE DIVERGENCE: the legacy deletes EVERY entity in the segment, NPCs
// included, and lets the generators put them all back. Here NPCs are skipped,
// because DespawnMob only queues a respawn for combat monsters
// (`Merchant == 0 && !NonCombatNPC`, world/api.go) — despawning a shopkeeper
// would delete it until the next server restart. That is not hypothetical: the
// Pesadelo N and M segments each hold a full row of shops (Aki, Balmers, Martin,
// Naomi, Rubyen / Arnold, Lainy, Reimers, RoPerion, Irena_, Smith), which is
// exactly what those two "unidentified villages" in the NPC panel turned out to
// be. Monsters come back on their generator's minute timer, as they do in the
// original.
func (d *Dispatcher) despawnPesadeloMobs(w *world.World, tier int) int {
	t := pesaTierTable[tier]
	// Collect first: DespawnMob frees the entity slot, which ForEachMob is walking.
	var ids []int
	w.ForEachMob(func(id int, e *world.Entity) {
		if !t.segment(e.X, e.Y) || e.Merchant != 0 || e.NonCombatNPC {
			return
		}
		ids = append(ids, id)
	})
	for _, id := range ids {
		// removeType 1 is the legacy DeleteMob(i, 1): it also decrements the
		// generator's population so the minute timer refills the instance.
		w.DespawnMob(id, 1)
	}
	return len(ids)
}

// tickPesadelo runs the per-minute instance wipe. Each tier is cleared one
// minute before it reopens: everyone still inside is recalled out and the run
// counter resets, so the next window starts empty.
//
// lastWipe holds the minute each tier was last wiped, because the tick fires far
// more often than once a minute and the wipe must not repeat inside one.
func (d *Dispatcher) tickPesadelo(w *world.World) {
	minute := d.now().Minute()
	for tier := range pesaTierTable {
		t := pesaTierTable[tier]
		if !t.wipeMinute(minute) || d.events.pesaLastWipe[tier] == minute {
			continue
		}
		d.events.pesaLastWipe[tier] = minute
		d.events.pesaRuns[tier] = 0
		despawned := d.despawnPesadeloMobs(w, tier)
		d.clearArea(w, t.box())
		d.log.Info("pesadelo wiped", "tier", t.name, "minute", minute, "mobs_despawned", despawned)
	}
}
