package handler

import (
	"context"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// slotDB is a fakeDB whose LoadCharacter is slot-aware, so one account can host
// two different characters (the relogin-leak scenario needs a buffed BM and a
// clean TK).
type slotDB struct {
	*fakeDB
	bySlot map[int]world.CharacterState
}

func (f *slotDB) LoadCharacter(_ context.Context, _ int64, slot int) (world.CharacterState, error) {
	return f.bySlot[slot], nil
}

// TestAffectsDoNotLeakAcrossCharacters covers the issue #21/#47 regression: a
// buff on the character being left must NOT survive the return to the selection
// screen. The entity is per-connection, and before the fix the next character
// picked on the same session showed and re-persisted the previous character's
// buffs.
func TestAffectsDoNotLeakAcrossCharacters(t *testing.T) {
	db := &slotDB{
		fakeDB: &fakeDB{accounts: map[string]*fakeAccount{
			"tester": {id: 7, pass: "secret", chars: []world.CharSummary{
				{Slot: 0, Name: "Beast", Class: 2, Level: 50},
				{Slot: 1, Name: "Knight", Class: 0, Level: 2},
			}},
		}},
		bySlot: map[int]world.CharacterState{
			0: {
				Slot: 0, Name: "Beast", Class: 2, X: 2100, Y: 2100,
				HP: 500, MaxHP: 500, Level: 50,
				DivineEnd: time.Now().Add(time.Hour).Unix(),
				Affects: []world.Affect{
					{Type: 16, Value: 2, Level: 100, Time: 140},
					{Type: world.AffectExpChest, Time: 140},
				},
			},
			1: {
				Slot: 1, Name: "Knight", Class: 0, X: 2100, Y: 2100,
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

	playAndLogout(0) // the BM: transform + chest + Divine rehydrated, then saved on logout
	beastSave, n := db.lastSavedChar()
	if n != 1 || beastSave.Slot != 0 {
		t.Fatalf("first logout: saves = %d (slot %d), want 1 save of slot 0", n, beastSave.Slot)
	}
	// The reset must not eat the leaving character's own buffs: its save still
	// carries the transform, XP chest and Divine deadline.
	if !hasSavedAffect(beastSave.Affects, 16) {
		t.Fatalf("BM save affects = %+v, want the persisted transform (Type 16)", beastSave.Affects)
	}
	if !hasSavedAffect(beastSave.Affects, world.AffectExpChest) {
		t.Fatalf("BM save affects = %+v, want the persisted XP chest (Type %d)", beastSave.Affects, world.AffectExpChest)
	}
	if beastSave.DivineEnd == 0 {
		t.Error("BM save DivineEnd = 0, want the live deadline preserved")
	}

	playAndLogout(1) // the TK on the SAME connection
	knightSave, n := db.lastSavedChar()
	if n != 2 || knightSave.Slot != 1 {
		t.Fatalf("second logout: saves = %d (slot %d), want 2nd save of slot 1", n, knightSave.Slot)
	}
	if len(knightSave.Affects) != 0 {
		t.Errorf("TK save affects = %+v, want none (BM buffs leaked across characters)", knightSave.Affects)
	}
	if knightSave.DivineEnd != 0 {
		t.Errorf("TK save DivineEnd = %d, want 0 (BM Divine leaked)", knightSave.DivineEnd)
	}
}

func hasSavedAffect(affects []world.Affect, typ uint8) bool {
	for _, a := range affects {
		if a.Type == typ {
			return true
		}
	}
	return false
}
