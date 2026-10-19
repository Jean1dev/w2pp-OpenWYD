package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestUsePilulaOrc drives _MSG_UseItem on the Pílula do Poder Mágico through
// the real EF_VOLATILE route (issue #404) and checks _MSG_UseItem.cpp:981-998:
// the first use sets QuestInfo.Mortal.PilulaOrc, adds 9 SkillBonus and consumes
// one unit; a second use changes nothing and leaves the item.
func TestUsePilulaOrc(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, perzenDB(itemPilulaOrc))
	c := enterWorld(t, addr)
	defer c.Close()

	tests := []struct {
		name      string
		flag      uint8
		amount    uint8 // EF_AMOUNT on the pill; 0 = a single unit
		wantBonus uint16
		wantFlag  uint8
		wantCarry int16
	}{
		{name: "first use adds 9", wantBonus: 147 + 9, wantFlag: 1},
		{name: "a stack loses one unit", amount: 2, wantBonus: 147 + 9, wantFlag: 1, wantCarry: itemPilulaOrc},
		{name: "second use is refused", flag: 1, wantBonus: 147, wantFlag: 1, wantCarry: itemPilulaOrc},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runInLoop(t, w, func() {
				d.itemVolatiles = map[int]int{itemPilulaOrc: volPilulaOrc}
				s, e := w.SessionByName("Hero")
				e.PilulaOrc, e.SkillBonus = tt.flag, 147
				e.Carry = [world.MaxCarry]world.Item{}
				e.Carry[0] = world.Item{Index: itemPilulaOrc}
				if tt.amount != 0 {
					e.Carry[0].Effects[0] = world.Effect{Effect: efAmount, Value: tt.amount}
				}

				body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
				d.useItem(w, s, protocol.Header{}, body.Encode())

				if e.SkillBonus != tt.wantBonus {
					t.Errorf("SkillBonus = %d, want %d", e.SkillBonus, tt.wantBonus)
				}
				if e.PilulaOrc != tt.wantFlag {
					t.Errorf("PilulaOrc = %d, want %d", e.PilulaOrc, tt.wantFlag)
				}
				if got := w.CharacterSaveFor(s, e).PilulaOrc; got != tt.wantFlag {
					t.Errorf("saved PilulaOrc = %d, want %d", got, tt.wantFlag)
				}
				if e.Carry[0].Index != tt.wantCarry {
					t.Errorf("Carry[0] = %d, want %d", e.Carry[0].Index, tt.wantCarry)
				}
			})
		})
	}
}

// TestPickUpPilulaOrc drives _MSG_GetItem on a Pílula do Poder Mágico lying on
// the floor (_MSG_GetItem.cpp:94-126, issue #404): without the flag the pill is
// consumed from the ground (+9 SkillBonus, flag set, never enters the
// inventory); with the flag it stays on the floor and nothing changes.
func TestPickUpPilulaOrc(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, perzenDB(0))
	c := enterWorld(t, addr)
	defer c.Close()

	tests := []struct {
		name       string
		flag       uint8
		wantBonus  uint16
		wantFlag   uint8
		wantGround bool
	}{
		{name: "first pickup consumes it from the floor", wantBonus: 147 + 9, wantFlag: 1},
		{name: "second pickup leaves it on the floor", flag: 1, wantBonus: 147, wantFlag: 1, wantGround: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runInLoop(t, w, func() {
				s, e := w.SessionByName("Hero")
				e.PilulaOrc, e.SkillBonus = tt.flag, 147
				e.Carry = [world.MaxCarry]world.Item{}
				id := w.CreateGroundItem(world.Item{Index: itemPilulaOrc}, e.X, e.Y)
				if id < 0 {
					t.Fatal("no ground item id")
				}
				defer w.RemoveGroundItem(id)

				body := protocol.MsgGetItemBody{ItemID: int32(world.GroundItemIDOffset + id), DestType: world.ItemPlaceCarry, DestPos: 0}
				d.getItem(w, s, protocol.Header{}, body.Encode())

				if e.SkillBonus != tt.wantBonus {
					t.Errorf("SkillBonus = %d, want %d", e.SkillBonus, tt.wantBonus)
				}
				if e.PilulaOrc != tt.wantFlag {
					t.Errorf("PilulaOrc = %d, want %d", e.PilulaOrc, tt.wantFlag)
				}
				if !e.Carry[0].Empty() {
					t.Errorf("Carry[0] = %d, want empty (the pill never enters the inventory)", e.Carry[0].Index)
				}
				if got := w.GroundItem(id) != nil; got != tt.wantGround {
					t.Errorf("pill on the floor = %v, want %v", got, tt.wantGround)
				}
			})
		})
	}
}
