package handler

import (
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Sub-Celestial (issue #410): the Pedra Ideal turns a Celestial into a
// CELESTIALCS by writing a fresh second sheet, SaveCelestial[1], and the Urnammu
// changes that sheet's class. Both leave the active (Celestial) sheet alone; the
// Pedra Misteriosa 4148 swaps the active sheet with the parked one.
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

// pedraMisteriosaKeptAffects are the affect types the swap leaves in place
// (_MSG_UseItem.cpp:3244-3250): 4, 8, 29, the Frango Assado, the Divine and
// Vigor potions and the Baú de Experiência. Everything else is cleared.
var pedraMisteriosaKeptAffects = map[uint8]bool{
	4: true, 8: true, 29: true,
	world.AffectForceMobDamage: true, world.AffectDivine: true,
	world.AffectVigor: true, world.AffectExpChest: true,
}

// usePedraMisteriosa ports the Pedra Misteriosa (4148, _MSG_UseItem.cpp:3186-3289):
// inside a village, a CELESTIALCS or SCELESTIAL parks the active sheet in
// SaveCelestial and plays the other one. Outside those tiers the legacy only
// resends the stone; outside a village it also says _NN_OnlyVillage.
//
// Parity quirks kept on purpose:
//   - Live HP/MP carry over from the sheet being left (GetCurrentScore keeps
//     CurrentScore.Hp/Mp) and are only clamped to the new maxima.
//   - BASE_GetHpMp runs before GetCurrentScore, so the new BaseMaxHP/MaxMP use
//     the level of the sheet being *left* (CurrentScore.Level is still stale).
//     SkillBonus is not persisted here and is re-derived at every login, so it
//     uses the new level instead of a stale one that would not survive a relog.
//   - The stone is consumed after the swap, and nothing is saved by the legacy
//     here; we save anyway so a crash cannot undo the swap the client was shown.
func (d *Dispatcher) usePedraMisteriosa(w *world.World, s *world.Session, e *world.Entity, src int) {
	if e.ClassMaster != classMasterCelestialCS && e.ClassMaster != classMasterSCelestial {
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}
	if v := world.Village(e.X, e.Y); v < 0 || v >= 5 {
		d.notify(w, s, NoticeOnlyVillage)
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}

	cl, ncl := 0, subCelestialSheet
	if e.ClassMaster == classMasterCelestialCS {
		e.ClassMaster = classMasterSCelestial
		e.CelestialLevel = int16(e.Level)
	} else {
		cl, ncl = subCelestialSheet, 0
		e.ClassMaster = classMasterCelestialCS
		e.SubCelestialLevel = int16(e.Level)
	}
	oldClass, oldLevel := e.Class, e.Level
	e.SaveCelestial[cl] = parkCelestialSheet(s, e)
	loadCelestialSheet(s, e, e.SaveCelestial[ncl])

	for i := range e.Affect {
		if !pedraMisteriosaKeptAffects[e.Affect[i].Type] {
			e.Affect[i] = world.Affect{}
		}
	}

	// Equip[0] = MortalFace + 5 + Class. The tmServer does not keep MortalFace,
	// but nothing rewrites a player's face after the Arch was made with that same
	// formula, so it is recovered from the current face and class.
	if face := e.Equip[0].Index; face > 0 {
		if mortalFace := int32(face) - 5 - int32(oldClass); mortalFace > 0 {
			e.Equip[0].Index = int16(mortalFace + 5 + int32(e.Class))
		}
	}
	d.sendSlot(w, s, world.ItemPlaceEquip, 0, e.Equip[0])
	w.SendTo(s, protocol.Header{Type: protocol.MsgSetShortSkill, ID: protocol.IDScene},
		protocol.EncodeSetShortSkill(e.SkillBar, s.ShortSkill))

	d.deriveSkillBonus(e)
	e.ScoreBonus = scoreBonusFor(e)
	e.BaseMaxHP, e.BaseMaxMP = celestialBaseHpMp(e.Class, e.BaseCon, e.BaseInt, oldLevel)
	d.refreshEquip(w, s, e) // new face on the model + GetCurrentScore + SendScore
	d.sendAffect(w, s, e)
	d.sendEtc(w, s, e)

	consumeOneItem(&e.Carry[src])
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	w.SaveCharacterThen(s, func(*world.World, *world.Session) {})
	d.log.Info("mystery stone change god2god", "conn", s.Conn, "name", e.Name,
		"classMaster", e.ClassMaster, "class", oldClass, "to", e.Class)
}

// parkCelestialSheet snapshots the active sheet into a SaveCelestial entry.
func parkCelestialSheet(s *world.Session, e *world.Entity) world.CelestialSheet {
	return world.CelestialSheet{
		Class:           int32(e.Class),
		Exp:             e.Exp,
		SPX:             e.SaveX,
		SPY:             e.SaveY,
		Level:           e.Level,
		AC:              e.BaseAC,
		Damage:          e.BaseDamage,
		MaxHP:           e.BaseMaxHP,
		MaxMP:           e.BaseMaxMP,
		HP:              e.HP,
		MP:              e.MP,
		Str:             e.BaseStr,
		Int:             e.BaseInt,
		Dex:             e.BaseDex,
		Con:             e.BaseCon,
		Special:         e.BaseSpecial,
		LearnedSkill:    e.LearnedSkill,
		SecLearnedSkill: e.SecLearnedSkill,
		ScoreBonus:      e.ScoreBonus,
		SpecialBonus:    e.SpecialBonus,
		SkillBonus:      e.SkillBonus,
		SkillBar1:       e.SkillBar,
		SkillBar2:       s.ShortSkill,
		Soul:            e.Soul,
	}
}

// loadCelestialSheet makes sh the active sheet. Live HP/MP are kept (see
// usePedraMisteriosa); BaseAC/BaseDamage are re-derived from the new tier and
// level like at login, since the tmServer never persists them.
func loadCelestialSheet(s *world.Session, e *world.Entity, sh world.CelestialSheet) {
	e.Class = uint8(sh.Class)
	e.Exp = sh.Exp
	e.SaveX, e.SaveY = sh.SPX, sh.SPY
	e.Level = sh.Level
	e.BaseMaxHP, e.BaseMaxMP = sh.MaxHP, sh.MaxMP
	e.BaseStr, e.BaseInt, e.BaseDex, e.BaseCon = sh.Str, sh.Int, sh.Dex, sh.Con
	e.BaseSpecial = sh.Special
	e.LearnedSkill = sh.LearnedSkill
	e.SecLearnedSkill = sh.SecLearnedSkill
	e.ScoreBonus = sh.ScoreBonus
	e.SpecialBonus = sh.SpecialBonus
	e.SkillBonus = sh.SkillBonus
	e.SkillBar = sh.SkillBar1
	s.ShortSkill = sh.SkillBar2
	e.Soul = sh.Soul
	e.BaseAC = playerBaseAC(e)
	e.BaseDamage = playerBaseDamage(e)
}

// celestialBaseHpMp is BASE_GetHpMp (Basedef.cpp:1214-1248) for the celestial
// tiers, whose level term adds MAX_LEVEL: class base + 2 per Con/Int above the
// class base + IncHP/IncMP per level, capped at MAX_HP/MAX_MP.
func celestialBaseHpMp(cls uint8, baseCon, baseInt int16, lvl int32) (maxHP, maxMP int32) {
	base := level.BaseStats(cls)
	maxHP = base[4] + (int32(baseCon)-base[3])*2 + (lvl+level.MaxLevel)*level.IncHP(cls)
	maxMP = base[5] + (int32(baseInt)-base[1])*2 + (lvl+level.MaxLevel)*level.IncMP(cls)
	return min(maxHP, level.MaxHPCap), min(maxMP, level.MaxMPCap)
}
