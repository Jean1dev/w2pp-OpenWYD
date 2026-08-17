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

// questNPCTemplate builds a raw STRUCT_MOB for a generic _MSG_Quest NPC:
// Merchant (+ EF_GRADE0 grade on Equip[0] when merchant==100) and Clan.
func questNPCTemplate(name string, merchant, grade, clan uint8) []byte {
	b := make([]byte, 816)
	copy(b[0:16], name)
	const cs = 92
	b[cs+12] = merchant
	b[cs+8] = clan
	binary.LittleEndian.PutUint32(b[cs+24:], 100) // Hp (alive)
	binary.LittleEndian.PutUint16(b[40:], 5)      // SPX
	binary.LittleEndian.PutUint16(b[42:], 5)      // SPY
	if merchant == 100 {
		binary.LittleEndian.PutUint16(b[140:], 11) // Equip[0].index
		b[140+2], b[140+3] = 100, grade            // Equip[0].eff0 = (EF_GRADE0, grade)
	}
	return b
}

// startServerQuestNPC is the clock harness with one quest NPC spawned at (5,5).
func startServerQuestNPC(t *testing.T, db world.Persistence, tmpl []byte) (string, func(), int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 16}, log, db, d.Handle)
	npcID := w.SpawnMob(tmpl, 5, 5)
	if npcID < 0 {
		t.Fatal("failed to spawn quest NPC")
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

// baseMortalState is a level/class starting point for the new quest-NPC tests;
// callers override Level/Carry/Equip as needed.
func baseMortalState(level int) world.CharacterState {
	return world.CharacterState{
		Slot: 0, Name: "Hero", Level: level, X: 5, Y: 5,
		HP: 1000, MaxHP: 1000, ClassMaster: classMasterMortal,
	}
}

func TestJardineiroStartsQuest256WithRealTemplate(t *testing.T) {
	tmpl, err := os.ReadFile(filepath.Join("..", "..", "..", "Release", "TMsrv", "run", "npc", "Jardineiro"))
	if err != nil {
		t.Fatalf("read real Jardineiro template: %v", err)
	}
	db := newDB()
	st := baseMortalState(115)
	st.Carry[7] = world.Item{Index: 4039}
	db.loadResult = st
	addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	questFrame(t, c, npcID)
	item := expect(t, c, protocol.MsgSendItem)
	if slot, idx := le16(item[2:4]), le16(item[4:6]); slot != 7 || idx != 0 {
		t.Fatalf("Jardineiro consume slot=%d idx=%d, want slot 7 cleared", slot, idx)
	}
	action := expectAction(t, c)
	if action.Effect != 1 || !inRange(action.TargetX, 2234) || !inRange(action.TargetY, 1714) {
		t.Fatalf("Jardineiro teleport effect=%d target=%d,%d, want effect 1 around 2234,1714", action.Effect, action.TargetX, action.TargetY)
	}
}

func TestJardineiroAcceptsArch(t *testing.T) {
	tmpl := questNPCTemplate("Jardineiro", 100, 1, 0)
	db := newDB()
	st := baseMortalState(189)
	st.ClassMaster = classMasterArch
	st.Carry[0] = world.Item{Index: 4039}
	db.loadResult = st
	addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	questFrame(t, c, npcID)
	expect(t, c, protocol.MsgSendItem)
	expectAction(t, c)
}

func TestJardineiroRejectsInvalidRequirements(t *testing.T) {
	tests := []struct {
		name        string
		level       int
		classMaster uint8
		item        int16
	}{
		{name: "below level", level: 114, classMaster: classMasterMortal, item: 4039},
		{name: "at upper bound", level: 190, classMaster: classMasterMortal, item: 4039},
		{name: "wrong class", level: 120, classMaster: classMasterCelestial, item: 4039},
		{name: "missing ticket", level: 120, classMaster: classMasterMortal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := questNPCTemplate("Jardineiro", 100, 1, 0)
			db := newDB()
			st := baseMortalState(tt.level)
			st.ClassMaster = tt.classMaster
			st.Carry[0] = world.Item{Index: tt.item}
			db.loadResult = st
			addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()

			questFrame(t, c, npcID)
			if code := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); code != NoticeReqNotMet {
				t.Fatalf("notice = %d, want NoticeReqNotMet", code)
			}
			if ty, _, ok := readMaybe(t, c); ok {
				t.Fatalf("rejected Jardineiro interaction produced %#x after notice", ty)
			}
		})
	}
}

func TestCapaverdeTeleport(t *testing.T) {
	tmpl := questNPCTemplate("Treinador_Aprendiz", 100, 14, 0)
	db := newDB()
	db.loadResult = baseMortalState(120)
	addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	questFrame(t, c, npcID)
	body := expectAction(t, c)
	if !inRange(body.TargetX, 2245) || !inRange(body.TargetY, 1576) {
		t.Fatalf("target = %d,%d, want around 2245,1576", body.TargetX, body.TargetY)
	}
}

func TestCapaverdeTeleportWrongLevelNoop(t *testing.T) {
	tmpl := questNPCTemplate("Treinador_Aprendiz", 100, 14, 0)
	db := newDB()
	db.loadResult = baseMortalState(200)
	addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	questFrame(t, c, npcID)
	if code := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); code != NoticeReqNotMet {
		t.Fatalf("wrong-level notice = %d, want NoticeReqNotMet", code)
	}
}

func TestCapaverdeTrade(t *testing.T) {
	tmpl := questNPCTemplate("Chefe_Treina.", 8, 0, 0)
	db := newDB()
	st := baseMortalState(120)
	st.Equip[13] = world.Item{Index: 4080}
	db.loadResult = st
	addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	questFrame(t, c, npcID)
	itemA := expect(t, c, protocol.MsgSendItem)
	itemB := expect(t, c, protocol.MsgSendItem)
	cleared := map[uint16]uint16{
		le16(itemA[2:4]): le16(itemA[4:6]),
		le16(itemB[2:4]): le16(itemB[4:6]),
	}
	if idx, ok := cleared[13]; !ok || idx != 0 {
		t.Fatalf("slot 13 (ticket) = idx %d ok=%v, want cleared", idx, ok)
	}
	if idx, ok := cleared[15]; !ok || idx != 4006 {
		t.Fatalf("slot 15 (cape) = idx %d ok=%v, want green cape 4006", idx, ok)
	}
	expect(t, c, protocol.MsgUpdateScore)
}

func TestCapaverdeTradeMissingTicketNoop(t *testing.T) {
	tmpl := questNPCTemplate("Chefe_Treina.", 8, 0, 0)
	db := newDB()
	db.loadResult = baseMortalState(120) // no item in Equip[13]
	addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	questFrame(t, c, npcID)
	if code := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); code != NoticeReqNotMet {
		t.Fatalf("missing-ticket notice = %d, want NoticeReqNotMet", code)
	}
}

func TestCapaverdeTradeAlreadyHasCape(t *testing.T) {
	tmpl := questNPCTemplate("Chefe_Treina.", 8, 0, 0)
	db := newDB()
	st := baseMortalState(120)
	st.Equip[13] = world.Item{Index: 4080}
	st.Equip[15] = world.Item{Index: 4006}
	db.loadResult = st
	addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	questFrame(t, c, npcID)
	if code := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); code != NoticeAlreadyDone {
		t.Fatalf("already-done notice = %d, want NoticeAlreadyDone", code)
	}
}

func TestMolarGargulaTeleport(t *testing.T) {
	tmpl := questNPCTemplate("Gargula_Molar", 100, 15, 0)
	db := newDB()
	db.loadResult = baseMortalState(220)
	addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	questFrame(t, c, npcID)
	body := expectAction(t, c)
	if !inRange(body.TargetX, 817) || !inRange(body.TargetY, 4062) {
		t.Fatalf("target = %d,%d, want around 817,4062", body.TargetX, body.TargetY)
	}
}

func TestMolarGargulaWrongLevelNoop(t *testing.T) {
	tmpl := questNPCTemplate("Gargula_Molar", 100, 15, 0)
	db := newDB()
	db.loadResult = baseMortalState(50)
	addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	questFrame(t, c, npcID)
	if code := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); code != NoticeReqNotMet {
		t.Fatalf("wrong-level notice = %d, want NoticeReqNotMet", code)
	}
}

// TestAmuMisticoCompletesForPartyLeader: a Mortal party leader confirming the
// quest sets the TerraMistica gate; a second attempt then rejects as already
// done (there is no client-visible success signal to assert on directly — the
// legacy case only SendClientMessages a text confirmation, UNVERIFIED wire
// format — so completion is observed through the gate flipping).
func TestAmuMisticoCompletesForPartyLeader(t *testing.T) {
	tmpl := questNPCTemplate("Cap.Mercenario", 10, 0, 0)
	db := newDB()
	db.loadResult = baseMortalState(100)
	addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
	defer stop()
	c := enterWorld(t, addr) // "tester", conn 1
	defer c.Close()
	c2 := enterWorldAs(t, addr, "tradeb") // conn 2
	defer c2.Close()

	reqPartyFrame(t, c, 1, 2)
	if ty, _, ok := readMaybe(t, c2); !ok || ty != protocol.MsgSendReqParty {
		t.Fatalf("invitee got %#x ok=%v, want SendReqParty", ty, ok)
	}
	acceptPartyFrame(t, c2, 1, "Hero")
	expect(t, c, protocol.MsgCNFAddParty)
	expect(t, c2, protocol.MsgCNFAddParty)
	drainRaw(t, c)
	drainRaw(t, c2)

	send(t, c, protocol.MsgQuest, protocol.EncodeStandardParm2(int32(npcID), 1))
	if ty, _, ok := readMaybe(t, c); ok {
		t.Fatalf("first completion produced %#x; legacy has no wire confirmation", ty)
	}

	send(t, c, protocol.MsgQuest, protocol.EncodeStandardParm2(int32(npcID), 1))
	if code := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); code != NoticeReqNotMet {
		t.Fatalf("second attempt notice = %d, want NoticeReqNotMet (already done)", code)
	}
}

func TestAmuMisticoSoloNoop(t *testing.T) {
	tmpl := questNPCTemplate("Cap.Mercenario", 10, 0, 0)
	db := newDB()
	db.loadResult = baseMortalState(100)
	addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgQuest, protocol.EncodeStandardParm2(int32(npcID), 1))
	if code := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); code != NoticeReqNotMet {
		t.Fatalf("solo notice = %d, want NoticeReqNotMet", code)
	}
}

// TestBlackOracleMergesSoulItems: the source items sit in high carry slots
// (10-12) so the reward — which lands in the first EMPTY slot after the
// sources are cleared, same as legacy PutItem — unambiguously lands in slot 0
// rather than reusing a just-cleared source slot.
func TestBlackOracleMergesSoulItems(t *testing.T) {
	tmpl := questNPCTemplate("Oraculo_Negro", 78, 0, 0)
	db := newDB()
	st := baseMortalState(300)
	st.Carry[10] = world.Item{Index: 1740}
	st.Carry[11] = world.Item{Index: 1741}
	st.Carry[12] = world.Item{Index: 4131} // 10 sapphire units in one stack
	db.loadResult = st
	addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	questFrame(t, c, npcID)
	got := map[uint16]uint16{}
	for i := 0; i < 4; i++ { // slot10, slot11, sapphire slot12, reward slot0
		item := expect(t, c, protocol.MsgSendItem)
		got[le16(item[2:4])] = le16(item[4:6])
	}
	if idx, ok := got[10]; !ok || idx != 0 {
		t.Fatalf("slot 10 (soul1) = idx %d ok=%v, want cleared", idx, ok)
	}
	if idx, ok := got[11]; !ok || idx != 0 {
		t.Fatalf("slot 11 (soul2) = idx %d ok=%v, want cleared", idx, ok)
	}
	if idx, ok := got[12]; !ok || idx != 0 {
		t.Fatalf("slot 12 (sapphire) = idx %d ok=%v, want cleared", idx, ok)
	}
	if idx, ok := got[0]; !ok || idx != idealStoneItem {
		t.Fatalf("slot 0 (reward) = idx %d ok=%v, want Pedra Ideal %d", idx, ok, idealStoneItem)
	}
}

func TestBlackOracleMissingSoulNoop(t *testing.T) {
	tmpl := questNPCTemplate("Oraculo_Negro", 78, 0, 0)
	db := newDB()
	st := baseMortalState(300)
	st.Carry[0] = world.Item{Index: 1740} // soul2 missing
	st.Carry[1] = world.Item{Index: 4131}
	db.loadResult = st
	addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	questFrame(t, c, npcID)
	if code := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); code != NoticeReqNotMet {
		t.Fatalf("missing-soul notice = %d, want NoticeReqNotMet", code)
	}
}

func TestCompSephiCraftsSephirotForClass(t *testing.T) {
	tmpl := questNPCTemplate("Composicao_Sephi", 19, 0, 0)
	// The template's Class field (offset 39 per STRUCT_MOB, same as world.Entity.Class)
	// selects which of the 4 Sephirot the NPC crafts; leave at 0 (TransKnight → 1760).
	db := newDB()
	st := baseMortalState(300)
	st.Coin = 30_000_000
	for j := 0; j < 8; j++ {
		st.Carry[j] = world.Item{Index: int16(1744 + j)}
	}
	db.loadResult = st
	addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgQuest, protocol.EncodeStandardParm2(int32(npcID), 1))
	expect(t, c, protocol.MsgUpdateEtc) // coin deducted
	got := map[uint16]uint16{}
	for i := 0; i < 9; i++ { // 8 consumed Pedras + 1 reward
		item := expect(t, c, protocol.MsgSendItem)
		got[le16(item[2:4])] = le16(item[4:6])
	}
	for j := 0; j < 8; j++ {
		if idx, ok := got[uint16(j)]; !ok || idx != 0 {
			t.Fatalf("pedra slot %d = idx %d ok=%v, want cleared", j, idx, ok)
		}
	}
	if idx, ok := got[8]; !ok || idx != archSephirotMin {
		t.Fatalf("reward slot 8 = idx %d ok=%v, want Sephirot %d", idx, ok, archSephirotMin)
	}
}

func TestCompSephiNotEnoughCoinNoop(t *testing.T) {
	tmpl := questNPCTemplate("Composicao_Sephi", 19, 0, 0)
	db := newDB()
	st := baseMortalState(300)
	st.Coin = 1_000 // short
	for j := 0; j < 8; j++ {
		st.Carry[j] = world.Item{Index: int16(1744 + j)}
	}
	db.loadResult = st
	addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgQuest, protocol.EncodeStandardParm2(int32(npcID), 1))
	if code := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); code != NoticeReqNotMet {
		t.Fatalf("short-coin notice = %d, want NoticeReqNotMet", code)
	}
}

func TestCompSephiConfirmZeroIsAskOnly(t *testing.T) {
	tmpl := questNPCTemplate("Composicao_Sephi", 19, 0, 0)
	db := newDB()
	st := baseMortalState(300)
	st.Coin = 30_000_000
	for j := 0; j < 8; j++ {
		st.Carry[j] = world.Item{Index: int16(1744 + j)}
	}
	db.loadResult = st
	addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	questFrame(t, c, npcID) // confirm=0
	if ty, _, ok := readMaybe(t, c); ok {
		t.Fatalf("confirm=0 produced %#x; want ask-only no-op", ty)
	}
}

// TestKingArchRejectsClanMismatch: the King's faction gate rejects a Mortal
// whose effective Clan doesn't match the King's, even with the right gear and
// level (_MSG_Quest.cpp:696-741).
func TestKingArchRejectsClanMismatch(t *testing.T) {
	tmpl := questNPCTemplate("Rei_Harabard", 111, 0, 7) // King is Clan 7
	db := newDB()
	st := baseMortalState(299)
	st.Equip[0] = world.Item{Index: 21}
	st.Equip[10] = world.Item{Index: 1742}
	st.Equip[11] = world.Item{Index: 1762}
	st.Clan = 8 // player is Clan 8 — mismatch
	db.loadResult = st
	addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgQuest, protocol.EncodeStandardParm2(int32(npcID), 1))
	if ty, _, ok := readMaybe(t, c); ok {
		t.Fatalf("clan mismatch produced %#x; want no-op", ty)
	}
}

// TestKingArchRejectsMissingPedraIdeal: without item 1742 equipped in slot 10,
// the Sephirot alone is not enough (_MSG_Quest.cpp:769).
func TestKingArchRejectsMissingPedraIdeal(t *testing.T) {
	tmpl := questNPCTemplate("Rei_Harabard", 111, 0, 0)
	db := newDB()
	st := baseMortalState(299)
	st.Equip[0] = world.Item{Index: 21}
	st.Equip[11] = world.Item{Index: 1762} // Sephirot present, Pedra Ideal missing
	db.loadResult = st
	addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgQuest, protocol.EncodeStandardParm2(int32(npcID), 1))
	if ty, _, ok := readMaybe(t, c); ok {
		t.Fatalf("missing Pedra Ideal produced %#x; want no-op", ty)
	}
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
