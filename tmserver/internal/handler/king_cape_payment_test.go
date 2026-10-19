package handler

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// kingPurchaseDB commits every kingdom-cape purchase, so the handler's
// post-commit path (slot updates, final entity state) can be observed.
type kingPurchaseDB struct {
	*fakeDB
	quote     world.KingdomCapeQuote
	purchases atomic.Int32
}

func (db *kingPurchaseDB) QuoteKingdomCape(context.Context) (world.KingdomCapeQuote, error) {
	return db.quote, nil
}

func (db *kingPurchaseDB) PurchaseKingdomCape(context.Context, int64, uint8, world.CharacterSave) (world.KingdomCapeQuote, bool, error) {
	db.purchases.Add(1)
	return db.quote, true, nil
}

type kingCapeCase struct {
	name        string
	classMaster uint8
	level       int
	cape        int16
	emblem      bool
	carry       map[int]int16
	wantCape    int16
	wantEmblem  bool
	wantCarry   map[int]int16
	wantBuy     bool
	wantTexts   []string
}

// runKingCapeCase talks to Rei_Harabard (Hekalotia, price 4) with confirm 1
// and returns the save written at logout.
func runKingCapeCase(t *testing.T, tc kingCapeCase) (world.CharacterSave, []string, int32) {
	t.Helper()
	st := baseMortalState(tc.level)
	if tc.classMaster != 0 {
		st.ClassMaster = tc.classMaster
	}
	if tc.cape != 0 {
		st.Equip[capeEquipSlot] = world.Item{Index: tc.cape}
	}
	if tc.emblem {
		st.Equip[kingdomEmblemEquipSlot] = world.Item{Index: kingdomEmblemItem}
	}
	for slot, idx := range tc.carry {
		st.Carry[slot] = world.Item{Index: idx}
	}
	db := &kingPurchaseDB{
		fakeDB: newDB(),
		quote:  world.KingdomCapeQuote{Revision: 3, HekalotiaCost: 4, AkeloniaCost: 32},
	}
	db.loadResult = st
	tmpl := questNPCTemplate("Rei_Harabard", kingQuestMerchant, 0, 0)
	tmpl[16] = clanHekalotia
	addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
	defer stop()
	player := enterWorld(t, addr)
	defer player.Close()
	drainRaw(t, player)

	send(t, player, protocol.MsgQuest, protocol.EncodeStandardParm2(int32(npcID), 1))
	var texts []string
	for {
		h, body, ok := readMaybeHeader(t, player)
		if !ok {
			break
		}
		if h.Type == protocol.MsgMessageChat {
			n := 0
			for n < len(body) && body[n] != 0 {
				n++
			}
			texts = append(texts, string(body[:n]))
		}
	}
	send(t, player, protocol.MsgCharacterLogout, nil)
	expect(t, player, protocol.MsgCNFCharacterLogout)
	save, n := db.lastSavedChar()
	if n == 0 {
		t.Fatal("no save at logout")
	}
	return save, texts, db.purchases.Load()
}

func checkKingCapeCase(t *testing.T, tc kingCapeCase, save world.CharacterSave, texts []string, purchases int32) {
	t.Helper()
	if got := purchases != 0; got != tc.wantBuy {
		t.Errorf("purchases = %d, want bought=%v", purchases, tc.wantBuy)
	}
	if tc.wantBuy && save.Clan != clanHekalotia {
		t.Errorf("Clan = %d, want %d derived from the new cape", save.Clan, clanHekalotia)
	}
	cape, _ := savedSlot(save.Equip, capeEquipSlot)
	if cape.Index != tc.wantCape {
		t.Errorf("Equip[15] = %d, want %d", cape.Index, tc.wantCape)
	}
	emblem, _ := savedSlot(save.Equip, kingdomEmblemEquipSlot)
	if got := emblem.Index == kingdomEmblemItem; got != tc.wantEmblem {
		t.Errorf("Equip[13] = %d, want emblem kept=%v", emblem.Index, tc.wantEmblem)
	}
	if len(save.Carry) != len(tc.wantCarry) {
		t.Errorf("carry = %+v, want %v", save.Carry, tc.wantCarry)
	}
	for slot, idx := range tc.wantCarry {
		if it, ok := savedSlot(save.Carry, slot); !ok || it.Index != idx {
			t.Errorf("Carry[%d] = %+v (present=%v), want %d", slot, it, ok, idx)
		}
	}
	if tc.wantTexts != nil {
		if len(texts) != len(tc.wantTexts) {
			t.Fatalf("texts = %q, want %q", texts, tc.wantTexts)
		}
		for i := range texts {
			if texts[i] != tc.wantTexts[i] {
				t.Errorf("text[%d] = %q, want %q", i, texts[i], tc.wantTexts[i])
			}
		}
	}
}

// TestKingCapeEmblem covers issue #399: an equipped Emblema_do_Reino (4081)
// pays for the kingdom cape instead of sapphires and is consumed
// (_MSG_Quest.cpp:997,1016,1038-1080). The Celestial branch has no exemption.
func TestKingCapeEmblem(t *testing.T) {
	tests := []kingCapeCase{
		{
			name: "emblem without sapphires grants 545", level: 219, emblem: true,
			wantCape: 545, wantBuy: true,
		},
		{
			name: "emblem upgrades 545 to 543 at 255", level: 255, cape: 545, emblem: true,
			wantCape: 543, wantBuy: true,
		},
		{
			name: "emblem keeps the sapphires in the bag", level: 219, emblem: true,
			carry:    map[int]int16{0: sapphireUnit1, 1: sapphireUnit1, 2: sapphireUnit1, 3: sapphireUnit1},
			wantCape: 545, wantBuy: true,
			wantCarry: map[int]int16{0: sapphireUnit1, 1: sapphireUnit1, 2: sapphireUnit1, 3: sapphireUnit1},
		},
		{
			name: "emblem below level 219 is refused", level: 218, emblem: true,
			wantEmblem: true,
		},
		{
			name: "celestial cannot pay with the emblem", classMaster: classMasterCelestial, level: 10,
			cape: 3199, emblem: true, wantCape: 3199, wantEmblem: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			save, texts, purchases := runKingCapeCase(t, tc)
			checkKingCapeCase(t, tc, save, texts, purchases)
		})
	}
}

// TestKingCapeSapphirePayment covers issue #400: any bag worth at least the
// price pays (697 = 1, 4131 = 10), consumed in bag order with no change, and
// the sale ends with the King's blessing (_MSG_Quest.cpp:977-1090).
func TestKingCapeSapphirePayment(t *testing.T) {
	bless := []string{msgKingBless1}
	tests := []kingCapeCase{
		{
			name: "one 4131 pays a price of 4", level: 219,
			carry:    map[int]int16{0: sapphireUnit10},
			wantCape: 545, wantBuy: true, wantTexts: bless,
		},
		{
			name: "bag order stops once covered", level: 219,
			carry:     map[int]int16{0: sapphireUnit1, 1: sapphireUnit1, 2: sapphireUnit10, 3: sapphireUnit1},
			wantCarry: map[int]int16{3: sapphireUnit1},
			wantCape:  545, wantBuy: true, wantTexts: bless,
		},
		{
			name: "exact singles still pay", level: 255, cape: 545,
			carry:    map[int]int16{0: sapphireUnit1, 1: sapphireUnit1, 2: sapphireUnit1, 3: sapphireUnit1},
			wantCape: 543, wantBuy: true, wantTexts: bless,
		},
		{
			name: "short of the price", level: 219,
			carry:     map[int]int16{0: sapphireUnit1, 1: sapphireUnit1, 2: sapphireUnit1},
			wantCarry: map[int]int16{0: sapphireUnit1, 1: sapphireUnit1, 2: sapphireUnit1},
			wantTexts: []string{"Sao necessarias 4 safiras."},
		},
		{
			name: "level is checked before the sapphires", level: 218,
			carry:     map[int]int16{0: sapphireUnit10},
			wantCarry: map[int]int16{0: sapphireUnit10},
			wantTexts: []string{msgNeedLevel},
		},
		{
			name: "celestial 3199 pays with a 4131", classMaster: classMasterCelestial, level: 10, cape: 3199,
			carry:    map[int]int16{0: sapphireUnit10},
			wantCape: 3197, wantBuy: true, wantTexts: bless,
		},
		{
			name: "finished cape is blessed", level: 255, cape: 543,
			carry:     map[int]int16{0: sapphireUnit10},
			wantCarry: map[int]int16{0: sapphireUnit10},
			wantCape:  543, wantTexts: bless,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			save, texts, purchases := runKingCapeCase(t, tc)
			checkKingCapeCase(t, tc, save, texts, purchases)
		})
	}
}
