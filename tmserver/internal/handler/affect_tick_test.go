package handler

import (
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// These tests drive the affect SWEEP over the wire (sweepAffects → processAffect,
// affect_tick.go) via the fast-tick harness startServerAffectTick: a character
// enters the world carrying a persisted periodic/expiring affect and the tick
// must float the right MSG_SetHpDam and refresh the affect snapshot. They are the
// direct-sweep coverage the transform-expiry test only exercised for Type 16.

// TestAffectHoTTickHealsAndFloatsSetHpDam: an Aura da Vida HoT (Type 17) below
// max HP heals +Level/2 + Value per tick and floats MSG_SetHpDam with a positive
// delta (affect_tick.go:54-66).
func TestAffectHoTTickHealsAndFloatsSetHpDam(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Healer", X: 5, Y: 5,
		HP: 100, MaxHP: 500, MP: 100, MaxMP: 100, Level: 100,
		Affects: []world.Affect{{Type: 17, Value: 50, Level: 100, Time: 1000}},
	}
	addr, stop := startServerAffectTick(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ty, p, ok := readMaybe(t, c)
		if !ok || ty != protocol.MsgSetHpDam {
			continue
		}
		hp, dam := int32(lend.Uint32(p[0:])), int32(lend.Uint32(p[4:]))
		if dam <= 0 {
			t.Fatalf("HoT SetHpDam delta = %d, want > 0 (heal)", dam)
		}
		if hp <= 100 {
			t.Fatalf("HoT hp = %d, want healed above the starting 100", hp)
		}
		return
	}
	t.Fatal("HoT affect never floated a MSG_SetHpDam heal")
}

// TestAffectPoisonTickDamagesAndFloatsSetHpDam: a poison DoT (Type 20) drains
// 1000 HP/tick (floor 1) and floats MSG_SetHpDam with a negative delta
// (affect_tick.go:67-71, applyPoisonTick).
func TestAffectPoisonTickDamagesAndFloatsSetHpDam(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Victim", X: 5, Y: 5,
		HP: 5000, MaxHP: 5000, MP: 100, MaxMP: 100, Level: 100,
		Affects: []world.Affect{{Type: 20, Value: 0, Level: 100, Time: 1000}},
	}
	addr, stop := startServerAffectTick(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ty, p, ok := readMaybe(t, c)
		if !ok || ty != protocol.MsgSetHpDam {
			continue
		}
		hp, dam := int32(lend.Uint32(p[0:])), int32(lend.Uint32(p[4:]))
		if dam != -1000 {
			t.Fatalf("poison SetHpDam delta = %d, want -1000", dam)
		}
		if hp != 4000 {
			t.Fatalf("poison hp = %d, want 4000 (5000-1000)", hp)
		}
		return
	}
	t.Fatal("poison affect never floated a MSG_SetHpDam drain")
}

// TestAffectExpiryClearsSlotAndRefreshes: a timed buff (Type 11, Time 1) is
// zeroed on its first sweep and the fresh MSG_SendAffect snapshot must carry no
// Type-11 slot (affect_tick.go:79-90). Each SendAffect slot is 8 bytes with Type
// at offset i*8.
func TestAffectExpiryClearsSlotAndRefreshes(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Buffed", X: 5, Y: 5,
		HP: 500, MaxHP: 500, MP: 100, MaxMP: 100, Level: 50,
		Affects: []world.Affect{{Type: 11, Value: 5, Level: 50, Time: 1}},
	}
	addr, stop := startServerAffectTick(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ty, p, ok := readMaybe(t, c)
		if !ok || ty != protocol.MsgSendAffect {
			continue
		}
		hasEleven := false
		for i := 0; i+8 <= len(p); i += 8 {
			if p[i] == 11 {
				hasEleven = true
			}
		}
		if !hasEleven {
			return // slot cleared after expiry
		}
	}
	t.Fatal("Type-11 affect never expired out of the SendAffect snapshot")
}
