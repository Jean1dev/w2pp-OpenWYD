package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestWaterRoomForVolatile pins the scroll → room mapping of all three chains,
// including the boss summon and the unreachable room 8: no catalog item carries
// that volatile, so it must NOT resolve.
func TestWaterRoomForVolatile(t *testing.T) {
	tests := []struct {
		name    string
		vol     int
		variant int
		room    int
		ok      bool
	}{
		{"M LV1", 21, waterM, 0, true},
		{"M LV8", 28, waterM, 7, true},
		{"M dead room", 29, 0, 0, false},
		{"M boss", 30, waterM, waterBossRoom, true},
		{"N LV1", 131, waterN, 0, true},
		{"N LV8", 138, waterN, 7, true},
		{"N dead room", 139, 0, 0, false},
		{"N boss", 140, waterN, waterBossRoom, true},
		{"A LV1", 161, waterA, 0, true},
		{"A LV8", 168, waterA, 7, true},
		{"A dead room", 169, 0, 0, false},
		{"A boss", 170, waterA, waterBossRoom, true},
		{"not a scroll", 11, 0, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			variant, room, ok := waterRoomForVolatile(tc.vol)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if ok && (variant != tc.variant || room != tc.room) {
				t.Errorf("= variant %d room %d, want %d/%d", variant, room, tc.variant, tc.room)
			}
		})
	}
}

// TestWaterRewardChain is the progression: clearing room N hands out the scroll
// that opens room N+1, and clearing the LAST numbered room (7, the LV8 scroll)
// hands out the boss summon. Those item ids are the ones that make the chain
// self-sustaining — only the LV1 scroll drops from mobs.
func TestWaterRewardChain(t *testing.T) {
	tests := []struct {
		name    string
		variant int
		room    int
		want    int16
	}{
		{"M room1 gives LV2", waterM, 0, 778},
		{"M room7 gives LV8", waterM, 6, 784},
		{"M room8 gives Neses", waterM, 7, 785},
		{"N room1 gives LV2", waterN, 0, 3174},
		{"N room8 gives Neses", waterN, 7, 3181},
		{"A room1 gives LV2", waterA, 0, 3183},
		{"A room8 gives Neses", waterA, 7, 3190},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := waterVariants[tc.variant].rewardBase + int16(tc.room)
			if got != tc.want {
				t.Errorf("reward = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestWaterRewardOpensNextRoom closes the loop: the item handed out for clearing
// room N must be the one whose volatile opens room N+1. A drift between
// rewardBase and volLo would break the chain silently.
func TestWaterRewardOpensNextRoom(t *testing.T) {
	// Catalog volatiles of the reward items, by variant (ItemList.csv).
	rewardVol := [3]int{waterN: 132, waterM: 22, waterA: 162}
	for v := range waterVariants {
		// Clearing room 0 gives rewardBase+0, which must carry the volatile that
		// resolves to room 1 of the same dungeon.
		variant, room, ok := waterRoomForVolatile(rewardVol[v])
		if !ok || variant != v || room != 1 {
			t.Errorf("variant %d: reward volatile %d → variant %d room %d ok=%v, want %d/1",
				v, rewardVol[v], variant, room, ok, v)
		}
	}
}

func TestWaterBlockRoom(t *testing.T) {
	tests := []struct {
		name    string
		gen     int
		variant int
		room    int
		ok      bool
	}{
		{"M room 1", world.WaterGenBaseM, waterM, 0, true},
		{"M room 8 (Aqua Golem)", world.WaterGenBaseM + 7, waterM, 7, true},
		{"M boss first", world.WaterGenBaseM + 8, waterM, waterBossRoom, true},
		{"M boss last", world.WaterGenBaseM + 11, waterM, waterBossRoom, true},
		{"N room 1", world.WaterGenBaseN, waterN, 0, true},
		{"A boss last", world.WaterGenBaseA + 11, waterA, waterBossRoom, true},
		{"unrelated block", 500, 0, 0, false},
		{"no generator", -1, 0, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			variant, room, ok := waterBlockRoom(tc.gen)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if ok && (variant != tc.variant || room != tc.room) {
				t.Errorf("= variant %d room %d, want %d/%d", variant, room, tc.variant, tc.room)
			}
		})
	}
}

// TestWaterBossBlockWeights pins the legacy roll distribution
// (_MSG_UseItem.cpp:1802-1815): 40% / 10% / 10% / 40%.
func TestWaterBossBlockWeights(t *testing.T) {
	counts := map[int]int{}
	for roll := 0; roll < 10; roll++ {
		counts[waterBossBlock(roll)]++
	}
	want := map[int]int{8: 4, 9: 1, 10: 1, 11: 4}
	for offset, n := range want {
		if counts[offset] != n {
			t.Errorf("offset +%d drawn %d/10 times, want %d", offset, counts[offset], n)
		}
	}
}

// TestInsideAnyWaterRoom checks the entry geometry: any room of the SAME dungeon
// counts (that is what lets a cleared party open the next room from where they
// stand), the radius widens for the boss room, and another dungeon's rooms do not.
func TestInsideAnyWaterRoom(t *testing.T) {
	mRoom0 := waterScrollPosition[waterM][0] // {1250, 3682}
	tests := []struct {
		name    string
		variant int
		x, y    int16
		want    bool
	}{
		{"dead centre of M room 1", waterM, mRoom0[0], mRoom0[1], true},
		{"M room 1 edge", waterM, mRoom0[0] + waterRoomRadius, mRoom0[1], true},
		{"just outside M room 1", waterM, mRoom0[0] + waterRoomRadius + 1, mRoom0[1] + 20, false},
		{"boss box is wider", waterM, 1214 + waterBossRadius, 3646, true},
		{"N centre is not in M", waterM, waterScrollPosition[waterN][0][0], waterScrollPosition[waterN][0][1], false},
		{"far away", waterM, 5, 5, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := insideAnyWaterRoom(tc.variant, tc.x, tc.y); got != tc.want {
				t.Errorf("insideAnyWaterRoom(%d, %d, %d) = %v, want %v", tc.variant, tc.x, tc.y, got, tc.want)
			}
		})
	}
}

// TestOnWaterStagingTile covers the square a chain is started from: standing in
// no room at all, the scroll is still accepted there (_MSG_UseItem.cpp:1750).
func TestOnWaterStagingTile(t *testing.T) {
	tests := []struct {
		name string
		x, y int16
		want bool
	}{
		{"first cell of the tile", 1964, 1772, true},
		{"last cell of the tile", 1967, 1775, true},
		{"one row below", 1964, 1776, false},
		{"one column left", 1963, 1772, false},
		// The exit lands ON the square since 2026-09-12 — see waterExit. It used
		// to land three tiles short, and a party thrown out had to find its way
		// back here before a scroll would work.
		{"the exit point itself", waterExit[0], waterExit[1], true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := onWaterStagingTile(tc.x, tc.y); got != tc.want {
				t.Errorf("onWaterStagingTile(%d, %d) = %v, want %v", tc.x, tc.y, got, tc.want)
			}
		})
	}
}

// TestWaterDungeonGeneratorsExcludedFromBoot guards the invariant the whole
// feature rests on: the room blocks must NOT be populated at boot, or a room is
// never empty, entry is always refused and the clear reward never fires.
func TestWaterDungeonGeneratorsExcludedFromBoot(t *testing.T) {
	for _, base := range []int{world.WaterGenBaseN, world.WaterGenBaseM, world.WaterGenBaseA} {
		for off := 0; off < 12; off++ {
			if !world.IsWaterDungeonGenerator(base + off) {
				t.Errorf("block %d (base %d +%d) is not recognised as a water room", base+off, base, off)
			}
		}
	}
	// Blocks outside every span. N (171..182) and A (183..194) are ADJACENT, so
	// the only free neighbours are below M and above A.
	for _, idx := range []int{world.WaterGenBaseM - 1, world.WaterGenBaseA + 12, 0, 500} {
		if world.IsWaterDungeonGenerator(idx) {
			t.Errorf("block %d is outside every water span but was claimed", idx)
		}
	}
}

// TestWaterCountdownUnits documents the timer arithmetic: one unit is 2 seconds,
// so the wire value (units*2) is the countdown in seconds — 60s in a numbered
// room, 30s in the boss room, cut to 30s/10s once cleared.
func TestWaterCountdownUnits(t *testing.T) {
	tests := []struct {
		name    string
		units   int
		seconds int
	}{
		{"numbered room entry", waterRoomTime, 60},
		{"boss room entry", waterBossTime, 30},
		{"numbered room cleared", waterRoomClearTime, 30},
		{"boss room cleared", waterBossClearTime, 10},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.units * waterTickPeriod; got != tc.seconds {
				t.Errorf("%d units = %ds, want %ds", tc.units, got, tc.seconds)
			}
		})
	}
}

// TestWaterRewardPaysOncePerRun is the anti-farm invariant. The clear is
// detected from the block's population reaching its last mob, and that can
// happen repeatedly within one run — which is how a single M scroll minted six.
// The payout is charged to the run, so only the first clear pays.
func TestWaterRewardPaysOncePerRun(t *testing.T) {
	d := &Dispatcher{}

	if !d.claimWaterReward(waterM, 0) {
		t.Fatal("first clear of a fresh run did not pay")
	}
	for i := 0; i < 5; i++ {
		if d.claimWaterReward(waterM, 0) {
			t.Fatalf("clear %d paid again within the same run", i+2)
		}
	}
	// Other rooms and other dungeons keep their own credit.
	if !d.claimWaterReward(waterM, 1) {
		t.Error("room 1 of the same dungeon was blocked by room 0's payout")
	}
	if !d.claimWaterReward(waterN, 0) {
		t.Error("dungeon N was blocked by dungeon M's payout")
	}

	// Re-entering re-arms the room: a new run owes a new reward.
	d.events.waterPaid[waterM][0] = false
	if !d.claimWaterReward(waterM, 0) {
		t.Error("a fresh run did not re-arm the payout")
	}
}

// TestWaterClassGate pins the per-chain tier scoping (a server rule, not legacy).
func TestWaterClassGate(t *testing.T) {
	tests := []struct {
		name    string
		variant int
		class   uint8
		level   int32
		want    bool
	}{
		// N: mortals only.
		{"N mortal", waterN, classMasterMortal, 399, true},
		{"N arch", waterN, classMasterArch, 200, false},
		{"N celestial", waterN, classMasterCelestial, 10, false},

		// M: arch at any level, celestial only to 40.
		{"M arch low", waterM, classMasterArch, 1, true},
		{"M arch max", waterM, classMasterArch, 399, true},
		{"M celestial at the cap", waterM, classMasterCelestial, 40, true},
		{"M celestial past the cap", waterM, classMasterCelestial, 41, false},
		{"M mortal", waterM, classMasterMortal, 399, false},
		{"M subcelestial", waterM, classMasterCelestialCS, 10, false},
		{"M supreme celestial", waterM, classMasterSCelestial, 10, false},

		// A: every celestial tier, uncapped.
		{"A celestial", waterA, classMasterCelestial, 199, true},
		{"A subcelestial", waterA, classMasterCelestialCS, 199, true},
		{"A supreme celestial", waterA, classMasterSCelestial, 199, true},
		{"A celestial past the M cap", waterA, classMasterCelestial, 41, true},
		{"A arch", waterA, classMasterArch, 399, false},
		{"A mortal", waterA, classMasterMortal, 399, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := waterClassAllowed(tc.variant, tc.class, tc.level); got != tc.want {
				t.Errorf("waterClassAllowed(%d, class %d, level %d) = %v, want %v",
					tc.variant, tc.class, tc.level, got, tc.want)
			}
		})
	}
}

// TestWaterRoomMobCapIsUniform guards the one-cap rule. The shipped blocks
// disagree (MaxNumMob 8, 24 and 38 across the M chain), which is what made a
// run's difficulty depend on which room you opened.
func TestWaterRoomMobCapIsUniform(t *testing.T) {
	if waterRoomMobCap != 20 {
		t.Errorf("waterRoomMobCap = %d, want 20", waterRoomMobCap)
	}
}
