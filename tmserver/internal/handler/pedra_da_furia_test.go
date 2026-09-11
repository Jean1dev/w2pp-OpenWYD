package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// furiaChar is a Celestial with the Pedra da Fúria in carry slot 0.
func furiaChar(level int, fame int32) world.CharacterState {
	st := world.CharacterState{
		Slot: 0, Name: "Celeste", Class: 1, X: 5, Y: 5, HP: 1000, MaxHP: 1000,
		Level: level, ClassMaster: classMasterCelestial, Fame: fame,
		CelLv40: 1,
	}
	st.Carry[0] = world.Item{Index: itemPedraDaFuria}
	return st
}

// savedAt is the item the save put in slot, or the zero item.
func savedAt(items []world.SavedItem, slot int) world.SavedItem {
	for _, it := range items {
		if it.Slot == slot {
			return it
		}
	}
	return world.SavedItem{}
}

// useFuria enters the world with st, uses carry slot 0 and logs out, returning
// what was saved.
func useFuria(t *testing.T, st world.CharacterState) world.CharacterSave {
	t.Helper()
	db := newDB()
	db.loadResult = st
	addr, stop := startServerClockVol(t, db, nil)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())
	send(t, c, protocol.MsgCharacterLogout, nil)
	expect(t, c, protocol.MsgCNFCharacterLogout)
	char, n := db.lastSavedChar()
	if n == 0 {
		t.Fatal("character never saved")
	}
	return char
}

// TestPedraDaFuriaNivel90 is the level-90 hand-in (_MSG_UseItem.cpp:3624-3677):
// at stored 89 with 500 Fame the Celestial passes the lock, spends the Fame and
// the stone, and gets the Cythera Mística. It is the player's way through a lock
// that only /destravar90 opened before.
func TestPedraDaFuriaNivel90(t *testing.T) {
	char := useFuria(t, furiaChar(furiaLevel90Min, furiaFameCost))
	if char.CelLv90 != 1 {
		t.Fatalf("saved CelLv90 = %d, want 1", char.CelLv90)
	}
	if char.Fame != 0 {
		t.Errorf("saved Fame = %d, want 0 (500 spent)", char.Fame)
	}
	if savedAt(char.Carry, 0).Index == itemPedraDaFuria {
		t.Error("the stone is still in carry slot 0")
	}
	if !hasItem(char.Carry, itemCytheraMistica) {
		t.Errorf("carry missing the Cythera Mística %d: %+v", itemCytheraMistica, char.Carry)
	}
}

// TestPedraDaFuriaRecusas: every refusal hands the stone back unspent and
// changes nothing — below the lock, short of Fame, already unlocked, or not a
// Celestial at all.
func TestPedraDaFuriaRecusas(t *testing.T) {
	jaFeito := furiaChar(furiaLevel90Min, furiaFameCost)
	jaFeito.CelLv90 = 1
	mortal := furiaChar(furiaLevel90Min, furiaFameCost)
	mortal.ClassMaster = classMasterMortal
	for _, c := range []struct {
		nome string
		st   world.CharacterState
	}{
		{"abaixo do 89", furiaChar(furiaLevel90Min-1, furiaFameCost)},
		{"sem fama", furiaChar(furiaLevel90Min, furiaFameCost-1)},
		{"ja destravado", jaFeito},
		{"mortal", mortal},
	} {
		t.Run(c.nome, func(t *testing.T) {
			char := useFuria(t, c.st)
			if got := savedAt(char.Carry, 0).Index; got != itemPedraDaFuria {
				t.Errorf("carry slot 0 = %d, want the stone handed back", got)
			}
			if char.Fame != c.st.Fame || char.CelLv90 != c.st.CelLv90 {
				t.Errorf("Fame/Lv90 = %d/%d, want unchanged %d/%d", char.Fame, char.CelLv90, c.st.Fame, c.st.CelLv90)
			}
		})
	}
}

// TestPedraDaFuriaArcana is the Arcana hand-in (_MSG_UseItem.cpp:3480-3578): at
// the cap with 500 Fame and the four Pedras Secretas, the Celestial becomes
// Circle 1 and its Cythera turns into the Arcana — keeping its effects, as the
// legacy only swaps the index.
func TestPedraDaFuriaArcana(t *testing.T) {
	st := furiaChar(furiaArcanaMinLevel, furiaFameCost)
	st.CelLv90 = 1
	st.Equip[arcanaEquipSlot] = world.Item{Index: itemCytheraMistica, Effects: [3]world.Effect{{Effect: 2, Value: 7}}}
	for i, idx := range pedrasSecretas {
		st.Carry[1+i] = world.Item{Index: idx}
	}
	char := useFuria(t, st)
	if char.CelCircle != 1 {
		t.Fatalf("saved CelCircle = %d, want 1", char.CelCircle)
	}
	if got := savedAt(char.Equip, arcanaEquipSlot); got.Index != arcanaItemIndex || got.Eff1 != 2 || got.EffV1 != 7 {
		t.Errorf("Equip[1] = %+v, want index %d with the old effect 2/7", got, arcanaItemIndex)
	}
	for _, idx := range pedrasSecretas {
		if hasItem(char.Carry, idx) {
			t.Errorf("Pedra Secreta %d still in carry", idx)
		}
	}
	if char.Fame != 0 || savedAt(char.Carry, 0).Index == itemPedraDaFuria {
		t.Errorf("Fame %d, slot 0 %d: want the Fame and the stone spent", char.Fame, savedAt(char.Carry, 0).Index)
	}
}

// TestPedraDaFuriaArcanaSemPedras: without all four Pedras Secretas the stone
// comes back and nothing is spent (_MSG_UseItem.cpp:3575-3576).
func TestPedraDaFuriaArcanaSemPedras(t *testing.T) {
	st := furiaChar(furiaArcanaMinLevel, furiaFameCost)
	st.CelLv90 = 1
	for i, idx := range pedrasSecretas[:3] {
		st.Carry[1+i] = world.Item{Index: idx}
	}
	char := useFuria(t, st)
	if char.CelCircle != 0 || char.Fame != furiaFameCost || savedAt(char.Carry, 0).Index != itemPedraDaFuria {
		t.Errorf("Circle %d Fame %d slot0 %d: want nothing spent", char.CelCircle, char.Fame, savedAt(char.Carry, 0).Index)
	}
	for _, idx := range pedrasSecretas[:3] {
		if !hasItem(char.Carry, idx) {
			t.Errorf("Pedra Secreta %d was taken without the fourth", idx)
		}
	}
}
