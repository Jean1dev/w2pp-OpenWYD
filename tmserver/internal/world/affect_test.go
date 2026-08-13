package world

import "testing"

// TestSetAffect and TestSetTick pass the ZERO AffectDuration on purpose: that is
// the legacy-faithful mode, so these stay parity tests of the original
// Server.cpp:9209/9256 formula. The issue #229 tuning is covered separately by
// TestAffectDurationPolicy.
func TestSetAffect(t *testing.T) {
	player := &Entity{ID: 1}
	// (AffectTime+1)*time/100: a 150-tick buff cast with Delay 120 → 181 ticks.
	if !player.SetAffect(11, 5, 150, 0, 120, 33, AffectDuration{}) {
		t.Fatal("SetAffect refused a valid player buff")
	}
	got := player.Affect[0]
	if got.Type != 11 || got.Value != 5 || got.Level != 33 || got.Time != 181 {
		t.Errorf("Affect[0] = %+v, want type 11 value 5 level 33 time 181", got)
	}
	// Re-applying the same type reuses the slot (GetEmptyAffect semantics).
	if !player.SetAffect(11, 5, 150, 0, 200, 40, AffectDuration{}) {
		t.Fatal("re-apply refused")
	}
	if player.Affect[0].Level != 40 || player.Affect[1].Type != 0 {
		t.Errorf("re-apply did not reuse slot 0: %+v / %+v", player.Affect[0], player.Affect[1])
	}

	// Mobs never take SetAffect (Server.cpp:9211 `conn > MAX_USER`).
	mob := &Entity{ID: MaxUser + 5}
	if mob.SetAffect(11, 5, 150, 0, 120, 33, AffectDuration{}) {
		t.Error("SetAffect must refuse mobs")
	}
	// RSV_BLOCK bounces aggressive casts only.
	blocked := &Entity{ID: 2, Rsv: RsvBlock}
	if blocked.SetAffect(20, 5, 150, 1, 120, 33, AffectDuration{}) {
		t.Error("aggressive affect must bounce off RSV_BLOCK")
	}
	if !blocked.SetAffect(11, 5, 150, 0, 120, 33, AffectDuration{}) {
		t.Error("friendly affect must pass RSV_BLOCK")
	}
}

func TestSetTick(t *testing.T) {
	// Mobs DO take ticks (poison) — only merchant NPCs are excluded.
	mob := &Entity{ID: MaxUser + 5}
	if !mob.SetTick(20, 75, 150, 1, 120, 33, AffectDuration{}) {
		t.Fatal("SetTick must allow mobs")
	}
	if got := mob.Affect[0]; got.Type != 20 || got.Value != 75 || got.Time != 181 {
		t.Errorf("tick slot = %+v, want type 20 value 75 time 181", got)
	}
	merchant := &Entity{ID: MaxUser + 6, Merchant: 1}
	if merchant.SetTick(20, 75, 150, 1, 120, 33, AffectDuration{}) {
		t.Error("SetTick must refuse merchant NPCs")
	}
	// Types 1/3/10 clamp long timers to 2 ticks.
	p := &Entity{ID: 3}
	if !p.SetTick(3, 10, 150, 0, 120, 5, AffectDuration{}) {
		t.Fatal("SetTick refused")
	}
	if p.Affect[0].Time != 2 {
		t.Errorf("type-3 tick time = %d, want clamp 2", p.Affect[0].Time)
	}
	if !p.SetTick(10, 10, 150, 0, 120, 5, AffectDuration{}) {
		t.Fatal("SetTick refused type 10")
	}
	if p.Affect[1].Time != 2 {
		t.Errorf("type-10 tick time = %d, want clamp 2", p.Affect[1].Time)
	}
	// A HoT (type 17) is NOT a 1/3/10 clamp type → it keeps the full
	// (affectTime+1)*delay/100 timer (one tick = 8s of real time).
	hoT := &Entity{ID: 4}
	if !hoT.SetTick(17, 40, 150, 0, 120, 100, AffectDuration{}) {
		t.Fatal("SetTick refused a HoT")
	}
	if got := hoT.Affect[0]; got.Type != 17 || got.Time != 181 {
		t.Errorf("HoT tick = %+v, want type 17 time 181 (unclamped)", got)
	}
}

// TestAffectDurationPolicy pins the issue #229 tuning at the unit level: the
// scale, the mastery cap, the friendly-only floor, and the fact that the legacy
// special cases (the 1/3/10 clamps and the "infinite" sentinel) are NOT tuned.
func TestAffectDurationPolicy(t *testing.T) {
	// Production defaults (cmd/tmserver/main.go): 15%, floor 60s, cap 10min.
	prod := AffectDuration{ScalePct: 15, MinTicks: 8, MaxTicks: 75}
	tests := []struct {
		name       string
		dur        AffectDuration
		affectTime int
		aggressive int
		delay      int
		want       uint32
	}{
		// Escudo Mágico (43) and friends: raw AffectTime 600 → 150 after the
		// legacy ÷4. Delay is 100+Special, so these walk the mastery curve the
		// #229 screenshot came from (Special 226 measured ≈ 33 min before).
		{"long buff, no mastery", prod, 150, 0, 100, 22},  // 2m56
		{"long buff, special 100", prod, 150, 0, 200, 45}, // 6m00
		{"long buff, special 226", prod, 150, 0, 326, 73}, // 9m44
		{"long buff, special 255", prod, 150, 0, 355, 75}, // capped at 10m
		{"long buff, special 400", prod, 150, 0, 500, 75}, // capped at 10m
		// Escudo Dourado (85): raw 30 → 7. Its legacy duration already fits under
		// the cap, so it keeps the legacy mastery curve instead of being scaled
		// into the floor — the second half of issue #236.
		{"short buff, no mastery", prod, 7, 0, 100, 8},   // 64s (floored)
		{"short buff, special 100", prod, 7, 0, 200, 16}, // 2m08, scaling again
		{"short buff, special 400", prod, 7, 0, 500, 40}, // 5m20
		// Enfraquecer (51): raw 5 → 1, aggressive. The floor must NOT apply, or
		// the tuning would LENGTHEN a debuff.
		{"short debuff is not floored", prod, 1, 1, 100, 1},
		// …and neither may the target-band bypass: a hostile affect stays on the
		// flat scale even though its legacy duration sits far under the cap.
		// Perseguição (16) at the Special cap is 5 legacy ticks, kept at 1.
		{"short debuff is still scaled", prod, 0, 1, 500, 1},
		// Zero value = legacy formula, untouched.
		{"zero value is legacy", AffectDuration{}, 150, 0, 500, 755},
		{"scale 100 is legacy", AffectDuration{ScalePct: 100}, 150, 0, 500, 755},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Entity{ID: 1}
			if !e.SetAffect(11, 5, tt.affectTime, tt.aggressive, tt.delay, 0, tt.dur) {
				t.Fatal("SetAffect refused")
			}
			if got := e.Affect[0].Time; got != tt.want {
				t.Errorf("Time = %d ticks (%ds), want %d (%ds)",
					got, got*8, tt.want, tt.want*8)
			}
		})
	}
}

// TestAffectDurationSkipsLegacyClamps: the tuning only touches the normal
// branch. The 1/3/10 clamps and the "infinite" sentinel keep their exact legacy
// values, so a scale of 15% cannot turn a 4-tick clamp into 0.
func TestAffectDurationSkipsLegacyClamps(t *testing.T) {
	dur := AffectDuration{ScalePct: 15, MinTicks: 8, MaxTicks: 75}

	// SetAffect: a slot whose PREVIOUS type is 1/3/10 clamps to 4 ticks.
	e := &Entity{ID: 1}
	e.Affect[0] = Affect{Type: 3, Time: 99}
	if !e.SetAffect(3, 5, 150, 0, 500, 0, dur) {
		t.Fatal("SetAffect refused")
	}
	if got := e.Affect[0].Time; got != 4 {
		t.Errorf("sType clamp = %d, want the legacy 4 (untuned)", got)
	}

	// SetAffect: the "infinite" sentinel survives the cap.
	inf := &Entity{ID: 2}
	if !inf.SetAffect(34, 0, 150, 0, 2139062143, 1, dur) {
		t.Fatal("SetAffect refused the sentinel")
	}
	if got := inf.Affect[0].Time; got != 2139062143 {
		t.Errorf("infinite sentinel = %d, want 2139062143 (untuned)", got)
	}

	// SetTick: types 1/3/10 clamp to 2 ticks.
	p := &Entity{ID: 3}
	if !p.SetTick(3, 10, 150, 0, 500, 5, dur) {
		t.Fatal("SetTick refused")
	}
	if got := p.Affect[0].Time; got != 2 {
		t.Errorf("tick clamp = %d, want the legacy 2 (untuned)", got)
	}
}
