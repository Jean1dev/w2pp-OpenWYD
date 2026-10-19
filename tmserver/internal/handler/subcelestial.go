package handler

import (
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Sub-Celestial (issue #410): the Pedra Ideal turns a Celestial into a
// CELESTIALCS by writing a fresh second sheet, SaveCelestial[1], and the Urnammu
// changes that sheet's class. Both leave the active (Celestial) sheet alone; the
// Pedra Misteriosa 4148 that swaps the two sheets is not ported yet.
const (
	subCelestialMinLevel   = 120 // CurrentScore.Level, inclusive (_MSG_UseItem.cpp:3032)
	subCelestialAC         = 230
	subCelestialSpecialPts = 855
	// learnedSkillKeepMask keeps the top byte of LearnedSkill (the class-free
	// bits, e.g. bit 30 that the Celestial turn sets) and drops the skills.
	learnedSkillKeepMask int32 = -0x01000000 // 0xFF000000

	urnammuMerchant     = 76   // MOB.Merchant of Urnammu/Urnamu (npcMerc == 76, _MSG_Quest.cpp:154)
	urnammuFameCost     = 2000 // _MSG_Quest.cpp:1262
	urnammuSpecialReset = 55   // per-Special refund cap (_MSG_Quest.cpp:1285)
	subCelestialSheet   = 1    // SaveCelestial index of the Sub-Celestial
)

// Language.txt lines, in the table's Windows-1252 bytes.
const (
	msgWantEquipItem = "Voc\xea deve equipar %s."              // _SN_WANTEQUIPITEM (477)
	msgNeedFame      = "Voc\xea precisa de %d pontos de fama." // _DN_NEEDFAME (523)
)

// equippedSephirotClass returns the 0-based class of the Sephirot (1760..1763)
// in Equip[11].
func equippedSephirotClass(e *world.Entity) (int32, bool) {
	idx := e.Equip[sephirotEquipSlot].Index
	if idx < archSephirotMin || idx > archSephirotMax {
		return 0, false
	}
	return int32(idx - archSephirotMin), true
}

// createSubCelestial is the CELESTIAL leg of the Pedra Ideal (Vol 211,
// _MSG_UseItem.cpp:3026-3085). Without a Sephirot or below level 120 the legacy
// only resends the stone (SendItem). Otherwise the character becomes CELESTIALCS
// and SaveCelestial[1] gets a level-0 sheet of the Sephirot's class; the active
// Celestial sheet is not touched, so nothing the player has is overwritten.
//
// Parity quirks kept on purpose: Str/Int/Dex/Con come from the *current* class
// (BaseSIDCHM[MOB.Class]) while HP/MP come from the Sephirot's class, and the
// fields the legacy does not assign (SPX/SPY, ScoreBonus, SkillBonus, the skill
// bars, SecLearnedSkill) keep whatever the slot already held.
func (d *Dispatcher) createSubCelestial(w *world.World, s *world.Session, e *world.Entity, src int) {
	cls, ok := equippedSephirotClass(e)
	if !ok || e.Level < subCelestialMinLevel {
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}
	e.ClassMaster = classMasterCelestialCS

	attrs := level.BaseStats(e.Class)
	hpmp := level.BaseStats(uint8(cls))
	sub := &e.SaveCelestial[subCelestialSheet]
	sub.Class = cls
	sub.LearnedSkill = e.LearnedSkill & learnedSkillKeepMask
	sub.Level = 0
	sub.Str, sub.Int, sub.Dex, sub.Con = int16(attrs[0]), int16(attrs[1]), int16(attrs[2]), int16(attrs[3])
	sub.Special = [4]int16{}
	sub.AC, sub.Damage = subCelestialAC, 0
	sub.HP, sub.MaxHP = hpmp[4], hpmp[4]
	sub.MP, sub.MaxMP = hpmp[5], hpmp[5]
	sub.SpecialBonus = subCelestialSpecialPts
	sub.Exp = 0
	sub.Soul = e.Soul

	e.Equip[sephirotEquipSlot] = world.Item{}
	d.sendSlot(w, s, world.ItemPlaceEquip, sephirotEquipSlot, e.Equip[sephirotEquipSlot])
	consumeOneItem(&e.Carry[src])
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	d.sendClientMessage(w, s, msgKingBless1)
	w.SaveCharacterThen(s, func(*world.World, *world.Session) {})
	d.log.Info("ideal stone used to make god2", "conn", s.Conn, "name", e.Name, "subclass", cls)
}

// isUrnammuNPC reads MOB.Merchant from the template like the legacy (byte 17).
func isUrnammuNPC(npc *world.Entity) bool {
	return npc != nil && len(npc.Template) > 17 && npc.Template[17] == urnammuMerchant
}

// urnammu ports the URNAMMU case (_MSG_Quest.cpp:1248-1311): a CELESTIALCS with
// a Sephirot equipped and 2000 Fame changes the class of the parked Sub sheet.
// The legacy never checks confirm. The attributes are reset from the *active*
// class (BaseSIDCHM[MOB.Class]), each Special refunds at most 55 points into the
// sheet's SpecialBonus, and the skill bars and learned skills are cleared.
func (d *Dispatcher) urnammu(w *world.World, s *world.Session, e *world.Entity) {
	if e.ClassMaster != classMasterCelestialCS {
		d.sendClientMessage(w, s, fmt.Sprintf(msgNeedClass, "Sub Celestial"))
		return
	}
	cls, ok := equippedSephirotClass(e)
	if !ok {
		d.sendClientMessage(w, s, fmt.Sprintf(msgWantEquipItem, "o Sephirot da classe escolhida"))
		return
	}
	if e.Fame < urnammuFameCost {
		d.sendClientMessage(w, s, fmt.Sprintf(msgNeedFame, urnammuFameCost))
		return
	}
	d.log.Info("change SubClass", "conn", s.Conn, "name", e.Name, "class", e.Class, "to", cls, "level", e.Level)
	e.Fame -= urnammuFameCost

	sub := &e.SaveCelestial[subCelestialSheet]
	sub.Class = cls
	attrs := level.BaseStats(e.Class)
	sub.Str, sub.Int, sub.Dex, sub.Con = int16(attrs[0]), int16(attrs[1]), int16(attrs[2]), int16(attrs[3])
	sub.SkillBar1, sub.SkillBar2 = [4]uint8{}, [16]uint8{}

	special := int32(sub.SpecialBonus)
	for i, v := range sub.Special {
		if v <= urnammuSpecialReset {
			special += int32(v)
			sub.Special[i] = 0
		} else {
			special += urnammuSpecialReset
			sub.Special[i] -= urnammuSpecialReset
		}
	}
	sub.SpecialBonus = uint16(special) // unsigned short in the legacy sheet
	sub.LearnedSkill &= learnedSkillKeepMask

	e.Equip[sephirotEquipSlot] = world.Item{}
	d.sendSlot(w, s, world.ItemPlaceEquip, sephirotEquipSlot, e.Equip[sephirotEquipSlot])
	d.sendClientMessage(w, s, msgKingBless1)
	w.SaveCharacterThen(s, func(*world.World, *world.Session) {})
}
