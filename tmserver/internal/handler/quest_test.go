package handler

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// perzenTemplate builds a raw STRUCT_MOB for a Perzen quest NPC: Merchant 100 with
// EF_GRADE0 grade on Equip[0], and an input→reward pair in Carry[0]/Carry[1].
func perzenTemplate(grade uint8, input, reward int16) []byte {
	b := make([]byte, 816)
	copy(b[0:16], "Perzen")
	const cs = 92
	b[cs+12] = 100                                         // Merchant = quest NPC
	binary.LittleEndian.PutUint32(b[cs+24:], 100)          // Hp (alive)
	binary.LittleEndian.PutUint16(b[40:], 5)               // SPX
	binary.LittleEndian.PutUint16(b[42:], 5)               // SPY
	binary.LittleEndian.PutUint16(b[140:], 11)             // Equip[0].index
	b[140+2], b[140+3] = 100, grade                        // Equip[0].eff0 = (EF_GRADE0, grade)
	binary.LittleEndian.PutUint16(b[268:], uint16(input))  // Carry[0] = required item
	binary.LittleEndian.PutUint16(b[276:], uint16(reward)) // Carry[1] = reward mount
	return b
}

// startServerPerzen is the clock harness with a Perzen NPC spawned at (5,5).
func startServerPerzen(t *testing.T, persist world.Persistence, grade uint8, input, reward int16) (string, func(), int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 16}, log, persist, d.Handle)
	npcID := w.SpawnMob(perzenTemplate(grade, input, reward), 5, 5)
	if npcID < 0 {
		t.Fatal("failed to spawn Perzen")
	}
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
	}, npcID
}

// perzenDB gives the tester character the input item in carry slot 0.
func perzenDB(input int16) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: input}
	db.loadResult = st
	return db
}

func questFrame(t *testing.T, c net.Conn, npcID int) {
	t.Helper()
	send(t, c, protocol.MsgQuest, protocol.EncodeStandardParm2(int32(npcID), 0))
}

func kingTemplate() []byte {
	b := make([]byte, 816)
	copy(b[0:16], "Rei_Harabard")
	const cs = 92
	b[cs+12] = 111                                // CurrentScore.Merchant for King templates
	binary.LittleEndian.PutUint32(b[cs+24:], 100) // Hp (alive)
	binary.LittleEndian.PutUint16(b[40:], 5)      // SPX
	binary.LittleEndian.PutUint16(b[42:], 5)      // SPY
	binary.LittleEndian.PutUint16(b[140:], 11)    // Equip[0].index
	return b
}

func TestIsKingQuestNPCShapes(t *testing.T) {
	tests := []struct {
		name string
		npc  *world.Entity
		want bool
	}{
		{"real current score merchant", &world.Entity{Merchant: 111}, true},
		{"legacy top-level harabard", &world.Entity{Merchant: 14}, true},
		{"legacy top-level glantuar", &world.Entity{Merchant: 15}, true},
		{"unsupported variant", &world.Entity{Merchant: 96}, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isKingQuestNPC(tt.npc); got != tt.want {
				t.Errorf("isKingQuestNPC() = %v, want %v", got, tt.want)
			}
		})
	}
}

func startServerKing(t *testing.T, db *fakeDB) (string, func(), int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 16}, log, db, d.Handle)
	npcID := w.SpawnMob(kingTemplate(), 5, 5)
	if npcID < 0 {
		t.Fatal("failed to spawn King")
	}
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
	}, npcID
}

func mestreGrifoTemplate() []byte {
	if b, err := os.ReadFile(filepath.Join("..", "..", "..", "Release", "TMsrv", "run", "npc", "Mestre_Grifo")); err == nil && len(b) == 816 {
		return b
	}
	b := make([]byte, 816)
	copy(b[0:16], "Mestre_Grifo")
	const cs = 92
	b[cs+12] = 23                               // Merchant = Mestre Grifo
	binary.LittleEndian.PutUint32(b[cs+24:], 1) // Hp (alive)
	return b
}

func startServerMestreGrifo(t *testing.T, st world.CharacterState, tick bool) (string, func(), int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	db := newDB()
	db.loadResult = st
	w := world.New(world.Config{GridDim: world.DefaultGridDim}, log, db, d.Handle)
	if tick {
		w.SetTickHandler(10*time.Millisecond, d.Tick)
	}
	npcID := w.SpawnMob(mestreGrifoTemplate(), 2116, 2080)
	if npcID < 0 {
		t.Fatal("failed to spawn Mestre_Grifo")
	}
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
	}, npcID
}

func expectAction(t *testing.T, c net.Conn) protocol.MsgActionBody {
	t.Helper()
	ty, payload := read(t, c)
	if ty != protocol.MsgAction {
		t.Fatalf("got %#x, want MsgAction", ty)
	}
	var body protocol.MsgActionBody
	if err := body.Decode(payload); err != nil {
		t.Fatal(err)
	}
	return body
}

func inRange(v, center int16) bool {
	return v >= center-3 && v <= center+1
}

func withMasterGriffTravelDelay(t *testing.T, delay time.Duration) {
	t.Helper()
	old := masterGriffTravelDelay
	masterGriffTravelDelay = delay
	t.Cleanup(func() { masterGriffTravelDelay = old })
}

// TestPerzenExchange: handing the required item to a Perzen NPC swaps it for the
// reward mount in the same inventory slot.
func TestPerzenExchange(t *testing.T) {
	const input, reward = 4130, 3987 // Esfera da Sorte A → Thoroughbred(30d)
	addr, stop, npcID := startServerPerzen(t, perzenDB(input), 9, input, reward)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	questFrame(t, c, npcID)
	send := expect(t, c, protocol.MsgSendItem) // carry slot 0 refreshed with the reward
	if slot := le16(send[2:4]); slot != 0 {
		t.Errorf("refreshed slot = %d, want 0", slot)
	}
	if idx := le16(send[4:6]); idx != reward {
		t.Errorf("reward item = %d, want %d", idx, reward)
	}
}

// TestPerzenMissingItem: without the required item, the exchange is a no-op.
func TestPerzenMissingItem(t *testing.T) {
	const input, reward = 4130, 3987
	addr, stop, npcID := startServerPerzen(t, perzenDB(999), 9, input, reward) // wrong item in carry
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	questFrame(t, c, npcID)
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("exchange without the item produced %#x; should be a no-op", ty)
	}
}

func TestKingCreatesArchCharacter(t *testing.T) {
	db := newDB()
	db.archOK = true
	db.archSlot = 2
	st := world.CharacterState{
		Slot: 0, Name: "Hero", Class: 0, Level: 299, X: 5, Y: 5,
		HP: 1000, MaxHP: 1000, ClassMaster: classMasterMortal,
	}
	st.Equip[0] = world.Item{Index: 21}
	st.Equip[10] = world.Item{Index: 1742}
	st.Equip[11] = world.Item{Index: 1762}
	db.loadResult = st

	addr, stop, npcID := startServerKing(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgQuest, protocol.EncodeStandardParm2(int32(npcID), 1))
	itemA := expect(t, c, protocol.MsgSendItem)
	itemB := expect(t, c, protocol.MsgSendItem)
	cleared := map[uint16]uint16{
		le16(itemA[2:4]): le16(itemA[4:6]),
		le16(itemB[2:4]): le16(itemB[4:6]),
	}
	if idx, ok := cleared[10]; !ok || idx != 0 {
		t.Fatalf("slot 10 clear = idx %d ok=%v; all cleared slots=%+v", idx, ok, cleared)
	}
	if idx, ok := cleared[11]; !ok || idx != 0 {
		t.Fatalf("slot 11 clear = idx %d ok=%v; all cleared slots=%+v", idx, ok, cleared)
	}
	if len(cleared) != 2 {
		t.Fatalf("cleared equip slots = %+v, want slots 10 and 11 empty", cleared)
	}
	expect(t, c, protocol.MsgCNFCharacterLogout)
	if body := expect(t, c, protocol.MsgCNFNewCharacter); len(body) != 844 {
		t.Fatalf("CNFNewCharacter body = %d, want 844", len(body))
	}
	effect := expect(t, c, protocol.MsgSendArchEffect)
	if parm, ok := protocol.StandardParm(effect); !ok || parm != 2 {
		t.Fatalf("SendArchEffect parm=%d ok=%v, want slot 2", parm, ok)
	}

	created, accountID, name, class, face, mortalSlot := db.archRequest()
	if created != 1 || accountID != 7 || name != "Hero" || class != 2 || face != 21 || mortalSlot != 0 {
		t.Fatalf("arch request created=%d account=%d name=%q class=%d face=%d mortalSlot=%d",
			created, accountID, name, class, face, mortalSlot)
	}
	save, n := db.lastSavedChar()
	if n == 0 {
		t.Fatal("Mortal was not saved after Arch creation")
	}
	for _, it := range save.Equip {
		if it.Slot == 10 || it.Slot == 11 {
			t.Fatalf("saved Mortal still has Arch creation equip slot: %+v", save.Equip)
		}
	}
}

func TestMestreGrifoTeleportsByLevel(t *testing.T) {
	tests := []struct {
		name  string
		level int
		wantX int16
		wantY int16
	}{
		{name: "coveiro", level: 50, wantX: 2398, wantY: 2105},
		{name: "kaizen", level: 200, wantX: 464, wantY: 3902},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := world.CharacterState{
				Slot: 0, Name: "Hero", Level: tt.level, X: 2113, Y: 2079,
				HP: 1000, MaxHP: 1000, ClassMaster: classMasterMortal,
			}
			addr, stop, npcID := startServerMestreGrifo(t, st, false)
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()

			questFrame(t, c, npcID)
			body := expectAction(t, c)
			if body.Effect != 1 {
				t.Errorf("teleport effect = %d, want 1", body.Effect)
			}
			if !inRange(body.TargetX, tt.wantX) || !inRange(body.TargetY, tt.wantY) {
				t.Errorf("target = %d,%d, want around %d,%d", body.TargetX, body.TargetY, tt.wantX, tt.wantY)
			}
			if ty, _, ok := readMaybe(t, c); ok && ty == protocol.MsgAction {
				t.Errorf("unexpected second teleport after Mestre Grifo: %#x", ty)
			}
		})
	}
}

func TestMestreGrifoOutOfRangeNoop(t *testing.T) {
	st := world.CharacterState{
		Slot: 0, Name: "Hero", Level: 350, X: 2113, Y: 2079,
		HP: 1000, MaxHP: 1000, ClassMaster: classMasterMortal,
	}
	addr, stop, npcID := startServerMestreGrifo(t, st, false)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	questFrame(t, c, npcID)
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("out-of-range Mestre Grifo produced %#x; want no-op", ty)
	}
}

func TestQuest256AreaGuardRecallsWithoutFlag(t *testing.T) {
	st := world.CharacterState{
		Slot: 0, Name: "Hero", Level: 50, X: 2398, Y: 2105,
		HP: 1000, MaxHP: 1000, LastCity: 0, ClassMaster: classMasterMortal,
	}
	addr, stop, _ := startServerMestreGrifo(t, st, true)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := expectAction(t, c)
	if body.Effect != 1 {
		t.Errorf("recall effect = %d, want 1", body.Effect)
	}
	if body.TargetX < 2086 || body.TargetX > 2100 || body.TargetY < 2093 || body.TargetY > 2107 {
		t.Errorf("recall target = %d,%d, want Armia city spawn", body.TargetX, body.TargetY)
	}
}

func TestMestreGrifoQuestFlagPreventsImmediateRecall(t *testing.T) {
	st := world.CharacterState{
		Slot: 0, Name: "Hero", Level: 50, X: 2113, Y: 2079,
		HP: 1000, MaxHP: 1000, LastCity: 0, ClassMaster: classMasterMortal,
	}
	addr, stop, npcID := startServerMestreGrifo(t, st, true)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	questFrame(t, c, npcID)
	body := expectAction(t, c)
	if !inRange(body.TargetX, 2398) || !inRange(body.TargetY, 2105) {
		t.Fatalf("quest target = %d,%d, want Coveiro", body.TargetX, body.TargetY)
	}
	if ty, _, ok := readMaybe(t, c); ok && ty == protocol.MsgAction {
		t.Errorf("Mestre Grifo target was recalled immediately: %#x", ty)
	}
}

func TestMestreGrifoRealTemplateTeleportsAndSurvivesGuard(t *testing.T) {
	tmpl, err := os.ReadFile(filepath.Join("..", "..", "..", "Release", "TMsrv", "run", "npc", "Mestre_Grifo"))
	if err != nil {
		t.Fatalf("read real Mestre_Grifo template: %v", err)
	}
	if len(tmpl) != 816 {
		t.Fatalf("real Mestre_Grifo template len = %d, want 816", len(tmpl))
	}
	if got := tmpl[92+12]; got != 23 {
		t.Fatalf("real Mestre_Grifo merchant = %d, want 23", got)
	}

	st := world.CharacterState{
		Slot: 0, Name: "Hero", Level: 50, X: 2113, Y: 2079,
		HP: 1000, MaxHP: 1000, LastCity: 0, ClassMaster: classMasterMortal,
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	db := newDB()
	db.loadResult = st
	w := world.New(world.Config{GridDim: world.DefaultGridDim}, log, db, d.Handle)
	w.SetTickHandler(10*time.Millisecond, d.Tick)
	npcID := w.SpawnMob(tmpl, 2116, 2080)
	if npcID < 0 {
		t.Fatal("failed to spawn real Mestre_Grifo")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}()

	c := enterWorld(t, ln.Addr().String())
	defer c.Close()
	questFrame(t, c, npcID)
	body := expectAction(t, c)
	if !inRange(body.TargetX, 2398) || !inRange(body.TargetY, 2105) {
		t.Fatalf("quest target = %d,%d, want Coveiro", body.TargetX, body.TargetY)
	}
	if ty, _, ok := readMaybe(t, c); ok && ty == protocol.MsgAction {
		t.Errorf("real Mestre_Grifo target was recalled immediately: %#x", ty)
	}
}

func TestMasterGriffOpcodeUsesWarpDestinations(t *testing.T) {
	withMasterGriffTravelDelay(t, 500*time.Millisecond)
	tests := []struct {
		name   string
		warpID int32
		wantX  int16
		wantY  int16
	}{
		{name: "defensor almas", warpID: 1, wantX: 2372, wantY: 2099},
		{name: "jardim deuses", warpID: 2, wantX: 2220, wantY: 1714},
		{name: "calabouco", warpID: 3, wantX: 2365, wantY: 2279},
		{name: "submundo", warpID: 4, wantX: 1826, wantY: 1771},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := world.CharacterState{
				Slot: 0, Name: "Hero", Level: 1, X: 2113, Y: 2079,
				HP: 1000, MaxHP: 1000, LastCity: 0, ClassMaster: classMasterMortal,
			}
			addr, stop, _ := startServerMestreGrifo(t, st, true)
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()

			send(t, c, protocol.MsgMasterGriff, protocol.EncodeStandardParm2(tt.warpID, 0))
			if ty, _, ok := readMaybe(t, c); ok && ty == protocol.MsgAction {
				t.Fatalf("MasterGriff teleported immediately with %#x; want delayed travel", ty)
			}
			time.Sleep(masterGriffTravelDelay + 100*time.Millisecond)
			body := expectAction(t, c)
			if body.TargetX != tt.wantX || body.TargetY != tt.wantY {
				t.Fatalf("master griff target = %d,%d, want %d,%d", body.TargetX, body.TargetY, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestMasterGriffWarpIDZeroDefaultsToFirstDestination(t *testing.T) {
	withMasterGriffTravelDelay(t, 20*time.Millisecond)
	st := world.CharacterState{
		Slot: 0, Name: "Hero", Level: 200, X: 2113, Y: 2079,
		HP: 1000, MaxHP: 1000, LastCity: 0, ClassMaster: classMasterMortal,
	}
	addr, stop, _ := startServerMestreGrifo(t, st, true)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgMasterGriff, protocol.EncodeStandardParm2(0, 0))
	time.Sleep(masterGriffTravelDelay + 100*time.Millisecond)
	body := expectAction(t, c)
	if body.TargetX != 2372 || body.TargetY != 2099 {
		t.Fatalf("master griff target = %d,%d, want first destination", body.TargetX, body.TargetY)
	}
}

func TestQuest256TicketTravelsImmediately(t *testing.T) {
	tests := []struct {
		name  string
		item  int16
		level int32
		wantX int16
		wantY int16
	}{
		{name: "coveiro", item: 4038, level: 50, wantX: 2398, wantY: 2105},
		{name: "jardineiro", item: 4039, level: 120, wantX: 2234, wantY: 1714},
		{name: "kaizen", item: 4040, level: 200, wantX: 464, wantY: 3902},
		{name: "hidra", item: 4041, level: 270, wantX: 668, wantY: 3756},
		{name: "elfos", item: 4042, level: 330, wantX: 1322, wantY: 4041},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := world.CharacterState{
				Slot: 0, Name: "Hero", Level: int(tt.level), X: 2113, Y: 2079,
				HP: 1000, MaxHP: 1000, LastCity: 0, ClassMaster: classMasterMortal,
			}
			st.Carry[0] = world.Item{Index: tt.item}
			addr, stop, _ := startServerMestreGrifo(t, st, false)
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()

			body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
			send(t, c, protocol.MsgUseItem, body.Encode())
			consumed := expect(t, c, protocol.MsgSendItem)
			if slot, idx := le16(consumed[2:4]), le16(consumed[4:6]); slot != 0 || idx != 0 {
				t.Fatalf("ticket consume slot=%d idx=%d, want slot 0 cleared", slot, idx)
			}
			action := expectAction(t, c)
			if action.Effect != 1 {
				t.Fatalf("ticket travel effect = %d, want 1", action.Effect)
			}
			if !inRange(action.TargetX, tt.wantX) || !inRange(action.TargetY, tt.wantY) {
				t.Fatalf("ticket target = %d,%d, want around %d,%d", action.TargetX, action.TargetY, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestQuest256TicketInvalidLevelDoesNotConsume(t *testing.T) {
	st := world.CharacterState{
		Slot: 0, Name: "Hero", Level: 38, X: 2113, Y: 2079,
		HP: 1000, MaxHP: 1000, LastCity: 0, ClassMaster: classMasterMortal,
	}
	st.Carry[0] = world.Item{Index: 4038}
	addr, stop, _ := startServerMestreGrifo(t, st, false)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())
	if code := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); code != NoticeReqNotMet {
		t.Fatalf("invalid-level notice = %d, want NoticeReqNotMet", code)
	}
	item := expect(t, c, protocol.MsgSendItem)
	if slot, idx := le16(item[2:4]), le16(item[4:6]); slot != 0 || idx != 4038 {
		t.Fatalf("invalid-level item slot=%d idx=%d, want ticket kept", slot, idx)
	}
	if ty, _, ok := readMaybe(t, c); ok && ty == protocol.MsgAction {
		t.Fatalf("invalid-level ticket still teleported with %#x", ty)
	}
}

// TestQuest256TicketSecondUseOnEmptySlotIsNoop: with the ticket consumed
// synchronously (no more pending-delay window), a second right-click lands on
// an already-empty slot. useItem's top-of-function Empty() guard makes that a
// clean no-op — no second consume, no second teleport.
func TestQuest256TicketSecondUseOnEmptySlotIsNoop(t *testing.T) {
	st := world.CharacterState{
		Slot: 0, Name: "Hero", Level: 50, X: 2113, Y: 2079,
		HP: 1000, MaxHP: 1000, LastCity: 0, ClassMaster: classMasterMortal,
	}
	st.Carry[0] = world.Item{Index: 4038}
	addr, stop, _ := startServerMestreGrifo(t, st, false)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())
	first := expect(t, c, protocol.MsgSendItem)
	if idx := le16(first[4:6]); idx != 0 {
		t.Fatalf("first consume idx=%d, want slot cleared", idx)
	}
	expectAction(t, c)

	send(t, c, protocol.MsgUseItem, body.Encode())
	if ty, _, ok := readMaybe(t, c); ok {
		t.Fatalf("second use on empty slot produced %#x; want no-op", ty)
	}
}
