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
