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
// Geography (issue #421): a death inside a city or a guild war area (victim's
// position, BASE_GetArena/BASE_GetVillage) takes the legacy arena branch — the
// victim keeps their EXP and gets _NN_In_Arena_No_Exp_Loss, and the killer pays
// the arena CP rate (pvpLostPkZone). Everywhere else takes the field branch.
// The orphaned `if (killed_x == 1 && killed_y == 31)` is the MapPK zone flag lost
// in decompilation; MapPK itself is not modeled.
//
// Deferred/UNVERIFIED (documented, not ported — see the plan for issue #210):
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
	if world.PvPZone(victim.X, victim.Y) {
		d.pvpKilledInZone(w, ks, vs, killer, victim)
	} else {
		d.pvpKilledInField(w, ks, vs, killer, victim)
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

// pvpKilledInZone is the legacy arena branch (MobKilled.cpp:3131-3228): no EXP
// loss, no victim PKPoint recovery, and — unless the victim was already Guilty —
// the killer loses CP at the arena rate, announced with _DD_PKPointMinus even
// when the loss rounds to 0 (the legacy sends it inside `if (!killed_guilty)`).
// The arena branch does not consult AtWar.
func (d *Dispatcher) pvpKilledInZone(w *world.World, ks, vs *world.Session, killer, victim *world.Entity) {
	if vs != nil {
		d.sendClientMessage(w, vs, inArenaNoExpLoss)
	}
	if pkGuilty(victim) {
		return
	}
	lost := pvpLostPkZone(victim.PKPoint)
	killer.PKPoint = clampPKPoint(int(killer.PKPoint) + lost)
	if ks != nil {
		// The legacy prints the negative Lostpk as-is ("Menos_-9").
		d.sendClientMessage(w, ks, fmt.Sprintf(pkPointMinus, int(killer.PKPoint)-75, lost))
	}
}

// pvpKilledInField is the open-field branch: EXP loss for the victim, the
// field CP rate for the killer, and the victim's PKPoint recovery.
func (d *Dispatcher) pvpKilledInField(w *world.World, ks, vs *world.Session, killer, victim *world.Entity) {
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
	return clampLostPk(3 * int(victimPKPoint) / -25)
}

// pvpLostPkZone is the killer's PKPoint change for a kill inside a city or a
// guild war area (MobKilled.cpp arena branch): the /-20 rate instead of the
// field's /-25, clamped to [-9,0]. The Guilty check is the caller's.
func pvpLostPkZone(victimPKPoint uint8) int {
	return clampLostPk(3 * int(victimPKPoint) / -20)
}

func clampLostPk(lost int) int {
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
