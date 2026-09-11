package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Monsters cast from MOB.SkillBar[4] (STRUCT_MOB @796, Basedef.h:590). GetAttack
// rolls once per swing and reads the bar by band (GetFunc.cpp:1569-1627):
//
//	 0-49  → SkillBar[0]
//	50-84  → SkillBar[1]
//	85-99  → SkillBar[2]
//	25-64  → SkillBar[3], but ONLY as a heal (InstanceType 6)
//
// skillBarEmpty is the empty-slot sentinel; 492 of the game's npc templates
// carry a real bar (Kefra's is 6f 6d 6f 6e — the three boss animations), and all
// 37 BaseSummon templates shipped empty, which is why no evocation ever cast
// anything.
const (
	skillBarEmpty = 255

	// Band edges of the one rand()%100 GetAttack draws per swing.
	skillBand0End  = 49
	skillBand1Star = 50
	skillBand1End  = 84
	skillBand2Star = 85
	skillHealStart = 25
	skillHealEnd   = 64

	// The heal slot fires when self or leader is at or below 8 tenths of
	// HP*10/(MaxHP+1), and restores a tenth of the maximum (GetFunc.cpp:1587-1604).
	// That divisor puts the real boundary at 90%, not 80% — integer division makes
	// 910/1000 read as 9 and 900/1000 as 8.
	skillHealTriggerTenths = 8
	skillHealDivisor       = 10

	// SetAffect/SetTick take the legacy's flat 100 as their delay
	// (ProcessSecMinTimer.cpp:2199-2201), which makes the installed duration
	// AffectTime+1 ticks.
	mobAffectDelay = 100

	// affectPoison is the DoT slot type (SkillData TickType 20) and
	// poisonTickDamage the flat drain the legacy player math collapses to
	// (affect_tick.go).
	affectPoison     = 20
	poisonTickDamage = 1000

	// healInstanceType is STRUCT_SPELL.InstanceType 6, the heal family — the
	// only kind the SkillBar[3] slot accepts (GetFunc.cpp:1586).
	healInstanceType = 6

	// noSkill is the SkillIndex of a plain swing. GetAttack initialises
	// sm->SkillIndex to -1 (GetFunc.cpp:1427) and the client reads anything
	// outside the table as "no spell, just hit".
	noSkill = -1
)

// mobSkill is what a monster's swing carries beyond the blow itself.
type mobSkill struct {
	index int  // spell to render and apply, or noSkill
	heal  bool // the swing is the SkillBar[3] self/leader heal instead of a hit
}

// rollMobSkill picks the skill a monster casts with this swing.
//
// DELIBERATE DIVERGENCE: the legacy reaches this code but can never USE it. The
// affect application it feeds is gated on `sm.SkillParm == 0`
// (ProcessSecMinTimer.cpp:2196), and every branch of the animation switch that
// runs first sets SkillParm to 1, 2, 3, 5 or -4 — never 0 (GetFunc.cpp:1449-1490,
// reached for every mob including the -1 default). So in the shipped server a
// monster's SkillBar picks an ANIMATION and nothing else: no poison, no slow, no
// weaken ever lands from a monster's hit. We drop that gate, because the whole
// point of giving the evocations a bar is the effect.
//
// The resist scaling of GetFunc.cpp:1633-1634 is deliberately NOT ported with
// it: `(200 - Resist[kind]) * dam / 100` DOUBLES a mob's damage against a target
// with no resistance in that element, and every evocation's damage was just
// calibrated against a bare AC. Folding it in here would silently double the
// numbers chosen for balance.
//
// The roll is drawn on every mob swing whether or not the bar holds anything, as
// the legacy does — mob AI shares the RNG stream with drops and refines, so
// making the draw conditional would shift it.
func rollMobSkill(w *world.World, spells *content.SkillData, e *world.Entity) mobSkill {
	return pickMobSkill(spells, e, w.Rand().Intn(100))
}

// pickMobSkill is the band table itself, split out so a test can pin each edge
// without driving the shared RNG stream.
func pickMobSkill(spells *content.SkillData, e *world.Entity, roll int) mobSkill {
	if e == nil || spells == nil {
		return mobSkill{index: noSkill}
	}

	// Heal slot first, and it wins the swing outright (GetFunc.cpp:1570-1605).
	if h := e.SkillBar[3]; h != skillBarEmpty && roll >= skillHealStart && roll <= skillHealEnd {
		if sp, ok := spells.Get(int(h)); ok && sp.InstanceType == healInstanceType {
			return mobSkill{index: int(h), heal: true}
		}
	}

	switch {
	case e.SkillBar[0] != skillBarEmpty && roll <= skillBand0End:
		return mobSkill{index: int(e.SkillBar[0])}
	case e.SkillBar[1] != skillBarEmpty && roll >= skillBand1Star && roll <= skillBand1End:
		return mobSkill{index: int(e.SkillBar[1])}
	case e.SkillBar[2] != skillBarEmpty && roll >= skillBand2Star:
		return mobSkill{index: int(e.SkillBar[2])}
	}
	return mobSkill{index: noSkill}
}

// applyMobSkill lands the chosen skill's affect on the victim. The damage of the
// swing is unchanged — the skill rides along with the hit, which is how a
// monster's bar works (there is no separate cast message for it).
func (d *Dispatcher) applyMobSkill(w *world.World, e, target *world.Entity, sk mobSkill) {
	if sk.index == noSkill || target == nil || target.HP <= 0 {
		return
	}
	if d.spells == nil {
		return
	}
	sp, ok := d.spells.Get(sk.index)
	if !ok || (sp.AffectType <= 0 && sp.TickType <= 0) {
		return // magia só de dano (o meteoro): o dano em área é do golpesDaArea (mobai.go)
	}
	delay, level := mobAffectDelay, 0
	// DELIBERATE DIVERGENCE for a PET. The legacy passes the caster mob own
	// mastery (ProcessSecMinTimer.cpp:2198), and a summon template carries none —
	// so every debuff would land at table strength forever: Enfraquecer at level 0
	// takes 10 damage off a character with thousands, and a slow expires in one
	// tick. The pet borrows its master Evocacao instead, the same mastery that
	// already sets its damage, AC and HP (summon.go), so the effect grows with the
	// BM exactly as the creature does.
	if e != nil && e.Summoner != 0 {
		if owner := w.Entity(e.Summoner); owner != nil {
			level = effectiveSpecial(owner, 2)
			delay += level
		}
	}
	// true: um PET pode debuffar monstro. É a divergência estreita descrita em
	// SetAffectOnMob; jogador contra monstro segue a regra do legado.
	d.applyOnHitSpell(w, target, target.ID, sk.index, delay, level, e != nil && e.Summoner != 0)
}

// healMobSkill runs the SkillBar[3] slot: a tenth of maximum HP onto whichever of
// the caster and its leader is worse off, but only when one of them has dropped
// to 80% or below (GetFunc.cpp:1587-1604).
//
// Returns whether the heal actually fired; when it does the swing is spent on it.
func (d *Dispatcher) healMobSkill(w *world.World, id int, e *world.Entity) bool {
	if e == nil || e.MaxHP <= 0 {
		return false
	}
	// Um PET cura só a si mesmo.
	//
	// O legado escolhe o paciente entre o lançador e o LÍDER, que para um monstro
	// de grupo é outro monstro. Para um pet o líder é o JOGADOR, e seguir a regra
	// ao pé da letra punha o Dragão gastando 40% dos golpes curando o dono — o
	// gatilho é o paciente estar abaixo de 90% de vida, o que numa caçada é quase
	// sempre, então o bicho quase parava de bater. E não era o que foi pedido: a
	// cura do Dragão é individual, nele mesmo.
	leaderID := id
	if e.Summoner == 0 {
		leaderID = e.Leader
		if leaderID <= 0 {
			leaderID = id
		}
	}
	leader := w.Entity(leaderID)
	if leader == nil || leader.MaxHP <= 0 {
		leader, leaderID = e, id
	}

	selfTenths := int(e.HP) * 10 / int(e.MaxHP+1)
	leadTenths := int(leader.HP) * 10 / int(leader.MaxHP+1)
	if selfTenths > skillHealTriggerTenths && leadTenths > skillHealTriggerTenths {
		return false
	}

	// The one in worse shape gets it — the legacy picks the leader when the
	// caster is the healthier of the two.
	patient, patientID := e, id
	if selfTenths > leadTenths {
		patient, patientID = leader, leaderID
	}
	heal := patient.MaxHP / skillHealDivisor
	if heal <= 0 {
		return false
	}
	if patient.HP+heal > patient.MaxHP {
		heal = patient.MaxHP - patient.HP
	}
	if heal <= 0 {
		return false
	}
	patient.HP += heal
	broadcastMobHeal(w, patientID, patient, heal)
	return true
}

// broadcastMobHeal floats the healed amount over the mob and corrects the HP bar
// on every client that can see it.
func broadcastMobHeal(w *world.World, id int, e *world.Entity, delta int32) {
	body := protocol.EncodeSetHpDam(e.HP, delta)
	hdr := protocol.Header{Type: protocol.MsgSetHpDam, ID: uint16(id)}
	w.ForEachInView(id, func(vs *world.Session, _ *world.Entity) {
		w.SendTo(vs, hdr, body)
	})
}

// sweepMobAffects is ProcessAffect for monsters. The legacy runs it on every mob
// in MOB_IDLE (ProcessSecMinTimer.cpp:1840), MOB_PEACE (:1854) and MOB_COMBAT
// (:2047, every 16th tick); this port had it on players only, so a debuff landed
// on a monster and then sat there forever — the timer that clears it never ran.
//
// Mobs have no session, so this is the session-free half of processAffect: the
// poison tick, the countdown, and the score rebuild when a slot clears.
func (d *Dispatcher) sweepMobAffects(w *world.World) {
	phase := d.tickCount % affectTickPeriod
	w.ForEachMob(func(id int, e *world.Entity) {
		if id%affectTickPeriod != phase || e.HP <= 0 || !e.HasAnyAffect() {
			return
		}
		d.processMobAffect(w, id, e)
	})
}

func (d *Dispatcher) processMobAffect(w *world.World, id int, e *world.Entity) {
	upScore := false
	var delta int32
	for i := range e.Affect {
		af := &e.Affect[i]
		if af.Type == 0 {
			continue
		}
		// The summon lifespan belongs to summonTick, which counts it on this very
		// phase and despawns the pet when it hits zero (summon.go). Touching it here
		// too made the slot clear before summonTick could see the expiry, and pets
		// stopped vanishing altogether.
		if af.Type == affectSummonLife {
			continue
		}
		if af.Type == affectPoison {
			if hp := e.HP - poisonTickDamage; hp != e.HP {
				if hp < 1 {
					hp = 1
				}
				delta += hp - e.HP
				e.HP = hp
			}
		}
		if af.Time < affectInfiniteTime && af.Time > 0 {
			af.Time--
		}
		if af.Time == 0 {
			*af = world.Affect{}
			upScore = true
		}
	}
	if delta != 0 {
		broadcastMobHeal(w, id, e, delta)
	}
	if upScore {
		// The affect share of the score is folded in by refreshScore; without
		// this the AC a slow or a weaken took away never comes back.
		d.refreshScore(e)
	}
}
