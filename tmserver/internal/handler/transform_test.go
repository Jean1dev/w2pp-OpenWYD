package handler

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// bmEntity builds a BM with fixed post-equip flats, so the transform's
// multiplicative deltas are hand-checkable against pTransBonus.
func bmEntity(learned int32) *world.Entity {
	e := &world.Entity{Class: 2, AC: 100, MaxHP: 1000, LearnedSkill: learned}
	e.Equip[0].Index = 21 // the BM body/class item
	return e
}

func TestApplyTransformScore(t *testing.T) {
	tests := []struct {
		name    string
		value   uint8 // Affect.Value (beast+1)
		level   uint16
		learned int32
		wantPct int32 // AffDamageMultiPct
		wantDam int32 // AffDamage (flat)
		wantAC  int32
		wantHP  int32
		wantRes int16
		wantRun int32
		wantAtt int32
		wantCri int16
		wantVis uint16 // equipVisual slot 0
	}{
		// Lobo learned (bit 0x20000), Level 200: adds Dam10/Hp5/Ac3; dam 120..140
		// →140; AC 98..108 →108 (+8 on 100) +5 flat; HP 100..110 →110 (+100).
		{"lobo learned", 1, 200, learnedWolf, 140, 10, 13, 100, 0, 1, 20, 5, 22},
		// Lobo unlearned: per-beast adds gated off, but the mesh-22 flat +10 dam /
		// +5 AC are NOT gated (Basedef.cpp:4176/4190). dam 110..130 →130; AC
		// 95..105 →105 (+5+5); HP 95..105 →105 (+50).
		{"lobo unlearned", 1, 200, 0, 130, 10, 10, 50, 0, 1, 20, 0, 22},
		// Urso learned (0x80000), Level 100: Hp15/Reg40; dam 80..100 →90; AC
		// 100..110 →105 (+5); HP 125..155 →140 (+400).
		{"urso learned", 2, 100, learnedBear, 90, 0, 5, 400, 40, 0, 40, 0, 23},
		// Astaroth learned (0x200000), Level 0: Dam10/Ac5/Hp5/Reg15; min of every
		// range: dam →110; AC →110 (+10); HP →105 (+50).
		{"astaroth learned level0", 3, 0, learnedAstaroth, 110, 0, 10, 50, 15, 1, 40, 5, 24},
		// Titã (no learned gate), Level 200: Hp5/Ac10; dam 90..110 →110; AC
		// 120..135 →135 (+35); HP 110..115 →115 (+150).
		{"tita", 4, 200, 0, 110, 0, 35, 150, 0, 0, 50, 0, 25},
		// Éden (no learned gate), Level 100: Dam10/Ac5/Hp10/Reg10; dam 115..130
		// →122; AC 115..125 →120 (+20); HP 115..125 →120 (+200).
		{"eden", 5, 100, 0, 122, 0, 20, 200, 10, 3, 40, 6, 32},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := bmEntity(tt.learned)
			e.Affect[0] = world.Affect{Type: 16, Value: tt.value, Level: tt.level, Time: 10}
			applyAffectScore(e)
			if e.AffDamageMultiPct != tt.wantPct {
				t.Errorf("AffDamageMultiPct = %d, want %d", e.AffDamageMultiPct, tt.wantPct)
			}
			if e.AffDamage != tt.wantDam {
				t.Errorf("AffDamage = %d, want %d", e.AffDamage, tt.wantDam)
			}
			if e.AffAC != tt.wantAC {
				t.Errorf("AffAC = %d, want %d", e.AffAC, tt.wantAC)
			}
			if e.AffMaxHP != tt.wantHP {
				t.Errorf("AffMaxHP = %d, want %d", e.AffMaxHP, tt.wantHP)
			}
			for k, r := range e.AffResist {
				if r != tt.wantRes {
					t.Errorf("AffResist[%d] = %d, want %d", k, r, tt.wantRes)
				}
			}
			if e.AffRunSpeed != tt.wantRun || e.AffAttackSpeed != tt.wantAtt || e.AffCritical != tt.wantCri {
				t.Errorf("AffRun/AffAttack/AffCritical = %d/%d/%d, want %d/%d/%d",
					e.AffRunSpeed, e.AffAttackSpeed, e.AffCritical, tt.wantRun, tt.wantAtt, tt.wantCri)
			}
			if v := equipVisual(e); v[0] != tt.wantVis {
				t.Errorf("equipVisual[0] = %d, want %d (beast mesh)", v[0], tt.wantVis)
			}
		})
	}
}

// TestTransformClassGuard is the issue #21 regression: a Type-16 affect on a
// non-BM (e.g. leaked onto a TransKnight) must neither change the score nor
// render the beast mesh.
func TestTransformClassGuard(t *testing.T) {
	e := bmEntity(learnedWolf)
	e.Class = 0 // TransKnight
	e.Affect[0] = world.Affect{Type: 16, Value: 1, Level: 200, Time: 10}
	applyAffectScore(e)
	if e.AffDamageMultiPct != 100 || e.AffDamage != 0 || e.AffAC != 0 || e.AffMaxHP != 0 {
		t.Errorf("non-BM transform changed the score: pct=%d dam=%d ac=%d hp=%d",
			e.AffDamageMultiPct, e.AffDamage, e.AffAC, e.AffMaxHP)
	}
	if v := equipVisual(e); v[0] != 21 {
		t.Errorf("equipVisual[0] = %d, want the real body item 21 (no beast mesh on a TK)", v[0])
	}
}

// TestTransformOutOfRangeValueInert covers Affect.Value 0 / >5 (corrupt or
// unknown beast): the slot is ignored rather than crashing or half-applying.
func TestTransformOutOfRangeValueInert(t *testing.T) {
	for _, v := range []uint8{0, 6, 200} {
		e := bmEntity(0)
		e.Affect[0] = world.Affect{Type: 16, Value: v, Level: 100, Time: 10}
		applyAffectScore(e)
		if e.AffDamageMultiPct != 100 || e.AffAC != 0 || e.AffMaxHP != 0 {
			t.Errorf("value %d: transform applied, want inert", v)
		}
		if vis := equipVisual(e); vis[0] != 21 {
			t.Errorf("value %d: equipVisual[0] = %d, want 21", v, vis[0])
		}
	}
}

// TestTransformDamageMultiplier checks effectiveDamage applies the DAMAGEMULTI
// percentage over Damage+AffDamage (Basedef.cpp:4654) — and that entities whose
// affect pass never ran (zero-value AffDamageMultiPct) are untouched.
func TestTransformDamageMultiplier(t *testing.T) {
	d := New(Config{Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	e := &world.Entity{Damage: 200}
	e.AffDamage = 10
	e.AffDamageMultiPct = 140
	if got := d.effectiveDamage(e); got != 294 { // (200+10)*140/100
		t.Errorf("effectiveDamage = %d, want 294", got)
	}
	mob := &world.Entity{Damage: 500} // mob: applyAffectScore never ran, pct == 0
	if got := d.effectiveDamage(mob); got != 500 {
		t.Errorf("mob effectiveDamage = %d, want 500 (zero-value pct must be neutral)", got)
	}
}

// startServerAffectTick is startServerClock plus the dispatcher tick (10ms), so
// the affect sweep runs — the transform-expiry path needs it.
func startServerAffectTick(t *testing.T, persist world.Persistence) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	clock := &atomic.Uint32{}
	clock.Store(serverTime)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 16, Now: clock.Load}, log, persist, d.Handle)
	w.SetTickHandler(10*time.Millisecond, d.Tick)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	return ln.Addr().String(), func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}
}

// TestTransformCastBroadcastsMesh drives the cast flow on the wire: a BM
// self-casts a transform skill (66 = Urso, AffectType 16/Value 2) and the
// server must push _MSG_UpdateEquip with the beast mesh 23 in slot 0 — the
// legacy GetCurrentScore+SendScore+SendEquip after the SetAffect
// (_MSG_Attack.cpp:1242-1248).
func TestTransformCastBroadcastsMesh(t *testing.T) {
	db := newDB()
	st := world.CharacterState{
		Slot: 0, Name: "Beast", Class: 2, X: 5, Y: 5,
		HP: 500, MaxHP: 500, MP: 500, MaxMP: 500, Level: 50,
		LearnedSkill: 1 << 18, // skill 66 learned (66%24 = bit 18)
	}
	st.Equip[0] = world.Item{Index: 21}
	db.loadResult = st

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	clock := &atomic.Uint32{}
	clock.Store(serverTime)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	spells := []content.Spell{{Index: 66, ManaSpent: 10, AffectType: 16, AffectValue: 2,
		AffectTime: 600, MaxTarget: 1, Name: "Urso"}}
	d := New(Config{Log: log, Spells: content.NewSkillData(spells)})
	w := world.New(world.Config{GridDim: 16, Now: clock.Load}, log, db, d.Handle)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}()

	c := enterWorld(t, ln.Addr().String())
	defer c.Close()

	skillAttackFrame(t, c, serverTime, 1, 66, damSkill) // self-cast (conn 1)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			continue
		}
		if ty != protocol.MsgUpdateEquip {
			continue
		}
		if got := binary.LittleEndian.Uint16(payload[0:2]); got != 23 {
			t.Fatalf("post-cast UpdateEquip slot0 = %d, want 23 (Urso mesh)", got)
		}
		return
	}
	t.Fatal("transform cast landed but no UpdateEquip mesh broadcast was sent")
}

// TestTransformExpiryRevertsMesh drives the wire flow: a BM enters the world
// with a nearly-expired persisted transform; when the affect sweep zeroes it,
// the server must broadcast _MSG_UpdateEquip with the REAL body item back in
// slot 0 (the legacy FaceChange → SendEquip revert, Server.cpp:5836-5886).
func TestTransformExpiryRevertsMesh(t *testing.T) {
	db := newDB()
	st := world.CharacterState{
		Slot: 0, Name: "Beast", Class: 2, X: 2100, Y: 2100,
		HP: 500, MaxHP: 500, Level: 50,
		Affects: []world.Affect{{Type: 16, Value: 2, Level: 100, Time: 1}},
	}
	st.Equip[0] = world.Item{Index: 21}
	db.loadResult = st
	addr, stop := startServerAffectTick(t, db)
	defer stop()

	c := enterWorld(t, addr)
	defer c.Close()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			continue
		}
		if ty != protocol.MsgUpdateEquip {
			continue
		}
		if got := binary.LittleEndian.Uint16(payload[0:2]); got != 21 {
			t.Fatalf("post-expiry UpdateEquip slot0 = %d, want 21 (reverted body item)", got)
		}
		return
	}
	t.Fatal("transform expired but no UpdateEquip revert was broadcast")
}
