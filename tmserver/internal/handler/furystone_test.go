package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/rng"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Seeds for the single rand()%115 of a Pedra da Fúria quest (MSVC LCG):
// seed 1 rolls 41, seed 20 rolls 103 (folded to 88, still a success) and
// seed 19 rolls exactly 100, the only failing outcome.
const (
	furySeedSuccess = 1
	furySeedFolded  = 20
	furySeedFail    = 19
)

// TestUseFuryStone drives _MSG_UseItem on the Pedra da Fúria (3020), which has
// no EF_VOLATILE, and checks the three branches of _MSG_UseItem.cpp:3470-3690
// in the legacy order: Arcana and Reset for a level-199 celestial tier, then
// Lv90 for a CELESTIAL only (issue #408).
func TestUseFuryStone(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, perzenDB(furyStoneItem))
	c := enterWorld(t, addr)
	defer c.Close()

	// Equip[1] is a worn armour with a refine, to show that the Arcana only
	// rewrites its sIndex.
	armour := world.Item{Index: 1101, Effects: [3]world.Effect{{Effect: efSanc, Value: 3}}}
	resetExp := level.NextLevelExpTier(furyStoneArcanaLevel-1, classMasterCelestial) + 5_000_000

	tests := []struct {
		name        string
		classMaster uint8
		level       int32
		fame        int32
		lv90        uint8
		circle      uint8
		reset       uint8
		stones      int // how many of the four Pedras Secretas sit in Carry[1..4]
		seed        uint32
		wantRolls   int // rand() calls the use must consume
		wantStone   bool
		wantFame    int32
		wantLv90    uint8
		wantCircle  uint8
		wantReset   uint8
		wantMistica bool // the 3502 reward is in the inventory
		wantArcana  bool // Equip[1] became the 3507
		wantStones  int  // Pedras Secretas left
	}{
		// Lv90 (_MSG_UseItem.cpp:3617-3690).
		{name: "lv90 success", classMaster: classMasterCelestial, level: 89, fame: 600, seed: furySeedSuccess,
			wantRolls: 1, wantFame: 100, wantLv90: 1, wantMistica: true},
		{name: "lv90 folded roll still succeeds", classMaster: classMasterCelestial, level: 89, fame: 600, seed: furySeedFolded,
			wantRolls: 1, wantFame: 100, wantLv90: 1, wantMistica: true},
		{name: "lv90 failure still spends stone and fame", classMaster: classMasterCelestial, level: 89, fame: 600, seed: furySeedFail,
			wantRolls: 1, wantFame: 100},
		{name: "lv90 without fame is refused", classMaster: classMasterCelestial, level: 89, fame: 499,
			wantStone: true, wantFame: 499},
		{name: "lv90 below stored 89 is refused", classMaster: classMasterCelestial, level: 88, fame: 600,
			wantStone: true, wantFame: 600},
		{name: "lv90 already done is refused", classMaster: classMasterCelestial, level: 89, fame: 600, lv90: 1,
			wantStone: true, wantFame: 600, wantLv90: 1},
		{name: "celestialcs below 199 is refused", classMaster: classMasterCelestialCS, level: 120, fame: 600,
			wantStone: true, wantFame: 600},
		{name: "mortal is refused", classMaster: classMasterMortal, level: 199, fame: 600, stones: 4,
			wantStone: true, wantFame: 600, wantStones: 4},

		// Arcana (_MSG_UseItem.cpp:3480-3568).
		{name: "arcana success", classMaster: classMasterCelestialCS, level: 199, fame: 600, lv90: 1, stones: 4, seed: furySeedSuccess,
			wantRolls: 1, wantFame: 100, wantLv90: 1, wantCircle: 1, wantArcana: true},
		{name: "arcana failure still takes the stones", classMaster: classMasterSCelestial, level: 199, fame: 600, stones: 4, seed: furySeedFail,
			wantRolls: 1, wantFame: 100},
		{name: "arcana with three stones is refused", classMaster: classMasterCelestial, level: 199, fame: 600, lv90: 1, stones: 3,
			wantStone: true, wantFame: 600, wantLv90: 1, wantStones: 3},
		{name: "level 199 without fame is refused", classMaster: classMasterCelestial, level: 199, fame: 499, lv90: 1, stones: 4,
			wantStone: true, wantFame: 499, wantLv90: 1, wantStones: 4},

		// Reset (_MSG_UseItem.cpp:3579-3615): no roll.
		{name: "reset", classMaster: classMasterCelestial, level: 199, fame: 600, lv90: 1, circle: 1, reset: 2, stones: 4,
			wantFame: 100, wantLv90: 1, wantCircle: 1, wantReset: 3, wantStones: 4},
		{name: "reset maxed falls to lv90 done", classMaster: classMasterCelestial, level: 199, fame: 600, lv90: 1, circle: 1, reset: 3,
			wantStone: true, wantFame: 600, wantLv90: 1, wantCircle: 1, wantReset: 3},
		{name: "reset maxed falls to lv90 unlock", classMaster: classMasterCelestial, level: 199, fame: 600, circle: 1, reset: 3, seed: furySeedSuccess,
			wantRolls: 1, wantFame: 100, wantLv90: 1, wantCircle: 1, wantReset: 3, wantMistica: true},
		{name: "reset maxed subcelestial is refused", classMaster: classMasterSCelestial, level: 199, fame: 600, circle: 1, reset: 3,
			wantStone: true, wantFame: 600, wantCircle: 1, wantReset: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runInLoop(t, w, func() {
				s, e := w.SessionByName("Hero")
				*w.Rand() = *rng.NewSeeded(tt.seed)
				probe := *rng.NewSeeded(tt.seed)
				for range tt.wantRolls {
					probe.Rand()
				}
				e.ClassMaster, e.Level, e.Fame = tt.classMaster, tt.level, tt.fame
				e.CelLv90, e.CelCircle, e.CelReset = tt.lv90, tt.circle, tt.reset
				e.Exp = resetExp
				e.Equip[arcanaEquipSlot] = armour
				e.Carry = [world.MaxCarry]world.Item{}
				e.Carry[0] = world.Item{Index: furyStoneItem}
				for k := 0; k < tt.stones; k++ {
					e.Carry[1+k] = world.Item{Index: pedraSecretaItems[k]}
				}

				body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
				d.useItem(w, s, protocol.Header{}, body.Encode())

				if *w.Rand() != probe {
					t.Errorf("rand() calls differ from the %d the legacy makes", tt.wantRolls)
				}
				if got := e.Carry[0].Index == furyStoneItem; got != tt.wantStone {
					t.Errorf("stone still in Carry[0] = %v, want %v", got, tt.wantStone)
				}
				if e.Fame != tt.wantFame {
					t.Errorf("Fame = %d, want %d", e.Fame, tt.wantFame)
				}
				save := w.CharacterSaveFor(s, e)
				if save.CelLv90 != tt.wantLv90 || save.CelCircle != tt.wantCircle || save.CelReset != tt.wantReset {
					t.Errorf("saved Lv90/Circle/Reset = %d/%d/%d, want %d/%d/%d",
						save.CelLv90, save.CelCircle, save.CelReset, tt.wantLv90, tt.wantCircle, tt.wantReset)
				}
				if got := hasCarry(e, cytheraMisticaItem); got != tt.wantMistica {
					t.Errorf("Cythera Mistica in carry = %v, want %v", got, tt.wantMistica)
				}
				wantEquip := armour
				if tt.wantArcana {
					wantEquip.Index = arcanaItemIndex
				}
				if e.Equip[arcanaEquipSlot] != wantEquip {
					t.Errorf("Equip[1] = %+v, want %+v", e.Equip[arcanaEquipSlot], wantEquip)
				}
				left := 0
				for _, stone := range pedraSecretaItems {
					if hasCarry(e, stone) {
						left++
					}
				}
				if left != tt.wantStones {
					t.Errorf("Pedras Secretas left = %d, want %d", left, tt.wantStones)
				}
			})
		})
	}
}

// TestFuryStoneResetExp checks the reset's exp and level rollback and the
// +200 free points (_MSG_UseItem.cpp:3581-3603, Basedef.cpp:989), and that the
// next level-up keeps counting the reset.
func TestFuryStoneResetExp(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, perzenDB(furyStoneItem))
	c := enterWorld(t, addr)
	defer c.Close()

	runInLoop(t, w, func() {
		s, e := w.SessionByName("Hero")
		e.ClassMaster, e.Level, e.Fame = classMasterCelestial, furyStoneArcanaLevel, 600
		e.CelLv40, e.CelLv90, e.CelCircle, e.CelReset = 1, 1, 1, 0
		startExp := level.NextLevelExpTier(furyStoneArcanaLevel-1, classMasterCelestial) + 5_000_000
		e.Exp = startExp

		body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
		d.useItem(w, s, protocol.Header{}, body.Encode())

		wantExp := startExp - celestialResetExp
		wantLevel := level.ForExpTier(wantExp, classMasterCelestial)
		if e.Exp != wantExp || e.Level != wantLevel {
			t.Fatalf("Exp/Level = %d/%d, want %d/%d", e.Exp, e.Level, wantExp, wantLevel)
		}
		if wantLevel >= furyStoneArcanaLevel {
			t.Fatalf("test setup: the reset should cost a level, still at %d", wantLevel)
		}
		wantBonus := uint16(level.ScoreBonusCelestial(e.Class, wantLevel, e.BaseStr, e.BaseInt, e.BaseDex, e.BaseCon, e.ArchCrystalStage, 1, e.CelestialArchLevel))
		if e.ScoreBonus != wantBonus {
			t.Errorf("ScoreBonus = %d, want %d", e.ScoreBonus, wantBonus)
		}
		if e.BaseAC != playerBaseAC(e) {
			t.Errorf("BaseAC = %d, want the login derivation %d", e.BaseAC, playerBaseAC(e))
		}
		if save := w.CharacterSaveFor(s, e); save.Exp != wantExp || save.Level != wantLevel || save.CelReset != 1 {
			t.Errorf("saved Exp/Level/Reset = %d/%d/%d, want %d/%d/1", save.Exp, save.Level, save.CelReset, wantExp, wantLevel)
		}

		// Earning the level back keeps the reset's +200 in the free points.
		e.Exp = startExp
		d.applyLevelUps(w, nil, e)
		wantBonus = uint16(level.ScoreBonusCelestial(e.Class, furyStoneArcanaLevel, e.BaseStr, e.BaseInt, e.BaseDex, e.BaseCon, e.ArchCrystalStage, 1, e.CelestialArchLevel))
		if e.Level != furyStoneArcanaLevel || e.ScoreBonus != wantBonus {
			t.Errorf("after level-up Level/ScoreBonus = %d/%d, want %d/%d", e.Level, e.ScoreBonus, furyStoneArcanaLevel, wantBonus)
		}
	})
}

func hasCarry(e *world.Entity, index int16) bool {
	for _, it := range e.Carry {
		if it.Index == index {
			return true
		}
	}
	return false
}
