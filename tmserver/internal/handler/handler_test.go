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

// --- in-memory fake dbServer ---

type fakeAccount struct {
	id             int64
	pass           string
	blocked        bool
	alreadyPlaying bool
	chars          []world.CharSummary
}

type fakeDB struct {
	world.NopPersistence
	accounts   map[string]*fakeAccount
	created    int
	deleted    int
	loadResult world.CharacterState
	loads      map[int64]world.CharacterState // per-account override (accountID → state)
	loadErr    error
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
		return world.LoginOutcome{Result: world.LoginOK, AccountID: a.id, Characters: a.chars}, nil
	}
}

func (f *fakeDB) CreateCharacter(context.Context, int64, int, string, int) (bool, error) {
	f.created++
	return true, nil
}

func (f *fakeDB) DeleteCharacter(context.Context, int64, int, string, string) (bool, error) {
	f.deleted++
	return true, nil
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
		if h.Type == protocol.MsgCreateMob || h.Type == protocol.MsgRemoveMob {
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
		"tester": {id: 7, pass: "secret", chars: []world.CharSummary{{Slot: 0, Name: "Hero", Class: 1, Level: 50}}},
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
	if lvl := binary.LittleEndian.Uint32(payload[100:104]); lvl != 50 {
		t.Errorf("slot-0 level = %d, want 50", lvl)
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
	c.SetReadDeadline(time.Now().Add(time.Second))
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
	if ty, _ := read(t, c); ty != protocol.MsgCNFDeleteCharacter {
		t.Errorf("got %#x, want CNFDeleteCharacter", ty)
	}
	if db.deleted != 1 {
		t.Errorf("backend DeleteCharacter called %d times, want 1", db.deleted)
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

	// Now in play → logout returns to selection.
	send(t, c, protocol.MsgCharacterLogout, nil)
	if ty, _ := read(t, c); ty != protocol.MsgCNFCharacterLogout {
		t.Errorf("got %#x, want CNFCharacterLogout", ty)
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
