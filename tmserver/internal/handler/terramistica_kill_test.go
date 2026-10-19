package handler

import (
	"encoding/binary"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/rng"
)

// faceMobTemplate is a loot-free, gold-free mob template wearing the given face
// (STRUCT_MOB.Equip[0].sIndex).
func faceMobTemplate(face uint16) []byte {
	b := expMobTemplate(100, 0, 0)
	binary.LittleEndian.PutUint16(b[mobFaceOffset:], face)
	return b
}

// TestTerraMisticaKillRoll checks MobKilled.cpp:2909-2921 call by call: a
// face-239/241 mob draws exactly one rand() % 20 and, on 0, moves the flag
// 1 -> 2; other faces draw nothing and other flags never change.
func TestTerraMisticaKillRoll(t *testing.T) {
	tests := []struct {
		name     string
		face     uint16
		flag     uint8
		wantRoll bool // whether the face consumes rand()
		canGrant bool // whether a winning roll moves the flag to 2
	}{
		{"Draco_Lich face 239 on mission", 239, 1, true, true},
		{"Basilisco face 241 on mission", 241, 1, true, true},
		{"face 240 never rolls", 240, 1, false, false},
		{"face 239 without the mission still rolls", 239, 0, true, false},
		{"face 241 after step 2 still rolls", 241, 2, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sawWin, sawLoss := false, false
			for seed := uint32(1); seed <= 200; seed++ {
				d, w, killer := mobKilledWorld(t)
				killer.TerraMistica = tt.flag
				*w.Rand() = *rng.NewSeeded(seed)
				mob := w.Entity(w.SpawnMob(faceMobTemplate(tt.face), 6, 5))

				probe := *rng.NewSeeded(seed)
				win := tt.wantRoll && probe.Intn(20) == 0
				if !tt.wantRoll {
					probe = *rng.NewSeeded(seed)
				}

				d.terraMisticaKill(w, nil, killer, mob)

				if *w.Rand() != probe {
					t.Fatalf("seed %d: rand() consumption differs from the legacy (roll=%v)", seed, tt.wantRoll)
				}
				want := tt.flag
				if win && tt.canGrant {
					want = terraMisticaRewardFlag
				}
				if killer.TerraMistica != want {
					t.Fatalf("seed %d: TerraMistica = %d, want %d", seed, killer.TerraMistica, want)
				}
				if win {
					sawWin = true
				} else {
					sawLoss = true
				}
			}
			if tt.wantRoll && (!sawWin || !sawLoss) {
				t.Errorf("seeds 1..200 did not cover both roll outcomes (win=%v loss=%v)", sawWin, sawLoss)
			}
		})
	}
}

// TestMobKilledTerraMisticaStep2 is issue #402 end-to-end: hunting face-239/241
// mobs with TerraMistica == 1 eventually reaches 2 (0.95^200 odds of missing
// in the legacy), while any other face leaves the flag at 1.
func TestMobKilledTerraMisticaStep2(t *testing.T) {
	tests := []struct {
		name string
		face uint16
		want uint8
	}{
		{"face 239", 239, terraMisticaRewardFlag},
		{"face 241", 241, terraMisticaRewardFlag},
		{"other face", 240, terraMisticaMissionFlag},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, w, killer := mobKilledWorld(t)
			killer.TerraMistica = terraMisticaMissionFlag
			for i := 0; i < 200 && killer.TerraMistica == terraMisticaMissionFlag; i++ {
				d.mobKilled(w, killer, w.Entity(w.SpawnMob(faceMobTemplate(tt.face), 6, 5)))
			}
			if killer.TerraMistica != tt.want {
				t.Errorf("TerraMistica = %d, want %d", killer.TerraMistica, tt.want)
			}
		})
	}
}
