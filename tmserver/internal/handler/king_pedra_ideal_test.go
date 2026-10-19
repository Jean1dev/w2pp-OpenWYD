package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

type kingFrame struct {
	place, slot, index uint16
	chat               string
}

// TestKingPedraIdeal covers issue #406: with Equip[10]=1742, a Sephirot in
// Equip[11] and the four Pedras Secretas carried, the King turns the first 5334
// into a 5338 for any ClassMaster, erases the first 5335/5336/5337, consumes the
// King gear and never creates the Arch (_MSG_Quest.cpp:769-840). Without the
// four stones the Mortal still becomes Arch (:842-866).
func TestKingPedraIdeal(t *testing.T) {
	tests := []struct {
		name        string
		classMaster uint8
		level       int
		stones      []int16 // carried in slots 20.., then a spare 5334 at 30
		confirm     int32
		wantStone   bool
		wantArch    bool
	}{
		{"CE-01 Arch with the four stones", classMasterArch, 356, []int16{5334, 5335, 5336, 5337}, 1, true, false},
		{"CE-03 Mortal 300 with the four stones", classMasterMortal, 300, []int16{5334, 5335, 5336, 5337}, 1, true, false},
		{"Celestial with stones out of order", classMasterCelestial, 10, []int16{5337, 5336, 5335, 5334}, 1, true, false},
		{"CE-02 Mortal without the 5337", classMasterMortal, 300, []int16{5334, 5335, 5336}, 1, false, true},
		{"confirm 0 only quotes", classMasterMortal, 300, []int16{5334, 5335, 5336, 5337}, 0, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newDB()
			db.archOK = true
			db.archSlot = 1
			st := baseMortalState(tt.level)
			st.ClassMaster = tt.classMaster
			st.Equip[0] = world.Item{Index: 21}
			st.Equip[immortalityStoneEquipSlot] = world.Item{Index: immortalityStoneItem}
			st.Equip[sephirotEquipSlot] = world.Item{Index: 1760}
			for i, stone := range tt.stones {
				st.Carry[20+i] = world.Item{Index: stone}
			}
			// A second 5334 after the first: the legacy converts only the first.
			st.Carry[30] = world.Item{Index: 5334}
			db.loadResult = st

			tmpl := kingTemplate()
			tmpl[16] = clanAkelonia // Rei_Glantuar; the Clan-0 player passes the gate
			addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()
			drainRaw(t, c)

			send(t, c, protocol.MsgQuest, protocol.EncodeStandardParm2(int32(npcID), tt.confirm))
			var frames []kingFrame
			for {
				ty, body, ok := readMaybe(t, c)
				if !ok {
					break
				}
				switch ty {
				case protocol.MsgSendItem:
					frames = append(frames, kingFrame{place: le16(body[0:2]), slot: le16(body[2:4]), index: le16(body[4:6])})
				case protocol.MsgMessageChat:
					frames = append(frames, kingFrame{chat: string(body)})
				}
			}

			created, _, _, _, _, _, _ := db.archRequest()
			if tt.wantArch != (created == 1) {
				t.Fatalf("arch requests = %d, want arch=%v", created, tt.wantArch)
			}
			if !tt.wantArch {
				send(t, c, protocol.MsgCharacterLogout, nil)
				expect(t, c, protocol.MsgCNFCharacterLogout)
			}
			save, n := db.lastSavedChar()
			if n == 0 {
				t.Fatal("character was never saved")
			}
			carry := func(slot int) int16 {
				it, _ := savedSlot(save.Carry, slot)
				return it.Index
			}
			equip := func(slot int) int16 {
				it, _ := savedSlot(save.Equip, slot)
				return it.Index
			}

			if !tt.wantStone {
				for i, stone := range tt.stones {
					if got := carry(20 + i); got != stone {
						t.Errorf("carry[%d] = %d, want %d kept", 20+i, got, stone)
					}
				}
				if !tt.wantArch && (equip(immortalityStoneEquipSlot) != immortalityStoneItem || equip(sephirotEquipSlot) != 1760) {
					t.Errorf("King gear consumed: equip10=%d equip11=%d", equip(immortalityStoneEquipSlot), equip(sephirotEquipSlot))
				}
				return
			}

			want := make([]kingFrame, 0, 7)
			for i, stone := range tt.stones {
				f := kingFrame{place: world.ItemPlaceCarry, slot: uint16(20 + i)}
				if stone == 5334 {
					f.index = pedraIdealItem
				}
				want = append(want, f)
			}
			want = append(want,
				kingFrame{place: world.ItemPlaceEquip, slot: immortalityStoneEquipSlot},
				kingFrame{place: world.ItemPlaceEquip, slot: sephirotEquipSlot},
				kingFrame{chat: string(protocol.EncodeMessageChatBody(msgKingBless1))},
			)
			if len(frames) != len(want) {
				t.Fatalf("frames = %+v, want %+v", frames, want)
			}
			for i := range want {
				if frames[i] != want[i] {
					t.Fatalf("frame %d = %+v, want %+v (all %+v)", i, frames[i], want[i], frames)
				}
			}
			for i, stone := range tt.stones {
				wantIdx := int16(0)
				if stone == 5334 {
					wantIdx = pedraIdealItem
				}
				if got := carry(20 + i); got != wantIdx {
					t.Errorf("saved carry[%d] = %d, want %d", 20+i, got, wantIdx)
				}
			}
			if got := carry(30); got != 5334 {
				t.Errorf("second 5334 = %d, want kept", got)
			}
			if equip(immortalityStoneEquipSlot) != 0 || equip(sephirotEquipSlot) != 0 {
				t.Errorf("King gear kept: equip10=%d equip11=%d", equip(immortalityStoneEquipSlot), equip(sephirotEquipSlot))
			}
		})
	}
}
