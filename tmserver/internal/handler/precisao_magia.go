package handler

import "github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"

// capMissStreak is the SERVER RULE that a skill cannot miss the same target
// more than maxStreak times in a row (combatrule.MaxMissStreak). miss is what
// ResolveParry rolled — 0 for a hit, -3/-4 for a dodge/block — and the return
// is what the blow becomes.
//
// The roll has already been made by the caller and is kept: forcing the hit
// here, instead of skipping ResolveParry, leaves the rand() stream exactly as
// the legacy consumes it, so every other roll in the fight still lines up.
//
// The count is per attacker and per target. Switching targets starts it over —
// "no more than two in a row" is a promise about one fight, and carrying a
// streak from one monster to the next would hand out free hits.
func capMissStreak(attacker *world.Entity, tid, miss, maxStreak int) int {
	if attacker == nil {
		return miss
	}
	if attacker.MissStreakTarget != int32(tid) {
		attacker.MissStreakTarget = int32(tid)
		attacker.MissStreak = 0
	}
	if miss == 0 {
		attacker.MissStreak = 0
		return 0
	}
	if maxStreak > 0 && int(attacker.MissStreak) >= maxStreak {
		attacker.MissStreak = 0
		return 0
	}
	attacker.MissStreak++
	return miss
}
