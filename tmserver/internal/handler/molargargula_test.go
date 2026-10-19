package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const itemMolarGargula = 4122

// sancPiece is an armour piece carrying an EF_SANC pair at the given level.
func sancPiece(index int16, level uint8) world.Item {
	return world.Item{Index: index, Effects: [3]world.Effect{{Effect: efSanc, Value: level}}}
}

// TestUseMolarGargula drives _MSG_UseItem on the Molar_de_Gárgula through the
// real EF_VOLATILE route (issue #398) and checks _MSG_UseItem.cpp:2494-2546:
// the Mortal 199..253 gate, the once-only QuestInfo.Mortal.MolarGargula flag,
// Equip[1..5] below +6 raised to +6 (empty slots, Equip[0]/[6] and pieces
// already at +6 or more untouched) and one unit consumed.
func TestUseMolarGargula(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, perzenDB(itemMolarGargula))
	c := enterWorld(t, addr)
	defer c.Close()

	gear := func() [world.MaxEquip]world.Item {
		var eq [world.MaxEquip]world.Item
		eq[0] = world.Item{Index: 11}
		eq[1] = sancPiece(1101, 0)
		eq[2] = sancPiece(1102, 3)
		eq[3] = sancPiece(1103, 5)
		eq[4] = sancPiece(1104, 7)
		eq[5] = sancPiece(1105, 1)
		eq[6] = sancPiece(1106, 2)
		return eq
	}
	tests := []struct {
		name        string
		classMaster uint8
		level       int32
		flag        uint8
		amount      uint8 // EF_AMOUNT on the molar; 0 = a single unit
		emptySlot   int   // Equip slot cleared before use; 0 = none
		wantSanc    [7]int
		wantFlag    uint8
		wantCarry   int16
	}{
		{name: "mortal 200 raises 1..5 to +6", classMaster: classMasterMortal, level: 199,
			wantSanc: [7]int{0, 6, 6, 6, 7, 6, 2}, wantFlag: 1},
		{name: "mortal 254 is the last level allowed", classMaster: classMasterMortal, level: 253,
			wantSanc: [7]int{0, 6, 6, 6, 7, 6, 2}, wantFlag: 1},
		{name: "empty Equip[3] is skipped", classMaster: classMasterMortal, level: 199, emptySlot: 3,
			wantSanc: [7]int{0, 6, 6, 0, 7, 6, 2}, wantFlag: 1},
		{name: "a stack loses one unit", classMaster: classMasterMortal, level: 199, amount: 2,
			wantSanc: [7]int{0, 6, 6, 6, 7, 6, 2}, wantFlag: 1, wantCarry: itemMolarGargula},
		{name: "level 198 is refused", classMaster: classMasterMortal, level: 198,
			wantSanc: [7]int{0, 0, 3, 5, 7, 1, 2}, wantCarry: itemMolarGargula},
		{name: "level 255 is refused", classMaster: classMasterMortal, level: 254,
			wantSanc: [7]int{0, 0, 3, 5, 7, 1, 2}, wantCarry: itemMolarGargula},
		{name: "arch is refused", classMaster: classMasterArch, level: 199,
			wantSanc: [7]int{0, 0, 3, 5, 7, 1, 2}, wantCarry: itemMolarGargula},
		{name: "second use is refused", classMaster: classMasterMortal, level: 199, flag: 1,
			wantSanc: [7]int{0, 0, 3, 5, 7, 1, 2}, wantFlag: 1, wantCarry: itemMolarGargula},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runInLoop(t, w, func() {
				d.itemVolatiles = map[int]int{itemMolarGargula: volMolarGargula}
				s, e := w.SessionByName("Hero")
				e.ClassMaster, e.Level, e.MolarGargula = tt.classMaster, tt.level, tt.flag
				e.Equip = gear()
				if tt.emptySlot != 0 {
					e.Equip[tt.emptySlot] = world.Item{}
				}
				e.Carry = [world.MaxCarry]world.Item{}
				e.Carry[0] = world.Item{Index: itemMolarGargula}
				if tt.amount != 0 {
					e.Carry[0].Effects[0] = world.Effect{Effect: efAmount, Value: tt.amount}
				}

				body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
				d.useItem(w, s, protocol.Header{}, body.Encode())

				for i, want := range tt.wantSanc {
					if got := refine.Level(e.Equip[i]); got != want {
						t.Errorf("Equip[%d] sanc = %d, want %d", i, got, want)
					}
				}
				if e.MolarGargula != tt.wantFlag {
					t.Errorf("MolarGargula = %d, want %d", e.MolarGargula, tt.wantFlag)
				}
				if got := w.CharacterSaveFor(s, e).MolarGargula; got != tt.wantFlag {
					t.Errorf("saved MolarGargula = %d, want %d", got, tt.wantFlag)
				}
				if e.Carry[0].Index != tt.wantCarry {
					t.Errorf("Carry[0] = %d, want %d", e.Carry[0].Index, tt.wantCarry)
				}
			})
		})
	}
}
