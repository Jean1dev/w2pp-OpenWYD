package handler

import (
	"github.com/jeanluca/w2pp-openwyd/internal/droprule"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// The Castelo Orc quest's own monsters (Release/TMsrv/run/npc/COrc_*, NPCGener
// blocks world.CasteloOrcGenFirst..Last). The quest is a new rule, not the
// legacy's: a run through Erion's castle, sized for Mortals 320-400, that pays
// in gold and loot and never in experience. The loot table is the Mesa de
// Drops (migration 0051); what the table cannot say — the stat roll on an
// amulet or a ring — is finished here.
//
// Keyed on the template FILE, as the Mesa de Drops is, so a GM's "criar" or a
// moderator's stat override on the same template stays inside the rule, and
// nothing else in the world is touched.
var casteloOrcTemplates = map[string]bool{
	droprule.Canonical("COrc_GraoLorde"): true,
	droprule.Canonical("COrc_Guarda"):    true,
	droprule.Canonical("COrc_Sentinela"): true,
	droprule.Canonical("COrc_Capitao"):   true,
	droprule.Canonical("COrc_Chefe"):     true,
	droprule.Canonical("COrc_Cavaleiro"): true,
	droprule.Canonical("COrc_Arqueiro"):  true,
	droprule.Canonical("COrc_MeioOrc"):   true,
}

func isCasteloOrcMob(mob *world.Entity) bool {
	return mob != nil && mob.TemplateName != "" && casteloOrcTemplates[droprule.Canonical(mob.TemplateName)]
}

// casteloOrcAwardsExp is false for the quest's monsters. The zero lives here and
// not only in the template's Exp: cmd/exptool restamps every monster's Exp from
// the level curve, and Clan 4 — the legacy's own no-experience flag — is the
// summon clan here, which quarters every blow the monster takes (pvp.go) and
// hides it from Tempestade de Raios.
func casteloOrcAwardsExp(mob *world.Entity) bool {
	return !isCasteloOrcMob(mob)
}

// addRoll is one line of an accessory's add table: which effect, and the
// inclusive range its value is drawn from.
type addRoll struct {
	effect   uint8
	min, max int
}

// The amulet and ring add tables the quest design asked for. One line is drawn,
// then one value inside it. The numbers are the item's own (what the tooltip
// shows): EF_CRITICAL and EF_MAGIC are summed over the whole equipment and
// divided by four afterwards, so a lone 1-2 of critical rounds to nothing until
// the rest of the set carries it past a multiple of four.
var (
	casteloOrcAmuletAdds = []addRoll{
		{efMagic, 4, 10},
		{efDamage, 10, 20},
		{efCritical, 1, 2},
		{efHp, 50, 70},
	}
	casteloOrcRingAdds = []addRoll{
		{efMagic, 1, 3},
		{efDamage, 5, 7},
		{efMp, 10, 20},
	}
)

const (
	itemRingFirst   = 501 // Anel de Hércules
	itemRingLast    = 506 // Anel de Hécate
	itemAmuletFirst = 551 // Amuleto de Prata (Special1 +2)
	itemAmuletLast  = 554 // Amuleto de Prata (Special4 +2)
)

// casteloOrcFinish marks one item a quest monster dropped, after the ordinary
// drop bonus (which leaves rings and amulets untouched).
//
// A ring or amulet gets +0 in the first slot — what makes it refinable, as the
// legacy's own amulet reward writes it (_MSG_Quest.cpp:1627-1743) — and one add
// in the second.
func (d *Dispatcher) casteloOrcFinish(w *world.World, mob *world.Entity, it *world.Item) {
	if !isCasteloOrcMob(mob) {
		return
	}
	idx := it.Index
	switch {
	case idx >= itemAmuletFirst && idx <= itemAmuletLast:
		stampAccessoryAdd(w, it, casteloOrcAmuletAdds)
	case idx >= itemRingFirst && idx <= itemRingLast:
		stampAccessoryAdd(w, it, casteloOrcRingAdds)
	}
}

func stampAccessoryAdd(w *world.World, it *world.Item, table []addRoll) {
	line := table[w.Rand().Intn(len(table))]
	value := line.min + w.Rand().Intn(line.max-line.min+1)
	it.Effects[0] = world.Effect{Effect: efSanc, Value: 0}
	it.Effects[1] = world.Effect{Effect: line.effect, Value: uint8(value)}
	it.Effects[2] = world.Effect{}
}

// casteloOrcKeyOneIn is how many paid entries of a Quest 256 arena hand out one
// Castelo Orc key, by the step's quest flag: one in four of the Hydra arena
// (step 4), one in three of the Elf arena (step 5). The arenas' monsters drop
// no key at all — the arenas have no clock and refill themselves, so a key on
// a kill would reward camping inside rather than entering.
var casteloOrcKeyOneIn = map[uint8]int{4: 4, 5: 3}

// casteloOrcKeyOnEntry rolls the key for a player who has just spent a Quest
// 256 ticket on step 4 or 5. Only a paid entry rolls: the Mestre Grifo's free
// ride into the same arenas does not call this, or walking in and out would
// farm keys.
//
// The roll comes from the world-event stream, not the drop stream: the draws
// the drop and refine goldens pin must keep their order.
func (d *Dispatcher) casteloOrcKeyOnEntry(w *world.World, s *world.Session, e *world.Entity, step quest256Step) {
	n, ok := casteloOrcKeyOneIn[step.flag]
	if !ok || d.eventRNG.Intn(n) != 0 {
		return
	}
	slot := firstEmptyAccessibleCarry(e)
	if slot < 0 {
		sendClientMessage(w, s, "Bolsa cheia: a Chave Portão Orc Sul se perdeu.")
		d.log.Info("castelo orc key lost to a full bag", "conn", s.Conn, "quest_flag", step.flag)
		return
	}
	e.Carry[slot] = world.Item{Index: itemChaveCasteloOrc}
	d.sendSlot(w, s, world.ItemPlaceCarry, slot, e.Carry[slot])
	sendClientMessage(w, s, "Você ganhou a Chave Portão Orc Sul: ela abre o Castelo Orc.")
	d.log.Info("castelo orc key on quest entry", "conn", s.Conn, "quest_flag", step.flag)
}
