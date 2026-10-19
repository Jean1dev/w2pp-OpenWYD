package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/rng"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestCircleRouting pins the issue #405 routes on the shipped templates: the
// MOB.Merchant byte (17) picks the Jeffi (12) and the Shaman (13). The
// Curandeiro has CurrentScore.Merchant 13 but byte 17 = 200 and must not
// reach the Shaman.
func TestCircleRouting(t *testing.T) {
	tests := []struct {
		tmpl      string
		wantJeffi bool
		wantShama bool
	}{
		{"Jeffi", true, false},
		{"Alchemy_Jeffi", true, false},
		{"Torre_Real", true, false},
		{"Shaman", false, true},
		{"Curandeiro", false, false},
		{"Cap.Explor", false, false},
		{"Sobrevivente", false, false},
	}
	_, _, w, _ := startKefraServer(t, perzenDB(0))
	for _, tt := range tests {
		t.Run(tt.tmpl, func(t *testing.T) {
			runInLoop(t, w, func() {
				id := w.SpawnMob(npcTemplate(t, tt.tmpl), 20, 20)
				if id < world.MaxUser {
					t.Fatal("spawn failed")
				}
				npc := w.Entity(id)
				if got := isJeffiNPC(npc); got != tt.wantJeffi {
					t.Errorf("isJeffiNPC = %v, want %v", got, tt.wantJeffi)
				}
				if got := isShamaNPC(npc); got != tt.wantShama {
					t.Errorf("isShamaNPC = %v, want %v", got, tt.wantShama)
				}
			})
		})
	}
}

func dust(index int16, amount uint8) world.Item {
	if amount == 0 {
		return world.Item{Index: index}
	}
	return world.Item{Index: index, Effects: [3]world.Effect{{Effect: efAmount, Value: amount}}}
}

// TestJeffi drives the Jeffi with rand() seeded at 1 (MSVC LCG: 41, 18467,
// 6334, 26500, 19169, 15724, ...), following _MSG_Quest.cpp:514-610 and
// Combine (Server.cpp:9448).
func TestJeffi(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, perzenDB(0))
	c := enterWorld(t, addr)
	defer c.Close()
	var npc int
	runInLoop(t, w, func() { npc = w.SpawnMob(npcTemplate(t, "Jeffi"), 6, 6) })
	ori := world.Item{Index: itemOri, Effects: [3]world.Effect{{Effect: 59, Value: 41}, {Effect: 59, Value: 67}, {Effect: 59, Value: 84}}}
	lac2 := world.Item{Index: itemLac, Effects: [3]world.Effect{{Effect: 59, Value: 0}, {Effect: 59, Value: 69}, {Effect: 59, Value: 74}}}
	tests := []struct {
		name      string
		circle    int16
		coin      int32
		carry     []world.Item
		wantCirc  int16
		wantCoin  int32
		wantCarry []world.Item
	}{
		{name: "common circle purified", circle: 447, coin: 1005000, wantCirc: 450, wantCoin: 5000},
		{name: "common circle without gold", circle: 447, coin: 999999, wantCirc: 447, wantCoin: 999999},
		{name: "complete circle purified", circle: 692, coin: 5000000, wantCirc: 695, wantCoin: 0},
		{name: "complete circle without gold", circle: 692, coin: 4999999, wantCirc: 692, wantCoin: 4999999},
		{name: "ten loose ori dust", coin: 1000000,
			carry:     []world.Item{dust(419, 0), dust(419, 0), dust(419, 0), dust(419, 0), dust(419, 0), dust(419, 0), dust(419, 0), dust(419, 0), dust(419, 0), dust(419, 0)},
			wantCarry: []world.Item{ori}},
		{name: "stack of 15 ori loses 10", coin: 1000000,
			carry:     []world.Item{dust(419, 15)},
			wantCarry: []world.Item{dust(419, 5), ori}},
		{name: "stacks of 5 and 8 are both eaten", coin: 1000000,
			carry:     []world.Item{dust(419, 5), dust(419, 8)},
			wantCarry: []world.Item{ori}},
		{name: "ori and lac in one visit, one charge", coin: 3000000,
			carry:     []world.Item{dust(419, 10), dust(420, 10)},
			wantCoin:  2000000,
			wantCarry: []world.Item{ori, lac2}},
		{name: "dust without gold", coin: 999999,
			carry:     []world.Item{dust(419, 10)},
			wantCoin:  999999,
			wantCarry: []world.Item{dust(419, 10)}},
		{name: "nine dust is not enough", coin: 1000000,
			carry:     []world.Item{dust(419, 9), dust(420, 9)},
			wantCoin:  1000000,
			wantCarry: []world.Item{dust(419, 9), dust(420, 9)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runInLoop(t, w, func() {
				s, e := w.SessionByName("Hero")
				e.Equip[circleEquipSlot] = world.Item{Index: tt.circle}
				e.Coin = tt.coin
				e.Carry = [world.MaxCarry]world.Item{}
				copy(e.Carry[:], tt.carry)
				*w.Rand() = *rng.NewSeeded(1)
				says := w.SentOfType(s, protocol.MsgMessageChat)

				d.quest(w, s, protocol.Header{}, protocol.EncodeStandardParm2(int32(npc), 0))

				if got := e.Equip[circleEquipSlot].Index; got != tt.wantCirc {
					t.Errorf("Equip[13] = %d, want %d", got, tt.wantCirc)
				}
				if e.Coin != tt.wantCoin {
					t.Errorf("Coin = %d, want %d", e.Coin, tt.wantCoin)
				}
				var want [world.MaxCarry]world.Item
				copy(want[:], tt.wantCarry)
				if e.Carry != want {
					t.Errorf("Carry[:3] = %+v, want %+v", e.Carry[:3], want[:3])
				}
				if got := w.SentOfType(s, protocol.MsgMessageChat) - says; got == 0 {
					t.Error("the NPC said nothing")
				}
			})
		})
	}
}

// TestShama follows _MSG_Quest.cpp:612-691 with the issue #405 CD-04 state:
// Special [60,60,60,60], all three trees learned.
func TestShama(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, perzenDB(0))
	c := enterWorld(t, addr)
	defer c.Close()
	var npc int
	runInLoop(t, w, func() { npc = w.SpawnMob(npcTemplate(t, "Shaman"), 6, 6) })
	tests := []struct {
		name        string
		circle      int16
		done        uint8
		special     [4]int16
		wantSpecial [4]int16
		wantBonus   uint16
		wantLearned int32
		wantDone    uint8
		wantCirc    int16
	}{
		{name: "common circle, first tree", circle: 448, special: [4]int16{60, 60, 60, 60},
			wantSpecial: [4]int16{10, 10, 10, 10}, wantBonus: 200, wantLearned: 0xffff00, wantDone: 1},
		{name: "common circle, third tree, low masteries", circle: 450, special: [4]int16{30, 0, 70, 50},
			wantSpecial: [4]int16{0, 0, 20, 0}, wantBonus: 130, wantLearned: 0x00ffff, wantDone: 1},
		{name: "common circle already used", circle: 449, done: 1, special: [4]int16{60, 60, 60, 60},
			wantSpecial: [4]int16{60, 60, 60, 60}, wantLearned: 0xffffff, wantDone: 1, wantCirc: 449},
		{name: "complete circle ignores the flag", circle: 694, done: 1, special: [4]int16{120, 60, 0, 100},
			wantSpecial: [4]int16{20, 0, 0, 0}, wantBonus: 260, wantLearned: 0xff00ff, wantDone: 1},
		{name: "complete circle does not set the flag", circle: 693, special: [4]int16{0, 0, 0, 0},
			wantLearned: 0xffff00},
		{name: "raw circle is refused", circle: 447, special: [4]int16{60, 60, 60, 60},
			wantSpecial: [4]int16{60, 60, 60, 60}, wantLearned: 0xffffff, wantCirc: 447},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runInLoop(t, w, func() {
				s, e := w.SessionByName("Hero")
				e.Equip[circleEquipSlot] = world.Item{Index: tt.circle}
				e.CelCircle = tt.done
				e.BaseSpecial = tt.special
				e.SpecialBonus = 0
				e.LearnedSkill = 0xffffff
				says := w.SentOfType(s, protocol.MsgMessageChat)

				d.quest(w, s, protocol.Header{}, protocol.EncodeStandardParm2(int32(npc), 0))

				if e.BaseSpecial != tt.wantSpecial {
					t.Errorf("BaseSpecial = %v, want %v", e.BaseSpecial, tt.wantSpecial)
				}
				if e.SpecialBonus != tt.wantBonus {
					t.Errorf("SpecialBonus = %d, want %d", e.SpecialBonus, tt.wantBonus)
				}
				if e.LearnedSkill != tt.wantLearned {
					t.Errorf("LearnedSkill = %#x, want %#x", e.LearnedSkill, tt.wantLearned)
				}
				if e.CelCircle != tt.wantDone {
					t.Errorf("Circle = %d, want %d", e.CelCircle, tt.wantDone)
				}
				if got := e.Equip[circleEquipSlot].Index; got != tt.wantCirc {
					t.Errorf("Equip[13] = %d, want %d", got, tt.wantCirc)
				}
				if got := w.SentOfType(s, protocol.MsgMessageChat) - says; got == 0 {
					t.Error("the NPC said nothing")
				}
			})
		})
	}
}
