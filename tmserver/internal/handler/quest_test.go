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
