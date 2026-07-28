package handler

import (
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Kill-streak clamps (GetFunc.cpp SetCurKill/SetTotKill).
const (
	maxCurKill = 200
	maxTotKill = 32767
)

// pkPointMin/pkPointMax are the legacy SetPKPoint clamp bounds (GetFunc.cpp:
// value<1→1, value>150→150); 0 is reserved to mean "never persisted" (see
// completeCharacterLogin).
const (
	pkPointMin uint8 = 1
	pkPointMax uint8 = 150
)

// pvpKilled runs the death penalties for a player slain by another player
// (MobKilled.cpp "#pragma region PvP", issue #210). This is a straight-line port
// of the reliably-extractable formulas only — MobKilled.cpp's actual PvP branch
// is riddled with what looks like a decompiler artifact (an orphaned, brace-less
// `if` on a coordinate check that appears to swallow the entire arena/exp/
// CurKill block), so this mirrors mobKilled's clean linear style instead of
// replicating that control flow.
//
// Deferred/UNVERIFIED (documented, not ported — see the plan for issue #210):
//   - arena/village/isWar/MapPK geography — combat.go already deliberately
//     skips this class of check (issue #67 regression from a coarse
//     city-rectangle approximation).
//   - Castle-siege/RvR-Tower AtWar bypasses — those systems are themselves
//     UNVERIFIED (tmserver/internal/war/tower.go); only the guild-war bypass is
//     implemented (guildsAtWar).
//   - SameClan (legacy: clan 7 vs clan 8, an undocumented kingdom-war NPC-faction
//     pair, NOT "same guild") — unmodeled anywhere in this repo, treated as
//     always false: a normal kill always increments the killer's streak/resets
//     the victim's.
//   - The item 548/549 "triple the PKPoint penalty" rule — unidentified
//     equipment, not chased.
//   - #ifdef PKDrop item-loss-on-death — disabled in the reference build
//     (Basedef.h:89, `//#define PKDrop`); intentionally not ported.
//   - extra.Hold/DEADPOINT banked-EXP-loss indirection — no analogue anywhere in
//     this codebase; the victim's EXP loss below is an IMMEDIATE deduction
//     instead (documented divergence in intent vs. mechanism).
func (d *Dispatcher) pvpKilled(w *world.World, killer, victim *world.Entity) {
	ks := w.Session(killer.ID)
	vs := w.Session(victim.ID)
	atWar := d.guildsAtWar(killer.Guild, victim.Guild)

	if loss := pvpExpLoss(victim.Level, victim.ClassMaster, victim.PKPoint); loss > 0 {
		victim.Exp -= loss
		if victim.Exp < 0 {
			victim.Exp = 0
		}
		if vs != nil {
			d.sendChatText(w, vs, fmt.Sprintf("Voce perdeu %d pontos de experiencia", loss))
		}
	}

	if lost := pvpLostPk(victim.PKPoint, pkGuilty(victim), atWar); lost != 0 {
		killer.PKPoint = clampPKPoint(int(killer.PKPoint) + lost)
		if ks != nil {
			d.sendChatText(w, ks, notifyPKPointDelta(killer.PKPoint, lost))
		}
	}
	if victim.PKPoint < pkPointNeutral && !atWar {
		victim.PKPoint++
		if vs != nil {
			d.sendChatText(w, vs, notifyPKPointDelta(victim.PKPoint, 1))
		}
	}

	// Kill-streak bookkeeping (SameClan treated as always false — see doc above).
	if killer.CurKill < maxCurKill {
		killer.CurKill++
	}
	if killer.TotKill < maxTotKill {
		killer.TotKill++
	}
	victim.CurKill = 0

	if ks != nil {
		d.broadcastPKState(w, ks, killer)
	}
	if vs != nil {
		d.broadcastPKState(w, vs, victim)
	}
}

// pvpExpLoss is the victim's EXP loss on a PvP death (MobKilled.cpp "Lose EXP"
// region): a tiered per-level divisor of the exp span between the victim's
// current and next level, scaled by how chaotic the victim already was, then a
// flat /6.
func pvpExpLoss(victimLevel int32, victimClassMaster uint8, victimPKPoint uint8) int64 {
	nextExp := level.NextLevelExpTier(victimLevel, victimClassMaster)
	curExp := level.NextLevelExpTier(victimLevel-1, victimClassMaster)
	alpha := nextExp - curExp
	delta := alpha / 20
	switch {
	case victimLevel >= 250:
		delta = alpha / 100
	case victimLevel >= 200:
		delta = alpha / 85
	case victimLevel >= 150:
		delta = alpha / 70
	case victimLevel >= 100:
		delta = alpha / 55
	case victimLevel >= 90:
		delta = alpha / 50
	case victimLevel >= 80:
		delta = alpha / 45
	case victimLevel >= 70:
		delta = alpha / 40
	case victimLevel >= 60:
		delta = alpha / 35
	case victimLevel >= 50:
		delta = alpha / 30
	case victimLevel >= 40:
		delta = alpha / 25
	case victimLevel >= 30:
		delta = alpha / 22
	}
	if delta < 0 {
		delta = 0
	}
	if delta > 150000 {
		delta = 150000
	}
	if victimPKPoint > 10 && victimPKPoint <= 25 {
		delta *= 3
	} else {
		delta *= 5
	}
	return delta / 6
}

// pvpLostPk is the killer's PKPoint change for landing a PvP kill
// (MobKilled.cpp:3316-3334, "PK Drop - CP"): scaled by how chaotic the victim
// already was, clamped to [-9,0], and zeroed entirely against an already-guilty
// victim or during a guild war.
func pvpLostPk(victimPKPoint uint8, victimGuilty, atWar bool) int {
	if victimGuilty || atWar {
		return 0
	}
	lost := 3 * int(victimPKPoint) / -25
	if lost < -9 {
		lost = -9
	}
	if lost > 0 {
		lost = 0
	}
	return lost
}

// clampPKPoint enforces the legacy SetPKPoint write bounds (GetFunc.cpp:
// [1,150]) — 0 is reserved to mean "never persisted" elsewhere in this code, so
// arithmetic must never produce it.
func clampPKPoint(v int) uint8 {
	if v < int(pkPointMin) {
		return pkPointMin
	}
	if v > int(pkPointMax) {
		return pkPointMax
	}
	return uint8(v)
}

// guildsAtWar reports mutual guild war (the only AtWar bypass implemented;
// MobKilled.cpp:3113-3125's guild-war check), mirroring the legacy double-check
// against d.guildWars (guild_state.go).
func (d *Dispatcher) guildsAtWar(a, b uint16) bool {
	if a == 0 || b == 0 {
		return false
	}
	return d.guildWars[a] == b && d.guildWars[b] == a
}
