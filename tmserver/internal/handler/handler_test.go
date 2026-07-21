package handler

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// --- in-memory fake dbServer ---

type fakeAccount struct {
	id             int64
	pass           string
	role           string // account.role ('player'/'moderator'/'admin'); GM authz
	blocked        bool
	alreadyPlaying bool
	chars          []world.CharSummary
	cargo          world.CargoState // account-shared warehouse loaded on login
}

type fakeDB struct {
	world.NopPersistence
	accounts    map[string]*fakeAccount
	created     int
	archCreated int
	archSlot    int
	archOK      bool
	archErr     error
	archReq     struct {
		accountID               int64
		name                    string
		class, face, mortalSlot int
	}
	deleted    int
	loadResult world.CharacterState
	loads      map[int64]world.CharacterState // per-account override (accountID → state)
	loadErr    error

	pending map[int64][]world.Delivery // accountID -> mailbox rows (donate drain)

	pinVerify    world.PinResult // VerifyPin result (default PinOK)
	pinVerifyErr error           // forces VerifyPin to error
	pinSetOK     bool            // SetPin ok flag
	pinSets      []string        // captured SetPin plaintext (test-only; prod never stores plaintext)

	mu           sync.Mutex
	savedChars   []world.CharacterSave // captured SaveOnShutdown calls
	savedCargos  []world.CargoSave     // captured SaveCargo calls
	drainSaves   []drainSave           // captured SaveCargoWithDeliveries calls
	blockedNames map[string]bool       // captured SetAccountBlocked calls (GM ban/unban)
	duelResults  []duelResult          // captured RecordDuelResult calls (issue #118)

	createdGuilds []world.GuildRecord
	guildCosts    []int32
	promoted      []uint8
	promoteCosts  []int32
	transfers     int
}

// duelResult captures one RecordDuelResult call for assertions.
type duelResult struct{ winner, loser string }

func (f *fakeDB) VerifyPin(_ context.Context, _ int64, _ string) (world.PinResult, error) {
	return f.pinVerify, f.pinVerifyErr
}

func (f *fakeDB) SetPin(_ context.Context, _ int64, pin string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pinSets = append(f.pinSets, pin)
	return f.pinSetOK, nil
}

func (f *fakeDB) setPinCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.pinSets)
}

// drainSave captures one SaveCargoWithDeliveries call for assertions.
type drainSave struct {
	save      world.CargoSave
	delivered []int64
	lost      []int64
}

func (f *fakeDB) SaveOnShutdown(_ context.Context, save world.CharacterSave) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.savedChars = append(f.savedChars, save)
	return nil
}

func (f *fakeDB) SaveCargo(_ context.Context, save world.CargoSave) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.savedCargos = append(f.savedCargos, save)
	return nil
}

func (f *fakeDB) ListPendingDeliveries(_ context.Context, accountID int64) ([]world.Delivery, error) {
	return f.pending[accountID], nil
}

// SetAccountBlocked records the GM ban/unban write (overrides the NopPersistence
// error so the ban callback proceeds to the online-kick).
func (f *fakeDB) SetAccountBlocked(_ context.Context, name string, blocked bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.blockedNames == nil {
		f.blockedNames = make(map[string]bool)
	}
	f.blockedNames[name] = blocked
	return nil
}

func (f *fakeDB) CreateGuild(_ context.Context, _ int64, _ int, _, guildName string, clan, citizen uint8, _ int, cost int32) (world.GuildRecord, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	g := world.GuildRecord{ID: uint16(100 + len(f.createdGuilds)), Name: guildName, Clan: clan, Citizen: citizen}
	f.createdGuilds = append(f.createdGuilds, g)
	f.guildCosts = append(f.guildCosts, cost)
	return g, true, nil
}

func (f *fakeDB) SetGuildMember(context.Context, int64, int, string, uint16, uint8) error {
	return nil
}

func (f *fakeDB) LeaveGuild(context.Context, int64, int) error { return nil }

func (f *fakeDB) PromoteGuildMember(_ context.Context, _ uint16, _ int64, _ int, _ int64, _ int, cost int32) (uint8, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	level := uint8(6 + len(f.promoted))
	f.promoted = append(f.promoted, level)
	f.promoteCosts = append(f.promoteCosts, cost)
	return level, true, nil
}

func (f *fakeDB) TransferGuildLeader(context.Context, uint16, int64, int, int64, int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.transfers++
	return nil
}

func (f *fakeDB) SetGuildRelation(context.Context, uint16, uint16, world.GuildRelationKind) error {
	return nil
}

func (f *fakeDB) ListGuilds(context.Context) ([]world.GuildRecord, error) { return nil, nil }

func (f *fakeDB) ListGuildRelations(context.Context) ([]world.GuildRelation, error) {
	return nil, nil
}

func (f *fakeDB) LoadGuildZones(context.Context) ([]world.GuildZone, error) { return nil, nil }

func (f *fakeDB) SaveGuildZone(context.Context, world.GuildZone) error { return nil }

func (f *fakeDB) LoadGuildTowerState(context.Context) (world.GuildTowerState, error) {
	return world.GuildTowerState{}, nil
}

func (f *fakeDB) SaveGuildTowerState(context.Context, world.GuildTowerState) error {
	return nil
}

func (f *fakeDB) LoadCastleQuestState(context.Context) (world.CastleQuestState, error) {
	return world.CastleQuestState{}, nil
}

func (f *fakeDB) SaveCastleQuestState(context.Context, world.CastleQuestState) error {
	return nil
}

// blockedState reports the last recorded block state for an account name, waiting
// briefly for the async ban RPC round-trip to land.
func (f *fakeDB) blockedState(t *testing.T, name string) (bool, bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		v, ok := f.blockedNames[name]
		f.mu.Unlock()
		if ok {
			return v, true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false, false
}

func (f *fakeDB) SaveCargoWithDeliveries(_ context.Context, save world.CargoSave, deliveredIDs, lostIDs []int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.drainSaves = append(f.drainSaves, drainSave{save: save, delivered: deliveredIDs, lost: lostIDs})
	return nil
}

// RecordDuelResult records the duel win/loss write (overrides the
// NopPersistence error so sweepDuelArena's off-loop call proceeds).
func (f *fakeDB) RecordDuelResult(_ context.Context, winnerName, loserName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.duelResults = append(f.duelResults, duelResult{winner: winnerName, loser: loserName})
	return nil
}

// lastDuelResult returns the most recent RecordDuelResult capture, waiting
// briefly for the async result round-trip to land.
func (f *fakeDB) lastDuelResult(t *testing.T) (duelResult, bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		n := len(f.duelResults)
		if n > 0 {
			dr := f.duelResults[n-1]
			f.mu.Unlock()
			return dr, true
		}
		f.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	return duelResult{}, false
}

// lastDrainSave returns the most recent SaveCargoWithDeliveries capture, waiting
// briefly for the async drain's off-loop save round-trip to land.
func (f *fakeDB) lastDrainSave(t *testing.T) (drainSave, bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		n := len(f.drainSaves)
		if n > 0 {
			ds := f.drainSaves[n-1]
			f.mu.Unlock()
			return ds, true
		}
		f.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	return drainSave{}, false
}

// lastSavedCargo returns the most recent SaveCargo snapshot (and how many landed).
func (f *fakeDB) lastSavedCargo() (world.CargoSave, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.savedCargos) == 0 {
		return world.CargoSave{}, 0
	}
	return f.savedCargos[len(f.savedCargos)-1], len(f.savedCargos)
}

// lastSavedChar returns the most recent SaveOnShutdown snapshot.
func (f *fakeDB) lastSavedChar() (world.CharacterSave, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.savedChars) == 0 {
		return world.CharacterSave{}, 0
	}
	return f.savedChars[len(f.savedChars)-1], len(f.savedChars)
}

func (f *fakeDB) archRequest() (int, int64, string, int, int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.archCreated, f.archReq.accountID, f.archReq.name, f.archReq.class, f.archReq.face, f.archReq.mortalSlot
}

func (f *fakeDB) AccountLogin(_ context.Context, name, pass string) (world.LoginOutcome, error) {
	a, ok := f.accounts[name]
	switch {
	case !ok:
		return world.LoginOutcome{Result: world.LoginNoAccount}, nil
	case a.blocked:
		return world.LoginOutcome{Result: world.LoginBlocked}, nil
	case a.alreadyPlaying:
		return world.LoginOutcome{Result: world.LoginAlreadyPlaying}, nil
	case a.pass != pass:
		return world.LoginOutcome{Result: world.LoginBadPassword}, nil
	default:
		cargo := a.cargo
		cargo.AccountID = a.id
		return world.LoginOutcome{
			Result: world.LoginOK, AccountID: a.id, Role: a.role, Characters: a.chars, Cargo: cargo,
			PendingDeliveries: f.pending[a.id],
		}, nil
	}
}

func (f *fakeDB) CreateCharacter(context.Context, int64, int, string, int) (bool, error) {
	f.created++
	return true, nil
}

func (f *fakeDB) CreateArchCharacter(_ context.Context, accountID int64, name string, class, mortalFace, mortalSlot int) (int, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.archCreated++
	f.archReq.accountID = accountID
	f.archReq.name = name
	f.archReq.class = class
	f.archReq.face = mortalFace
	f.archReq.mortalSlot = mortalSlot
	if f.archErr != nil {
		return 0, false, f.archErr
	}
	ok := f.archOK
	if !ok && f.archSlot == 0 {
		return 0, false, nil
	}
	slot := f.archSlot
	if slot == 0 {
		slot = 1
	}
	for _, a := range f.accounts {
		if a.id != accountID {
			continue
		}
		a.chars = append(a.chars, world.CharSummary{Slot: slot, Name: name, Class: class, Level: 1})
		break
	}
	return slot, ok, nil
}

func (f *fakeDB) DeleteCharacter(_ context.Context, accountID int64, slot int, _, _ string) (bool, error) {
	f.deleted++
	for _, a := range f.accounts {
		if a.id != accountID {
			continue
		}
		kept := a.chars[:0]
		for _, c := range a.chars {
			if c.Slot != slot {
				kept = append(kept, c)
			}
		}
		a.chars = kept
	}
	return true, nil
}

// ListCharacters mirrors the account's remaining chars (minus deletions), so
// tests can assert the post-delete SELCHAR response reflects the real list
// instead of an empty/stale one.
func (f *fakeDB) ListCharacters(_ context.Context, accountID int64) ([]world.CharSummary, error) {
	for _, a := range f.accounts {
		if a.id == accountID {
			return a.chars, nil
		}
	}
	return nil, nil
}

func (f *fakeDB) LoadCharacter(_ context.Context, accountID int64, _ int) (world.CharacterState, error) {
	if st, ok := f.loads[accountID]; ok {
		return st, f.loadErr
	}
	return f.loadResult, f.loadErr
}

// --- harness ---

func startServer(t *testing.T, persist world.Persistence) (string, func()) {
	return startServerBilling(t, persist, nil)
}

// denyBilling refuses every account (binServer "expired/blocked" path).
type denyBilling struct{}

func (denyBilling) Check(context.Context, string) (bool, error) { return false, nil }

// startServerBilling is startServer with an injected billing gate (nil keeps the
// AllowAllBilling default).
func startServerBilling(t *testing.T, persist world.Persistence, b world.Billing) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 16}, log, persist, d.Handle)
	w.SetBilling(b)
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

func startServerBaseMobs(t *testing.T, persist world.Persistence, baseMobs map[int][]byte) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, BaseMobs: baseMobs})
	w := world.New(world.Config{}, log, persist, d.Handle)
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

func dial(t *testing.T, addr string) net.Conn {
	t.Helper()
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	var ic [4]byte
	binary.LittleEndian.PutUint32(ic[:], protocol.InitCode)
	if _, err := c.Write(ic[:]); err != nil {
		t.Fatal(err)
	}
	return c
}

func send(t *testing.T, c net.Conn, ty protocol.Type, payload []byte) {
	t.Helper()
	wire, err := protocol.Encode(protocol.Header{Type: ty}, payload, 7)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write(wire); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, c net.Conn) (protocol.Type, []byte) {
	t.Helper()
	for {
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		var sz [2]byte
		if _, err := io.ReadFull(c, sz[:]); err != nil {
			t.Fatalf("read size: %v", err)
		}
		buf := make([]byte, binary.LittleEndian.Uint16(sz[:]))
		copy(buf, sz[:])
		if _, err := io.ReadFull(c, buf[2:]); err != nil {
			t.Fatalf("read body: %v", err)
		}
		h, payload, _, err := protocol.Decode(buf)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		// Entity-visibility packets are background noise for gameplay assertions.
		if h.Type == protocol.MsgCreateMob || h.Type == protocol.MsgRemoveMob || h.Type == protocol.MsgPKInfo {
			continue
		}
		return h.Type, payload
	}
}

func loginBody(name, pass string, version int32) []byte {
	var b protocol.MsgAccountLoginBody
	copy(b.AccountName[:], name)
	copy(b.AccountPassword[:], pass)
	b.ClientVersion = version
	return b.Encode()
}

func noticeCode(t *testing.T, payload []byte) Notice {
	t.Helper()
	if len(payload) < 4 {
		t.Fatalf("notice payload too short: %d", len(payload))
	}
	return Notice(binary.LittleEndian.Uint32(payload))
}

// --- tests ---

func newDB() *fakeDB {
	return &fakeDB{accounts: map[string]*fakeAccount{
		"tester": {id: 7, pass: "secret", chars: []world.CharSummary{
			{Slot: 0, Name: "Hero", Class: 1, Level: 50, Coin: 987654, MaxHp: 1500, Str: 75},
			{Slot: 1, Name: "Sidekick", Class: 1, Level: 10},
		}},
		"banned": {id: 8, pass: "x", blocked: true},
		"online": {id: 9, pass: "x", alreadyPlaying: true},
		"tradeb": {id: 11, pass: "secret", chars: []world.CharSummary{{Slot: 0, Name: "HeroB", Class: 1, Level: 50}}},
	}}
}

func TestLoginOK(t *testing.T) {
	addr, stop := startServer(t, newDB())
	defer stop()
	c := dial(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgAccountLogin, loginBody("Tester", "secret", protocol.AppVersion))
	ty, payload := read(t, c)
	if ty != protocol.MsgCNFAccountLogin {
		t.Fatalf("response = %#x, want CNFAccountLogin", ty)
	}
	// Byte-exact STRUCT_SELCHAR: body is 1996 bytes; slot-0 name at body[36:52]
	// (sel@20 + Name[][]@16), slot-0 Score.Level at body[100:104] (sel@20 + Score@80).
	if len(payload) != 1996 {
		t.Fatalf("CNFAccountLogin body = %d bytes, want 1996", len(payload))
	}
	if got := cstr(payload[36:52]); got != "Hero" {
		t.Errorf("slot-0 name = %q, want Hero", got)
	}
	if lvl := binary.LittleEndian.Uint32(payload[100:104]); lvl != 49 {
		t.Errorf("slot-0 wire level = %d, want 49 so the client displays 50", lvl)
	}
	// slot-0 gold is the real value, not a placeholder: Coin[0] at sel@20 + 792.
	if coin := binary.LittleEndian.Uint32(payload[812:816]); coin != 987654 {
		t.Errorf("slot-0 coin = %d, want 987654", coin)
	}
	// slot-0 MaxHp is the real value: Score[0].MaxHp at sel@20 + 80 + 16 = 116.
	if hp := binary.LittleEndian.Uint32(payload[116:120]); hp != 1500 {
		t.Errorf("slot-0 max_hp = %d, want 1500", hp)
	}
}

func TestLoginSelectionUsesPersistedEquip(t *testing.T) {
	db := newDB()
	db.accounts["tester"].chars[0].Equip[1] = world.Item{
		Index:   4321,
		Effects: [3]world.Effect{{Effect: 1, Value: 9}},
	}

	addr, stop := startServer(t, db)
	defer stop()
	c := dial(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgAccountLogin, loginBody("Tester", "secret", protocol.AppVersion))
	ty, payload := read(t, c)
	if ty != protocol.MsgCNFAccountLogin {
		t.Fatalf("response = %#x, want CNFAccountLogin", ty)
	}
	const equip1Off = 20 + 272 + 8 // sel@20 + Equip[0][1] @272 + one STRUCT_ITEM
	if got := binary.LittleEndian.Uint16(payload[equip1Off : equip1Off+2]); got != 4321 {
		t.Fatalf("slot-0 equip[1] = %d, want persisted item 4321", got)
	}
	if got := payload[equip1Off+3]; got != 9 {
		t.Fatalf("slot-0 equip[1] EffV1 = %d, want 9", got)
	}
}

func TestSelCharsFromFallsBackToBaseMobEquip(t *testing.T) {
	tmpl := make([]byte, content.BaseMobSize)
	binary.LittleEndian.PutUint16(tmpl[140+8:140+10], 2222)
	d := &Dispatcher{baseMobs: map[int][]byte{1: tmpl}}

	chars := d.selCharsFrom([]world.CharSummary{{Slot: 0, Name: "fresh", Class: 1}})
	if len(chars) != 1 {
		t.Fatalf("sel chars = %d, want 1", len(chars))
	}
	if got := chars[0].Equip[1].Index; got != 2222 {
		t.Fatalf("fallback equip[1] = %d, want BaseMob item 2222", got)
	}
}

func TestLoginSendsCargo(t *testing.T) {
	db := newDB()
	cargo := world.CargoState{Coin: 777}
	cargo.Items[0] = world.Item{Index: 4140}
	cargo.Items[33] = world.Item{Index: 5134}
	db.accounts["tester"].cargo = cargo

	addr, stop := startServer(t, db)
	defer stop()
	c := dial(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgAccountLogin, loginBody("tester", "secret", protocol.AppVersion))
	ty, payload := read(t, c)
	if ty != protocol.MsgCNFAccountLogin {
		t.Fatalf("response = %#x, want CNFAccountLogin", ty)
	}
	const cargoOff, coinOff = 860, 1884
	if got := binary.LittleEndian.Uint16(payload[cargoOff : cargoOff+2]); got != 4140 {
		t.Errorf("cargo[0] = %d, want 4140", got)
	}
	if got := binary.LittleEndian.Uint16(payload[cargoOff+33*8 : cargoOff+33*8+2]); got != 5134 {
		t.Errorf("cargo[33] = %d, want 5134", got)
	}
	if got := binary.LittleEndian.Uint32(payload[coinOff : coinOff+4]); got != 777 {
		t.Errorf("cargo coin = %d, want 777", got)
	}
}

func TestSelCharWireLevel(t *testing.T) {
	tests := []struct {
		name  string
		level int
		want  int32
	}{
		{name: "zero", level: 0, want: 0},
		{name: "one", level: 1, want: 0},
		{name: "normal", level: 50, want: 49},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selCharWireLevel(tt.level); got != tt.want {
				t.Errorf("selCharWireLevel(%d) = %d, want %d", tt.level, got, tt.want)
			}
		})
	}
}

func TestLoginBadVersionClosed(t *testing.T) {
	addr, stop := startServer(t, newDB())
	defer stop()
	c := dial(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgAccountLogin, loginBody("tester", "secret", 1234))
	ty, payload := read(t, c)
	if ty != protocol.MsgMessageBoxOk || noticeCode(t, payload) != NoticeVersionMismatch {
		t.Errorf("got %#x/%d, want version-mismatch notice", ty, noticeCode(t, payload))
	}
	// Connection must be closed after a version mismatch.
	if err := c.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Read(make([]byte, 1)); err == nil {
		t.Errorf("expected connection to be closed")
	}
}

func TestLoginBadPasswordThenLockout(t *testing.T) {
	addr, stop := startServer(t, newDB())
	defer stop()
	c := dial(t, addr)
	defer c.Close()

	// Three wrong passwords are accepted (mode resets each time)...
	for i := 0; i < 3; i++ {
		send(t, c, protocol.MsgAccountLogin, loginBody("tester", "wrong", protocol.AppVersion))
		ty, payload := read(t, c)
		if ty != protocol.MsgMessageBoxOk || noticeCode(t, payload) != NoticeBadPass {
			t.Fatalf("attempt %d: got %#x/%d, want bad-pass", i, ty, noticeCode(t, payload))
		}
	}
	// ...the fourth is locked out before hitting the backend.
	send(t, c, protocol.MsgAccountLogin, loginBody("tester", "secret", protocol.AppVersion))
	ty, payload := read(t, c)
	if ty != protocol.MsgMessageBoxOk || noticeCode(t, payload) != Notice3WrongPass {
		t.Errorf("got %#x/%d, want 3-wrong-pass lockout", ty, noticeCode(t, payload))
	}
}

func TestLoginNoAccountAndBlocked(t *testing.T) {
	addr, stop := startServer(t, newDB())
	defer stop()

	c1 := dial(t, addr)
	defer c1.Close()
	send(t, c1, protocol.MsgAccountLogin, loginBody("ghost", "x", protocol.AppVersion))
	if ty, p := read(t, c1); ty != protocol.MsgMessageBoxOk || noticeCode(t, p) != NoticeNoAccount {
		t.Errorf("no-account: got %#x/%d", ty, noticeCode(t, p))
	}

	c2 := dial(t, addr)
	defer c2.Close()
	send(t, c2, protocol.MsgAccountLogin, loginBody("banned", "x", protocol.AppVersion))
	if ty, p := read(t, c2); ty != protocol.MsgMessageBoxOk || noticeCode(t, p) != NoticeBlocked {
		t.Errorf("blocked: got %#x/%d", ty, noticeCode(t, p))
	}
}

func TestWrongModeRejectsSecondLogin(t *testing.T) {
	addr, stop := startServer(t, newDB())
	defer stop()
	c := dial(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgAccountLogin, loginBody("tester", "secret", protocol.AppVersion))
	read(t, c) // CNFAccountLogin → now in SELCHAR
	send(t, c, protocol.MsgAccountLogin, loginBody("tester", "secret", protocol.AppVersion))
	if ty, p := read(t, c); ty != protocol.MsgMessageBoxOk || noticeCode(t, p) != NoticeLoginNow {
		t.Errorf("second login: got %#x/%d, want login-now notice", ty, noticeCode(t, p))
	}
}

// loginAndSelect logs in successfully and returns the connection in SELCHAR.
func loginAndSelect(t *testing.T, addr string) net.Conn {
	t.Helper()
	c := dial(t, addr)
	send(t, c, protocol.MsgAccountLogin, loginBody("tester", "secret", protocol.AppVersion))
	if ty, _ := read(t, c); ty != protocol.MsgCNFAccountLogin {
		t.Fatalf("login failed, got %#x", ty)
	}
	return c
}

func TestCreateCharacter(t *testing.T) {
	db := newDB()
	addr, stop := startServer(t, db)
	defer stop()
	c := loginAndSelect(t, addr)
	defer c.Close()

	var body protocol.MsgCreateCharacterBody
	body.Slot = 1
	copy(body.MobName[:], "NewHero")
	body.MobClass = 2
	send(t, c, protocol.MsgCreateCharacter, body.Encode())
	if ty, _ := read(t, c); ty != protocol.MsgCNFNewCharacter {
		t.Errorf("got %#x, want CNFNewCharacter", ty)
	}
	if db.created != 1 {
		t.Errorf("backend CreateCharacter called %d times, want 1", db.created)
	}
}

func TestCreateCharacterInvalidName(t *testing.T) {
	db := newDB()
	addr, stop := startServer(t, db)
	defer stop()
	c := loginAndSelect(t, addr)
	defer c.Close()

	var body protocol.MsgCreateCharacterBody
	copy(body.MobName[:], "bad name!") // space + '!' are invalid
	send(t, c, protocol.MsgCreateCharacter, body.Encode())
	if ty, _ := read(t, c); ty != protocol.MsgNewCharacterFail {
		t.Errorf("got %#x, want NewCharacterFail", ty)
	}
	if db.created != 0 {
		t.Errorf("backend should not be called for an invalid name")
	}
}

func TestDeleteCharacter(t *testing.T) {
	db := newDB()
	addr, stop := startServer(t, db)
	defer stop()
	c := loginAndSelect(t, addr)
	defer c.Close()

	var body protocol.MsgDeleteCharacterBody
	body.Slot = 0
	copy(body.MobName[:], "Hero")
	copy(body.Password[:], "secret")
	send(t, c, protocol.MsgDeleteCharacter, body.Encode())
	ty, payload := read(t, c)
	if ty != protocol.MsgCNFDeleteCharacter {
		t.Fatalf("got %#x, want CNFDeleteCharacter", ty)
	}
	if db.deleted != 1 {
		t.Errorf("backend DeleteCharacter called %d times, want 1", db.deleted)
	}
	// MSG_CNFDeleteCharacter (Basedef.h:1646-1652) carries a full STRUCT_SELCHAR,
	// byte-identical to MSG_CNFNewCharacter's body (sel@4, 840 bytes). Sending an
	// empty payload here leaves the client's pre-delete slot array stale, which is
	// the "other characters disappear / bugged entries" bug (issue #20).
	if len(payload) != 844 {
		t.Fatalf("CNFDeleteCharacter body = %d bytes, want 844 (refreshed SELCHAR)", len(payload))
	}
	if got := cstr(payload[4+16 : 4+16+16]); got != "" {
		t.Errorf("deleted slot-0 name = %q, want empty", got)
	}
	if got := cstr(payload[4+16+16 : 4+16+32]); got != "Sidekick" {
		t.Errorf("slot-1 name = %q, want Sidekick (remaining char must still be reported)", got)
	}
}

func TestCharacterLoginAndLogout(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{Slot: 0, Name: "Hero", X: 2100, Y: 2100, HP: 1200, MaxHP: 1200}
	addr, stop := startServer(t, db)
	defer stop()
	c := loginAndSelect(t, addr)
	defer c.Close()

	var body protocol.MsgCharacterLoginBody
	body.Slot = 0
	send(t, c, protocol.MsgCharacterLogin, body.Encode())
	if ty, _ := read(t, c); ty != protocol.MsgCNFCharacterLogin {
		t.Fatalf("got %#x, want CNFCharacterLogin", ty)
	}
	drainLoginScore(t, c)

	// Now in play → logout returns to selection.
	send(t, c, protocol.MsgCharacterLogout, nil)
	if ty, _ := read(t, c); ty != protocol.MsgCNFCharacterLogout {
		t.Errorf("got %#x, want CNFCharacterLogout", ty)
	}
}

// TestCharacterLoginColorsOwnNickWhite is the login-time regression guard for
// issue #59: a freshly logged-in character must (a) get its OWN CreateMob (the
// legacy self-CreateMob that colors the own nick) and (b) that CreateMob's
// MobName[12] (PKPoint) must be 75 = neutral/white — NOT 0, which renders the
// nick red/blinking. Both the login blob and the self-CreateMob carry PKPoint 75.
func TestCharacterLoginColorsOwnNickWhite(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{Slot: 0, Name: "Hero", X: 2100, Y: 2100, HP: 1200, MaxHP: 1200}
	addr, stop := startServer(t, db)
	defer stop()
	c := loginAndSelect(t, addr)
	defer c.Close()

	var body protocol.MsgCharacterLoginBody
	body.Slot = 0
	send(t, c, protocol.MsgCharacterLogin, body.Encode())
	// The login blob's own MobName[12] must be 75 (white). mob @ body4, MobName[0].
	ty, payload, ok := readMaybeRaw(t, c)
	if !ok || ty != protocol.MsgCNFCharacterLogin {
		t.Fatalf("got %#x ok=%v, want CNFCharacterLogin", ty, ok)
	}
	if payload[4+12] != 75 {
		t.Errorf("login blob MobName[12] = %d, want 75 (neutral/white nick)", payload[4+12])
	}
	// The self-CreateMob (id = own conn) must also carry MobName[12] = 75. Scan the
	// post-login frames for a CreateMob whose MobID is the player's conn.
	found := false
	for i := 0; i < 20 && !found; i++ {
		ty, payload, ok = readMaybeRaw(t, c)
		if !ok {
			break
		}
		if ty != protocol.MsgCreateMob {
			continue
		}
		_, _, mobID := createMobFields(t, payload)
		if mobID == 1 { // first (only) player conn
			found = true
			if payload[6+12] != 75 { // MobName @ body6, [12] = PKPoint
				t.Errorf("self-CreateMob MobName[12] = %d, want 75 (white)", payload[6+12])
			}
		}
	}
	if !found {
		t.Error("no self-CreateMob received at login — own nick can't be colored/recolored")
	}
}

func TestCharacterLoginFallbackCNFUsesComputedCitySpawn(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{Slot: 0, Name: "Hero", Level: 50, LastCity: 2, HP: 1200, MaxHP: 1200}
	addr, stop := startServer(t, db)
	defer stop()
	c := loginAndSelect(t, addr)
	defer c.Close()

	var body protocol.MsgCharacterLoginBody
	body.Slot = 0
	send(t, c, protocol.MsgCharacterLogin, body.Encode())
	ty, payload := read(t, c)
	if ty != protocol.MsgCNFCharacterLogin {
		t.Fatalf("got %#x, want CNFCharacterLogin", ty)
	}
	if len(payload) != 1820 {
		t.Fatalf("CNFCharacterLogin body = %d, want 1820", len(payload))
	}

	msgX := int16(binary.LittleEndian.Uint16(payload[0:]))
	msgY := int16(binary.LittleEndian.Uint16(payload[2:]))
	if city := world.Village(msgX, msgY); city != int(db.loadResult.LastCity) {
		t.Fatalf("CNF spawn = %d,%d in city %d, want city %d", msgX, msgY, city, db.loadResult.LastCity)
	}
	if binary.LittleEndian.Uint16(payload[1028:]) != 0 ||
		binary.LittleEndian.Uint16(payload[1030:]) != 1 ||
		binary.LittleEndian.Uint16(payload[1032:]) != 0 {
		t.Fatalf("tail Slot/ClientID/Weather = %d/%d/%d, want 0/1/0",
			binary.LittleEndian.Uint16(payload[1028:]),
			binary.LittleEndian.Uint16(payload[1030:]),
			binary.LittleEndian.Uint16(payload[1032:]))
	}
}

func TestCharacterLoginTemplateCNFSeparatesLoginPosFromSavePoint(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{Slot: 0, Name: "Hero", Class: 1, Level: 50, LastCity: 2, HP: 1200, MaxHP: 1200}
	tmpl := make([]byte, content.BaseMobSize)
	copy(tmpl[0:16], "Template")
	binary.LittleEndian.PutUint16(tmpl[40:], 2112)
	binary.LittleEndian.PutUint16(tmpl[42:], 2112)
	addr, stop := startServerBaseMobs(t, db, map[int][]byte{1: tmpl})
	defer stop()
	c := loginAndSelect(t, addr)
	defer c.Close()

	var body protocol.MsgCharacterLoginBody
	body.Slot = 0
	send(t, c, protocol.MsgCharacterLogin, body.Encode())
	ty, payload := read(t, c)
	if ty != protocol.MsgCNFCharacterLogin {
		t.Fatalf("got %#x, want CNFCharacterLogin", ty)
	}

	msgX := int16(binary.LittleEndian.Uint16(payload[0:]))
	msgY := int16(binary.LittleEndian.Uint16(payload[2:]))
	if city := world.Village(msgX, msgY); city != int(db.loadResult.LastCity) {
		t.Fatalf("CNF login pos = %d,%d in city %d, want city %d", msgX, msgY, city, db.loadResult.LastCity)
	}
	mobSPX := int16(binary.LittleEndian.Uint16(payload[4+40:]))
	mobSPY := int16(binary.LittleEndian.Uint16(payload[4+42:]))
	if mobSPX != 2112 || mobSPY != 2112 {
		t.Fatalf("embedded SPX/SPY = %d,%d, want template save point 2112,2112", mobSPX, mobSPY)
	}
	if msgX == mobSPX && msgY == mobSPY {
		t.Fatalf("CNF login pos unexpectedly equals save point: %d,%d", msgX, msgY)
	}
}

func TestCharacterLoginLowLevelMortalUsesLastCitySpawn(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{Slot: 0, Name: "Hero", Class: 1, Level: 1, LastCity: 2, HP: 1200, MaxHP: 1200}
	tmpl := make([]byte, content.BaseMobSize)
	copy(tmpl[0:16], "Template")
	binary.LittleEndian.PutUint16(tmpl[40:], 2112)
	binary.LittleEndian.PutUint16(tmpl[42:], 2112)
	addr, stop := startServerBaseMobs(t, db, map[int][]byte{1: tmpl})
	defer stop()
	c := loginAndSelect(t, addr)
	defer c.Close()

	var body protocol.MsgCharacterLoginBody
	body.Slot = 0
	send(t, c, protocol.MsgCharacterLogin, body.Encode())
	ty, payload := read(t, c)
	if ty != protocol.MsgCNFCharacterLogin {
		t.Fatalf("got %#x, want CNFCharacterLogin", ty)
	}
	msgX := int16(binary.LittleEndian.Uint16(payload[0:]))
	msgY := int16(binary.LittleEndian.Uint16(payload[2:]))
	if city := world.Village(msgX, msgY); city != int(db.loadResult.LastCity) {
		t.Fatalf("low-level login pos = %d,%d in city %d, want last city %d", msgX, msgY, city, db.loadResult.LastCity)
	}
	if city := world.Village(msgX, msgY); city == 0 {
		t.Fatalf("low-level login pos = %d,%d spawned in Armia, want last city %d", msgX, msgY, db.loadResult.LastCity)
	}
}

func TestCharacterLoginBillingDenied(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{Slot: 0, Name: "Hero"}
	addr, stop := startServerBilling(t, db, denyBilling{})
	defer stop()
	c := loginAndSelect(t, addr)
	defer c.Close()

	var body protocol.MsgCharacterLoginBody
	body.Slot = 0
	send(t, c, protocol.MsgCharacterLogin, body.Encode())
	if ty, p := read(t, c); ty != protocol.MsgMessageBoxOk || noticeCode(t, p) != NoticeBillingDenied {
		t.Errorf("got %#x/%d, want billing-denied notice", ty, noticeCode(t, p))
	}
}

func TestCharacterLoginBadSlot(t *testing.T) {
	addr, stop := startServer(t, newDB())
	defer stop()
	c := loginAndSelect(t, addr)
	defer c.Close()

	var body protocol.MsgCharacterLoginBody
	body.Slot = 9 // out of [0,4)
	send(t, c, protocol.MsgCharacterLogin, body.Encode())
	if ty, p := read(t, c); ty != protocol.MsgMessageBoxOk || noticeCode(t, p) != NoticeSelectCharacter {
		t.Errorf("got %#x/%d, want select-character notice", ty, noticeCode(t, p))
	}
}
