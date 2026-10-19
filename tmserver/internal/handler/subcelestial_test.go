package handler

import (
	"os"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestCreateSubCelestial drives the Pedra Ideal (5338, EF_VOLATILE 211) on a
// Celestial through useItem (issue #410, _MSG_UseItem.cpp:3026-3085): with a
// Sephirot in Equip[11] and CurrentScore.Level >= 120 the character becomes
// CELESTIALCS and SaveCelestial[1] gets a level-0 sheet of the Sephirot's class
// (attributes from the current class, HP/MP from the Sephirot's), the Sephirot
// and one stone are spent, and the active Celestial sheet is untouched.
// Otherwise nothing changes.
func TestCreateSubCelestial(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, perzenDB(0))
	c := enterWorld(t, addr)
	defer c.Close()

	// A sheet left over in the slot: the legacy only assigns some fields, so the
	// others (SPX/SPY, ScoreBonus, SkillBonus, the bars) must survive.
	leftover := world.CelestialSheet{SPX: 2100, SPY: 2100, ScoreBonus: 7, SkillBonus: 3, SkillBar1: [4]uint8{9}, Special: [4]int16{1, 2, 3, 4}, Exp: 99, Level: 50}
	parkedCelestial := world.CelestialSheet{Class: 2, Level: 33}
	tests := []struct {
		name     string
		level    int32
		sephirot int16
		amount   uint8 // EF_AMOUNT on the stone; 0 = a single unit
		wantSub  bool
	}{
		{name: "level 120 with the HT sephirot", level: 120, sephirot: 1763, wantSub: true},
		{name: "level 199 with the TK sephirot", level: 199, sephirot: 1760, wantSub: true},
		{name: "a stack loses one unit", level: 150, sephirot: 1761, amount: 3, wantSub: true},
		{name: "level 119 is refused", level: 119, sephirot: 1763},
		{name: "no sephirot is refused", level: 150, sephirot: 0},
		{name: "item 1764 is not a sephirot", level: 150, sephirot: 1764},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runInLoop(t, w, func() {
				d.itemVolatiles = map[int]int{pedraIdealItem: volIdealStone}
				s, e := w.SessionByName("Hero")
				e.ClassMaster, e.Level, e.Class, e.Soul = classMasterCelestial, tt.level, 0, 4
				e.BaseStr, e.BaseInt, e.BaseDex, e.BaseCon = 300, 40, 70, 60
				e.LearnedSkill = 1<<30 | 0x00ABCDEF
				e.SaveCelestial = [2]world.CelestialSheet{parkedCelestial, leftover}
				e.Equip[sephirotEquipSlot] = world.Item{Index: tt.sephirot}
				e.Carry = [world.MaxCarry]world.Item{}
				e.Carry[0] = world.Item{Index: pedraIdealItem}
				if tt.amount != 0 {
					e.Carry[0].Effects[0] = world.Effect{Effect: efAmount, Value: tt.amount}
				}
				panels := w.SentOfType(s, protocol.MsgMessagePanel)

				d.useItem(w, s, protocol.Header{}, (&protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}).Encode())

				save := w.CharacterSaveFor(s, e)
				if !tt.wantSub {
					if e.ClassMaster != classMasterCelestial || save.SaveCelestial != [2]world.CelestialSheet{parkedCelestial, leftover} {
						t.Errorf("refused use changed the character: master %d sheets %+v", e.ClassMaster, save.SaveCelestial)
					}
					if e.Equip[sephirotEquipSlot].Index != tt.sephirot || e.Carry[0].Index != pedraIdealItem {
						t.Errorf("refused use spent items: Equip[11] %d Carry[0] %d", e.Equip[sephirotEquipSlot].Index, e.Carry[0].Index)
					}
					return
				}
				if save.ClassMaster != classMasterCelestialCS {
					t.Errorf("saved ClassMaster = %d, want %d", save.ClassMaster, classMasterCelestialCS)
				}
				cls := int32(tt.sephirot - archSephirotMin)
				hpmp := [4][2]int32{{80, 45}, {60, 65}, {70, 55}, {75, 60}}[cls]
				want := leftover
				want.Class, want.Level, want.Exp, want.Soul = cls, 0, 0, 4
				want.Str, want.Int, want.Dex, want.Con = 8, 4, 7, 6 // TK: the current class, not the Sephirot's
				want.Special = [4]int16{}
				want.AC, want.Damage = 230, 0
				want.HP, want.MaxHP, want.MP, want.MaxMP = hpmp[0], hpmp[0], hpmp[1], hpmp[1]
				want.SpecialBonus = 855
				want.LearnedSkill = 1 << 30
				if save.SaveCelestial[1] != want {
					t.Errorf("SaveCelestial[1] = %+v\nwant %+v", save.SaveCelestial[1], want)
				}
				if save.SaveCelestial[0] != parkedCelestial {
					t.Errorf("SaveCelestial[0] changed: %+v", save.SaveCelestial[0])
				}
				if e.Level != tt.level || e.BaseStr != 300 || e.LearnedSkill != 1<<30|0x00ABCDEF {
					t.Errorf("active sheet changed: level %d str %d learned %#x", e.Level, e.BaseStr, e.LearnedSkill)
				}
				if !e.Equip[sephirotEquipSlot].Empty() {
					t.Errorf("Sephirot kept: %d", e.Equip[sephirotEquipSlot].Index)
				}
				wantCarry := int16(0)
				if tt.amount > 1 {
					wantCarry = pedraIdealItem
				}
				if e.Carry[0].Index != wantCarry {
					t.Errorf("Carry[0] = %d, want %d", e.Carry[0].Index, wantCarry)
				}
				if got := w.SentOfType(s, protocol.MsgMessagePanel) - panels; got != 1 {
					t.Errorf("message panels = %d, want 1 (_NN_My_King_Bless1)", got)
				}
			})
		})
	}
}

func urnammuTemplate(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../../../Release/TMsrv/run/npc/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestIsUrnammuNPC(t *testing.T) {
	for _, name := range []string{"Urnammu", "Urnamu"} {
		if !isUrnammuNPC(&world.Entity{Template: urnammuTemplate(t, name)}) {
			t.Errorf("%s template not routed to the Urnammu", name)
		}
	}
	if isUrnammuNPC(&world.Entity{Template: uxmalTemplate(t)}) {
		t.Error("Uxmal routed to the Urnammu")
	}
}

// TestUrnammu drives MSG_Quest on the Urnammu (issue #410, _MSG_Quest.cpp:1248-1311):
// the three refusals change nothing, and a CELESTIALCS with a Sephirot and 2000
// Fame pays the Fame and the Sephirot to rebuild the parked Sub sheet.
func TestUrnammu(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, perzenDB(0))
	c := enterWorld(t, addr)
	defer c.Close()
	var npcID int
	runInLoop(t, w, func() {
		npcID = w.SpawnMob(urnammuTemplate(t, "Urnammu"), 6, 6)
		if npcID < world.MaxUser {
			t.Fatal("spawn Urnammu failed")
		}
	})
	sub := world.CelestialSheet{Class: 3, Level: 150, Exp: 12345, Str: 400, Int: 40, Dex: 300, Con: 90,
		Special: [4]int16{10, 55, 56, 300}, SpecialBonus: 100, LearnedSkill: 1<<30 | 0x0000ABCD,
		SkillBar1: [4]uint8{1, 2, 3, 4}, SkillBar2: [16]uint8{5, 6}, ScoreBonus: 11}
	tests := []struct {
		name        string
		classMaster uint8
		sephirot    int16
		fame        int32
		wantChange  bool
	}{
		{name: "plain celestial is refused", classMaster: classMasterCelestial, sephirot: 1760, fame: 2500},
		{name: "sub-celestial playing is refused", classMaster: classMasterSCelestial, sephirot: 1760, fame: 2500},
		{name: "no sephirot is refused", classMaster: classMasterCelestialCS, fame: 2500},
		{name: "1999 fame is refused", classMaster: classMasterCelestialCS, sephirot: 1760, fame: 1999},
		{name: "2000 fame changes the class", classMaster: classMasterCelestialCS, sephirot: 1760, fame: 2000, wantChange: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runInLoop(t, w, func() {
				s, e := w.SessionByName("Hero")
				e.ClassMaster, e.Class, e.Fame = tt.classMaster, 1, tt.fame
				e.SaveCelestial = [2]world.CelestialSheet{{Class: 1, Level: 199}, sub}
				e.Equip[sephirotEquipSlot] = world.Item{Index: tt.sephirot}
				panels := w.SentOfType(s, protocol.MsgMessagePanel)

				d.quest(w, s, protocol.Header{}, protocol.EncodeStandardParm2(int32(npcID), 0))

				if got := w.SentOfType(s, protocol.MsgMessagePanel) - panels; got != 1 {
					t.Errorf("message panels = %d, want 1", got)
				}
				save := w.CharacterSaveFor(s, e)
				if !tt.wantChange {
					if save.SaveCelestial[1] != sub || e.Fame != tt.fame || e.Equip[sephirotEquipSlot].Index != tt.sephirot {
						t.Errorf("refusal changed the character: fame %d Equip[11] %d sub %+v", e.Fame, e.Equip[sephirotEquipSlot].Index, save.SaveCelestial[1])
					}
					return
				}
				want := sub
				want.Class = 0                                           // the Sephirot 1760 (TK)
				want.Str, want.Int, want.Dex, want.Con = 5, 8, 5, 5      // the active class (FM)
				want.Special = [4]int16{0, 0, 1, 245}                    // at most 55 back per Special
				want.SpecialBonus = 100 + 10 + 55 + 55 + 55              // the refunded points
				want.LearnedSkill = 1 << 30                              // skills dropped
				want.SkillBar1, want.SkillBar2 = [4]uint8{}, [16]uint8{} // bars cleared
				if save.SaveCelestial[1] != want {
					t.Errorf("SaveCelestial[1] = %+v\nwant %+v", save.SaveCelestial[1], want)
				}
				if save.SaveCelestial[0] != (world.CelestialSheet{Class: 1, Level: 199}) {
					t.Errorf("SaveCelestial[0] changed: %+v", save.SaveCelestial[0])
				}
				if save.Fame != 0 {
					t.Errorf("saved Fame = %d, want 0", save.Fame)
				}
				if !e.Equip[sephirotEquipSlot].Empty() {
					t.Errorf("Sephirot kept: %d", e.Equip[sephirotEquipSlot].Index)
				}
				if e.ClassMaster != classMasterCelestialCS || e.Class != 1 {
					t.Errorf("active character changed: master %d class %d", e.ClassMaster, e.Class)
				}
			})
		})
	}
}

// TestScoreBonusParkedTier pins that CELESTIALCS counts SubCelestialLevel and
// SCELESTIAL counts CelestialLevel (Basedef.cpp:1084-1101, :1149-1166), while a
// plain CELESTIAL ignores both (issue #410).
func TestScoreBonusParkedTier(t *testing.T) {
	tests := []struct {
		name        string
		classMaster uint8
		want        uint16
	}{
		{"celestial ignores both", classMasterCelestial, 1001},
		{"celestialcs counts the sub", classMasterCelestialCS, 1001 + 30},
		{"scelestial counts the celestial", classMasterSCelestial, 1001 + 360 + 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &world.Entity{ClassMaster: tt.classMaster, BaseStr: 8, BaseInt: 4, BaseDex: 7, BaseCon: 6,
				SubCelestialLevel: 10, CelestialLevel: 120}
			if got := scoreBonusFor(e); got != tt.want {
				t.Errorf("scoreBonusFor = %d, want %d", got, tt.want)
			}
		})
	}
}
