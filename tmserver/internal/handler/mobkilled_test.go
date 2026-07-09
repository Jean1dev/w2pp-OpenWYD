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

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestAddClamp guards the per-level HP/MP accumulation against int32 overflow and
// the engine cap (the level-up loop can add to an already-large MaxHp).
func TestAddClamp(t *testing.T) {
	tests := []struct {
		name          string
		v, inc, limit int32
		want          int32
	}{
		{"normal add", 800, 3, level.MaxHPCap, 803},
		{"clamp at cap", level.MaxHPCap - 1, 5, level.MaxHPCap, level.MaxHPCap},
		{"no negative", 0, -10, level.MaxHPCap, 0},
		{"overflow guarded", 2_000_000_000, 2_000_000_000, level.MaxHPCap, level.MaxHPCap},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := addClamp(tt.v, tt.inc, tt.limit); got != tt.want {
				t.Errorf("addClamp(%d,%d,%d) = %d, want %d", tt.v, tt.inc, tt.limit, got, tt.want)
			}
		})
	}
}

// expMobTemplate builds an 816-byte STRUCT_MOB monster template (Merchant=0)
// with the given level, kill reward and clan.
func expMobTemplate(lvl int32, exp int64, clan uint8) []byte {
	b := make([]byte, 816)
	copy(b[0:16], "Cobaia")
	b[16] = clan
	binary.LittleEndian.PutUint64(b[32:], uint64(exp))
	const cs = 92
	binary.LittleEndian.PutUint32(b[cs+0:], uint32(lvl))
	binary.LittleEndian.PutUint32(b[cs+16:], 100) // MaxHp
	binary.LittleEndian.PutUint32(b[cs+24:], 100) // Hp
	return b
}

// mobKilledWorld builds a non-serving world plus a detached mortal killer (no
// session — grantExp's packet sends are skipped, the state math is the same).
func mobKilledWorld(t *testing.T) (*Dispatcher, *world.World, *world.Entity) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 16}, log, nil, d.Handle)
	killer := &world.Entity{
		ID: 0, Mode: world.MobUser, Name: "Heroi",
		Level: 1, ClassMaster: classMasterMortal,
		BaseMaxHP: 80, BaseMaxMP: 45, HP: 40, MaxHP: 80, MP: 20, MaxMP: 45,
		Str: 8, Int: 4, Dex: 7, Con: 6, // TK base attributes
	}
	return d, w, killer
}

// TestMobKilledGrantsExp is the issue #43 end-to-end: a default (mortal)
// level-1 character killing a same-level mob must gain the general-field
// branch reward — not the near-zero celestial routing, not dozens of levels.
func TestMobKilledGrantsExp(t *testing.T) {
	d, w, killer := mobKilledWorld(t)
	mobID := w.SpawnMob(expMobTemplate(1, 1000, 0), 6, 5)
	if mobID < 0 {
		t.Fatal("SpawnMob failed")
	}

	d.mobKilled(w, killer, w.Entity(mobID))
	// 450*1000/31=14516 → ÷1 → ×0.6=8709 → eMob cap 1000 → Kefra down 500 → −15%.
	if killer.Exp != 425 {
		t.Errorf("killer.Exp = %d, want 425", killer.Exp)
	}
	if killer.Level != 1 {
		t.Errorf("killer.Level = %d, want 1 (425 < nextLevel[2]=1124)", killer.Level)
	}
}

// TestMobKilledLevelUp crosses one threshold and checks the level-up grants.
func TestMobKilledLevelUp(t *testing.T) {
	d, w, killer := mobKilledWorld(t)
	killer.Exp = 1100 // nextLevel[2]=1124: the next kill must cross exactly one level
	mobID := w.SpawnMob(expMobTemplate(1, 1000, 0), 6, 5)

	d.mobKilled(w, killer, w.Entity(mobID))
	if killer.Exp != 1525 {
		t.Fatalf("killer.Exp = %d, want 1525", killer.Exp)
	}
	if killer.Level != 2 {
		t.Fatalf("killer.Level = %d, want 2 (1525 < nextLevel[3]=1826)", killer.Level)
	}
	if killer.SkillBonus != 3 || killer.SpecialBonus != 2 {
		t.Errorf("SkillBonus/SpecialBonus = %d/%d, want 3/2", killer.SkillBonus, killer.SpecialBonus)
	}
	if killer.BaseMaxHP != 83 { // TK IncHP = 3
		t.Errorf("BaseMaxHP = %d, want 83", killer.BaseMaxHP)
	}
	if killer.HP != killer.MaxHP || killer.MP != killer.MaxMP || killer.MaxHP <= 0 {
		t.Errorf("HP/MP = %d/%d of %d/%d, want full heal", killer.HP, killer.MP, killer.MaxHP, killer.MaxMP)
	}
}

// TestMobKilledClan4NoExp: clan-4 mobs sit outside the whole EXP distribution
// (MobKilled.cpp:402) but still drop gold/items.
func TestMobKilledClan4NoExp(t *testing.T) {
	d, w, killer := mobKilledWorld(t)
	mobID := w.SpawnMob(expMobTemplate(1, 1000, 4), 6, 5)

	d.mobKilled(w, killer, w.Entity(mobID))
	if killer.Exp != 0 {
		t.Errorf("killer.Exp = %d, want 0 for a clan-4 mob", killer.Exp)
	}
}

// startServerExpMob is a serving harness with one killable (1 HP) exp-bearing
// monster next to the player spawn.
func startServerExpMob(t *testing.T, persist world.Persistence, mob []byte) (string, func()) {
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
	w.SpawnMob(mob, 6, 5) // adjacent to the player spawn (5,5)
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

// TestKillGrantsExpOverWire is the full issue #43 regression through the wire:
// a freshly loaded character (the DB contract carries no ClassMaster) kills a
// same-level mob and the MSG_Attack echo must carry the MORTAL-path reward. If
// character load ever stops defaulting ClassMaster to mortal, the celestial
// divisors turn this exact gain into 0.
func TestKillGrantsExpOverWire(t *testing.T) {
	mob := expMobTemplate(10, 1000, 0)            // same level as skillCombatDB's char
	binary.LittleEndian.PutUint32(mob[92+16:], 1) // 1 HP: first melee kills
	binary.LittleEndian.PutUint32(mob[92+24:], 1)
	addr, stop := startServerExpMob(t, skillCombatDB(0), mob)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	skillAttackFrame(t, c, serverTime, world.MaxUser, -1, -2) // plain melee

	for i := 0; i < 10; i++ {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			t.Fatal("no MsgAttack echo after the killing blow")
		}
		if ty != protocol.MsgAttack {
			continue // skip the despawn/etc noise
		}
		var got protocol.MsgAttackBody
		if err := got.Decode(payload); err != nil {
			t.Fatal(err)
		}
		// 450*1000/40=11250 → ÷1 → ×0.6 → eMob cap 1000 → Kefra down 500 → −15%.
		if got.CurrentExp != 425 {
			t.Fatalf("CurrentExp = %d, want 425 (mortal general-field reward)", got.CurrentExp)
		}
		return
	}
	t.Fatal("never received the MsgAttack echo")
}
