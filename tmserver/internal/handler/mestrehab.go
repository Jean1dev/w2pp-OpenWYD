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
			"Traga o Retorno da Habilidade e devolverei %d pontos de atributo.", retornoHabilidadePoints))
		return
	}

	slot := -1
	for i := 0; i < activeCarryLimit(e); i++ {
		if e.Carry[i].Index == itemRetornoDaHabilidade {
			slot = i
			break
		}
	}
	if slot < 0 {
		sendSay(w, npc, fmt.Sprintf("Você deve trazer o item %s.", d.itemName(itemRetornoDaHabilidade)))
		return
	}

	refund, taken := refundBuild(e, retornoHabilidadePoints)
	if refund == 0 {
		// Nothing above the class base: the character has no points invested, so
		// there is nothing to give back and the item must not be eaten for it.
		sendSay(w, npc, "Você não tem pontos distribuídos para devolver.")
		return
	}

	e.Carry[slot] = world.Item{}
	d.sendSlot(w, s, world.ItemPlaceCarry, slot, e.Carry[slot])

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
	return sum, taken
}
