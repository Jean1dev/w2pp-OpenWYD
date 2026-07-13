package world

import "testing"

func TestSetAffect(t *testing.T) {
	player := &Entity{ID: 1}
	// (AffectTime+1)*time/100: a 150-tick buff cast with Delay 120 → 181 ticks.
	if !player.SetAffect(11, 5, 150, 0, 120, 33) {
		t.Fatal("SetAffect refused a valid player buff")
	}
	got := player.Affect[0]
	if got.Type != 11 || got.Value != 5 || got.Level != 33 || got.Time != 181 {
		t.Errorf("Affect[0] = %+v, want type 11 value 5 level 33 time 181", got)
	}
	// Re-applying the same type reuses the slot (GetEmptyAffect semantics).
	if !player.SetAffect(11, 5, 150, 0, 200, 40) {
		t.Fatal("re-apply refused")
	}
	if player.Affect[0].Level != 40 || player.Affect[1].Type != 0 {
		t.Errorf("re-apply did not reuse slot 0: %+v / %+v", player.Affect[0], player.Affect[1])
	}

	// Mobs never take SetAffect (Server.cpp:9211 `conn > MAX_USER`).
	mob := &Entity{ID: MaxUser + 5}
	if mob.SetAffect(11, 5, 150, 0, 120, 33) {
		t.Error("SetAffect must refuse mobs")
	}
	// RSV_BLOCK bounces aggressive casts only.
	blocked := &Entity{ID: 2, Rsv: RsvBlock}
	if blocked.SetAffect(20, 5, 150, 1, 120, 33) {
		t.Error("aggressive affect must bounce off RSV_BLOCK")
	}
	if !blocked.SetAffect(11, 5, 150, 0, 120, 33) {
		t.Error("friendly affect must pass RSV_BLOCK")
	}
}

func TestSetTick(t *testing.T) {
	// Mobs DO take ticks (poison) — only merchant NPCs are excluded.
	mob := &Entity{ID: MaxUser + 5}
	if !mob.SetTick(20, 75, 150, 1, 120, 33) {
		t.Fatal("SetTick must allow mobs")
	}
	if got := mob.Affect[0]; got.Type != 20 || got.Value != 75 || got.Time != 181 {
		t.Errorf("tick slot = %+v, want type 20 value 75 time 181", got)
	}
	merchant := &Entity{ID: MaxUser + 6, Merchant: 1}
	if merchant.SetTick(20, 75, 150, 1, 120, 33) {
		t.Error("SetTick must refuse merchant NPCs")
	}
	// Types 1/3/10 clamp long timers to 2 ticks.
	p := &Entity{ID: 3}
	if !p.SetTick(3, 10, 150, 0, 120, 5) {
		t.Fatal("SetTick refused")
	}
	if p.Affect[0].Time != 2 {
		t.Errorf("type-3 tick time = %d, want clamp 2", p.Affect[0].Time)
	}
	if !p.SetTick(10, 10, 150, 0, 120, 5) {
		t.Fatal("SetTick refused type 10")
	}
	if p.Affect[1].Time != 2 {
		t.Errorf("type-10 tick time = %d, want clamp 2", p.Affect[1].Time)
	}
	// A HoT (type 17) is NOT a 1/3/10 clamp type → it keeps the full
	// (affectTime+1)*delay/100 timer (one tick = 8s of real time).
	hoT := &Entity{ID: 4}
	if !hoT.SetTick(17, 40, 150, 0, 120, 100) {
		t.Fatal("SetTick refused a HoT")
	}
	if got := hoT.Affect[0]; got.Type != 17 || got.Time != 181 {
		t.Errorf("HoT tick = %+v, want type 17 time 181 (unclamped)", got)
	}
}
