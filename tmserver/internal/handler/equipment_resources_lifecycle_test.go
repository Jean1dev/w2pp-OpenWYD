package handler

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// resourceRoundTripDB uses actual captured CharacterSave values on the next
// login. Its lock protects only fake persistence, never live world state.
type resourceRoundTripDB struct {
	*fakeDB
	initial map[int]world.CharacterState
}

func (f *resourceRoundTripDB) LoadCharacter(_ context.Context, _ int64, slot int) (world.CharacterState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	st := f.initial[slot]
	for i := len(f.savedChars) - 1; i >= 0; i-- {
		save := f.savedChars[i]
		if save.Slot != slot {
			continue
		}
		st.HP, st.MaxHP, st.MP, st.MaxMP = save.HP, save.MaxHP, save.MP, save.MaxMP
		st.Str, st.Int, st.Dex, st.Con = save.Str, save.Int, save.Dex, save.Con
		st.Level, st.Exp, st.ClassMaster = int(save.Level), save.Exp, save.ClassMaster
		st.ScoreBonus = save.ScoreBonus
		st.Equip, st.Carry = [world.MaxEquip]world.Item{}, [world.MaxCarry]world.Item{}
		for _, group := range []struct {
			saved []world.SavedItem
			items []world.Item
		}{{save.Equip, st.Equip[:]}, {save.Carry, st.Carry[:]}} {
			for _, it := range group.saved {
				group.items[it.Slot] = world.Item{Index: it.Index, ExpiresAt: it.ExpiresAt, Effects: [3]world.Effect{
					{Effect: it.Eff1, Value: it.EffV1}, {Effect: it.Eff2, Value: it.EffV2}, {Effect: it.Eff3, Value: it.EffV3},
				}}
			}
		}
		break
	}
	return st, nil
}

func resourceCharacterState() world.CharacterState {
	st := world.CharacterState{
		Slot: 0, Name: "Hero", Class: 0, ClassMaster: classMasterMortal, Level: 10, X: 5, Y: 5,
		HP: 400, MaxHP: 1000, MP: 300, MaxMP: 1000,
		Str: 100, Int: 100, Dex: 100, Con: 100, ScoreBonus: 2,
	}
	st.Equip[0] = world.Item{Index: 1} // avoid starter-gear seeding
	return st
}

func startEquipmentResourcesServer(t *testing.T, persist world.Persistence) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := equipmentResourcesConfig(t)
	cfg.Log = log
	d := New(cfg)
	w := world.New(world.Config{GridDim: 16}, log, persist, d.Handle)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Serve(ctx, ln) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil && err != context.Canceled {
				t.Errorf("serve: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	})
	return ln.Addr().String()
}

func assertResourceScore(t *testing.T, payload []byte, hp, maxHP, mp, maxMP int32) {
	t.Helper()
	if len(payload) != 140 {
		t.Fatalf("score payload length = %d", len(payload))
	}
	for _, tt := range []struct {
		offset int
		want   int32
	}{{16, maxHP}, {20, maxMP}, {24, hp}, {28, mp}, {124, hp}, {128, mp}} {
		if got := int32(binary.LittleEndian.Uint32(payload[tt.offset:])); got != tt.want {
			t.Fatalf("score offset %d = %d, want %d", tt.offset, got, tt.want)
		}
	}
}

func selectResourceCharacter(t *testing.T, c net.Conn, slot int32) []byte {
	t.Helper()
	body := protocol.MsgCharacterLoginBody{Slot: slot}
	send(t, c, protocol.MsgCharacterLogin, body.Encode())
	expect(t, c, protocol.MsgCNFCharacterLogin)
	return expect(t, c, protocol.MsgUpdateScore)
}

func logoutResourceCharacter(t *testing.T, c net.Conn) {
	t.Helper()
	send(t, c, protocol.MsgCharacterLogout, nil)
	expect(t, c, protocol.MsgCNFCharacterLogout)
}

func TestEquipmentResourcesTradingAndAllocation(t *testing.T) {
	db := newDB()
	db.loadResult = resourceCharacterState()
	db.loadResult.Carry[0] = world.Item{Index: 4185}
	db.loadResult.Carry[1] = world.Item{Index: 4187}
	c := enterWorld(t, startEquipmentResourcesServer(t, db))
	defer c.Close()
	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceEquip, 12, 0)
	assertResourceScore(t, expect(t, c, protocol.MsgUpdateScore), 400, 1500, 300, 1500)
	// Wrong slot rejects the replacement. The next valid allocation is also a
	// response barrier: its score proves the costume and maxima did not change.
	tradeItemFrame(t, c, world.ItemPlaceCarry, 1, world.ItemPlaceEquip, 2, 0)
	for _, detail := range []int16{protocol.DetailCon, protocol.DetailInt} {
		body := protocol.MsgApplyBonusBody{BonusType: protocol.BonusScore, Detail: detail}
		send(t, c, protocol.MsgApplyBonus, body.Encode())
		maxMP := int32(1500)
		if detail == protocol.DetailInt {
			maxMP = 1502
		}
		assertResourceScore(t, expect(t, c, protocol.MsgUpdateScore), 400, 1502, 300, maxMP)
	}
	tradeItemFrame(t, c, world.ItemPlaceCarry, 1, world.ItemPlaceEquip, 12, 0)
	assertResourceScore(t, expect(t, c, protocol.MsgUpdateScore), 400, 1002, 300, 2002)
	tradeItemFrame(t, c, world.ItemPlaceEquip, 12, world.ItemPlaceCarry, 2, 0)
	assertResourceScore(t, expect(t, c, protocol.MsgUpdateScore), 400, 1002, 300, 1002)
	logoutResourceCharacter(t, c)
	save, _ := db.lastSavedChar()
	if save.MaxHP != 1002 || save.MaxMP != 1002 || hasItem(save.Equip, 4185) || hasItem(save.Equip, 4187) {
		t.Fatal("transition save retained costume resources")
	}
}

func TestEquipmentResourcesSaveLoginAndExpiration(t *testing.T) {
	st := resourceCharacterState()
	st.Equip[12] = world.Item{Index: 4185}
	st.Str, st.Int, st.Dex, st.Con = 350, 350, 350, 350
	// A pre-fix save has these attributes but only 1000 flat HP/MP maxima.
	expired := st
	expired.Slot = 1
	expired.Equip[12].ExpiresAt = time.Now().Add(-time.Hour).Unix()
	db := &resourceRoundTripDB{fakeDB: newDB(), initial: map[int]world.CharacterState{0: st, 1: expired}}
	c := loginAndSelect(t, startEquipmentResourcesServer(t, db))
	defer c.Close()
	for range 3 {
		assertResourceScore(t, selectResourceCharacter(t, c, 0), 400, 1500, 300, 1500)
		logoutResourceCharacter(t, c)
		save, _ := db.lastSavedChar()
		if save.MaxHP != 1000 || save.MaxMP != 1000 || save.Int != 350 || save.Con != 350 {
			t.Fatalf("save drift: maxima %d/%d attributes %d/%d", save.MaxHP, save.MaxMP, save.Int, save.Con)
		}
	}
	// Same session/entity, different character with expired gear: cached
	// resources from the previous character must not survive initialization.
	assertResourceScore(t, selectResourceCharacter(t, c, 1), 400, 1000, 300, 1000)
	logoutResourceCharacter(t, c)
	expiredSave, _ := db.lastSavedChar()
	if expiredSave.MaxHP != 1000 || expiredSave.MaxMP != 1000 || hasItem(expiredSave.Equip, 4185) {
		t.Fatal("expired costume leaked into the save")
	}
	assertResourceScore(t, selectResourceCharacter(t, c, 0), 400, 1500, 300, 1500)
	tradeItemFrame(t, c, world.ItemPlaceEquip, 12, world.ItemPlaceCarry, 0, 0)
	assertResourceScore(t, expect(t, c, protocol.MsgUpdateScore), 400, 1000, 300, 1000)
	logoutResourceCharacter(t, c)
	assertResourceScore(t, selectResourceCharacter(t, c, 0), 400, 1000, 300, 1000)
}

func TestEquipmentResourcesFullBarsClampOnUnequip(t *testing.T) {
	db := newDB()
	db.loadResult = resourceCharacterState()
	db.loadResult.Equip[12] = world.Item{Index: 4185}
	db.loadResult.Str, db.loadResult.Int, db.loadResult.Dex, db.loadResult.Con = 350, 350, 350, 350
	// Current resources persist independently from the historical flat maxima.
	db.loadResult.HP, db.loadResult.MP = 1500, 1500
	c := loginAndSelect(t, startEquipmentResourcesServer(t, db))
	defer c.Close()
	assertResourceScore(t, selectResourceCharacter(t, c, 0), 1500, 1500, 1500, 1500)
	tradeItemFrame(t, c, world.ItemPlaceEquip, 12, world.ItemPlaceCarry, 0, 0)
	assertResourceScore(t, expect(t, c, protocol.MsgUpdateScore), 1000, 1000, 1000, 1000)
	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceEquip, 12, 0)
	assertResourceScore(t, expect(t, c, protocol.MsgUpdateScore), 1000, 1500, 1000, 1500)
}
