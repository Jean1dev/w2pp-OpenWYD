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
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/worldcfg"
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
		// TK class-base attributes, no allocated points and no gear, so the live
		// CurrentScore (Str/…) equals the equipment-free BaseScore (BaseStr/…).
		Str: 8, Int: 4, Dex: 7, Con: 6,
		BaseStr: 8, BaseInt: 4, BaseDex: 7, BaseCon: 6,
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
	// A TK at the class base with nothing allocated gets level*5 free attribute
	// points (B10): 2 levels → 10. Zero here is the "no points on level-up" bug.
	if killer.ScoreBonus != 10 {
		t.Errorf("ScoreBonus = %d, want 10 (level 2 × 5, nothing spent)", killer.ScoreBonus)
	}
	if killer.BaseMaxHP != 83 { // TK IncHP = 3
		t.Errorf("BaseMaxHP = %d, want 83", killer.BaseMaxHP)
	}
	if killer.HP != killer.MaxHP || killer.MP != killer.MaxMP || killer.MaxHP <= 0 {
		t.Errorf("HP/MP = %d/%d of %d/%d, want full heal", killer.HP, killer.MP, killer.MaxHP, killer.MaxMP)
	}
}

// TestMobKilledLevelUpPointsIgnoreEquipAttributes is the regression guard for the
// "no attribute points on level-up while wearing gear" bug: BASE_GetBonusScorePoint
// must read the equipment-free BaseScore. Here the live CurrentScore attributes are
// inflated far above the class base by equipment (Str 8→108), while the allocated
// BaseScore is untouched. The free-point grant must stay level*5 (nothing allocated);
// the pre-fix code fed the inflated e.Str in, counted 100 points as "spent", and
// handed the player ZERO.
func TestMobKilledLevelUpPointsIgnoreEquipAttributes(t *testing.T) {
	d, w, killer := mobKilledWorld(t)
	// +100 Str from equipment: CurrentScore diverges from the allocated BaseScore.
	killer.Str = killer.BaseStr + 100
	killer.Exp = 1100 // crosses exactly one level (nextLevel[2]=1124)
	mobID := w.SpawnMob(expMobTemplate(1, 1000, 0), 6, 5)

	d.mobKilled(w, killer, w.Entity(mobID))
	if killer.Level != 2 {
		t.Fatalf("killer.Level = %d, want 2", killer.Level)
	}
	if killer.ScoreBonus != 10 {
		t.Errorf("ScoreBonus = %d, want 10 — equipment attribute bonus must not consume free points", killer.ScoreBonus)
	}
}

// TestMobKilledLevelUpPointsAfterAllocation: points already spent into the
// BaseScore reduce the grant (the idempotent BASE_GetBonusScorePoint identity),
// while equipment on top of them still does not. Level 2 grants 10; 4 allocated
// into Str leaves 6 free even with gear inflating the live Str further.
func TestMobKilledLevelUpPointsAfterAllocation(t *testing.T) {
	d, w, killer := mobKilledWorld(t)
	killer.BaseStr = 8 + 4           // 4 points allocated above the TK base
	killer.Str = killer.BaseStr + 30 // plus equipment on top
	killer.Exp = 1100
	mobID := w.SpawnMob(expMobTemplate(1, 1000, 0), 6, 5)

	d.mobKilled(w, killer, w.Entity(mobID))
	if killer.ScoreBonus != 6 {
		t.Errorf("ScoreBonus = %d, want 6 (level 2 × 5 − 4 allocated)", killer.ScoreBonus)
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

func TestApplyWorldEventConfigUpdatesExpAndDropState(t *testing.T) {
	d, w, _ := mobKilledWorld(t)
	snap := worldcfg.Snapshot{
		Version: 9,
		Event: worldcfg.EventConfig{
			Enabled: true, ItemIndex: 777, Rate: 10,
			StartIndex: 100, CurrentIndex: 101, EndIndex: 200,
			Indexed: true, NoticeEnabled: true,
			DoubleExpEnabled: true, NewbieEventEnabled: true,
		},
	}

	d.applyWorldEventConfig(w, snap)
	if !d.expEvents.DoubleMode || !d.expEvents.NewbieEvent {
		t.Fatalf("exp events = %+v, want double+newbie enabled", d.expEvents)
	}
	got := w.WorldEventConfig()
	if got.Version != 9 || !got.Enabled || got.ItemIndex != 777 || got.CurrentIndex != 101 || !got.Indexed {
		t.Errorf("world event config = %+v, want snapshot applied", got)
	}
}

func TestMobKilledWorldEventDropIndexed(t *testing.T) {
	d, w, killer := mobKilledWorld(t)
	w.SetWorldEventConfig(world.WorldEventConfig{
		Version: 3, Enabled: true, ItemIndex: 777, Rate: 1,
		StartIndex: 100, CurrentIndex: 100, EndIndex: 101,
		Indexed: true,
	})
	mobID := w.SpawnMob(expMobTemplate(1, 1000, 0), 6, 5)

	d.mobKilled(w, killer, w.Entity(mobID))

	if got := w.WorldEventConfig().CurrentIndex; got != 101 {
		t.Fatalf("CurrentIndex = %d, want 101", got)
	}
	gi := w.GroundItemAt(6, 5)
	if gi == nil {
		t.Fatal("event drop was not placed on the ground")
	}
	if gi.Item.Index != 777 {
		t.Fatalf("event item index = %d, want 777", gi.Item.Index)
	}
	want := [2]world.Effect{{Effect: eventSerialHi, Value: 0}, {Effect: eventSerialLo, Value: 100}}
	if gi.Item.Effects[0] != want[0] || gi.Item.Effects[1] != want[1] || gi.Item.Effects[2].Effect != eventSerialRand {
		t.Errorf("event effects = %+v, want serial hi/lo and rand effect", gi.Item.Effects)
	}
}

func TestMobKilledWorldEventDropExhaustedDoesNothing(t *testing.T) {
	d, w, killer := mobKilledWorld(t)
	w.SetWorldEventConfig(world.WorldEventConfig{
		Version: 3, Enabled: true, ItemIndex: 777, Rate: 1,
		StartIndex: 100, CurrentIndex: 101, EndIndex: 101,
		Indexed: true,
	})
	mobID := w.SpawnMob(expMobTemplate(1, 1000, 0), 6, 5)

	d.mobKilled(w, killer, w.Entity(mobID))

	if gi := w.GroundItemAt(6, 5); gi != nil {
		t.Fatalf("exhausted event placed ground item %+v", gi)
	}
	if got := w.WorldEventConfig().CurrentIndex; got != 101 {
		t.Errorf("CurrentIndex = %d, want unchanged 101", got)
	}
}

func TestApplyBabyMountKillGrowth(t *testing.T) {
	mount := babyMountItem(2330, 80)
	mount.Effects[1].Effect = 5
	mount.Effects[2].Value = 1

	changed, leveled := applyBabyMountKill(&mount, 6)
	if !changed || leveled {
		t.Fatalf("applyBabyMountKill changed/leveled = %v/%v, want true/false", changed, leveled)
	}
	if got := mount.Effects[2].Value; got != 2 {
		t.Errorf("growth = %d, want 2", got)
	}
	if got := mount.Effects[1].Effect; got != 5 {
		t.Errorf("level = %d, want unchanged 5", got)
	}
}

func TestApplyBabyMountKillLevelUp(t *testing.T) {
	mount := babyMountItem(2330, 80)
	mount.Effects[1].Effect = 5
	mount.Effects[2].Value = 29 // threshold for 2330 at level 5 is 30.

	changed, leveled := applyBabyMountKill(&mount, 6)
	if !changed || !leveled {
		t.Fatalf("applyBabyMountKill changed/leveled = %v/%v, want true/true", changed, leveled)
	}
	if got := mount.Effects[1].Effect; got != 6 {
		t.Errorf("level = %d, want 6", got)
	}
	if got := mount.Effects[2].Value; got != 1 {
		t.Errorf("growth after level-up = %d, want 1", got)
	}
}

func TestApplyBabyMountKillDefaultThreshold(t *testing.T) {
	mount := babyMountItem(2336, 80)
	mount.Effects[1].Effect = 5
	mount.Effects[2].Value = 104 // default threshold is XP+100 = 105.

	changed, leveled := applyBabyMountKill(&mount, 6)
	if !changed || !leveled {
		t.Fatalf("applyBabyMountKill changed/leveled = %v/%v, want true/true", changed, leveled)
	}
	if got := mount.Effects[1].Effect; got != 6 {
		t.Errorf("level = %d, want 6", got)
	}
}

func TestApplyBabyMountKillRejectsIneligible(t *testing.T) {
	tests := []struct {
		name     string
		mount    world.Item
		mobLevel int
	}{
		{name: "not mount", mount: babyMountItem(2329, 80), mobLevel: 6},
		{name: "mob level too low", mount: babyMountItem(2330, 80), mobLevel: 5},
		{name: "level cap", mount: func() world.Item {
			it := babyMountItem(2330, 80)
			it.Effects[1].Effect = 100
			return it
		}(), mobLevel: 101},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := tt.mount
			changed, leveled := applyBabyMountKill(&tt.mount, tt.mobLevel)
			if changed || leveled {
				t.Fatalf("applyBabyMountKill changed/leveled = %v/%v, want false/false", changed, leveled)
			}
			if tt.mount != before {
				t.Errorf("mount changed from %+v to %+v", before, tt.mount)
			}
		})
	}
}

func TestBabyMountKillEligible(t *testing.T) {
	killer := &world.Entity{
		ID:          world.MaxUser,
		Clan:        summonClan,
		Summoner:    1,
		EquipVisual: [world.MaxEquip]uint16{babyMountFaceLo},
	}
	mob := &world.Entity{ID: world.MaxUser + 1, Clan: 0}
	if !babyMountKillEligible(killer, mob) {
		t.Fatal("babyMountKillEligible = false, want true")
	}

	for _, tc := range []struct {
		name string
		k    *world.Entity
		m    *world.Entity
	}{
		{"player killer", &world.Entity{ID: 1, Clan: summonClan, Summoner: 1, EquipVisual: killer.EquipVisual}, mob},
		{"non summon clan", &world.Entity{ID: world.MaxUser, Clan: 0, Summoner: 1, EquipVisual: killer.EquipVisual}, mob},
		{"non mount face", &world.Entity{ID: world.MaxUser, Clan: summonClan, Summoner: 1}, mob},
		{"player victim", killer, &world.Entity{ID: 2, Clan: 0}},
		{"summon victim", killer, &world.Entity{ID: world.MaxUser + 1, Clan: summonClan}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if babyMountKillEligible(tc.k, tc.m) {
				t.Fatal("babyMountKillEligible = true, want false")
			}
		})
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
