package handler

import (
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
// UNVERIFIED: the ARCH (+112) / Celestial (+1500) tier adders and the
// PilulaOrc quest (+9) are not modeled yet (tiers wait on the celestial plan).
// A negative rest clamps to 0 (the legacy would wrap an unsigned field).
func (d *Dispatcher) deriveSkillBonus(e *world.Entity) {
	if d.spells == nil {
		e.SkillBonus = 0
		return
	}
	per := int(e.Level) * 3
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

// learnSkill is ApplyBonus BonusType==2 (_MSG_ApplyBonus.cpp:131-249): learn the
// skill g_pItemList index Detail (5000+idx) from a class-master NPC (TargetID).
// Runs in the loop goroutine.
func (d *Dispatcher) learnSkill(w *world.World, s *world.Session, e *world.Entity, detail int, targetID int) {
	if d.spells == nil || detail < 5000 || detail > 5095 {
		return
	}
	if targetID < world.MaxUser || targetID >= world.MaxMob {
		return // learn requests must come through an NPC
	}
	skillclass := (detail - 5000) / content.MaxSkill
	skillpos := (detail - 5000) % content.MaxSkill
	if int(e.Class) != skillclass {
		d.notify(w, s, NoticeOtherClassSkill)
		return
	}
	sp, ok := d.spells.Get(skillpos + content.MaxSkill*skillclass)
	if !ok {
		return
	}
	if sp.SkillPoint > int(e.SkillBonus) {
		d.notify(w, s, NoticeNotEnoughSkillPoint)
		return
	}
	isEighth := skillpos == 7 || skillpos == 15 || skillpos == 23
	if isEighth {
		// The 8th skill of each tree is exclusive: only one of the three, only
		// after the tree's 7 previous skills, and it costs 50M gold on top.
		if e.LearnedSkill&(1<<7|1<<15|1<<23) != 0 {
			d.notify(w, s, NoticeOnlyOneEighthSkill)
			return
		}
		learned := 0
		for i := 1; i <= 7; i++ {
			if e.LearnedSkill&(1<<(skillpos-i)) != 0 {
				learned++
			}
		}
		if learned != 7 {
			d.notify(w, s, NoticeLearnPrereq)
			return
		}
		if e.Coin < eighthSkillCoin {
			d.notify(w, s, NoticeNotEnoughCoin)
			return
		}
	}
	if e.LearnedSkill&(1<<skillpos) != 0 {
		d.notify(w, s, NoticeAlreadyLearned)
		return
	}
	// Learn requirements live in g_pItemList[5000+idx] (all zero in this fork's
	// ItemList.csv, but the gate is kept): level, and per-tree mastery vs the
	// live Special[1..3].
	if req, ok := d.itemReqs[detail]; ok {
		if e.Level < int32(req.Lvl) {
			d.notify(w, s, NoticeReqNotMet)
			return
		}
		if e.Special[1] < req.Int || e.Special[2] < req.Dex || e.Special[3] < req.Con {
			d.notify(w, s, NoticeReqNotMet)
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
	maxSpecial := int16(200)
	if (detail == 1 && e.LearnedSkill&(1<<7) != 0) ||
		(detail == 2 && e.LearnedSkill&(1<<15) != 0) ||
		(detail == 3 && e.LearnedSkill&(1<<23) != 0) {
		maxSpecial = 255
	}
	// UNVERIFIED: the Celestial tier adds 3*400 to maxSpecialLevel — tiers are
	// not modeled yet (celestial-system-plan.md).
	if int(e.BaseSpecial[detail]) >= maxSpecialLevel>>1 || e.BaseSpecial[detail] >= maxSpecial {
		d.notify(w, s, NoticeMaxPoint)
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
