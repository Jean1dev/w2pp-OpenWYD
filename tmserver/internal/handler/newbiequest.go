package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// The Campo de Treino newbie quest (TREINADORNEWBIE1..4, _MSG_Quest.cpp:1895-2100).
// The legacy routes the three Treinadores by the template's MOB.Merchant byte
// (offset 17): their CurrentScore.Merchant (100/104/105) collides with the
// Coveiro and the plain trainers, so routing them by it sent Treinador1 into
// the Quest 256 Coveiro case (issue #395). The Chefe_de_Treino is MOB.Merchant
// 100 with EF_GRADE0 16.
const (
	newbieTrainer1Merchant = 36
	newbieTrainer2Merchant = 40
	newbieTrainer3Merchant = 41
	newbieChiefMerchant    = 100
	newbieChiefGrade       = 16

	// freeExpLevel is FREEEXP (Server.cpp:50): the quest is for CurrentScore.Level
	// below it (displayed level 35 or lower).
	freeExpLevel = 35

	newbieWeaponSlot = 6  // Equip[6]: the weapon whose ReqLvl drives the rolls
	newbieMaxWeapon  = 39 // step 2 only rerolls weapons up to ReqLvl 39
	newbieBonusBase  = 50 // SetItemBonus(item, 50 + ReqLvl, 1, 0)
	newbieRefine     = 3  // step 3 refines Equip[1..7] to +3

	newbieAffectSkill = 44  // SetAffect(conn, 44, 200, 200)
	newbieAffectTime  = 200 // SetAffect time and level
)

// Rewards: 682 is the Caixa de Pocao de Cura, handed out as a stack of 20.
const (
	itemNewbieHealBox int16 = 682
	itemNewbieReward2 int16 = 481
	itemNewbieReward3 int16 = 652
	newbieHealBoxAmt        = 20
)

// newbieStep is one Treinador: the key it asks for and its SendSay /
// SendClientMessage lines (Language.txt 230-245, Windows-1252 bytes).
type newbieStep struct {
	key                                int16
	already, cheerup, complete, reward string
}

var newbieSteps = [4]newbieStep{
	{451, "Deus te aben\xe7oe.",
		"Se n\xe3o possuir a chave Gremlin Isca, n\xe3o poderei lhe dar nada.",
		"Come\xe7ou. Pegue isto e siga em frente.",
		"Recebeu Caixa de Po\xe7\xe3o de Cura (20) como recompensa."},
	{452, "Voc\xea \xe9 um cara de sorte.",
		"Se n\xe3o possuir a chave para o portal 2, n\xe3o poderei ajud\xe1-lo.",
		"Sua arma parece ruim. Posso resolver isto.",
		"Como recompensa, a op\xe7\xe3o da arma foi alterada."},
	{453, "Prepara\xe7\xf5es est\xe3o conclu\xeddas exceto por uma coisa.",
		"Tome a chave que pertence ao Orc Atirador.",
		"Aqui, isto lhe dar\xe1 for\xe7a.",
		"Armas e Armaduras foram refinadas."},
	{524, "H\xe1 um Castelo Orc ao norte de Armia.",
		"Traga-me a Capa do Orc ou n\xe3o haver\xe1 recompensa.",
		"Voc\xea n\xe3o \xe9 mais um iniciante. Precisar\xe1 disto.",
		"Ganhou um pr\xeamio por finalizar a quest."},
}

// newbieTrainerStep returns the quest step (1..4) an NPC hands out, read like
// the legacy from the template's MOB.Merchant byte.
func newbieTrainerStep(npc *world.Entity) (int, bool) {
	if npc == nil || len(npc.Template) <= 17 {
		return 0, false
	}
	switch npc.Template[17] {
	case newbieTrainer1Merchant:
		return 1, true
	case newbieTrainer2Merchant:
		return 2, true
	case newbieTrainer3Merchant:
		return 3, true
	case newbieChiefMerchant:
		if npc.Grade == newbieChiefGrade {
			return 4, true
		}
	}
	return 0, false
}

// newbieQuest runs one Treinador step (_MSG_Quest.cpp:1895-2100). Each step
// needs a Mortal below FREEEXP whose QuestInfo.Mortal.Newbie is the previous
// step, and the step's key in the inventory; the key is only consumed by the
// Chefe_de_Treino. The legacy case never checks confirm.
func (d *Dispatcher) newbieQuest(w *world.World, s *world.Session, e, npc *world.Entity, step int) {
	st := newbieSteps[step-1]
	if e.ClassMaster != classMasterMortal || e.Level >= freeExpLevel || int(e.NewbieQuest) != step-1 {
		d.sendNPCChatText(w, s, npc.ID, st.already)
		return
	}
	slot := -1
	for i := 0; i < activeCarryLimit(e); i++ {
		if e.Carry[i].Index == st.key {
			slot = i
			break
		}
	}
	if slot < 0 {
		d.sendNPCChatText(w, s, npc.ID, st.cheerup)
		return
	}

	e.NewbieQuest = uint8(step)
	d.sendNPCChatText(w, s, npc.ID, st.complete)
	d.sendClientMessage(w, s, st.reward)
	d.log.Info("newbie quest complete", "conn", s.Conn, "name", e.Name, "step", step)
	// The flag gates the next Treinador; save it now like the other QuestInfo
	// gates (amuMistico) so a crash before the periodic save cannot replay it.
	defer w.SaveCharacterThen(s, func(*world.World, *world.Session) {})

	switch step {
	case 1:
		d.newbiePutItem(w, s, e, healBox())
	case 2:
		weapon := e.Equip[newbieWeaponSlot]
		if weapon.Index > 0 && int(weapon.Index) < maxItemList {
			req := int(d.itemReqs[int(weapon.Index)].Lvl)
			if req > newbieMaxWeapon {
				// The legacy breaks out here: no affect, score or carry refresh.
				return
			}
			d.newbieReroll(w, s, e, newbieWeaponSlot, req, false)
		}
	case 3:
		for j := 1; j < 8; j++ {
			// Parity quirk: every slot reads the weapon's ReqLvl, not its own
			// (_MSG_Quest.cpp:2018).
			weapon := e.Equip[newbieWeaponSlot]
			if weapon.Index > 0 && int(weapon.Index) < maxItemList {
				d.newbieReroll(w, s, e, j, int(d.itemReqs[int(weapon.Index)].Lvl), true)
			}
		}
	case 4:
		e.Carry[slot] = world.Item{}
		var reward world.Item
		switch w.Rand().Intn(3) {
		case 0:
			reward = healBox()
		case 1:
			reward = world.Item{Index: itemNewbieReward2}
		default:
			reward = world.Item{Index: itemNewbieReward3}
		}
		d.newbiePutItem(w, s, e, reward)
	}

	d.questAffect(e, newbieAffectSkill, newbieAffectTime, newbieAffectTime)
	d.refreshScore(e)
	d.sendScore(w, s, e)
	d.sendAffect(w, s, e)
	d.sendCarry(w, s, e)
}

func healBox() world.Item {
	return world.Item{Index: itemNewbieHealBox, Effects: [3]world.Effect{{Effect: efAmount, Value: newbieHealBoxAmt}}}
}

// newbieReroll clears an equip slot's three effects and rolls them again with
// SetItemBonus(item, 50 + ReqLvl, 1, 0), optionally refining it to +3.
func (d *Dispatcher) newbieReroll(w *world.World, s *world.Session, e *world.Entity, slot, reqLvl int, sanc bool) {
	it := &e.Equip[slot]
	it.Effects = [3]world.Effect{}
	refine.ItemBonus(it, newbieBonusBase+reqLvl, true, 0, d.bonusItem(it.Index), w.Rand().Rand)
	if sanc {
		refine.Set(it, newbieRefine, 0)
	}
	d.sendSlot(w, s, world.ItemPlaceEquip, slot, *it)
}

// bonusItem gathers the STRUCT_ITEMLIST fields SetItemBonus reads.
func (d *Dispatcher) bonusItem(index int16) refine.BonusItem {
	idx := int(index)
	info := refine.BonusItem{Unique: d.itemUnique[idx], ReqLvl: int(d.itemReqs[idx].Lvl), Pos: d.itemPos[idx]}
	for _, ef := range d.itemEffects[idx] {
		info.Static = append(info.Static, refine.StaticEffect{Effect: ef.Eff, Value: ef.Val})
	}
	return info
}

// newbiePutItem is PutItem (Server.cpp:1059): first free carry slot, or the
// no-space notice and the item is lost.
func (d *Dispatcher) newbiePutItem(w *world.World, s *world.Session, e *world.Entity, it world.Item) {
	slot := firstEmptyAccessibleCarry(e)
	if slot < 0 {
		d.notify(w, s, NoticeNoSpaceToTrade)
		return
	}
	e.Carry[slot] = it
	d.sendSlot(w, s, world.ItemPlaceCarry, slot, it)
}

// questAffect is the legacy SetAffect(conn, skill, time, level) the quest NPCs
// use to buff the player, reading the affect from the skill catalog.
func (d *Dispatcher) questAffect(e *world.Entity, skill, delay, level int) {
	if d.spells == nil {
		return
	}
	sp, ok := d.spells.Get(skill)
	if !ok {
		return
	}
	e.SetAffect(sp.AffectType, sp.AffectValue, sp.AffectTime, sp.Aggressive, delay, level, d.affectDur)
}
