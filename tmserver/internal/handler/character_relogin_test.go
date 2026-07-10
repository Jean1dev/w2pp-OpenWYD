package handler

import (
	"context"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// slotDB is a fakeDB whose LoadCharacter is slot-aware, so one account can host
// two different characters (the relogin-leak scenario needs a chest-buffed
// character and a clean one).
type slotDB struct {
	*fakeDB
	bySlot map[int]world.CharacterState
}

func (f *slotDB) LoadCharacter(_ context.Context, _ int64, slot int) (world.CharacterState, error) {
	return f.bySlot[slot], nil
}

// TestAffectsDoNotLeakAcrossCharacters is the issue #47 regression: the Baú de
// Experiência buff on the character being left must NOT survive the return to
// the selection screen — the entity is per-connection, and before the fix the
// next character picked on the same session showed (and re-persisted!) the
// previous character's chest buff.
func TestAffectsDoNotLeakAcrossCharacters(t *testing.T) {
	db := &slotDB{
		fakeDB: &fakeDB{accounts: map[string]*fakeAccount{
			"tester": {id: 7, pass: "secret", chars: []world.CharSummary{
				{Slot: 0, Name: "Chested", Class: 1, Level: 50},
				{Slot: 1, Name: "Clean", Class: 0, Level: 2},
			}},
		}},
		bySlot: map[int]world.CharacterState{
			0: {
				Slot: 0, Name: "Chested", Class: 1, X: 2100, Y: 2100,
				HP: 500, MaxHP: 500, Level: 50,
				Affects: []world.Affect{{Type: world.AffectExpChest, Time: 140}},
			},
			1: {
				Slot: 1, Name: "Clean", Class: 0, X: 2100, Y: 2100,
				HP: 105, MaxHP: 105, Level: 2,
			},
		},
	}
	addr, stop := startServer(t, db)
	defer stop()
	c := loginAndSelect(t, addr)
	defer c.Close()

	playAndLogout := func(slot uint8) {
		t.Helper()
		var body protocol.MsgCharacterLoginBody
		body.Slot = int32(slot)
		send(t, c, protocol.MsgCharacterLogin, body.Encode())
		if ty, _ := read(t, c); ty != protocol.MsgCNFCharacterLogin {
			t.Fatalf("slot %d: got %#x, want CNFCharacterLogin", slot, ty)
		}
		drainLoginScore(t, c)
		send(t, c, protocol.MsgCharacterLogout, nil)
		for {
			ty, _ := read(t, c)
			if ty == protocol.MsgCNFCharacterLogout {
				return
			}
		}
	}

	playAndLogout(0) // the chest: buff rehydrated, then saved on logout
	chestedSave, n := db.lastSavedChar()
	if n != 1 || chestedSave.Slot != 0 {
		t.Fatalf("first logout: saves = %d (slot %d), want 1 save of slot 0", n, chestedSave.Slot)
	}
	// The reset must not eat the leaving character's own buff: its save still
	// carries the XP chest affect.
	if len(chestedSave.Affects) != 1 || chestedSave.Affects[0].Type != world.AffectExpChest {
		t.Fatalf("chested save affects = %+v, want the persisted XP chest (Type %d)", chestedSave.Affects, world.AffectExpChest)
	}

	playAndLogout(1) // the clean character on the SAME connection
	cleanSave, n := db.lastSavedChar()
	if n != 2 || cleanSave.Slot != 1 {
		t.Fatalf("second logout: saves = %d (slot %d), want 2nd save of slot 1", n, cleanSave.Slot)
	}
	if len(cleanSave.Affects) != 0 {
		t.Errorf("clean save affects = %+v, want none (XP chest leaked across characters)", cleanSave.Affects)
	}
}
