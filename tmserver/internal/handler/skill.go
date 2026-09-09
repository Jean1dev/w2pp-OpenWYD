package handler

import (
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// eighthSkillCoin is the gold price of the exclusive 8th skill of a tree
// (_MSG_ApplyBonus.cpp: 50M regardless of tier).
const eighthSkillCoin = 50_000_000

// deriveSkillBonus recomputes the free skill points from level and the learned
// mask — SkillBonus is DERIVED, not persisted (BASE_GetBonusSkillPoint,
// Basedef.cpp:850, called on character load): level*3 (+1/level past 199)
// minus the SkillPoint cost of every learned class skill (bits 0-23; the
// Sephira bits 24-31 are book-taught and cost nothing).
//
// The tier adders are part of the formula, not a refinement of it: an Arch gets
// +112 and every celestial tier +1500 (Basedef.cpp:854-860). Leaving them out is
// why a freshly reborn Celestial had no skill points at all — its level is 0, so
// level*3 grants nothing and the whole allowance is the flat 1500.
//
// Still not modeled: the PilulaOrc quest (+9), which belongs to the Mortal quest
// line this port has not reached. That under-grants a character who did it by 9
// points rather than inventing them.
//
// A negative rest clamps to 0 (the legacy would wrap an unsigned field).
func (d *Dispatcher) deriveSkillBonus(e *world.Entity) {
	if d.spells == nil {
		e.SkillBonus = 0
		return
	}
	per := int(e.Level)*3 + skillTierBonus(e.ClassMaster)
	if mod := int(e.Level) - 199; mod > 0 {
		per += mod
	}
	spent := 0
	for j := 0; j < content.MaxSkill; j++ {
		if e.LearnedSkill&(1<<j) == 0 {
			continue
		}
		if sp, ok := d.spells.Get(int(e.Class)*content.MaxSkill + j); ok {
			spent += sp.SkillPoint
		}
	}
	rest := per - spent
	if rest < 0 {
		rest = 0
	}
	e.SkillBonus = uint16(rest)
}

// SkillItemFirst/SkillItemLast bound the catalog rows that are SKILLS rather than
// merchandise: g_pItemList[5000+idx] holds a skill's learn requirements, and
// learnSkill refuses anything outside the range.
//
// Exported because the range is the only way to tell a class master's menu from
// a shop's stock. The 96 rows in that menu sit in the NPC's Carry like goods do,
// but nothing about them is a purchase: learning costs SKILL POINTS and never
// touches Carry, so "buying" one puts a useless row in the bag and teaches
// nothing. A boot audit of unpriced stock has to count them apart or its number
// never reaches zero.
const (
	SkillItemFirst = 5000
	SkillItemLast  = 5095
)

// IsSkillItem reports whether a catalog index is a learnable skill row.
func IsSkillItem(index int) bool { return index >= SkillItemFirst && index <= SkillItemLast }

// learnSkill is ApplyBonus BonusType==2 (_MSG_ApplyBonus.cpp:131-249): learn the
// skill g_pItemList index Detail (5000+idx) from a class-master NPC (TargetID).
// Runs in the loop goroutine.
func (d *Dispatcher) learnSkill(w *world.World, s *world.Session, e *world.Entity, detail int, targetID int) {
	if d.spells == nil || !IsSkillItem(detail) {
		d.log.Info("learn skill refused: detail out of range",
			"conn", s.Conn, "account", s.AccountName, "detail", detail,
			"catalog", d.spells != nil)
		return
	}
	if targetID < world.MaxUser || targetID >= world.MaxMob {
		d.log.Info("learn skill refused: target is not an NPC",
			"conn", s.Conn, "account", s.AccountName, "detail", detail, "target", targetID)
		return // learn requests must come through an NPC
	}
	skillclass := (detail - SkillItemFirst) / content.MaxSkill
	skillpos := (detail - SkillItemFirst) % content.MaxSkill
	if int(e.Class) != skillclass {
		d.log.Info("learn skill refused: another class's skill",
			"conn", s.Conn, "account", s.AccountName, "detail", detail,
			"class", e.Class, "skill_class", skillclass)
		d.notify(w, s, NoticeOtherClassSkill)
		sendClientMessage(w, s, msgOtherClassSkill)
		return
	}
	sp, ok := d.spells.Get(skillpos + content.MaxSkill*skillclass)
	if !ok {
		d.log.Info("learn skill refused: no such row in SkillData",
			"conn", s.Conn, "account", s.AccountName, "detail", detail,
			"spell", skillpos+content.MaxSkill*skillclass)
		return
	}
	// What the affordability check measures is NOT the character's points for a
	// celestial: the legacy substitutes a flat 1500 there
	// (_MSG_ApplyBonus.cpp:145-146), while the debit below still comes off the
	// real balance. So a celestial may learn any skill priced under 1500 no
	// matter what it has left — an intentional allowance for a tier that is
	// rebuilding its whole kit at level 0, reproduced rather than "corrected".
	affordable := int(e.SkillBonus)
	if isCelestialTier(e.ClassMaster) {
		affordable = skillBonusCelestial
	}
	if sp.SkillPoint > affordable {
		d.log.Info("learn skill refused: not enough points",
			"conn", s.Conn, "account", s.AccountName, "skill", skillpos,
			"cost", sp.SkillPoint, "have", e.SkillBonus, "affordable", affordable)
		d.notify(w, s, NoticeNotEnoughSkillPoint)
		sendClientMessage(w, s, fmt.Sprintf("%s (custa %d, você tem %d)",
			msgNotEnoughSkillPoint, sp.SkillPoint, affordable))
		return
	}
	isEighth := skillpos == 7 || skillpos == 15 || skillpos == 23
	if isEighth {
		// The 8th skill of each tree is exclusive: only one of the three, only
		// after the tree's 7 previous skills, and it costs 50M gold on top.
		if e.LearnedSkill&(1<<7|1<<15|1<<23) != 0 {
			d.log.Info("learn skill refused: an 8th is already learned",
				"conn", s.Conn, "account", s.AccountName, "skill", skillpos)
			d.notify(w, s, NoticeOnlyOneEighthSkill)
			sendClientMessage(w, s, msgOnlyOneEighthSkill)
			return
		}
		learned := 0
		for i := 1; i <= 7; i++ {
			if e.LearnedSkill&(1<<(skillpos-i)) != 0 {
				learned++
			}
		}
		if learned != 7 {
			d.log.Info("learn skill refused: tree incomplete",
				"conn", s.Conn, "account", s.AccountName, "skill", skillpos,
				"learned_before_it", learned, "mask", e.LearnedSkill)
			d.notify(w, s, NoticeLearnPrereq)
			sendClientMessage(w, s, msgBeforeEighthSkill)
			return
		}
		if e.Coin < eighthSkillCoin {
			d.log.Info("learn skill refused: gold",
				"conn", s.Conn, "account", s.AccountName, "skill", skillpos,
				"gold", e.Coin, "needed", eighthSkillCoin)
			d.notify(w, s, NoticeNotEnoughCoin)
			// The 8th skill costs fifty million on top of its skill points, and
			// nothing on screen says so — the tooltip lists the level and the
			// mastery, never the gold. A player who meets every visible
			// requirement clicks and gets silence.
			sendClientMessage(w, s, fmt.Sprintf(msgEighthSkillCost, eighthSkillCoin))
			return
		}
	}
	if e.LearnedSkill&(1<<skillpos) != 0 {
		d.log.Info("learn skill refused: already learned",
			"conn", s.Conn, "account", s.AccountName, "skill", skillpos)
		d.notify(w, s, NoticeAlreadyLearned)
		sendClientMessage(w, s, msgAlreadyLearned)
		return
	}
	// Learn requirements live in g_pItemList[5000+idx], the dotted 4th column
	// "ReqLvl.ReqStr.ReqInt.ReqDex.ReqCon": a level, and a per-tree mastery
	// compared against the live Special[1..3]. They are NOT all zero in this
	// fork's ItemList.csv (an earlier comment here claimed they were): only the
	// first three skills of a tree are free, and from Samaritano (5003, 87 in the
	// Special[1] tree) on, every row carries one. That is the wall a level-1
	// celestial hits — it has 855 mastery points to spend and none spent yet, so
	// the gate holds until it distributes them.
	if req, ok := d.itemReqs[detail]; ok {
		// The LEVEL requirement applies to Mortal and Arch only; every celestial
		// tier reads it as zero (_MSG_ApplyBonus.cpp:190). A reborn Celestial is
		// level 1, so enforcing it would lock the tier out of its own kit the
		// moment a skill carries any requirement at all. The mastery requirements
		// below are NOT waived — the legacy checks those for everyone.
		reqLvl := int32(req.Lvl)
		if isCelestialTier(e.ClassMaster) {
			reqLvl = 0
		}
		if e.Level < reqLvl {
			d.log.Info("learn skill refused: level",
				"conn", s.Conn, "account", s.AccountName, "skill", skillpos,
				"level", e.Level, "required", reqLvl)
			d.notify(w, s, NoticeReqNotMet)
			sendClientMessage(w, s, msgNeedLevelToLearn)
			return
		}
		if e.Special[1] < req.Int || e.Special[2] < req.Dex || e.Special[3] < req.Con {
			d.log.Info("learn skill refused: mastery",
				"conn", s.Conn, "account", s.AccountName, "skill", skillpos,
				"special", e.Special, "req_int", req.Int, "req_dex", req.Dex, "req_con", req.Con)
			d.notify(w, s, NoticeReqNotMet)
			// The legacy line alone ("no mastery for this skill") does not say how
			// much is missing, and the player has no other way to find out — the
			// requirement is not on the client's tooltip. The numbers are appended
			// for that reason, in the Special[1..3] order the check reads them.
			sendClientMessage(w, s, fmt.Sprintf("%s (precisa %d/%d/%d, você tem %d/%d/%d)",
				msgNeedMasteryToLearn, req.Int, req.Dex, req.Con,
				e.Special[1], e.Special[2], e.Special[3]))
			return
		}
	}
	if isEighth {
		e.Coin -= eighthSkillCoin
	}
	e.LearnedSkill |= 1 << skillpos
	e.SkillBonus -= uint16(sp.SkillPoint)
	d.refreshScore(e)
	d.sendScore(w, s, e)
	d.sendEtc(w, s, e)
	d.log.Info("skill learned", "conn", s.Conn, "skill", skillpos+content.MaxSkill*skillclass,
		"name", sp.Name, "cost", sp.SkillPoint, "rest", e.SkillBonus)
}

// applySpecialBonus is ApplyBonus BonusType==1 (_MSG_ApplyBonus.cpp:85-129):
// allocate one mastery point into BaseSpecial[detail]. The per-point caps: half
// of 3*(level+1), and an absolute 200 — raised to 255 for the tree whose 8th
// skill is learned (bits 7/15/23 ↔ Detail 1/2/3).
func (d *Dispatcher) applySpecialBonus(w *world.World, s *world.Session, e *world.Entity, detail int) {
	if e.SpecialBonus == 0 {
		d.sendEtc(w, s, e)
		return
	}
	if detail < 0 || detail > 3 {
		return
	}
	maxSpecialLevel := 3 * (int(e.Level) + 1)
	// The celestial tiers get a flat +1200 on the level allowance
	// (_MSG_ApplyBonus.cpp:97-98). Without it a reborn Celestial is capped by its
	// own level: at level 0 the allowance is 3, half of it is 1, and the second
	// mastery point is refused — which is exactly how this surfaced.
	if isCelestialTier(e.ClassMaster) {
		maxSpecialLevel += 3 * 400
	}
	maxSpecial := int16(200)
	if (detail == 1 && e.LearnedSkill&(1<<7) != 0) ||
		(detail == 2 && e.LearnedSkill&(1<<15) != 0) ||
		(detail == 3 && e.LearnedSkill&(1<<23) != 0) {
		maxSpecial = 255
	}
	// The legacy answers these two refusals with DIFFERENT strings (:115,:118):
	// the level allowance is "no more points for now" (it lifts as you level),
	// while the flat 200/255 ceiling is its own message. Telling them apart
	// matters — one is temporary and one is final.
	if int(e.BaseSpecial[detail]) >= maxSpecialLevel>>1 {
		d.notify(w, s, NoticeMaxPoint)
		sendClientMessage(w, s, msgMaxPointNow)
		return
	}
	if e.BaseSpecial[detail] >= maxSpecial {
		d.notify(w, s, NoticeMaxPoint)
		sendClientMessage(w, s, msgMaxPoint200)
		return
	}
	e.SpecialBonus--
	e.BaseSpecial[detail]++
	d.refreshScore(e)
	d.sendScore(w, s, e)
	d.sendEtc(w, s, e)
}

// setShortSkill handles _MSG_SetShortSkill (0x0378, 32-byte body): the client
// pushes its hotbar layout — Skill1[4] → MOB.SkillBar (persisted with the
// character), Skill2[16] → CUser.CharShortSkill (echoed back at login in the
// CNFCharacterLogin tail). No response (_MSG_SetShortSkill.cpp).
func (d *Dispatcher) setShortSkill(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || s.Mode != world.UserPlay {
		return
	}
	if len(payload) < 20 {
		return
	}
	copy(e.SkillBar[:], payload[0:4])
	copy(s.ShortSkill[:], payload[4:20])
}

// The flat skill-point allowance each tier carries (Basedef.cpp:854-860). The
// legacy writes the celestial one as "not ARCH and not MORTAL", which catches
// CELESTIAL, CELESTIALCS and SCELESTIAL alike — spelled out here so adding a
// tier later cannot silently inherit it.
const (
	skillBonusArch      = 112
	skillBonusCelestial = 1500
)

func skillTierBonus(classMaster uint8) int {
	switch classMaster {
	case classMasterArch:
		return skillBonusArch
	case classMasterCelestial, classMasterCelestialCS, classMasterSCelestial:
		return skillBonusCelestial
	default:
		// Mortal, and the unset ClassMaster 0 that completeCharacterLogin treats
		// as Mortal: no adder.
		return 0
	}
}

// The two mastery refusals, copied verbatim from Language.txt (105, 106). They
// are different on purpose: the first lifts as the character levels, the second
// is the hard 200/255 ceiling.
// The panel encodes to CP1252 (protocol.ClientText), so these carry their real
// accents — writing them flat would be wrong on screen, not safer.
const (
	msgMaxPointNow = "Não pode colocar mais pontos."
	msgMaxPoint200 = "Não pode passar do nível 200."
)

// The class-master refusals, from Language.txt (109, 110, 111, 402, 403). Every
// one of them used to be a bare d.notify — a numeric code the client does not
// render — which is why learning a skill failed as silence rather than as an
// answer. The mastery one (111) is the wall a level-1 celestial actually hits.
//
// 403 is quoted with its typo repaired ("aprendar" → "aprender"); the rest are
// verbatim. msgOtherClassSkill has no legacy string at all — the original answers
// that case with nothing — so it is written here.
const (
	msgAlreadyLearned     = "Você já aprendeu essa habilidade."
	msgNeedLevelToLearn   = "Não possui nível para aprender essa habilidade."
	msgNeedMasteryToLearn = "Não possui aprendizado para obter essa habilidade."
	msgOnlyOneEighthSkill = "8ª Skill pode ser somente da 1ª Classe."
	msgBeforeEighthSkill  = "É necessário aprender todas as skills antes da 8ª Skill."
	msgOtherClassSkill    = "Essa habilidade não é da sua classe."

	// The two that were missed when the other refusals were given a voice: they
	// kept answering with the bare numeric notice, which the client does not
	// render. The gold one is the worse of the pair — the 8th skill's fifty
	// million appears on no tooltip, so a player who meets every requirement the
	// client DOES show is refused by a cost nothing ever mentioned.
	msgNotEnoughSkillPoint = "Não possui pontos suficientes." // Language.txt:108
	msgEighthSkillCost     = "Você precisa de %d de ouro."    // Language.txt:204
)
