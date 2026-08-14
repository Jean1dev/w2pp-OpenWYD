package handler

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestMeetsEquipReq(t *testing.T) {
	d := New(Config{ItemReqs: map[int]content.ItemReq{
		900: {Lvl: 100, Str: 50}, // a heavy sword
	}})
	sword := world.Item{Index: 900}
	if d.meetsEquipReq(&world.Entity{ClassMaster: classMasterMortal, Level: 99, Str: 60}, sword) {
		t.Error("equip allowed below the level requirement")
	}
	if d.meetsEquipReq(&world.Entity{ClassMaster: classMasterMortal, Level: 100, Str: 49}, sword) {
		t.Error("equip allowed below the str requirement")
	}
	if !d.meetsEquipReq(&world.Entity{ClassMaster: classMasterMortal, Level: 100, Str: 50}, sword) {
		t.Error("equip rejected when requirements are met")
	}
	if !d.meetsEquipReq(&world.Entity{ClassMaster: classMasterMortal}, world.Item{Index: 1}) {
		t.Error("an item with no catalog requirement must always pass")
	}
	// Basedef.cpp:5024-5028 (BASE_CanEquip): the level/attribute requirement only
	// gates MORTAL — Arch and Celestial characters bypass it outright, even far
	// below the item's requirement (issue #209).
	if !d.meetsEquipReq(&world.Entity{ClassMaster: classMasterArch, Level: 1, Str: 1}, sword) {
		t.Error("arch character blocked by an equip requirement it should bypass")
	}
	if !d.meetsEquipReq(&world.Entity{ClassMaster: classMasterCelestial, Level: 1, Str: 1}, sword) {
		t.Error("celestial character blocked by an equip requirement it should bypass")
	}
}

func itemDB(carry0 int16) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: carry0}
	db.loadResult = st
	return db
}

func dropFrame(t *testing.T, c net.Conn, sourPos int, gx, gy uint16) {
	t.Helper()
	body := protocol.MsgDropItemBody{SourType: world.ItemPlaceCarry, SourPos: int32(sourPos), GridX: gx, GridY: gy}
	send(t, c, protocol.MsgDropItem, body.Encode())
}

func getFrame(t *testing.T, c net.Conn, itemID int32, destPos int) {
	t.Helper()
	body := protocol.MsgGetItemBody{ItemID: itemID, DestType: world.ItemPlaceCarry, DestPos: int32(destPos)}
	send(t, c, protocol.MsgGetItem, body.Encode())
}

func TestDropAndGet(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	dropFrame(t, c, 0, 5, 5)
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgCNFDropItem {
		t.Fatalf("drop got %#x ok=%v, want CNFDropItem", ty, ok)
	}

	// First ground item gets id 1 ⇒ wire ItemID 10001.
	getFrame(t, c, world.GroundItemIDOffset+1, 0)
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgCNFGetItem {
		t.Errorf("get got %#x ok=%v, want CNFGetItem", ty, ok)
	}
}

func TestDropBlacklisted(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(508)) // 508 is non-droppable
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	dropFrame(t, c, 0, 5, 5)
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("blacklisted drop produced %#x; should be blocked", ty)
	}
}

func TestGetDecayed(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	// Nothing dropped yet ⇒ ground id is empty ⇒ DecayItem.
	getFrame(t, c, world.GroundItemIDOffset+1, 0)
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgDecayItem {
		t.Errorf("get got %#x ok=%v, want DecayItem", ty, ok)
	}
}

func TestGetTooFar(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	dropFrame(t, c, 0, 15, 15) // player is at (5,5); item is >3 cells away
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgCNFDropItem {
		t.Fatalf("drop got %#x ok=%v", ty, ok)
	}
	getFrame(t, c, world.GroundItemIDOffset+1, 0)
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("far get produced %#x; should be rejected", ty)
	}
}

// TestDupRace proves the atomic claim: two gets of the same ground item ⇒ exactly
// one CNFGetItem, the other DecayItem (the loop serializes them).
func TestDupRace(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	dropFrame(t, c, 0, 5, 5)
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgCNFDropItem {
		t.Fatalf("drop got %#x ok=%v", ty, ok)
	}

	getFrame(t, c, world.GroundItemIDOffset+1, 0)
	getFrame(t, c, world.GroundItemIDOffset+1, 1)

	ty1, _, _ := readMaybe(t, c)
	ty2, _, _ := readMaybe(t, c)
	got := map[protocol.Type]int{ty1: 1}
	got[ty2]++
	if got[protocol.MsgCNFGetItem] != 1 || got[protocol.MsgDecayItem] != 1 {
		t.Errorf("dup race responses = %#x, %#x; want one CNFGetItem + one DecayItem", ty1, ty2)
	}
}

// equipDB seeds the tester character with an optional carry-0 item and an
// optional equip-slot-1 item (to exercise equip and unequip).
func equipDB(carry0, equip1 int16) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	if carry0 != 0 {
		st.Carry[0] = world.Item{Index: carry0}
	}
	if equip1 != 0 {
		st.Equip[1] = world.Item{Index: equip1}
	}
	db.loadResult = st
	return db
}

func equipItemDB(carry0 world.Item) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = carry0
	db.loadResult = st
	return db
}

// TestTradingItemEquip drags an inventory item onto an equip slot (0x0376) and
// asserts the rendered gear is refreshed via _MSG_UpdateEquip with the item code.
func TestTradingItemEquip(t *testing.T) {
	addr, stop, _ := startServerClock(t, equipDB(1100, 0))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceEquip, 1, 0)
	expect(t, c, protocol.MsgTradingItem)
	expect(t, c, protocol.MsgSendItem)
	expect(t, c, protocol.MsgSendItem)
	ue := expect(t, c, protocol.MsgUpdateEquip)
	if got := le16(ue[2:4]); got != 1100 { // Equip[1] visual code @body2
		t.Errorf("equip visual[1] = %d, want 1100", got)
	}
}

func TestTradingItemEquipSendsRefineGlow(t *testing.T) {
	item := world.Item{Index: 1100, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}}
	addr, stop, _ := startServerClock(t, equipItemDB(item))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceEquip, 1, 0)
	expect(t, c, protocol.MsgTradingItem)
	expect(t, c, protocol.MsgSendItem)
	expect(t, c, protocol.MsgSendItem)
	ue := expect(t, c, protocol.MsgUpdateEquip)
	if want, got := uint16(1100|9*0x1000), le16(ue[2:4]); got != want {
		t.Errorf("equip visual[1] = %d, want %d (+9 overlay)", got, want)
	}
	if got := ue[33]; got != efSanc { // AnctCode[1] @body32+1
		t.Errorf("equip anct[1] = %#x, want EF_SANC", got)
	}
}

// TestTradingItemUnequip proves the equipment is loaded from the DB (so the slot
// has something to remove) and unequipping clears the rendered gear.
func TestTradingItemUnequip(t *testing.T) {
	addr, stop, _ := startServerClock(t, equipDB(0, 2200))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceEquip, 1, world.ItemPlaceCarry, 5, 0)
	expect(t, c, protocol.MsgTradingItem)
	expect(t, c, protocol.MsgSendItem) // equip slot 1 (now empty)
	expect(t, c, protocol.MsgSendItem) // carry slot 5 (now holds the item)
	ue := expect(t, c, protocol.MsgUpdateEquip)
	if got := le16(ue[2:4]); got != 0 { // Equip[1] now empty
		t.Errorf("equip visual[1] = %d, want 0 after unequip", got)
	}
}

// weaponAutoShiftDB seeds the tester character with a single item in carry
// slot 0, for exercising the weapon-hand auto-shift on quick-equip.
func weaponAutoShiftDB(item int16) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: item}
	db.loadResult = st
	return db
}

// startServerClockItemPos is startServerClock with an injected ItemPos catalog,
// needed to exercise equip-slot gating (canEquipSlot / shiftWeaponToRightHand).
func startServerClockItemPos(t *testing.T, persist world.Persistence, itemPos map[int]int) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, ItemPos: itemPos})
	w := world.New(world.Config{GridDim: 16}, log, persist, d.Handle)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	return ln.Addr().String(), func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}
}

// TestTradingItemWeaponAutoShift proves the issue #65 fix: quick-equipping a
// one-handed weapon (nPos 192, fits either hand) into the off-hand slot (7)
// while the primary hand (6) is empty gets auto-shifted into slot 6, mirroring
// _MSG_TradingItem.cpp:394-414.
func TestTradingItemWeaponAutoShift(t *testing.T) {
	addr, stop := startServerClockItemPos(t, weaponAutoShiftDB(1100), map[int]int{1100: 192})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceEquip, 7, 0)
	expect(t, c, protocol.MsgTradingItem) // primary swap echo
	expect(t, c, protocol.MsgSendItem)    // carry slot 0, now empty
	expect(t, c, protocol.MsgSendItem)    // equip slot 7, now holds 1100

	expect(t, c, protocol.MsgTradingItem) // auto-shift echo: 7 -> 6
	expect(t, c, protocol.MsgSendItem)    // equip slot 6, now holds 1100
	expect(t, c, protocol.MsgSendItem)    // equip slot 7, now empty again

	ue := expect(t, c, protocol.MsgUpdateEquip)
	if got := le16(ue[12:14]); got != 1100 { // Equip[6] visual code
		t.Errorf("equip visual[6] = %d, want 1100 after auto-shift", got)
	}
	if got := le16(ue[14:16]); got != 0 { // Equip[7] visual code
		t.Errorf("equip visual[7] = %d, want 0 after auto-shift", got)
	}
}

// TestTradingItemShieldNotShifted proves a real shield (nPos 128, off-hand
// only) is exempt from the auto-shift: it must stay in slot 7 even with slot 6
// empty (_MSG_TradingItem.cpp:400 hab != 128 guard).
func TestTradingItemShieldNotShifted(t *testing.T) {
	addr, stop := startServerClockItemPos(t, weaponAutoShiftDB(2200), map[int]int{2200: 128})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceEquip, 7, 0)
	expect(t, c, protocol.MsgTradingItem) // primary swap echo
	expect(t, c, protocol.MsgSendItem)    // carry slot 0, now empty
	expect(t, c, protocol.MsgSendItem)    // equip slot 7, now holds 2200

	ue := expect(t, c, protocol.MsgUpdateEquip)
	if got := le16(ue[14:16]); got != 2200 { // Equip[7] visual code
		t.Errorf("equip visual[7] = %d, want 2200 (shield stays put)", got)
	}
	if got := le16(ue[12:14]); got != 0 { // Equip[6] visual code
		t.Errorf("equip visual[6] = %d, want 0 (nothing shifted)", got)
	}
}

func TestUseItemEquip(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{
		SourType: world.ItemPlaceCarry, SourPos: 0,
		DestType: world.ItemPlaceEquip, DestPos: 0,
	}
	send(t, c, protocol.MsgUseItem, body.Encode())
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgUseItem {
		t.Errorf("equip got %#x ok=%v, want UseItem echo", ty, ok)
	}
}

func territorioDB(item int16, classMaster uint8) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000, ClassMaster: classMaster}
	st.Carry[0] = world.Item{Index: item}
	db.loadResult = st
	return db
}

// TestUseEntradaTerritorio covers the Vol-188 "Entrada do Território" (LAN)
// tickets (issue #203, _MSG_UseItem.cpp:4202-4236): the right class tier teleports
// into the matching Lan_N/M/A coords (center ± [-3,+1] per axis) and consumes one;
// the wrong tier is rejected with a slot resync and no teleport.
func TestUseEntradaTerritorio(t *testing.T) {
	tests := []struct {
		name         string
		item         int16
		classMaster  uint8
		wantTeleport bool
		wantX, wantY int16
	}{
		{"normal_mortal", 4111, classMasterMortal, true, 3639, 3639},
		{"medio_mortal", 4112, classMasterMortal, true, 3782, 3527},
		{"avancado_celestial", 4113, classMasterCelestial, true, 3911, 3655},
		{"avancado_mortal_rejected", 4113, classMasterMortal, false, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := territorioDB(tc.item, tc.classMaster)
			addr, stop := startServerClockVol(t, db, map[int]int{int(tc.item): volEntradaTerritorio})
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()

			body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
			send(t, c, protocol.MsgUseItem, body.Encode())

			var sawTeleport, sawConsume, sawKeep bool
			var gotX, gotY int16
			for {
				ty, payload, ok := readMaybe(t, c)
				if !ok {
					break
				}
				switch ty {
				case protocol.MsgAction:
					var ab protocol.MsgActionBody
					if err := ab.Decode(payload); err != nil {
						t.Fatalf("decode action: %v", err)
					}
					if ab.Effect == 1 {
						sawTeleport = true
						gotX, gotY = ab.TargetX, ab.TargetY
					}
				case protocol.MsgSendItem:
					if int(le16(payload[0:2])) != world.ItemPlaceCarry || int(le16(payload[2:4])) != 0 {
						continue
					}
					switch int16(le16(payload[4:6])) {
					case 0:
						sawConsume = true
					case tc.item:
						sawKeep = true
					}
				}
			}

			if tc.wantTeleport {
				if !sawTeleport {
					t.Fatal("expected teleport jump, got none")
				}
				if gotX < tc.wantX-3 || gotX > tc.wantX+1 || gotY < tc.wantY-3 || gotY > tc.wantY+1 {
					t.Errorf("teleport to (%d,%d), want ~(%d,%d) ±[-3,+1]", gotX, gotY, tc.wantX, tc.wantY)
				}
				if !sawConsume {
					t.Error("ticket not consumed")
				}
			} else {
				if sawTeleport {
					t.Error("rejected ticket teleported the player")
				}
				if !sawKeep {
					t.Error("rejected ticket: expected slot resync keeping the item")
				}
			}
		})
	}
}

// idealStoneDB builds a fakeDB for a character holding a Pedra Ideal (item 1742) in
// carry slot 0, at the given tier/level (issue #222).
func idealStoneDB(classMaster uint8, lvl int) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000, ClassMaster: classMaster, Level: lvl}
	st.Carry[0] = world.Item{Index: idealStoneItem}
	db.loadResult = st
	return db
}

// TestUseIdealStoneCreatesCelestial covers the Arch→Celestial transformation
// (issue #222): an Arch at the tier cap (level.MaxLevel) right-clicking the Pedra
// Ideal becomes a Celestial at level 1, with the item consumed and the celestial
// quest gates reset, and the change persists.
func TestUseIdealStoneCreatesCelestial(t *testing.T) {
	db := idealStoneDB(classMasterArch, int(level.MaxLevel))
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	expect(t, c, protocol.MsgSendItem) // Pedra Ideal removed from carry
	ty, payload, ok := readMaybe(t, c)
	if !ok || ty != protocol.MsgUpdateScore {
		t.Fatalf("got %#x ok=%v, want UpdateScore after Celestial conversion", ty, ok)
	}
	// The fixture has zero STR/DEX/Special and no damage-bearing gear. A fresh
	// Celestial therefore has only its level term (1+MAX_LEVEL); the Mortal/Arch
	// BaseScore.Damage=5 must have been reset before this score was emitted.
	if want := level.MaxLevel + 1; scoreDamage(payload) != want {
		t.Errorf("Celestial Damage = %d, want %d (zero base + level term)", scoreDamage(payload), want)
	}
	expect(t, c, protocol.MsgCombineComplete) // unlock signal
	expect(t, c, protocol.MsgMotion)          // unlock emote

	send(t, c, protocol.MsgCharacterLogout, nil)
	expect(t, c, protocol.MsgCNFCharacterLogout)
	char, n := db.lastSavedChar()
	if n == 0 {
		t.Fatal("character was never saved")
	}
	if char.ClassMaster != classMasterCelestial {
		t.Errorf("saved ClassMaster = %d, want %d (celestial)", char.ClassMaster, classMasterCelestial)
	}
	if char.Level != 1 {
		t.Errorf("saved Level = %d, want 1 (reset)", char.Level)
	}
	if char.Exp != 0 {
		t.Errorf("saved Exp = %d, want 0 (reset)", char.Exp)
	}
	if char.CelLv40 != 0 || char.CelLv90 != 0 || char.CelCircle != 0 {
		t.Errorf("saved celestial gates = %d/%d/%d, want all 0 (fresh)", char.CelLv40, char.CelLv90, char.CelCircle)
	}
	if hasItem(char.Carry, idealStoneItem) {
		t.Error("saved carry still has the Pedra Ideal; want consumed")
	}
}

// TestUseIdealStoneRequiresMaxArchLevel verifies an Arch below the tier cap is
// rejected: no tier change, the item stays put, and the client gets a slot resync
// instead of the unlock signals.
func TestUseIdealStoneRequiresMaxArchLevel(t *testing.T) {
	db := idealStoneDB(classMasterArch, int(level.MaxLevel)-1)
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	sawResync := false
	for {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			break
		}
		if ty == protocol.MsgCombineComplete || ty == protocol.MsgMotion {
			t.Fatalf("got %#x for a below-level Arch; want no unlock signal", ty)
		}
		if ty == protocol.MsgSendItem && int16(le16(payload[4:6])) == idealStoneItem {
			sawResync = true
		}
	}
	if !sawResync {
		t.Error("expected a slot resync keeping the Pedra Ideal")
	}

	send(t, c, protocol.MsgCharacterLogout, nil)
	expect(t, c, protocol.MsgCNFCharacterLogout)
	char, n := db.lastSavedChar()
	if n == 0 {
		t.Fatal("character was never saved")
	}
	if char.ClassMaster != classMasterArch {
		t.Errorf("saved ClassMaster = %d, want %d (unchanged, still arch)", char.ClassMaster, classMasterArch)
	}
}

// TestUseIdealStoneMortalStillEquips is a regression guard: a Mortal carrying the
// Pedra Ideal must still fall through to the normal equip path (the kingArch
// Mortal→Arch flow needs it equipped in slot 10), unaffected by the new Arch-only
// self-use interception.
func TestUseIdealStoneMortalStillEquips(t *testing.T) {
	db := idealStoneDB(classMasterMortal, 100)
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{
		SourType: world.ItemPlaceCarry, SourPos: 0,
		DestType: world.ItemPlaceEquip, DestPos: idealStoneEquipSlot,
	}
	send(t, c, protocol.MsgUseItem, body.Encode())
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgUseItem {
		t.Errorf("mortal equip got %#x ok=%v, want UseItem echo (equip path)", ty, ok)
	}
}

const (
	itemMagicBeanBlue    = 3407
	itemMagicBeanLight   = 3416
	itemMagicBeanRemover = 3417
)

func magicBeanDB(bean, equip world.Item) *fakeDB {
	return magicBeanDBAt(bean, equip, 1, "")
}

func magicBeanDBAt(bean, equip world.Item, equipSlot int, role string) *fakeDB {
	db := newDB()
	db.accounts["tester"].role = role
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = bean
	st.Equip[equipSlot] = equip
	db.loadResult = st
	return db
}

func useMagicBeanFrame(t *testing.T, c net.Conn, dstSlot int) {
	t.Helper()
	body := protocol.MsgUseItemBody{
		SourType: world.ItemPlaceCarry, SourPos: 0,
		DestType: world.ItemPlaceEquip, DestPos: int32(dstSlot),
	}
	send(t, c, protocol.MsgUseItem, body.Encode())
}

func magicBeanVols(bean int16) map[int]int {
	return map[int]int{int(bean): volMagicBean}
}

func TestUseMagicBeanPaintsEquippedSet(t *testing.T) {
	armor := world.Item{Index: itemArmor, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}}
	addr, stop := startServerClockVol(t, magicBeanDB(world.Item{Index: itemMagicBeanBlue}, armor), magicBeanVols(itemMagicBeanBlue))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useMagicBeanFrame(t, c, 1)

	if code := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); code != NoticeRefineSuccess {
		t.Fatalf("notice = %d, want RefineSuccess", code)
	}
	expect(t, c, protocol.MsgUpdateScore)
	item := expect(t, c, protocol.MsgSendItem)
	if got := le16(item[0:2]); got != world.ItemPlaceEquip {
		t.Fatalf("send item place = %d, want equip", got)
	}
	if got := le16(item[2:4]); got != 1 {
		t.Fatalf("send item slot = %d, want 1", got)
	}
	if got := le16(item[4:6]); got != itemArmor {
		t.Fatalf("painted item index = %d, want armor", got)
	}
	if item[6] != magicBeanPaintLo || item[7] != 9 {
		t.Fatalf("effect0 = %d.%d, want paint %d preserving sanc value 9", item[6], item[7], magicBeanPaintLo)
	}
}

func TestUseMagicBeanRejectsWeaponForPlayers(t *testing.T) {
	weapon := world.Item{Index: 900, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}}
	db := magicBeanDBAt(world.Item{Index: itemMagicBeanBlue}, weapon, weaponSlotR, "")
	addr, stop := startServerClockVol(t, db, magicBeanVols(itemMagicBeanBlue))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useMagicBeanFrame(t, c, weaponSlotR)

	if code := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); code != NoticeCantUseHere {
		t.Fatalf("notice = %d, want CantUseHere", code)
	}
	item := expect(t, c, protocol.MsgSendItem)
	if got := le16(item[0:2]); got != world.ItemPlaceCarry {
		t.Fatalf("reject send item place = %d, want carry", got)
	}
	if got := le16(item[4:6]); got != itemMagicBeanBlue {
		t.Fatalf("reject source item = %d, want magic bean", got)
	}
	if ty, _, ok := readMaybe(t, c); ok {
		t.Fatalf("player weapon magic bean use produced extra frame %#x", ty)
	}

	send(t, c, protocol.MsgCharacterLogout, nil)
	expect(t, c, protocol.MsgCNFCharacterLogout)
	save, n := db.lastSavedChar()
	if n == 0 {
		t.Fatal("character was not saved on logout")
	}
	carry0, ok := savedItemAt(save.Carry, 0)
	if !ok || carry0.Index != itemMagicBeanBlue {
		t.Fatalf("saved carry0 = %+v ok=%v, want unconsumed magic bean", carry0, ok)
	}
	equip6, ok := savedItemAt(save.Equip, weaponSlotR)
	if !ok || equip6.Index != 900 || equip6.Eff1 != efSanc || equip6.EffV1 != 9 {
		t.Fatalf("saved weapon = %+v ok=%v, want unpainted +9 weapon", equip6, ok)
	}
}

func TestUseMagicBeanAllowsWeaponForModerators(t *testing.T) {
	weapon := world.Item{Index: 900, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}}
	addr, stop := startServerClockVol(t, magicBeanDBAt(world.Item{Index: itemMagicBeanBlue}, weapon, weaponSlotR, "moderator"), magicBeanVols(itemMagicBeanBlue))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useMagicBeanFrame(t, c, weaponSlotR)

	if code := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); code != NoticeRefineSuccess {
		t.Fatalf("notice = %d, want RefineSuccess", code)
	}
	expect(t, c, protocol.MsgUpdateScore)
	item := expect(t, c, protocol.MsgSendItem)
	if got := le16(item[0:2]); got != world.ItemPlaceEquip {
		t.Fatalf("send item place = %d, want equip", got)
	}
	if got := le16(item[2:4]); got != weaponSlotR {
		t.Fatalf("send item slot = %d, want weapon slot", got)
	}
	if got := le16(item[4:6]); got != 900 {
		t.Fatalf("painted item index = %d, want weapon", got)
	}
	if item[6] != magicBeanPaintLo || item[7] != 9 {
		t.Fatalf("effect0 = %d.%d, want paint %d preserving sanc value 9", item[6], item[7], magicBeanPaintLo)
	}
}

func TestUseMagicBeanHighestColorAndRemover(t *testing.T) {
	tests := []struct {
		name string
		bean int16
		dst  world.Item
		want uint8
	}{
		{
			name: "light blue writes 125",
			bean: itemMagicBeanLight,
			dst:  world.Item{Index: itemArmor, Effects: [3]world.Effect{{Effect: efSanc, Value: 8}}},
			want: magicBeanPaintHi,
		},
		{
			name: "remover writes EF_SANC",
			bean: itemMagicBeanRemover,
			dst:  world.Item{Index: itemArmor, Effects: [3]world.Effect{{Effect: magicBeanPaintLo + 4, Value: 8}}},
			want: efSanc,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, stop := startServerClockVol(t, magicBeanDB(world.Item{Index: tt.bean}, tt.dst), magicBeanVols(tt.bean))
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()

			useMagicBeanFrame(t, c, 1)

			expect(t, c, protocol.MsgMessageBoxOk)
			expect(t, c, protocol.MsgUpdateScore)
			item := expect(t, c, protocol.MsgSendItem)
			if item[6] != tt.want || item[7] != 8 {
				t.Fatalf("effect0 = %d.%d, want %d.8", item[6], item[7], tt.want)
			}
		})
	}
}

// TestUseMagicBeanBroadcastsVisual proves the #157 fix: applying or removing paint
// refreshes the cached EquipVisual/EquipAnct and pushes MSG_UpdateEquip, so the worn
// color survives a later teleport (enterWorldView/createMobFrom rebroadcast from that
// cache). Pre-fix useMagicBean sent no MSG_UpdateEquip, so this stream would go quiet.
func TestUseMagicBeanBroadcastsVisual(t *testing.T) {
	tests := []struct {
		name     string
		bean     int16
		dst      world.Item
		wantAnct uint8
	}{
		{
			name:     "paint sets the color overlay",
			bean:     itemMagicBeanBlue,
			dst:      world.Item{Index: itemArmor, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}},
			wantAnct: magicBeanPaintLo - 3, // VisualAnctCode returns ef-3 for a paint effect
		},
		{
			name:     "remover clears the overlay back to sanc",
			bean:     itemMagicBeanRemover,
			dst:      world.Item{Index: itemArmor, Effects: [3]world.Effect{{Effect: magicBeanPaintLo + 4, Value: 8}}},
			wantAnct: efSanc,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, stop := startServerClockVol(t, magicBeanDB(world.Item{Index: tt.bean}, tt.dst), magicBeanVols(tt.bean))
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()

			useMagicBeanFrame(t, c, 1)

			ue := expect(t, c, protocol.MsgUpdateEquip)
			if got := ue[33]; got != tt.wantAnct { // AnctCode[1] @ body32+1
				t.Fatalf("equip anct[1] = %#x, want %#x", got, tt.wantAnct)
			}
		})
	}
}

func TestUseMagicBeanRejectsMissingSanc(t *testing.T) {
	tests := []struct {
		name  string
		equip world.Item
	}{
		{name: "empty equip slot"},
		{name: "unrefined item", equip: world.Item{Index: itemArmor}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, stop := startServerClockVol(t, magicBeanDB(world.Item{Index: itemMagicBeanBlue}, tt.equip), magicBeanVols(itemMagicBeanBlue))
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()

			useMagicBeanFrame(t, c, 1)

			item := expect(t, c, protocol.MsgSendItem)
			if got := le16(item[0:2]); got != world.ItemPlaceCarry {
				t.Fatalf("reject send item place = %d, want carry", got)
			}
			if got := le16(item[4:6]); got != itemMagicBeanBlue {
				t.Fatalf("reject source item = %d, want magic bean", got)
			}
			if ty, _, ok := readMaybe(t, c); ok {
				t.Fatalf("missing-sanc magic bean use produced extra frame %#x", ty)
			}
		})
	}
}

func TestUseMagicBeanRejectsNonSetSlot(t *testing.T) {
	armor := world.Item{Index: itemArmor, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}}
	addr, stop := startServerClockVol(t, magicBeanDB(world.Item{Index: itemMagicBeanBlue}, armor), magicBeanVols(itemMagicBeanBlue))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useMagicBeanFrame(t, c, 0)

	if code := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); code != NoticeOnlyToEquips {
		t.Fatalf("notice = %d, want OnlyToEquips", code)
	}
	item := expect(t, c, protocol.MsgSendItem)
	if got := le16(item[0:2]); got != world.ItemPlaceCarry {
		t.Fatalf("reject send item place = %d, want carry", got)
	}
	if got := le16(item[4:6]); got != itemMagicBeanBlue {
		t.Fatalf("reject source item = %d, want magic bean", got)
	}
}

func TestUseMagicBeanStackPersistsOneConsumed(t *testing.T) {
	armor := world.Item{Index: itemArmor, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}}
	bean := world.Item{Index: itemMagicBeanBlue, Effects: [3]world.Effect{{Effect: efAmount, Value: 3}}}
	db := magicBeanDB(bean, armor)
	addr, stop := startServerClockVol(t, db, magicBeanVols(itemMagicBeanBlue))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useMagicBeanFrame(t, c, 1)
	expect(t, c, protocol.MsgSendItem)

	send(t, c, protocol.MsgCharacterLogout, nil)
	expect(t, c, protocol.MsgCNFCharacterLogout)

	save, n := db.lastSavedChar()
	if n == 0 {
		t.Fatal("character was not saved on logout")
	}
	carry0, ok := savedItemAt(save.Carry, 0)
	if !ok {
		t.Fatal("saved carry slot 0 is empty; want stacked magic bean")
	}
	if carry0.Index != itemMagicBeanBlue || carry0.Eff1 != efAmount || carry0.EffV1 != 2 {
		t.Fatalf("saved carry0 = %+v, want magic bean amount 2", carry0)
	}
	equip1, ok := savedItemAt(save.Equip, 1)
	if !ok {
		t.Fatal("saved equip slot 1 is empty; want painted armor")
	}
	if equip1.Eff1 != magicBeanPaintLo || equip1.EffV1 != 9 {
		t.Fatalf("saved equip1 effects = %+v, want paint %d value 9", equip1, magicBeanPaintLo)
	}
}

func TestMagicBeanEffectSlotScan(t *testing.T) {
	it := world.Item{Effects: [3]world.Effect{
		{Effect: efDamage, Value: 1},
		{Effect: efAc, Value: 2},
		{Effect: efHp, Value: 3},
	}}
	if got := magicBeanEffectSlot(it, false); got != -1 {
		t.Fatalf("paint slot = %d, want -1 for three real effects", got)
	}
	it.Effects[1] = world.Effect{Effect: efSanc, Value: 9}
	if got := magicBeanEffectSlot(it, false); got != 1 {
		t.Fatalf("paint slot = %d, want EF_SANC slot 1", got)
	}
	if got := magicBeanEffectSlot(it, true); got != -1 {
		t.Fatalf("remover slot = %d, want -1 when only EF_SANC is available", got)
	}
	it.Effects[2] = world.Effect{Effect: magicBeanPaintLo + 2, Value: 9}
	if got := magicBeanEffectSlot(it, true); got != 2 {
		t.Fatalf("remover slot = %d, want paint slot 2", got)
	}
}

func savedItemAt(items []world.SavedItem, slot int) (world.SavedItem, bool) {
	for _, it := range items {
		if it.Slot == slot {
			return it, true
		}
	}
	return world.SavedItem{}, false
}

// TestTradingItemCarryMove is the most basic case the user hit: drag an item from
// one inventory slot to an empty one via _MSG_TradingItem (0x0376). The item moves
// and both slots are refreshed (the empty source, the now-filled destination).
func TestTradingItemCarryMove(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100)) // item 1100 in carry slot 0
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceCarry, 3, 0)
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgTradingItem {
		t.Fatalf("move echo = %#x ok=%v, want TradingItem", ty, ok)
	}
	src := expect(t, c, protocol.MsgSendItem) // slot 0, now empty
	if le16(src[2:4]) != 0 || le16(src[4:6]) != 0 {
		t.Errorf("source slot = %d item %d, want slot 0 empty", le16(src[2:4]), le16(src[4:6]))
	}
	dst := expect(t, c, protocol.MsgSendItem) // slot 3, now holds the item
	if le16(dst[2:4]) != 3 || le16(dst[4:6]) != 1100 {
		t.Errorf("dest slot = %d item %d, want slot 3 item 1100", le16(dst[2:4]), le16(dst[4:6]))
	}
}

// TestEquipBonusDivines verifies the divine effect types beyond the basic SIDC set
// fold into the score: EF_SPECIAL1-4, EF_DAMAGEADD and EF_ACADD as instance effects.
func TestEquipBonusDivines(t *testing.T) {
	// Item 700 is a damage jewel (nUnique 45 ∈ [41,50]) so its EF_DAMAGEADD counts.
	d := New(Config{ItemUnique: map[int]int{700: 45}})
	e := &world.Entity{}
	// A ring with a divine special, a divine flat-damage and a divine flat-AC.
	e.Equip[0] = world.Item{Index: 700, Effects: [3]world.Effect{
		{Effect: efSpecial1, Value: 10},
		{Effect: efDamageAdd, Value: 25},
		{Effect: efAcAdd, Value: 7},
	}}
	b := d.equipBonus(e)
	if b.special[0] != 10 {
		t.Errorf("special[0] = %d, want 10", b.special[0])
	}
	if b.damage != 25 {
		t.Errorf("damage = %d, want 25 (EF_DAMAGEADD on a jewel)", b.damage)
	}
	if b.ac != 7 {
		t.Errorf("ac = %d, want 7 (EF_ACADD)", b.ac)
	}
}

func TestEquipBonusSpecialAll(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{}
	e.Equip[0] = world.Item{Index: 700, Effects: [3]world.Effect{
		{Effect: efSpecialAll, Value: 5},
	}}

	b := d.equipBonus(e)
	want := [4]int16{0, 5, 5, 5}
	if b.special != want {
		t.Errorf("special = %v, want %v", b.special, want)
	}
}

// TestEquipBonusDamageAddGate confirms EF_DAMAGEADD is ignored on a NON-jewel item
// (nUnique outside [41,50]) — only damage jewels contribute it (captura §B/E).
func TestEquipBonusDamageAddGate(t *testing.T) {
	d := New(Config{}) // no nUnique → not a jewel
	e := &world.Entity{}
	e.Equip[0] = world.Item{Index: 701, Effects: [3]world.Effect{{Effect: efDamageAdd, Value: 25}}}
	if got := d.equipBonus(e).damage; got != 0 {
		t.Errorf("damage = %d, want 0 (EF_DAMAGEADD only counts on jewels)", got)
	}
}

// TestEquipBonusHpAddPercent confirms EF_HPADD is a PERCENT (not flat): it accumulates
// into hpAddPct and effectiveMaxHP multiplies the base MaxHP by it (captura §E).
func TestEquipBonusHpAddPercent(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{BaseMaxHP: 1000}
	e.Equip[0] = world.Item{Index: 702, Effects: [3]world.Effect{{Effect: efHpAdd, Value: 10}}}
	d.refreshScore(e) // HPADD is percent → cached in HpAddPct, applied at read time
	if e.HpAddPct != 10 {
		t.Fatalf("HpAddPct = %d, want 10", e.HpAddPct)
	}
	if got := effectiveMaxHP(e); got != 1100 {
		t.Errorf("effectiveMaxHP = %d, want 1100 (1000 +10%%)", got)
	}
}

// TestCanEquipSlot verifies the nPos bitmask gate: an item fits a slot iff nPos has
// that slot's bit; consumables (nPos 0) fit nowhere; unknown items are allowed.
func TestCanEquipSlot(t *testing.T) {
	d := New(Config{ItemPos: map[int]int{
		3381: 0,     // Poção Divina: fits nowhere
		11:   1,     // body item: slot 0 (1<<0)
		861:  192,   // dual weapon: slots 6,7
		342:  16384, // mount: slot 14
	}})
	cases := []struct {
		idx  int16
		slot int
		want bool
	}{
		{3381, 0, false}, {11, 0, true}, {11, 1, false},
		{861, 6, true}, {861, 7, true}, {861, 0, false},
		{342, 14, true}, {342, 7, false},
		{0, 0, true}, {9999, 0, true}, // empty + unknown are allowed
	}
	for _, c := range cases {
		if got := d.canEquipSlot(c.idx, c.slot); got != c.want {
			t.Errorf("canEquipSlot(%d, %d) = %v, want %v", c.idx, c.slot, got, c.want)
		}
	}
}

// TestRepairEquip confirms a mis-equipped consumable (potion in the body slot) is
// moved back to the inventory and the valid gear is left in place.
func TestRepairEquip(t *testing.T) {
	d := New(Config{ItemPos: map[int]int{3381: 0, 1406: 4}}) // 1406 nPos 4 = slot 2
	st := world.CharacterState{Class: 1}
	st.Equip[0] = world.Item{Index: 3381} // potion wrongly in the body slot
	st.Equip[2] = world.Item{Index: 1406} // armor correctly in slot 2
	d.repairEquip(&st)
	if st.Equip[0].Index == 3381 {
		t.Error("potion still in the body slot after repair")
	}
	if st.Equip[2].Index != 1406 {
		t.Error("valid armor was wrongly relocated")
	}
	found := false
	for _, it := range st.Carry {
		if it.Index == 3381 {
			found = true
		}
	}
	if !found {
		t.Error("displaced potion was not preserved in the inventory")
	}
}

// TestDivineAffectBonus verifies the Poção Divina buff (Affect 34) adds +20% to the
// effective MaxHp/MaxMp/Damage at read time, and is the identity when absent (captura §C).
func TestDivineAffectBonus(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{ID: world.MaxUser + 1, BaseMaxHP: 1000, BaseMaxMP: 500, BaseDamage: 200}
	d.refreshScore(e)
	if got := effectiveMaxHP(e); got != 1000 {
		t.Fatalf("no-buff effectiveMaxHP = %d, want 1000", got)
	}
	if got := d.effectiveDamage(e); got != 200 {
		t.Fatalf("no-buff effectiveDamage = %d, want 200", got)
	}
	e.Affect[0] = world.Affect{Type: world.AffectDivine, Level: 1}
	if got := effectiveMaxHP(e); got != 1200 {
		t.Errorf("divine effectiveMaxHP = %d, want 1200 (+20%%)", got)
	}
	if got := effectiveMaxMP(e); got != 600 {
		t.Errorf("divine effectiveMaxMP = %d, want 600 (+20%%)", got)
	}
	if got := d.effectiveDamage(e); got != 240 {
		t.Errorf("divine effectiveDamage = %d, want 240 (+20%%)", got)
	}
}

// TestSendAffectDivineTicksNotRawSeconds is the issue #202 regression: a Poção
// Divina's displayed remaining time must reach the client as ticks-of-8s, like
// every other buff (affect1H=450/h, affect1D=10800/day) — sendAffect used to send
// the raw DivineEnd-now seconds difference instead, inflating the displayed
// duration 8x (and, once DivineEnd had passed, letting the raw 2e9 "infinite"
// sentinel leak through unconverted, showing a wildly garbled duration).
func TestSendAffectDivineTicksNotRawSeconds(t *testing.T) {
	db := newDB()
	const remainingSeconds = 10 * 86400 // a 10-day-remaining Divine buff
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Divine", X: 5, Y: 5, HP: 1000, MaxHP: 1000,
		DivineEnd: time.Now().Unix() + remainingSeconds,
	}
	addr, stop := startServerClockVol(t, db, nil)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	affect := expect(t, c, protocol.MsgSendAffect)
	if affect[0] != world.AffectDivine {
		t.Fatalf("slot 0 affect type = %d, want %d (Divine)", affect[0], world.AffectDivine)
	}
	ticks := int64(binary.LittleEndian.Uint32(affect[4:8]))
	if diff := ticks*8 - remainingSeconds; diff < -16 || diff > 16 {
		t.Errorf("divine affect Time = %d ticks (%ds of 8s-ticks), want ~%ds remaining (diff %ds) — "+
			"raw seconds or the infinite sentinel leaked onto the wire", ticks, ticks*8, remainingSeconds, diff)
	}
}

// TestVigorAffectBonus verifies Poção de Vigor (Affect 35) adds +10% MaxHp/MaxMp.
func TestVigorAffectBonus(t *testing.T) {
	e := &world.Entity{BaseMaxHP: 1000, BaseMaxMP: 500}
	d := New(Config{})
	d.refreshScore(e)
	e.Affect[0] = world.Affect{Type: world.AffectVigor, Level: 1}
	if got := effectiveMaxHP(e); got != 1100 {
		t.Errorf("vigor effectiveMaxHP = %d, want 1100 (+10%%)", got)
	}
	if got := effectiveMaxMP(e); got != 550 {
		t.Errorf("vigor effectiveMaxMP = %d, want 550 (+10%%)", got)
	}
}

// TestUseVigorConsumesOneFromStack is the issue #259 regression: useVigor used to
// wipe the whole carry slot, so drinking one potion out of a "Pack Poção de Vigor"
// destroyed the entire stack. The legacy decrements instead — every arm of the
// Vol-58 block ends with `if (amount > 1) BASE_SetItemAmount(item, amount - 1);
// else memset(item, 0, ...)` (_MSG_UseItem.cpp:2197/2218/2239/2260). The buff
// duration per sIndex is asserted alongside, since nothing covered it end to end.
func TestUseVigorConsumesOneFromStack(t *testing.T) {
	cases := []struct {
		name     string
		item     int16
		wantTime uint32
	}{
		{"1h", 3313, affect1H},
		{"7d", 3364, affect1D * 7},
		{"15d", 3365, affect1D * 15},
		{"30d", 3366, affect1D * 30},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := newDB()
			st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
			st.Carry[0] = world.Item{Index: tc.item, Effects: [3]world.Effect{{Effect: efAmount, Value: 5}}}
			db.loadResult = st
			addr, stop := startServerClockVol(t, db, map[int]int{int(tc.item): volVigor})
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()

			body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
			send(t, c, protocol.MsgUseItem, body.Encode())

			item := expect(t, c, protocol.MsgSendItem)
			if le16(item[4:6]) != uint16(tc.item) {
				t.Fatalf("carry slot 0 item = %d, want %d (whole pack consumed)", le16(item[4:6]), tc.item)
			}
			if item[6] != efAmount || item[7] != 4 {
				t.Errorf("amount effect = (%d,%d), want (%d,4)", item[6], item[7], efAmount)
			}
			expect(t, c, protocol.MsgUpdateScore)
			affect := expect(t, c, protocol.MsgSendAffect)
			if affect[0] != world.AffectVigor {
				t.Fatalf("slot 0 affect type = %d, want %d (Vigor)", affect[0], world.AffectVigor)
			}
			if got := binary.LittleEndian.Uint32(affect[4:8]); got != tc.wantTime {
				t.Errorf("affect time = %d, want %d", got, tc.wantTime)
			}
		})
	}
}

// TestUseVigorLastUnitClearsSlot is the other half of the legacy idiom: a lone
// potion (no EF_AMOUNT at all, so itemAmount reports 1) still empties the slot.
func TestUseVigorLastUnitClearsSlot(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: 3313}
	db.loadResult = st
	addr, stop := startServerClockVol(t, db, map[int]int{3313: volVigor})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	item := expect(t, c, protocol.MsgSendItem)
	if le16(item[2:4]) != 0 || le16(item[4:6]) != 0 {
		t.Errorf("slot/item = %d/%d, want slot 0 empty", le16(item[2:4]), le16(item[4:6]))
	}
}

// TestUseVigorThenMoveKeepsReducedAmount is the symptom the player actually sees:
// without a real server-side decrement the client shows the stack gone, and the
// next slot resync (a move, a shop buy, a relog) "revives" it — or, with the old
// wipe, the whole pack really was gone. Mirrors TestUseThenMoveKeepsReducedAmount.
func TestUseVigorThenMoveKeepsReducedAmount(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: 3313, Effects: [3]world.Effect{{Effect: efAmount, Value: 10}}}
	db.loadResult = st
	addr, stop := startServerClockVol(t, db, map[int]int{3313: volVigor})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())
	afterUse := expect(t, c, protocol.MsgSendItem)
	if le16(afterUse[4:6]) != 3313 || afterUse[6] != efAmount || afterUse[7] != 9 {
		t.Fatalf("after use: item %d amount-effect (%d,%d), want 3313 (%d,9)", le16(afterUse[4:6]), afterUse[6], afterUse[7], efAmount)
	}
	expect(t, c, protocol.MsgUpdateScore)
	expect(t, c, protocol.MsgSendAffect)

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceCarry, 1, 0)
	expect(t, c, protocol.MsgTradingItem) // move echo
	expect(t, c, protocol.MsgSendItem)    // source slot 0, now empty
	moved := expect(t, c, protocol.MsgSendItem)
	if le16(moved[2:4]) != 1 || le16(moved[4:6]) != 3313 {
		t.Fatalf("dest slot = %d item %d, want slot 1 item 3313", le16(moved[2:4]), le16(moved[4:6]))
	}
	if moved[6] != efAmount || moved[7] != 9 {
		t.Errorf("moved item amount-effect = (%d,%d), want (%d,9)", moved[6], moved[7], efAmount)
	}
}

// TestUseDivineConsumesOneFromStack is the issue #259 sibling: useDivine carried
// the same whole-slot wipe (its "stacking not modeled yet" note predated
// consumeOneItem). Legacy: _MSG_UseItem.cpp:2165. Item 3360 is the Panqueca (Vol 66).
func TestUseDivineConsumesOneFromStack(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: 3360, Effects: [3]world.Effect{{Effect: efAmount, Value: 5}}}
	db.loadResult = st
	addr, stop := startServerClockVol(t, db, map[int]int{3360: volDivine30})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	item := expect(t, c, protocol.MsgSendItem)
	if le16(item[4:6]) != 3360 {
		t.Fatalf("carry slot 0 item = %d, want 3360 (whole pack consumed)", le16(item[4:6]))
	}
	if item[6] != efAmount || item[7] != 4 {
		t.Errorf("amount effect = (%d,%d), want (%d,4)", item[6], item[7], efAmount)
	}
	expect(t, c, protocol.MsgUpdateScore)
	if affect := expect(t, c, protocol.MsgSendAffect); affect[0] != world.AffectDivine {
		t.Errorf("slot 0 affect type = %d, want %d (Divine)", affect[0], world.AffectDivine)
	}
}

// TestEquipBonusRefine verifies the refine (+9) THRESHOLD on a defense piece: a
// refined (sanc>=9) item whose nPos is a defense slot (4/8/128) gains a flat +25 AC on
// top of its catalog AC; below +9 there is no threshold bonus (captura §E).
func TestEquipBonusRefine(t *testing.T) {
	d := New(Config{
		ItemEffects: map[int][]content.BaseEffect{555: {{Eff: efAc, Val: 100}}}, // armor, base AC 100
		ItemPos:     map[int]int{555: 4},                                        // nPos 4 = defense
	})
	armor8 := world.Item{Index: 555, Effects: [3]world.Effect{{Effect: efSanc, Value: 8}}} // +8 (below threshold)
	armor9 := world.Item{Index: 555, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}} // +9 (threshold)

	e8 := &world.Entity{}
	e8.Equip[0] = armor8
	e9 := &world.Entity{}
	e9.Equip[0] = armor9

	if ac8 := d.equipBonus(e8).ac; ac8 != 100 {
		t.Errorf("+8 AC = %d, want 100 (no threshold below +9)", ac8)
	}
	if ac9 := d.equipBonus(e9).ac; ac9 != 125 {
		t.Errorf("+9 AC = %d, want 125 (100 + 25 refine threshold)", ac9)
	}
}

// TestWeaponDamageRefine verifies the refine (+9) threshold adds +40 to a weapon hand
// (nPos 64/192) at sanc>=9 (captura §E).
func TestWeaponDamageRefine(t *testing.T) {
	d := New(Config{
		ItemEffects: map[int][]content.BaseEffect{900: {{Eff: efDamage, Val: 100}}},
		ItemPos:     map[int]int{900: 64}, // weapon hand
	})
	e := &world.Entity{}
	e.Equip[6] = world.Item{Index: 900} // +0 single weapon
	if got := d.weaponDamage(e); got != 100 {
		t.Errorf("+0 weaponDamage = %d, want 100", got)
	}
	e.Equip[6] = world.Item{Index: 900, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}}
	if got := d.weaponDamage(e); got != 140 {
		t.Errorf("+9 weaponDamage = %d, want 140 (100 + 40 threshold)", got)
	}
}

// TestItemSancUnpacksPity is the issue #103 regression: the EF_SANC cValue packs
// the refine level together with the pity counter (level + 10*pity), so reading
// it raw reports a wildly wrong level. A +5 item that had failed two refines
// stores 25; the old raw read clamped that to 15 and handed the item the +9
// threshold bonus it never earned. Any item touched by the legacy server or
// restored from a DB dump can carry a packed value.
func TestItemSancUnpacksPity(t *testing.T) {
	cases := []struct {
		name string
		cVal uint8
		want int
	}{
		{"+5 no pity", 5, 5},
		{"+5 with pity 2", 25, 5},
		{"+0 with pity 1", 10, 0},
		{"+9 with max pity", 209, 9},
		{"+9 clean still reads +9", 9, 9},
		{"+11 packs as 234", 234, 11},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			it := world.Item{Index: 555, Effects: [3]world.Effect{{Effect: efSanc, Value: tt.cVal}}}
			if got := itemSanc(it); got != tt.want {
				t.Errorf("itemSanc(cValue %d) = %d, want %d", tt.cVal, got, tt.want)
			}
		})
	}
}

// The threshold bonus must key off the real level, not the packed byte: a +5
// item with pity must NOT be granted the +9 AC bonus.
func TestEquipBonusRefineIgnoresPity(t *testing.T) {
	d := New(Config{
		ItemEffects: map[int][]content.BaseEffect{555: {{Eff: efAc, Val: 100}}},
		ItemPos:     map[int]int{555: 4},
	})
	e := &world.Entity{}
	e.Equip[0] = world.Item{Index: 555, Effects: [3]world.Effect{{Effect: efSanc, Value: 25}}} // +5, pity 2
	if ac := d.equipBonus(e).ac; ac != 100 {
		t.Errorf("+5 (pity 2) AC = %d, want 100 — pity must not unlock the +9 threshold", ac)
	}
}

// TestAttackRunOfBoots verifies EF_RUNSPEED (boots) raises the move-speed (low)
// nibble of AttackRun to the issue #64 tier: bare=2, boots=4, mount=6.
func TestAttackRunOfBoots(t *testing.T) {
	d := New(Config{
		ItemEffects: map[int][]content.BaseEffect{
			321: {{Eff: efRunSpeed, Val: 1}},  // boots
			322: {{Eff: efRunSpeed, Val: 99}}, // overboosted boots still use the boots tier
		},
	})

	bare := &world.Entity{}
	d.refreshScore(bare)
	if got := attackRunOf(bare); got != baseAttackRun {
		t.Errorf("no boots: attackRunOf = %#x, want base %#x", got, baseAttackRun)
	}

	booted := &world.Entity{}
	booted.Equip[5] = world.Item{Index: 321} // boots occupy equip slot 5 (nPos 32)
	d.refreshScore(booted)
	if got := attackRunOf(booted); got != (baseAttackRun&0xF0)|bootMoveSpeed {
		t.Errorf("boots: attackRunOf = %#x, want %#x", got, (baseAttackRun&0xF0)|bootMoveSpeed)
	}

	overboosted := &world.Entity{}
	overboosted.Equip[5] = world.Item{Index: 322}
	d.refreshScore(overboosted)
	if got := attackRunOf(overboosted); got != (baseAttackRun&0xF0)|bootMoveSpeed {
		t.Errorf("overboosted boots: attackRunOf = %#x, want %#x", got, (baseAttackRun&0xF0)|bootMoveSpeed)
	}

	mounted := &world.Entity{}
	mounted.Equip[5] = world.Item{Index: 321}
	mounted.Equip[mountEquipSlot] = world.Item{Index: 342}
	d.refreshScore(mounted)
	if got := attackRunOf(mounted); got != (baseAttackRun&0xF0)|mountedMoveSpeed {
		t.Errorf("mounted with boots: attackRunOf = %#x, want %#x", got, (baseAttackRun&0xF0)|mountedMoveSpeed)
	}
}

// TestRefreshScoreSpecial confirms refreshScore folds a divine special into the live
// entity (and so into the score sent to the client), and that a clean
// deriveBaseScore→refreshScore round-trip reproduces the loaded score (no double count).
func TestRefreshScoreSpecial(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{Str: 80, AC: 120, MaxHP: 1000, HP: 1000}
	e.Equip[0] = world.Item{Index: 700, Effects: [3]world.Effect{
		{Effect: efSpecial2, Value: 15},
		{Effect: efStr, Value: 5},
	}}
	d.deriveBaseScore(e) // base = loaded current − equipment
	d.refreshScore(e)    // re-add equipment

	if e.Str != 80 {
		t.Errorf("Str = %d, want 80 (round-trip stable)", e.Str)
	}
	if want := playerBaseAC(e); e.AC != want {
		t.Errorf("AC = %d, want rebuilt base %d", e.AC, want)
	}
	if e.Special[1] != 15 {
		t.Errorf("Special[1] = %d, want 15", e.Special[1])
	}
}

func TestRefreshScoreSpecialAll(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{
		ID:          1,
		BaseSpecial: [4]int16{0, 10, 20, 30},
		Special:     [4]int16{0, 15, 25, 35},
		Str:         80, AC: 120, MaxHP: 1000, HP: 1000,
	}
	e.Equip[0] = world.Item{Index: 700, Effects: [3]world.Effect{
		{Effect: efSpecialAll, Value: 5},
	}}
	d.deriveBaseScore(e)
	d.refreshScore(e)
	d.refreshScore(e)

	wantBase := [4]int16{0, 10, 20, 30}
	want := [4]int16{0, 15, 25, 35}
	if e.BaseSpecial != wantBase {
		t.Errorf("BaseSpecial = %v, want %v", e.BaseSpecial, wantBase)
	}
	if e.Special != want {
		t.Errorf("Special = %v, want %v", e.Special, want)
	}
}

// TestTradingItemEmptyMove rejects a swap when both slots are empty (nothing to do).
func TestTradingItemEmptyMove(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 10, world.ItemPlaceCarry, 11, 0) // both empty
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("empty→empty move produced %#x; should be a no-op", ty)
	}
}

func expChestDB() *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: 4140}
	db.loadResult = st
	return db
}

func silverBarDB(itemIdx int16, coin int32) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Coin: coin}
	st.Carry[0] = world.Item{Index: itemIdx}
	db.loadResult = st
	return db
}

func fairyDustDB(levelValue int, item world.Item) *fakeDB {
	db := newDB()
	st := world.CharacterState{
		Slot: 0, Name: "Hero", Class: 1, Level: levelValue,
		X: 5, Y: 5, HP: 1000, MaxHP: 1000, MP: 100, MaxMP: 100,
	}
	st.Carry[0] = item
	db.loadResult = st
	return db
}

func startServerClockVol(t *testing.T, persist world.Persistence, vols map[int]int) (string, func()) {
	t.Helper()
	return startServerClockItems(t, persist, vols, nil)
}

func startServerClockItems(t *testing.T, persist world.Persistence, vols map[int]int, effects map[int][]content.BaseEffect) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, ItemVolatiles: vols, ItemEffects: effects, ExpEvents: level.ExpEvents{KefraLive: true}})
	w := world.New(world.Config{GridDim: 16}, log, persist, d.Handle)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	return ln.Addr().String(), func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}
}

// startServerTickItems is startServerClockItems with the simulation tick running, so
// regenPlayers/applyHp actually close the bars on their ReqHp target. Tests that
// assert an exact mid-flight HP should NOT use this: the passive trickle
// (naturalRegenTicks) also runs and drifts HP upward on its own.
func startServerTickItems(t *testing.T, persist world.Persistence, vols map[int]int, effects map[int][]content.BaseEffect) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, ItemVolatiles: vols, ItemEffects: effects, ExpEvents: level.ExpEvents{KefraLive: true}})
	w := world.New(world.Config{GridDim: 16}, log, persist, d.Handle)
	w.SetTickHandler(10*time.Millisecond, d.Tick)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	return ln.Addr().String(), func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}
}

func healPotionDB(item world.Item, hp, maxHP, mp, maxMP int32) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: hp, MaxHP: maxHP, MP: mp, MaxMP: maxMP}
	st.Carry[0] = item
	db.loadResult = st
	return db
}

func paintDB(bean, target world.Item, targetSlot int) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = bean
	st.Equip[targetSlot] = target
	db.loadResult = st
	return db
}

func sendPaintUse(t *testing.T, c net.Conn, destSlot int) {
	t.Helper()
	body := protocol.MsgUseItemBody{
		SourType: world.ItemPlaceCarry, SourPos: 0,
		DestType: world.ItemPlaceEquip, DestPos: int32(destSlot),
	}
	send(t, c, protocol.MsgUseItem, body.Encode())
}

func savedSlot(items []world.SavedItem, slot int) (world.SavedItem, bool) {
	for _, it := range items {
		if it.Slot == slot {
			return it, true
		}
	}
	return world.SavedItem{}, false
}

func TestUsePaintBeanAppliesColorOverWire(t *testing.T) {
	bean := world.Item{Index: 3408, Effects: [3]world.Effect{{Effect: efAmount, Value: 3}}}
	target := world.Item{Index: 555, Effects: [3]world.Effect{{Effect: efSanc, Value: 4}}}
	db := paintDB(bean, target, 1)
	addr, stop := startServerClockVol(t, db, map[int]int{3408: volPaint})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	sendPaintUse(t, c, 1)

	sawNotice, sawEquip, sawScore, sawTarget := false, false, false, false
	for range 8 {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			break
		}
		switch ty {
		case protocol.MsgMessageBoxOk:
			if noticeCode(t, payload) != NoticeRefineSuccess {
				t.Fatalf("notice = %v, want NoticeRefineSuccess", noticeCode(t, payload))
			}
			sawNotice = true
		case protocol.MsgUpdateEquip:
			sawEquip = true
			if got := payload[33]; got != 117-3 { // AnctCode[1] @body32+1
				t.Errorf("paint anct[1] = %#x, want %#x", got, 117-3)
			}
		case protocol.MsgUpdateScore:
			sawScore = true
		case protocol.MsgSendItem:
			place := int(binary.LittleEndian.Uint16(payload[0:2]))
			slot := int(binary.LittleEndian.Uint16(payload[2:4]))
			if place != world.ItemPlaceEquip || slot != 1 {
				continue
			}
			sawTarget = true
			if idx := le16(payload[4:6]); idx != 555 {
				t.Fatalf("paint target index = %d, want 555", idx)
			}
			if payload[6] != 117 || payload[7] != 4 {
				t.Fatalf("paint target effect0 = %d.%d, want 117.4", payload[6], payload[7])
			}
		}
	}
	if !sawNotice || !sawEquip || !sawScore || !sawTarget {
		t.Fatalf("paint responses notice=%v equip=%v score=%v target=%v, want all true", sawNotice, sawEquip, sawScore, sawTarget)
	}

	send(t, c, protocol.MsgCharacterLogout, nil)
	expect(t, c, protocol.MsgCNFCharacterLogout)
	save, n := db.lastSavedChar()
	if n == 0 {
		t.Fatal("character was not saved on logout")
	}
	carry0, ok := savedSlot(save.Carry, 0)
	if !ok || carry0.Index != 3408 || carry0.Eff1 != efAmount || carry0.EffV1 != 2 {
		t.Fatalf("saved carry slot 0 = %+v ok=%v, want stacked paint bean amount 2", carry0, ok)
	}
	equip1, ok := savedSlot(save.Equip, 1)
	if !ok || equip1.Index != 555 || equip1.Eff1 != 117 || equip1.EffV1 != 4 {
		t.Fatalf("saved equip slot 1 = %+v ok=%v, want painted item 117.4", equip1, ok)
	}
}

func TestUsePaintRemoverRestoresSancCarrier(t *testing.T) {
	target := world.Item{Index: 555, Effects: [3]world.Effect{{Effect: 119, Value: 6}}}
	addr, stop := startServerClockVol(t, paintDB(world.Item{Index: 3417}, target, 1), map[int]int{3417: volPaint})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	sendPaintUse(t, c, 1)

	for range 8 {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			break
		}
		if ty != protocol.MsgSendItem {
			continue
		}
		place := int(binary.LittleEndian.Uint16(payload[0:2]))
		slot := int(binary.LittleEndian.Uint16(payload[2:4]))
		if place != world.ItemPlaceEquip || slot != 1 {
			continue
		}
		if payload[6] != efSanc || payload[7] != 6 {
			t.Fatalf("removed paint effect0 = %d.%d, want EF_SANC.6", payload[6], payload[7])
		}
		return
	}
	t.Fatal("missing target SendItem after paint remover")
}

func TestUsePaintBeanRejectsUnrefinedTarget(t *testing.T) {
	addr, stop := startServerClockVol(t, paintDB(world.Item{Index: 3408}, world.Item{Index: 555}, 1), map[int]int{3408: volPaint})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	sendPaintUse(t, c, 1)

	ty, payload, ok := readMaybe(t, c)
	if !ok || ty != protocol.MsgSendItem {
		t.Fatalf("unrefined paint response = %#x ok=%v, want source SendItem", ty, ok)
	}
	if place, slot, idx := le16(payload[0:2]), le16(payload[2:4]), le16(payload[4:6]); place != world.ItemPlaceCarry || slot != 0 || idx != 3408 {
		t.Fatalf("source resend place/slot/index = %d/%d/%d, want carry/0/3408", place, slot, idx)
	}
	if ty, _, ok := readMaybe(t, c); ok {
		t.Fatalf("unrefined paint produced extra frame %#x", ty)
	}
}

func TestUsePaintBeanRejectsForbiddenEquipSlot(t *testing.T) {
	target := world.Item{Index: 555, Effects: [3]world.Effect{{Effect: efSanc, Value: 1}}}
	addr, stop := startServerClockVol(t, paintDB(world.Item{Index: 3408}, target, 8), map[int]int{3408: volPaint})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	sendPaintUse(t, c, 8)

	if ty, payload, ok := readMaybe(t, c); !ok || ty != protocol.MsgMessageBoxOk || noticeCode(t, payload) != NoticeOnlyToEquips {
		t.Fatalf("forbidden slot notice = %#x/%v ok=%v, want NoticeOnlyToEquips", ty, noticeCode(t, payload), ok)
	}
	item := expect(t, c, protocol.MsgSendItem)
	if place, slot, idx := le16(item[0:2]), le16(item[2:4]), le16(item[4:6]); place != world.ItemPlaceCarry || slot != 0 || idx != 3408 {
		t.Fatalf("source resend place/slot/index = %d/%d/%d, want carry/0/3408", place, slot, idx)
	}
}

func TestUsePaintRemoverRejectsWithoutEmptyOrPaintSlot(t *testing.T) {
	target := world.Item{Index: 555, Effects: [3]world.Effect{
		{Effect: efSanc, Value: 1},
		{Effect: efSanc, Value: 2},
		{Effect: efSanc, Value: 3},
	}}
	addr, stop := startServerClockVol(t, paintDB(world.Item{Index: 3417}, target, 1), map[int]int{3417: volPaint})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	sendPaintUse(t, c, 1)

	if ty, payload, ok := readMaybe(t, c); !ok || ty != protocol.MsgMessageBoxOk || noticeCode(t, payload) != NoticeCantRefineMore {
		t.Fatalf("remover reject notice = %#x/%v ok=%v, want NoticeCantRefineMore", ty, noticeCode(t, payload), ok)
	}
	item := expect(t, c, protocol.MsgSendItem)
	if place, slot, idx := le16(item[0:2]), le16(item[2:4]), le16(item[4:6]); place != world.ItemPlaceCarry || slot != 0 || idx != 3417 {
		t.Fatalf("source resend place/slot/index = %d/%d/%d, want carry/0/3417", place, slot, idx)
	}
}

func TestUseExpChest(t *testing.T) {
	addr, stop := startServerClockVol(t, expChestDB(), map[int]int{4140: volExpChest})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	sawSendItem, sawAffect, sawScore := false, false, false
	for range 6 {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			break
		}
		switch ty {
		case protocol.MsgSendItem:
			sawSendItem = true
			if le16(payload[4:6]) != 0 {
				t.Errorf("carry slot 0 item = %d, want empty after use", le16(payload[4:6]))
			}
		case protocol.MsgSendAffect:
			sawAffect = true
			if payload[0] != world.AffectExpChest {
				t.Errorf("affect type = %d, want %d", payload[0], world.AffectExpChest)
			}
			if got := binary.LittleEndian.Uint32(payload[4:8]); got != affectExpChestInc {
				t.Errorf("affect time = %d, want %d", got, affectExpChestInc)
			}
		case protocol.MsgUpdateScore:
			sawScore = true
		}
	}
	if !sawSendItem {
		t.Error("missing MsgSendItem after using exp chest")
	}
	if !sawAffect {
		t.Error("missing MsgSendAffect after using exp chest")
	}
	if !sawScore {
		t.Error("missing MsgUpdateScore after using exp chest")
	}
}

func TestUseExpChestStack(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: 4140, Effects: [3]world.Effect{{Effect: efAmount, Value: 3}}}
	db.loadResult = st

	addr, stop := startServerClockVol(t, db, map[int]int{4140: volExpChest})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	payload := expect(t, c, protocol.MsgSendItem)
	if got := le16(payload[4:6]); got != 4140 {
		t.Fatalf("carry slot 0 item = %d, want stacked chest", got)
	}
	if payload[6] != efAmount || payload[7] != 2 {
		t.Errorf("effect0 = %d.%d, want %d.2", payload[6], payload[7], efAmount)
	}
}

// TestUseFrangoAssado is the issue #134 regression: using the item did nothing
// because Vol 63 had no case in the useItem switch. It should consume the item
// and grant Affect 30 (AffectForceMobDamage) for 4h (_MSG_UseItem.cpp:2308-2341).
func TestUseFrangoAssado(t *testing.T) {
	addr, stop := startServerClockVol(t, expChestDB(), map[int]int{4140: volFrango})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	sawSendItem, sawAffect, sawScore := false, false, false
	for range 6 {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			break
		}
		switch ty {
		case protocol.MsgSendItem:
			sawSendItem = true
			if le16(payload[4:6]) != 0 {
				t.Errorf("carry slot 0 item = %d, want empty after use", le16(payload[4:6]))
			}
		case protocol.MsgSendAffect:
			sawAffect = true
			if payload[0] != world.AffectForceMobDamage {
				t.Errorf("affect type = %d, want %d", payload[0], world.AffectForceMobDamage)
			}
			if got := binary.LittleEndian.Uint32(payload[4:8]); got != affect1H*4 {
				t.Errorf("affect time = %d, want %d", got, affect1H*4)
			}
		case protocol.MsgUpdateScore:
			sawScore = true
		}
	}
	if !sawSendItem {
		t.Error("missing MsgSendItem after using frango assado")
	}
	if !sawAffect {
		t.Error("missing MsgSendAffect after using frango assado")
	}
	if !sawScore {
		t.Error("missing MsgUpdateScore after using frango assado")
	}
}

func TestUseLegacyBuffConsumables(t *testing.T) {
	cases := []struct {
		name      string
		item      int16
		vol       int
		wantValue uint8
		wantTime  uint32
	}{
		{"kappa", 787, volBuffKappa, legacyBuffKappa, 80},
		{"kappa 30m", 3310, volBuffKappa30, legacyBuffKappa, affect1H / 2},
		{"kappa 20h", 3319, volBuffKappa20h, legacyBuffKappa, affect1H * 20},
		{"combat", 1764, volBuffCombat, legacyBuffCombat, 80},
		{"combat 60m", 3311, volBuffCombat60, legacyBuffCombat, affect1H},
		{"combat 20h", 3320, volBuffCombat20h, legacyBuffCombat, affect1H * 20},
		{"mental", 1765, volBuffMental, legacyBuffMental, 80},
		{"mental 60m", 3312, volBuffMental60, legacyBuffMental, affect1H},
		{"mental 20h", 3321, volBuffMental20h, legacyBuffMental, affect1H * 20},
		{"sephira 7d", 3361, volBuffMental20h, legacyBuffSephira, affect1H * 168},
		{"sephira 15d", 3362, volBuffMental20h, legacyBuffSephira, affect1H * 360},
		{"sephira 30d", 3363, volBuffMental20h, legacyBuffSephira, affect1H * 720},
		{"antidote", 416, volBuffKappa, legacyBuffAntidote, affect1H / 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := newDB()
			st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
			st.Carry[0] = world.Item{Index: tc.item, Effects: [3]world.Effect{{Effect: efAmount, Value: 5}}}
			db.loadResult = st
			addr, stop := startServerClockVol(t, db, map[int]int{int(tc.item): tc.vol})
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()

			body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
			send(t, c, protocol.MsgUseItem, body.Encode())

			item := expect(t, c, protocol.MsgSendItem)
			if le16(item[4:6]) != uint16(tc.item) || item[6] != efAmount || item[7] != 4 {
				t.Fatalf("after use item = %d effects (%d,%d), want %d (%d,4)", le16(item[4:6]), item[6], item[7], tc.item, efAmount)
			}
			expect(t, c, protocol.MsgUpdateScore)
			affect := expect(t, c, protocol.MsgSendAffect)
			if affect[0] != affectLegacyConsumableBuff || affect[1] != tc.wantValue {
				t.Errorf("affect = type %d value %d, want type %d value %d", affect[0], affect[1], affectLegacyConsumableBuff, tc.wantValue)
			}
			if got := binary.LittleEndian.Uint32(affect[4:8]); got != tc.wantTime {
				t.Errorf("affect time = %d, want %d", got, tc.wantTime)
			}
		})
	}
}

func TestUseLegacyBuffConsumableFullAffectsDoesNotConsume(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: 3310, Effects: [3]world.Effect{{Effect: efAmount, Value: 5}}}
	st.Affects = make([]world.Affect, world.MaxAffect)
	for i := range st.Affects {
		st.Affects[i] = world.Affect{Type: uint8(50 + i), Time: 100}
	}
	db.loadResult = st
	addr, stop := startServerClockVol(t, db, map[int]int{3310: volBuffKappa30})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()
	for {
		if _, _, ok := readMaybe(t, c); !ok {
			break
		}
	}

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	notice := expect(t, c, protocol.MsgMessageBoxOk)
	if noticeCode(t, notice) != NoticeCantEatMore {
		t.Errorf("notice = %d, want NoticeCantEatMore", noticeCode(t, notice))
	}
	item := expect(t, c, protocol.MsgSendItem)
	if le16(item[4:6]) != 3310 || item[6] != efAmount || item[7] != 5 {
		t.Errorf("rejected item = %d effects (%d,%d), want 3310 (%d,5)", le16(item[4:6]), item[6], item[7], efAmount)
	}
}

func TestUseUnknownVolatileRejectedAndResynced(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: 496, Effects: [3]world.Effect{{Effect: efAmount, Value: 5}}}
	db.loadResult = st
	addr, stop := startServerClockVol(t, db, map[int]int{496: 60})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	notice := expect(t, c, protocol.MsgMessageBoxOk)
	if noticeCode(t, notice) != NoticeCantUseHere {
		t.Errorf("notice = %d, want NoticeCantUseHere", noticeCode(t, notice))
	}
	item := expect(t, c, protocol.MsgSendItem)
	if le16(item[4:6]) != 496 || item[6] != efAmount || item[7] != 5 {
		t.Errorf("resynced item = %d effects (%d,%d), want 496 (%d,5)", le16(item[4:6]), item[6], item[7], efAmount)
	}
}

// TestUseCoracaoDoce is the issue #135 regression for the Vol-205 consumable:
// it grants Velocidade (Affect 2) + Defesa (Affect 11) and actually decrements
// the stack (previously a no-op, since Vol 205 had no useItem case).
func TestUseCoracaoDoce(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: 4145, Effects: [3]world.Effect{{Effect: efAmount, Value: 10}}}
	db.loadResult = st
	addr, stop := startServerClockVol(t, db, map[int]int{4145: volCoracaoDoce})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	item := expect(t, c, protocol.MsgSendItem)
	if le16(item[4:6]) != 4145 {
		t.Fatalf("carry slot 0 item = %d, want 4145 (stack not emptied)", le16(item[4:6]))
	}
	expect(t, c, protocol.MsgUpdateScore)
	affect := expect(t, c, protocol.MsgSendAffect)
	if affect[0] != 2 || affect[1] != 2 {
		t.Errorf("slot 0 affect = type %d value %d, want type 2 value 2 (Velocidade)", affect[0], affect[1])
	}
	if got := binary.LittleEndian.Uint32(affect[4:8]); got != affect1H/5 {
		t.Errorf("slot 0 affect time = %d, want %d", got, affect1H/5)
	}
	if affect[8] != 11 {
		t.Errorf("slot 1 affect type = %d, want 11 (Defesa)", affect[8])
	}
	if got := binary.LittleEndian.Uint32(affect[12:16]); got != affect1H/5 {
		t.Errorf("slot 1 affect time = %d, want %d", got, affect1H/5)
	}
}

// TestUseChocolateDoAmor is the issue #135 regression for the Vol-204 consumable:
// Dano (Affect 9) + Skill (Affect 15, Value 55).
func TestUseChocolateDoAmor(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: 1739, Effects: [3]world.Effect{{Effect: efAmount, Value: 10}}}
	db.loadResult = st
	addr, stop := startServerClockVol(t, db, map[int]int{1739: volChocolate})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	expect(t, c, protocol.MsgSendItem)
	expect(t, c, protocol.MsgUpdateScore)
	affect := expect(t, c, protocol.MsgSendAffect)
	if affect[0] != 9 {
		t.Errorf("slot 0 affect type = %d, want 9 (Dano)", affect[0])
	}
	if affect[8] != 15 || affect[9] != 55 {
		t.Errorf("slot 1 affect = type %d value %d, want type 15 value 55 (Skill)", affect[8], affect[9])
	}
}

// TestUseThenMoveKeepsReducedAmount is the end-to-end issue #135 regression: using
// a partial stack must leave the reduced Amount live in the server's own slot data,
// so a later _MSG_TradingItem move (which just resyncs the true slot state) does not
// appear to "revert" the consumption — the bug this whole fix addresses.
func TestUseThenMoveKeepsReducedAmount(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: 4145, Effects: [3]world.Effect{{Effect: efAmount, Value: 10}}}
	db.loadResult = st
	addr, stop := startServerClockVol(t, db, map[int]int{4145: volCoracaoDoce})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())
	afterUse := expect(t, c, protocol.MsgSendItem)
	if le16(afterUse[4:6]) != 4145 || afterUse[6] != efAmount || afterUse[7] != 9 {
		t.Fatalf("after use: item %d amount-effect (%d,%d), want 4145 (61,9)", le16(afterUse[4:6]), afterUse[6], afterUse[7])
	}
	expect(t, c, protocol.MsgUpdateScore)
	expect(t, c, protocol.MsgSendAffect)

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceCarry, 1, 0)
	expect(t, c, protocol.MsgTradingItem) // move echo
	expect(t, c, protocol.MsgSendItem)    // source slot 0, now empty
	moved := expect(t, c, protocol.MsgSendItem)
	if le16(moved[2:4]) != 1 || le16(moved[4:6]) != 4145 {
		t.Fatalf("dest slot = %d item %d, want slot 1 item 4145", le16(moved[2:4]), le16(moved[4:6]))
	}
	if moved[6] != efAmount || moved[7] != 9 {
		t.Errorf("moved item amount-effect = (%d,%d), want (61,9) — quantity reverted on move (issue #135)", moved[6], moved[7])
	}
}

// startServerAdamantita starts a server whose Dispatcher carries the catalog
// maps useAdamantita needs (ItemUnique/ItemGrades/ItemExtra), which the plain
// startServerClockVol harness doesn't expose.
func startServerAdamantita(t *testing.T, persist world.Persistence, vols, unique, grades, extra map[int]int) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{
		Log: log, ItemVolatiles: vols, ItemUnique: unique, ItemGrades: grades, ItemExtra: extra,
		ExpEvents: level.ExpEvents{KefraLive: true},
	})
	w := world.New(world.Config{GridDim: 16}, log, persist, d.Handle)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	return ln.Addr().String(), func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}
}

// TestUseAdamantitaWrongTier rejects an Adamantita family item combined onto a
// target whose nUnique doesn't match the source's tier: no consumption, the
// dust-refine-style reject re-syncs the SOURCE slot (refineReject), and the
// target is left untouched (no resync at all, matching the legacy's SendItem-only-
// on-the-source behavior for this guard).
func TestUseAdamantitaWrongTier(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: 578} // Adamantita, Type = 578-575 = 3
	st.Carry[1] = world.Item{Index: 900} // target: nUnique buckets to Type 0, not 3
	db.loadResult = st
	addr, stop := startServerAdamantita(t, db,
		map[int]int{578: volAdamantita},
		map[int]int{900: 5}, // bucket 0
		map[int]int{900: 2},
		nil,
	)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0, DestType: world.ItemPlaceCarry, DestPos: 1}
	send(t, c, protocol.MsgUseItem, body.Encode())

	notice := expect(t, c, protocol.MsgMessageBoxOk)
	if noticeCode(t, notice) != NoticeCantRefineMore {
		t.Errorf("notice = %d, want NoticeCantRefineMore", noticeCode(t, notice))
	}
	src := expect(t, c, protocol.MsgSendItem)
	if le16(src[2:4]) != 0 || le16(src[4:6]) != 578 {
		t.Errorf("source resync = slot %d item %d, want slot 0 item 578 (unconsumed)", le16(src[2:4]), le16(src[4:6]))
	}
}

// TestUseAdamantitaMatchingTierConsumes rolls an Adamantita combine on a matching,
// gradeable target: regardless of the roll outcome, the target slot is resynced
// and the source Adamantita is spent (issue #135 — this path used to be a
// complete no-op since Vol 9 had no useItem case at all).
func TestUseAdamantitaMatchingTierConsumes(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: 578} // Adamantita, Type = 3
	st.Carry[1] = world.Item{Index: 900} // target: nUnique bucket 3, grade 2
	db.loadResult = st
	addr, stop := startServerAdamantita(t, db,
		map[int]int{578: volAdamantita},
		map[int]int{900: 8}, // bucket 3
		map[int]int{900: 2},
		map[int]int{900: 950}, // Extra result index on success
	)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0, DestType: world.ItemPlaceCarry, DestPos: 1}
	send(t, c, protocol.MsgUseItem, body.Encode())

	notice := expect(t, c, protocol.MsgMessageBoxOk)
	if code := noticeCode(t, notice); code != NoticeRefineSuccess && code != NoticeFailToRefine {
		t.Errorf("notice = %d, want NoticeRefineSuccess or NoticeFailToRefine", code)
	}
	dst := expect(t, c, protocol.MsgSendItem)
	if le16(dst[2:4]) != 1 {
		t.Fatalf("resynced slot = %d, want 1 (the target)", le16(dst[2:4]))
	}
	if got := le16(dst[4:6]); got != 900 && got != 950 {
		t.Errorf("target item = %d, want 900 (fail, unchanged) or 950 (success, Extra)", got)
	}
	// No further MsgSendItem for the source slot (client already predicted the
	// drag removal — matches refineSucceed/refineFail's convention).
	if ty, p, ok := readMaybe(t, c); ok && ty == protocol.MsgSendItem {
		t.Errorf("unexpected second MsgSendItem for source slot: %v", p)
	}
}

// TestUseSeloDoGuerreiro is the issue #135 regression for sIndex 4146 (no
// EF_VOLATILE, so it must not fall into the vol==0 equip path): grants an
// entry Clan-7 amulet to a level-354+ mortal with no amulet equipped, and
// actually consumes the seal.
func TestUseSeloDoGuerreiro(t *testing.T) {
	db := newDB()
	st := world.CharacterState{
		Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000,
		Level: 354, ClassMaster: classMasterMortal, Clan: 7,
	}
	st.Carry[0] = world.Item{Index: itemSeloDoGuerreiro}
	db.loadResult = st
	addr, stop := startServerClockVol(t, db, nil)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	amulet := expect(t, c, protocol.MsgSendItem)
	if le16(amulet[2:4]) != amuletSlot || le16(amulet[4:6]) != 3191 {
		t.Fatalf("equip slot %d item %d, want slot %d item 3191 (Elite de Hekalotia)", le16(amulet[2:4]), le16(amulet[4:6]), amuletSlot)
	}
	expect(t, c, protocol.MsgUpdateScore)
	src := expect(t, c, protocol.MsgSendItem)
	if le16(src[4:6]) != 0 {
		t.Errorf("carry slot 0 item = %d, want empty after use", le16(src[4:6]))
	}
}

// TestUseWaterScrollRejected is the issue #135 "safe fallback" for the 3 blocked
// items: the real behavior needs data absent from Source/, so it must reject
// cleanly (NoticeCantUseHere + resync) instead of no-op'ing — a no-op would
// recreate the exact "phantom consumption reverts on move" bug this fix closes.
func TestUseWaterScrollRejected(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: 3182, Effects: [3]world.Effect{{Effect: efAmount, Value: 5}}} // Água (A) LV1
	db.loadResult = st
	addr, stop := startServerClockVol(t, db, map[int]int{3182: 161})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	notice := expect(t, c, protocol.MsgMessageBoxOk)
	if noticeCode(t, notice) != NoticeCantUseHere {
		t.Errorf("notice = %d, want NoticeCantUseHere", noticeCode(t, notice))
	}
	item := expect(t, c, protocol.MsgSendItem)
	if le16(item[4:6]) != 3182 || item[7] != 5 {
		t.Errorf("resynced item = index %d amount %d, want 3182 amount 5 (unconsumed)", le16(item[4:6]), item[7])
	}
}

// setHpMpFields decodes MSG_SetHpMp (protocol/score.go:110): Hp, Mp, ReqHp, ReqMp.
func setHpMpFields(t *testing.T, p []byte) (hp, mp, reqHp, reqMp int32) {
	t.Helper()
	if len(p) < 16 {
		t.Fatalf("SetHpMp payload too short: %d", len(p))
	}
	return int32(binary.LittleEndian.Uint32(p[0:])), int32(binary.LittleEndian.Uint32(p[4:])),
		int32(binary.LittleEndian.Uint32(p[8:])), int32(binary.LittleEndian.Uint32(p[12:]))
}

// The potion tests below run WITHOUT a tick handler on purpose: the potion only
// raises the ReqHp/ReqMp request target, which is fully deterministic, while the
// live bar is closed on it later by the tick (applyHp). The ramp itself is covered
// by TestUseHealPotionRampsOverTicks, where a tick runs.

// TestUseHealPotion: the potion stages the heal as a ReqHp target and leaves HP
// untouched — the original never writes CurrentScore.Hp (_MSG_UseItem.cpp:117-126).
func TestUseHealPotion(t *testing.T) {
	const potion = 400
	db := healPotionDB(world.Item{Index: potion}, 500, 1000, 0, 0)
	effects := map[int][]content.BaseEffect{potion: {{Eff: efHp, Val: 50}}}
	addr, stop := startServerClockItems(t, db, map[int]int{potion: volHpMpPotion}, effects)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	itemPayload := expect(t, c, protocol.MsgSendItem)
	if got := le16(itemPayload[4:6]); got != 0 {
		t.Errorf("carry slot 0 item = %d, want empty after use", got)
	}
	hp, _, reqHp, _ := setHpMpFields(t, expect(t, c, protocol.MsgSetHpMp))
	if reqHp != 550 {
		t.Errorf("ReqHp = %d, want 550 (500 + 50)", reqHp)
	}
	if hp != 500 {
		t.Errorf("Hp = %d, want 500 — the potion stages a target, the tick heals", hp)
	}
}

func TestUseHealPotionClampsToMax(t *testing.T) {
	const potion = 404
	db := healPotionDB(world.Item{Index: potion}, 950, 1000, 0, 0)
	effects := map[int][]content.BaseEffect{potion: {{Eff: efHp, Val: 500}}}
	addr, stop := startServerClockItems(t, db, map[int]int{potion: volHpMpPotion}, effects)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	expect(t, c, protocol.MsgSendItem)
	if _, _, reqHp, _ := setHpMpFields(t, expect(t, c, protocol.MsgSetHpMp)); reqHp != 1000 {
		t.Errorf("ReqHp = %d, want clamped to 1000", reqHp)
	}
}

// TestUseHealPotionRampsOverTicks is the gradual-heal behavior end to end: the bar
// closes on the potion's target across ticks (applyHp, <=2000/call) rather than
// snapping at use time.
func TestUseHealPotionRampsOverTicks(t *testing.T) {
	const potion = 406
	db := healPotionDB(world.Item{Index: potion}, 500, 100000, 0, 0)
	effects := map[int][]content.BaseEffect{potion: {{Eff: efHp, Val: 1000}}}
	addr, stop := startServerTickItems(t, db, map[int]int{potion: volHpMpPotion}, effects)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())
	expect(t, c, protocol.MsgSendItem)

	// The tick's sendScore reports the healed bar. MaxHP is huge so the passive
	// trickle (Level+30 per 10 ticks) can't reach 1500 within the window on its own.
	p := expect(t, c, protocol.MsgUpdateScore)
	if got := int32(binary.LittleEndian.Uint32(p[24:28])); got < 1500 {
		t.Errorf("Hp = %d, want >= 1500 (500 + the 1000 potion) once the tick applies it", got)
	}
}

// TestUseHealPotionCooldown pins the PotionDelay anti-spam gate
// (_MSG_UseItem.cpp:105-115): a second use inside the window is refused, the slot is
// re-synced, and — crucially — the stack is NOT consumed twice.
func TestUseHealPotionCooldown(t *testing.T) {
	const potion = 407
	item := world.Item{Index: potion, Effects: [3]world.Effect{{Effect: efAmount, Value: 5}}}
	db := healPotionDB(item, 500, 100000, 0, 0)
	effects := map[int][]content.BaseEffect{potion: {{Eff: efHp, Val: 50}}}
	addr, stop := startServerClockItems(t, db, map[int]int{potion: volHpMpPotion}, effects)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())
	if p := expect(t, c, protocol.MsgSendItem); p[7] != 4 {
		t.Fatalf("first use: stack = %d, want 4 (one consumed)", p[7])
	}
	_, _, reqHp, _ := setHpMpFields(t, expect(t, c, protocol.MsgSetHpMp))

	// Immediately again — the injected clock does not advance, so this is inside the
	// 100ms window and must be refused.
	send(t, c, protocol.MsgUseItem, body.Encode())
	p := expect(t, c, protocol.MsgSendItem)
	if p[7] != 4 {
		t.Errorf("second use: stack = %d, want 4 — a refused potion must not be consumed", p[7])
	}
	if ty, _, ok := readMaybe(t, c); ok && ty == protocol.MsgSetHpMp {
		t.Error("refused potion still sent SetHpMp, want the use dropped")
	}
	if reqHp != 550 {
		t.Errorf("ReqHp = %d, want 550 — only the first potion should count", reqHp)
	}
}

func TestUseManaPotion(t *testing.T) {
	const potion = 405
	db := healPotionDB(world.Item{Index: potion}, 1000, 1000, 50, 200)
	effects := map[int][]content.BaseEffect{potion: {{Eff: efMp, Val: 50}}}
	addr, stop := startServerClockItems(t, db, map[int]int{potion: volHpMpPotion}, effects)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	expect(t, c, protocol.MsgSendItem)
	if _, _, _, reqMp := setHpMpFields(t, expect(t, c, protocol.MsgSetHpMp)); reqMp != 100 {
		t.Errorf("ReqMp = %d, want 100 (50 + 50)", reqMp)
	}
}

func TestUseHealPotionStack(t *testing.T) {
	const potion = 428
	item := world.Item{Index: potion, Effects: [3]world.Effect{{Effect: efAmount, Value: 5}}}
	db := healPotionDB(item, 500, 1000, 0, 0)
	effects := map[int][]content.BaseEffect{potion: {{Eff: efHp, Val: 200}}}
	addr, stop := startServerClockItems(t, db, map[int]int{potion: volHpMpPotion}, effects)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	itemPayload := expect(t, c, protocol.MsgSendItem)
	if got := le16(itemPayload[4:6]); got != potion {
		t.Fatalf("carry slot 0 item = %d, want stacked potion to remain", got)
	}
	if itemPayload[6] != efAmount || itemPayload[7] != 4 {
		t.Errorf("effect0 = %d.%d, want %d.4", itemPayload[6], itemPayload[7], efAmount)
	}
	if _, _, reqHp, _ := setHpMpFields(t, expect(t, c, protocol.MsgSetHpMp)); reqHp != 700 {
		t.Errorf("ReqHp = %d, want 700 (500 + 200)", reqHp)
	}
}

// TestUseHealPotionBroadcastsToViewers is the issue-#99 regression: when a player
// pots, everyone in view must learn the new HP. The client tracks another entity's
// HP by SUBTRACTING the STRUCT_DAM deltas in MSG_Attack from its own local copy, so
// an out-of-band heal is invisible to an attacker unless the server pushes an
// explicit score — which the legacy does by GridMulticasting SendScore
// (SendFunc.cpp:1298). The bug was that our sendScore only replied to the drinker.
func TestUseHealPotionBroadcastsToViewers(t *testing.T) {
	const potion = 408
	// loadResult is the fallback for ANY account (handler_test.go:172), so both
	// clients spawn at (5,5) — Chebyshev 0, well inside ViewRange 16.
	db := healPotionDB(world.Item{Index: potion}, 500, 100000, 0, 0)
	effects := map[int][]content.BaseEffect{potion: {{Eff: efHp, Val: 1000}}}
	addr, stop := startServerTickItems(t, db, map[int]int{potion: volHpMpPotion}, effects)
	defer stop()

	drinker := enterWorld(t, addr) // conn 1
	defer drinker.Close()
	observer := enterWorldAs(t, addr, "tradeb") // conn 2, in view of conn 1
	defer observer.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, drinker, protocol.MsgUseItem, body.Encode())

	// The observer must receive the DRINKER's score: HEADER.ID identifies the subject,
	// so it must be conn 1, not the observer's own conn.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		h, p := readFrameHeader(t, observer)
		if h.Type != protocol.MsgUpdateScore || h.ID != 1 {
			continue
		}
		hp := int32(binary.LittleEndian.Uint32(p[24:28]))
		if hp < 1500 {
			continue // the passive trickle can emit a score before the potion lands
		}
		if curr := int32(binary.LittleEndian.Uint32(p[124:128])); curr != hp {
			t.Errorf("CurrHp = %d, want %d (must agree with Score.Hp)", curr, hp)
		}
		return
	}
	t.Fatal("observer never saw the drinker's healed score — the pot heal is invisible to the attacker (#99)")
}

// TestUseHealPotionNoBroadcastOutOfView pins BroadcastInView's distance filter: a
// player beyond ViewRange must not receive the drinker's score.
func TestUseHealPotionNoBroadcastOutOfView(t *testing.T) {
	const potion = 409
	db := healPotionDB(world.Item{Index: potion}, 500, 100000, 0, 0)
	// Account 11 spawns far away; account 7 keeps the (5,5) loadResult.
	far := db.loadResult
	far.X, far.Y = 500, 500
	db.loads = map[int64]world.CharacterState{11: far}
	effects := map[int][]content.BaseEffect{potion: {{Eff: efHp, Val: 1000}}}
	addr, stop := startServerTickItems(t, db, map[int]int{potion: volHpMpPotion}, effects)
	defer stop()

	drinker := enterWorld(t, addr)
	defer drinker.Close()
	observer := enterWorldAs(t, addr, "tradeb")
	defer observer.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, drinker, protocol.MsgUseItem, body.Encode())

	for i := 0; i < 8; i++ {
		h, _, ok := readMaybeHeader(t, observer)
		if !ok {
			return // stream went quiet — nothing leaked
		}
		if h.Type == protocol.MsgUpdateScore && h.ID == 1 {
			t.Fatal("out-of-view player received the drinker's score, want it filtered by ViewRange")
		}
	}
}

// TestEnterWorldViewScoreStaysUnicast locks the sendScoreSelf carve-out: a newcomer's
// score must NOT be multicast, because enterWorldView pushes it BEFORE broadcasting
// the newcomer's CreateMob — in-view clients would get a score for an entity they
// have not created yet. The legacy has no SendScore in this path at all
// (ProcessDBMessage.cpp:1017-1037); observers learn the newcomer's HP from CreateMob.
func TestEnterWorldViewScoreStaysUnicast(t *testing.T) {
	db := healPotionDB(world.Item{}, 500, 1000, 0, 0)
	addr, stop := startServerClockItems(t, db, nil, nil)
	defer stop()

	first := enterWorld(t, addr) // conn 1, already in world
	defer first.Close()
	second := enterWorldAs(t, addr, "tradeb") // conn 2 logs in at the same cell
	defer second.Close()

	// conn 1 may see conn 2's CreateMob/PKInfo, but never conn 2's UpdateScore.
	for i := 0; i < 8; i++ {
		h, _, ok := readMaybeHeader(t, first)
		if !ok {
			return
		}
		if h.Type == protocol.MsgUpdateScore && h.ID == 2 {
			t.Fatal("newcomer's score was multicast before its CreateMob — unknown-entity packet (B1)")
		}
	}
}

func TestUseSephiraBookConsumesOneFromStack(t *testing.T) {
	const book = 4200
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: book, Effects: [3]world.Effect{{Effect: efAmount, Value: 3}}}
	db.loadResult = st

	addr, stop := startServerClockVol(t, db, map[int]int{book: volSephiraLo})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	item := expect(t, c, protocol.MsgSendItem)
	if got := le16(item[4:6]); got != book {
		t.Fatalf("carry slot 0 item = %d, want stacked Sephira book", got)
	}
	if item[6] != efAmount || item[7] != 2 {
		t.Fatalf("book amount effect = %d.%d, want %d.2", item[6], item[7], efAmount)
	}
	etc := expect(t, c, protocol.MsgUpdateEtc)
	if learn := int64(binary.LittleEndian.Uint64(etc[12:])); learn != 1<<24 {
		t.Fatalf("UpdateEtc.Learn = %#x, want Sephira bit24", learn)
	}
}

func TestUseFairyDust(t *testing.T) {
	const dust = 5001
	addr, stop := startServerClockVol(t, fairyDustDB(0, world.Item{Index: dust}), map[int]int{dust: volFairyDust})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	sawSendItem, sawScore, sawEtc, sawMotion := false, false, false, false
	for range 6 {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			break
		}
		switch ty {
		case protocol.MsgSendItem:
			sawSendItem = true
			if le16(payload[4:6]) != 0 {
				t.Errorf("carry slot 0 item = %d, want empty after use", le16(payload[4:6]))
			}
		case protocol.MsgUpdateScore:
			sawScore = true
		case protocol.MsgUpdateEtc:
			sawEtc = true
			if got := int64(binary.LittleEndian.Uint64(payload[4:12])); got != level.NextLevelExp(0) {
				t.Errorf("UpdateEtc.Exp = %d, want %d", got, level.NextLevelExp(0))
			}
		case protocol.MsgMotion:
			sawMotion = true
			if got := le16(payload[0:2]); got != motionLevelUp {
				t.Errorf("motion = %d, want %d", got, motionLevelUp)
			}
		}
	}
	if !sawSendItem {
		t.Error("missing MsgSendItem after using fairy dust")
	}
	if !sawScore {
		t.Error("missing MsgUpdateScore after using fairy dust")
	}
	if !sawEtc {
		t.Error("missing MsgUpdateEtc after using fairy dust")
	}
	if !sawMotion {
		t.Error("missing MsgMotion after using fairy dust")
	}
}

func TestUseFairyDustStack(t *testing.T) {
	const dust = 5001
	item := world.Item{Index: dust, Effects: [3]world.Effect{{Effect: efAmount, Value: 3}}}
	addr, stop := startServerClockVol(t, fairyDustDB(0, item), map[int]int{dust: volFairyDust})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	payload := expect(t, c, protocol.MsgSendItem)
	if got := le16(payload[4:6]); got != dust {
		t.Fatalf("carry slot 0 item = %d, want stacked dust", got)
	}
	if payload[6] != efAmount || payload[7] != 2 {
		t.Errorf("effect0 = %d.%d, want %d.2", payload[6], payload[7], efAmount)
	}
}

func TestUseFairyDustAtCap(t *testing.T) {
	const dust = 5001
	addr, stop := startServerClockVol(t, fairyDustDB(int(level.MaxLevel), world.Item{Index: dust}), map[int]int{dust: volFairyDust})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	sawSendItem, sawEtc := false, false
	for range 4 {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			break
		}
		switch ty {
		case protocol.MsgSendItem:
			sawSendItem = true
			if le16(payload[4:6]) != 0 {
				t.Errorf("carry slot 0 item = %d, want empty after use", le16(payload[4:6]))
			}
		case protocol.MsgUpdateEtc:
			sawEtc = true
			if got := int64(binary.LittleEndian.Uint64(payload[4:12])); got != level.MaxExp {
				t.Errorf("UpdateEtc.Exp = %d, want %d", got, level.MaxExp)
			}
		case protocol.MsgMotion:
			t.Fatalf("got level-up motion at cap")
		}
	}
	if !sawSendItem {
		t.Error("missing MsgSendItem after using fairy dust at cap")
	}
	if !sawEtc {
		t.Error("missing MsgUpdateEtc after using fairy dust at cap")
	}
}

func TestUseSilverBar1Bi(t *testing.T) {
	addr, stop := startServerClockVol(t, silverBarDB(4011, 0), map[int]int{4011: volSilverBar})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	sawSendItem, sawEtc := false, false
	for range 4 {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			break
		}
		switch ty {
		case protocol.MsgSendItem:
			sawSendItem = true
			if le16(payload[2:4]) != 0 || le16(payload[4:6]) != 0 {
				t.Errorf("slot/item = %d/%d, want slot 0 empty", le16(payload[2:4]), le16(payload[4:6]))
			}
		case protocol.MsgUpdateEtc:
			sawEtc = true
			if got := int32(le(payload[28:32])); got != 1_000_000_000 {
				t.Errorf("coin = %d, want 1000000000", got)
			}
		}
	}
	if !sawSendItem {
		t.Error("missing MsgSendItem after using silver bar")
	}
	if !sawEtc {
		t.Error("missing MsgUpdateEtc after using silver bar")
	}
}

// TestUseSilverBarConsumesOneFromStack is the issue #259 sibling: useSilverBar also
// wiped the slot, so cashing one bar out of a stack burned the rest. Legacy:
// _MSG_UseItem.cpp:4277. Only one bar's worth of gold may be credited per use.
func TestUseSilverBarConsumesOneFromStack(t *testing.T) {
	db := silverBarDB(4026, 0)
	st := db.loadResult
	st.Carry[0] = world.Item{Index: 4026, Effects: [3]world.Effect{{Effect: efAmount, Value: 5}}}
	db.loadResult = st
	addr, stop := startServerClockVol(t, db, map[int]int{4026: volSilverBar})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	sawSendItem, sawEtc := false, false
	for range 4 {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			break
		}
		switch ty {
		case protocol.MsgSendItem:
			sawSendItem = true
			if le16(payload[4:6]) != 4026 {
				t.Fatalf("carry slot 0 item = %d, want 4026 (whole stack consumed)", le16(payload[4:6]))
			}
			if payload[6] != efAmount || payload[7] != 4 {
				t.Errorf("amount effect = (%d,%d), want (%d,4)", payload[6], payload[7], efAmount)
			}
		case protocol.MsgUpdateEtc:
			sawEtc = true
			if got := int32(le(payload[28:32])); got != 1_000_000 {
				t.Errorf("coin = %d, want 1000000 (one bar, not the stack)", got)
			}
		}
	}
	if !sawSendItem {
		t.Error("missing MsgSendItem after using silver bar")
	}
	if !sawEtc {
		t.Error("missing MsgUpdateEtc after using silver bar")
	}
}

func TestUseSilverBarOver2G(t *testing.T) {
	addr, stop := startServerClockVol(t, silverBarDB(4011, 1_500_000_000), map[int]int{4011: volSilverBar})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	if ty, p, ok := readMaybe(t, c); !ok || ty != protocol.MsgMessageBoxOk || noticeCode(t, p) != NoticeCargoFull {
		t.Fatalf("overflow notice = %#x/%v ok=%v, want NoticeCargoFull", ty, noticeCode(t, p), ok)
	}
	item := expect(t, c, protocol.MsgSendItem)
	if le16(item[2:4]) != 0 || le16(item[4:6]) != 4011 {
		t.Errorf("slot/item = %d/%d, want slot 0 item 4011 preserved", le16(item[2:4]), le16(item[4:6]))
	}
	if ty, _, ok := readMaybe(t, c); ok && ty == protocol.MsgUpdateEtc {
		t.Error("overflow use sent MsgUpdateEtc; coin should be unchanged")
	}
}

// Gema Estelar (issue #140): item 700 records the warp save-point; item 776/3430
// (Pergaminho de Portal) teleports back to it. Tests below use the real catalog
// indices for readability, but the harness classifies items purely by the vols
// map, not a real ItemList lookup.

// gemaEstelarDB seeds a character at (x,y) with an existing save-point
// (saveX,saveY — 0,0 for "never saved") and the given carried items.
func gemaEstelarDB(x, y, saveX, saveY int16, carry ...world.Item) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: x, Y: y, SaveX: saveX, SaveY: saveY, HP: 1000, MaxHP: 1000}
	for i, it := range carry {
		if i >= len(st.Carry) {
			break
		}
		st.Carry[i] = it
	}
	db.loadResult = st
	return db
}

// startServerClockVolGrid is startServerClockVol with a configurable grid
// dimension, needed for tests that place the player at real city/Pesadelo map
// coordinates (world.Village and inPesadelo compare against actual legacy
// segment/rectangle constants, which exceed the usual 16x16 test grid).
func startServerClockVolGrid(t *testing.T, persist world.Persistence, vols map[int]int, gridDim int) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, ItemVolatiles: vols})
	w := world.New(world.Config{GridDim: gridDim}, log, persist, d.Handle)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	return ln.Addr().String(), func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}
}

func recallScrollDB(lastCity int16, hp, mp int32, carry ...world.Item) *fakeDB {
	db := newDB()
	st := world.CharacterState{
		Slot: 0, Name: "Hero", X: 5, Y: 5, LastCity: lastCity,
		HP: hp, MaxHP: 1000, MP: mp, MaxMP: 500,
	}
	for i, it := range carry {
		if i >= len(st.Carry) {
			break
		}
		st.Carry[i] = it
	}
	db.loadResult = st
	return db
}

func useItemFrame(t *testing.T, c net.Conn, slot int) {
	t.Helper()
	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: int32(slot)}
	send(t, c, protocol.MsgUseItem, body.Encode())
}

// TestUseRecallScrollConsumesAndRecalls ports _MSG_UseItem.cpp's "Retorno" block:
// EF_VOLATILE 11 calls DoRecall and consumes one Pergaminho_Retorno.
func TestUseRecallScrollConsumesAndRecalls(t *testing.T) {
	const recall = 410
	db := recallScrollDB(2, 321, 123, world.Item{Index: recall})
	addr, stop := startServerClockVolGrid(t, db, map[int]int{recall: volRecallScroll}, 2600)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useItemFrame(t, c, 0)
	item := expect(t, c, protocol.MsgSendItem)
	if got := le16(item[4:6]); got != 0 {
		t.Errorf("recall slot = %d, want empty after use", got)
	}
	action := expectAction(t, c)
	if action.PosX != 5 || action.PosY != 5 || action.Effect != 1 {
		t.Errorf("recall action origin/effect = (%d,%d)/%d, want (5,5)/1", action.PosX, action.PosY, action.Effect)
	}
	if city := world.Village(action.TargetX, action.TargetY); city != 2 {
		t.Fatalf("recall target = (%d,%d) in city %d, want last city 2", action.TargetX, action.TargetY, city)
	}
	if ty, _, ok := readMaybe(t, c); ok && ty == protocol.MsgSetHpMp {
		t.Fatal("recall scroll sent SetHpMp, want no HP/MP bar mutation")
	}

	send(t, c, protocol.MsgCharacterLogout, nil)
	expect(t, c, protocol.MsgCNFCharacterLogout)
	save, n := db.lastSavedChar()
	if n == 0 {
		t.Fatal("character was not saved on logout")
	}
	if save.HP != 321 || save.MP != 123 {
		t.Fatalf("saved HP/MP = %d/%d, want unchanged 321/123", save.HP, save.MP)
	}
	if _, ok := savedItemAt(save.Carry, 0); ok {
		t.Fatal("saved carry slot 0 still has recall scroll, want consumed")
	}
}

// TestUseRecallScrollStack decrements EF_AMOUNT on the 10x return scroll and still
// recalls to the character's last-city spawn.
func TestUseRecallScrollStack(t *testing.T) {
	const recall10x = 411
	db := recallScrollDB(1, 1000, 500, world.Item{Index: recall10x, Effects: [3]world.Effect{{Effect: efAmount, Value: 10}}})
	addr, stop := startServerClockVolGrid(t, db, map[int]int{recall10x: volRecallScroll}, 2700)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useItemFrame(t, c, 0)
	item := expect(t, c, protocol.MsgSendItem)
	if got := le16(item[4:6]); got != recall10x {
		t.Fatalf("carry slot 0 item = %d, want stacked recall scroll", got)
	}
	if item[6] != efAmount || item[7] != 9 {
		t.Errorf("effect0 = %d.%d, want %d.9", item[6], item[7], efAmount)
	}
	action := expectAction(t, c)
	if city := world.Village(action.TargetX, action.TargetY); city != 1 {
		t.Fatalf("recall target = (%d,%d) in city %d, want last city 1", action.TargetX, action.TargetY, city)
	}
}

// TestUseGemaEstelarThenPortalRoundTrip: saving at the current position, then
// using the portal scroll, teleports the player back to that exact tile —
// the core mechanic issue #140 reports as broken (neither item did anything).
func TestUseGemaEstelarThenPortalRoundTrip(t *testing.T) {
	const gema, portal = 700, 776
	db := gemaEstelarDB(5, 5, 0, 0, world.Item{Index: gema}, world.Item{Index: portal})
	addr, stop := startServerClockVol(t, db, map[int]int{gema: volGemaEstelar, portal: volPortalScroll})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useItemFrame(t, c, 0) // Gema Estelar
	saved := expect(t, c, protocol.MsgSendItem)
	if got := le16(saved[4:6]); got != 0 {
		t.Errorf("gema slot = %d, want empty after use", got)
	}
	if got := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); got != NoticeSetWarp {
		t.Errorf("notice = %v, want NoticeSetWarp", got)
	}

	useItemFrame(t, c, 1) // Pergaminho de Portal
	scroll := expect(t, c, protocol.MsgSendItem)
	if got := le16(scroll[4:6]); got != 0 {
		t.Errorf("portal slot = %d, want empty after use", got)
	}
	action := expectAction(t, c)
	if action.TargetX != 5 || action.TargetY != 5 {
		t.Errorf("teleport target = (%d,%d), want (5,5)", action.TargetX, action.TargetY)
	}
}

// TestUsePortalScrollStack: a multi-charge scroll (EF_AMOUNT) decrements
// instead of clearing, and still teleports.
func TestUsePortalScrollStack(t *testing.T) {
	const portal = 776
	db := gemaEstelarDB(5, 5, 20, 20, world.Item{Index: portal, Effects: [3]world.Effect{{Effect: efAmount, Value: 3}}})
	addr, stop := startServerClockVol(t, db, map[int]int{portal: volPortalScroll})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useItemFrame(t, c, 0)
	payload := expect(t, c, protocol.MsgSendItem)
	if got := le16(payload[4:6]); got != portal {
		t.Fatalf("carry slot 0 item = %d, want stacked scroll", got)
	}
	if payload[6] != efAmount || payload[7] != 2 {
		t.Errorf("effect0 = %d.%d, want %d.2", payload[6], payload[7], efAmount)
	}
	action := expectAction(t, c)
	if action.TargetX != 20 || action.TargetY != 20 {
		t.Errorf("teleport target = (%d,%d), want (20,20)", action.TargetX, action.TargetY)
	}
}

// TestUsePortalScrollNoSavePoint: a character that never used a Gema Estelar
// has SaveX==SaveY==0; the scroll must no-op rather than warp to the map origin.
func TestUsePortalScrollNoSavePoint(t *testing.T) {
	const portal = 776
	db := gemaEstelarDB(5, 5, 0, 0, world.Item{Index: portal})
	addr, stop := startServerClockVol(t, db, map[int]int{portal: volPortalScroll})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useItemFrame(t, c, 0)
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("got %#x, want no response (no save point set)", ty)
	}
}

// TestUseGemaEstelarBlockedInCity: the legacy refuses to record a save-point
// while standing inside a city's limits (BASE_GetVillage); the item is not
// consumed and the client is told why. Armia's CityLimit rectangle is
// (2052,2052)-(2171,2163) (world/city.go), so the test grid must be large
// enough to place the player there.
func TestUseGemaEstelarBlockedInCity(t *testing.T) {
	const gema = 700
	const armiaX, armiaY = 2100, 2100
	db := gemaEstelarDB(armiaX, armiaY, 0, 0, world.Item{Index: gema})
	addr, stop := startServerClockVolGrid(t, db, map[int]int{gema: volGemaEstelar}, 2200)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useItemFrame(t, c, 0)
	if ty, p, ok := readMaybe(t, c); !ok || ty != protocol.MsgMessageBoxOk || noticeCode(t, p) != NoticeCantUseHere {
		t.Fatalf("notice = %#x/%v ok=%v, want NoticeCantUseHere", ty, noticeCode(t, p), ok)
	}
	item := expect(t, c, protocol.MsgSendItem)
	if got := le16(item[4:6]); got != gema {
		t.Errorf("carry slot 0 item = %d, want %d preserved (not consumed)", got, gema)
	}
}

// TestUseGemaEstelarPesadeloCarveOut: the two Pesadelo instanced-dungeon
// segments are exempt from the city/arena gate (legacy "goto CanSave"), so a
// save still succeeds there even though the tile would otherwise be blocked.
// (9*128, 1*128) = (1152,128) falls inside the Pesadelo_A segment.
func TestUseGemaEstelarPesadeloCarveOut(t *testing.T) {
	const gema = 700
	const pesadeloX, pesadeloY = 1200, 150
	db := gemaEstelarDB(pesadeloX, pesadeloY, 0, 0, world.Item{Index: gema})
	addr, stop := startServerClockVolGrid(t, db, map[int]int{gema: volGemaEstelar}, 1300)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useItemFrame(t, c, 0)
	saved := expect(t, c, protocol.MsgSendItem)
	if got := le16(saved[4:6]); got != 0 {
		t.Errorf("gema slot = %d, want empty after use (Pesadelo carve-out should allow the save)", got)
	}
	if got := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); got != NoticeSetWarp {
		t.Errorf("notice = %v, want NoticeSetWarp", got)
	}
}

// TestUsePortalScrollBlockedFromPesadeloSavePoint: a saved point that happens
// to fall inside a Pesadelo segment refuses the portal (can't warp into the
// instance from outside), independent of the player's own current position.
func TestUsePortalScrollBlockedFromPesadeloSavePoint(t *testing.T) {
	const portal = 776
	const pesadeloX, pesadeloY = 1200, 150
	db := gemaEstelarDB(5, 5, pesadeloX, pesadeloY, world.Item{Index: portal})
	addr, stop := startServerClockVol(t, db, map[int]int{portal: volPortalScroll})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useItemFrame(t, c, 0)
	if ty, p, ok := readMaybe(t, c); !ok || ty != protocol.MsgMessageBoxOk || noticeCode(t, p) != NoticeCantUseHere {
		t.Fatalf("notice = %#x/%v ok=%v, want NoticeCantUseHere", ty, noticeCode(t, p), ok)
	}
	item := expect(t, c, protocol.MsgSendItem)
	if got := le16(item[4:6]); got != portal {
		t.Errorf("carry slot 0 item = %d, want %d preserved (not consumed)", got, portal)
	}
}
