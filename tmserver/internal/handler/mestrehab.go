package handler

import (
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// itemRetornoDaHabilidade is ItemList.csv #3336, the premium stat reset. It
// carries EF_VOLATILE 0, which is the "equippable" class, so clicking it in the
// bag falls into the equip path and does nothing — the item is spent by handing
// it to the Mestre de Habilidade, exactly as its own tooltip says.
const itemRetornoDaHabilidade = 3336

// The sapphire payment the Mestre de Habilidade also takes: item 697 is one
// Safira and 4131 is the Pacote_Safiras(10), which the original counts as ten
// whatever its stack really holds (_MSG_Quest.cpp:1775) — it eats the whole item
// either way, so a short pack is spent in the player's favour.
const (
	itemSafira       = 697
	itemPacoteSafira = 4131
	pacoteSafiraVale = 10
)

// statSapphireCost is the legacy StatSapphire config, which this server never had:
// the number of sapphires one reset costs. sapphireRefundPoints is what that reset
// gives back, and the pair is exactly what the shipped line _DN_Want_Stat_Init
// promises — "100 pontos de atributos serão redistribuídos se trouxer %d safiras".
const (
	statSapphireCost     = 10
	sapphireRefundPoints = 100
)

// retornoHabilidadePoints is how many attribute points the Mestre de Habilidade
// hands back for one Retorno da Habilidade.
//
// DECIDED RATE, and a large divergence from the legacy: _MSG_Quest.cpp:1826 uses
// `resetp = 100` applied to EACH attribute, so it returns at most 400 and at most
// 100 from any single one. That ceiling makes the item nearly worthless on a real
// build — a BeastMaster with everything in CON gets 100 points back out of three
// thousand spent, because the other three attributes are already sitting on the
// class base and have nothing to give.
//
// Here the 1000 is a budget over the WHOLE build instead of a per-attribute cap.
const retornoHabilidadePoints = 1000

// skillMasterReset is the Mestre de Habilidade (Merchant 31, MESTREHAB —
// _MSG_Quest.cpp:1745). It refunds attribute points for a Retorno da Habilidade.
//
// The refund never invents a point and never touches the class base. Free points
// in this port are DERIVED, not stored: level.ScoreBonus is granted minus spent,
// where spent is the sum of the four attributes above baseSIDCHM. So lowering the
// attributes is the whole operation — the points reappear on their own, and they
// cannot exceed what the character actually paid.
//
// The sapphire path the legacy also accepts here (items 697 and 4131, priced by
// the StatSapphire config) is NOT ported: that config does not exist in this
// server yet, and inventing a price is worse than leaving the free path alone.
func (d *Dispatcher) skillMasterReset(w *world.World, s *world.Session, e *world.Entity, npc *world.Entity, confirm int32) {
	if confirm == 0 {
		sendSay(w, npc, fmt.Sprintf(
			"Traga o Retorno da Habilidade para %d pontos, ou %d safiras para %d.",
			retornoHabilidadePoints, statSapphireCost, sapphireRefundPoints))
		return
	}

	// The Retorno da Habilidade wins when both are in the bag: it is the paid item
	// and it is worth ten times the sapphire reset, so spending the sapphires while
	// it sits there would be the expensive mistake. The original picks it first for
	// the same reason (_MSG_Quest.cpp:1788).
	budget := int32(retornoHabilidadePoints)
	var cost []int
	if slot := retornoSlot(e); slot >= 0 {
		cost = []int{slot}
	} else {
		cost = sapphireSlotsToSpend(e)
		if cost == nil {
			sendSay(w, npc, fmt.Sprintf(
				"Traga o %s, ou %d safiras.", d.itemName(itemRetornoDaHabilidade), statSapphireCost))
			return
		}
		budget = sapphireRefundPoints
	}

	// Count the refund BEFORE charging: a character with nothing above the class
	// base must not pay for zero points.
	refund, taken := refundBuild(e, budget)
	if refund == 0 {
		sendSay(w, npc, "Você não tem pontos distribuídos para devolver.")
		return
	}
	d.clearSlots(w, s, e, cost)

	// Re-derive rather than add: ScoreBonus is a pure function of level, tier and
	// the attributes we just lowered (character.go:361). Adding by hand here would
	// be the one way to conjure a point that was never spent.
	e.ScoreBonus = uint16(level.ScoreBonus(scoreBonusInput(e)))
	d.refreshScore(e)
	d.sendScore(w, s, e)
	d.sendEtc(w, s, e)

	sendSay(w, npc, fmt.Sprintf("Devolvi %d pontos de atributo.", refund))
	d.log.Info("retorno da habilidade",
		"conn", s.Conn, "name", e.Name, "refund", refund,
		"str", taken[0], "int", taken[1], "dex", taken[2], "con", taken[3],
		"score_bonus", e.ScoreBonus)
}

// retornoSlot is the bag slot holding a Retorno da Habilidade, or -1.
func retornoSlot(e *world.Entity) int {
	for i := 0; i < activeCarryLimit(e); i++ {
		if e.Carry[i].Index == itemRetornoDaHabilidade {
			return i
		}
	}
	return -1
}

// countSapphires values a loose Safira at one and a Pacote at ten, the way the
// original counts them.
func countSapphires(e *world.Entity) int {
	n := 0
	for i := 0; i < activeCarryLimit(e); i++ {
		switch e.Carry[i].Index {
		case itemSafira:
			n++
		case itemPacoteSafira:
			n += pacoteSafiraVale
		}
	}
	return n
}

// sapphireSlotsToSpend picks exactly which slots pay for one reset. A Pacote is
// only taken while ten are still owed, so settling a debt of four never eats a
// whole pack when four loose stones would do — the original scans in the same
// order (_MSG_Quest.cpp:1802-1816). Returns nil when the bag cannot pay.
//
// Kept separate from the spending so the choice can be tested on its own: it is
// the part with a rule in it, and the part a mistake would be expensive in.
func sapphireSlotsToSpend(e *world.Entity) []int {
	owed := statSapphireCost
	var slots []int
	for i := 0; i < activeCarryLimit(e) && owed > 0; i++ {
		switch {
		case e.Carry[i].Index == itemSafira:
			slots = append(slots, i)
			owed--
		case e.Carry[i].Index == itemPacoteSafira && owed >= pacoteSafiraVale:
			slots = append(slots, i)
			owed -= pacoteSafiraVale
		}
	}
	if owed > 0 {
		return nil
	}
	return slots
}

// clearSlots empties the chosen slots and tells the client about each one.
func (d *Dispatcher) clearSlots(w *world.World, s *world.Session, e *world.Entity, slots []int) {
	for _, i := range slots {
		e.Carry[i] = world.Item{}
		d.sendSlot(w, s, world.ItemPlaceCarry, i, e.Carry[i])
	}
}

func maxInt32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

// refundBuild lowers the four attributes by up to budget points TOTAL and reports
// how many it took, plus the split. It never goes below the class base, so a
// character can only ever get back what it put in.
//
// The split is proportional to what each attribute holds above the base, which
// keeps the shape of the build: a character that was two thirds CON stays two
// thirds CON, one thousand points lighter. Taking in attribute order instead
// would empty STR and INT before touching CON and hand back a different
// character than the one that walked in.
func refundBuild(e *world.Entity, budget int32) (int32, [4]int32) {
	base := level.BaseAttributes(e.Class)
	attrs := [4]*int16{&e.BaseStr, &e.BaseInt, &e.BaseDex, &e.BaseCon}

	var spare [4]int32
	var total int32
	for i, p := range attrs {
		if v := int32(*p) - base[i]; v > 0 {
			spare[i] = v
			total += v
		}
	}
	if total == 0 {
		return 0, [4]int32{}
	}
	if budget > total {
		budget = total // "o máximo é o máximo de pontos": never more than was spent
	}

	var taken [4]int32
	var sum int32
	for i := range attrs {
		t := spare[i] * budget / total // floor; the remainder is settled below
		taken[i] = t
		sum += t
	}
	// Integer division loses up to three points. Hand them out one at a time to
	// whoever still has room, so the refund is exactly the budget and not 997.
	for i := 0; sum < budget; i = (i + 1) % 4 {
		if taken[i] < spare[i] {
			taken[i]++
			sum++
		}
	}
	for i, p := range attrs {
		*p = int16(int32(*p) - taken[i])
	}
	// Undo the OTHER half of the allocation. applyScoreBonus does two things per
	// point (misc.go:56-64): it raises the attribute AND, for INT and CON, adds
	// 2×points straight into BaseMaxMP/BaseMaxHP. Lowering only the attribute left
	// the pool behind, so a reset handed back the points while keeping the HP and
	// MP they had bought — spend them again and both grow for free, which turns the
	// item into a pool printer instead of a respec.
	e.BaseMaxMP = maxInt32(e.BaseMaxMP-2*taken[1], 0)
	e.BaseMaxHP = maxInt32(e.BaseMaxHP-2*taken[3], 0)
	return sum, taken
}
