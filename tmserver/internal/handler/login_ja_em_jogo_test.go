package handler

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// One account, one session (CFileDB.cpp:685-703). Until 11/09/2026 the port let
// a second login of the same account in beside the first, and two live copies
// of the same characters saved on their own.

// readHeader is read with the header kept, for the replies whose HEADER.ID is
// part of the parity (SendClientSignal puts ESCENE_FIELD+2 there).
func readHeader(t *testing.T, c net.Conn) protocol.Header {
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
		h, _, _, err := protocol.Decode(buf)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if h.Type == protocol.MsgCreateMob || h.Type == protocol.MsgRemoveMob || h.Type == protocol.MsgPKInfo {
			continue
		}
		return h
	}
}

// expectClosed drains c until the server hangs up. A read that times out
// instead means the connection was left open.
func expectClosed(t *testing.T, c net.Conn) {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1024)
	for {
		_, err := c.Read(buf)
		if err == nil {
			continue
		}
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			t.Errorf("connection still open")
		}
		return
	}
}

func loginBodyTakeOver(name, pass string) []byte {
	var b protocol.MsgAccountLoginBody
	copy(b.AccountName[:], name)
	copy(b.AccountPassword[:], pass)
	b.ClientVersion = protocol.AppVersion
	b.DBNeedSave = 1
	return b.Encode()
}

// loginUntilIn logs in on fresh connections until the account is let in. Right
// after a quit-save lands, the release is still riding back into the loop, so
// one more refusal is allowed; anything but "already playing" fails.
func loginUntilIn(t *testing.T, addr, name, pass string) net.Conn {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		c := dial(t, addr)
		send(t, c, protocol.MsgAccountLogin, loginBody(name, pass, protocol.AppVersion))
		h := readHeader(t, c)
		if h.Type == protocol.MsgCNFAccountLogin {
			return c
		}
		_ = c.Close()
		if h.Type != protocol.MsgAlreadyPlaying || time.Now().After(deadline) {
			t.Fatalf("login = %#x, want CNFAccountLogin once the account is free", h.Type)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func loginOK(t *testing.T, addr string) net.Conn {
	t.Helper()
	c := dial(t, addr)
	send(t, c, protocol.MsgAccountLogin, loginBody("tester", "secret", protocol.AppVersion))
	if ty, _ := read(t, c); ty != protocol.MsgCNFAccountLogin {
		t.Fatalf("login = %#x, want CNFAccountLogin", ty)
	}
	return c
}

// TestLoginAlreadyPlaying is the parity case login_already_playing
// (parity-tests.md §2.1): a second login of an account that is connected gets
// _MSG_AlreadyPlaying and is closed, and the first session carries on.
func TestLoginAlreadyPlaying(t *testing.T) {
	addr, stop := startServer(t, newDB())
	defer stop()
	c1 := loginOK(t, addr)
	defer c1.Close()

	c2 := dial(t, addr)
	defer c2.Close()
	send(t, c2, protocol.MsgAccountLogin, loginBody("tester", "secret", protocol.AppVersion))
	h := readHeader(t, c2)
	if h.Type != protocol.MsgAlreadyPlaying || h.ID != protocol.IDSelChar {
		t.Fatalf("second login = %#x id %d, want AlreadyPlaying id %d", h.Type, h.ID, protocol.IDSelChar)
	}
	expectClosed(t, c2)

	// The first session is untouched: still at the character screen, where a
	// repeated login is the "login now" refusal (TestWrongModeRejectsSecondLogin).
	send(t, c1, protocol.MsgAccountLogin, loginBody("tester", "secret", protocol.AppVersion))
	if ty, p := read(t, c1); ty != protocol.MsgMessageBoxOk || noticeCode(t, p) != NoticeLoginNow {
		t.Errorf("first session after the refusal: got %#x, want it still at the character screen", ty)
	}
}

// TestLoginTakeOverClosesOldSession: with DBNeedSave set, the new connection
// gets _MSG_StillPlaying and is closed, the session holding the account is told
// _NN_Your_Account_From_Others and closed with its save, and the next attempt
// gets in.
func TestLoginTakeOverClosesOldSession(t *testing.T) {
	db := newDB()
	addr, stop := startServer(t, db)
	defer stop()
	c1 := loginOK(t, addr)
	defer c1.Close()

	c2 := dial(t, addr)
	defer c2.Close()
	send(t, c2, protocol.MsgAccountLogin, loginBodyTakeOver("tester", "secret"))
	h := readHeader(t, c2)
	if h.Type != protocol.MsgStillPlaying || h.ID != protocol.IDSelChar {
		t.Fatalf("take-over login = %#x id %d, want StillPlaying id %d", h.Type, h.ID, protocol.IDSelChar)
	}
	expectClosed(t, c2)

	if ty, p := read(t, c1); ty != protocol.MsgMessageBoxOk || noticeCode(t, p) != NoticeAccountFromOthers {
		t.Fatalf("old session: got %#x, want the account-from-others notice", ty)
	}
	expectClosed(t, c1)

	// The old session's cargo went to the database on its way out.
	deadline := time.Now().Add(2 * time.Second)
	for {
		db.mu.Lock()
		n := len(db.savedCargos)
		db.mu.Unlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("cargo saves = %d, want 1 from the closed session", n)
		}
		time.Sleep(10 * time.Millisecond)
	}

	c3 := loginUntilIn(t, addr, "tester", "secret")
	_ = c3.Close()
}

// gatedCargoDB holds every cargo save at gate, so a test can stand in the window
// where a session has closed and its quit-save has not landed yet.
type gatedCargoDB struct {
	*fakeDB
	entered chan struct{}
	gate    chan struct{}
}

func (g *gatedCargoDB) SaveCargo(ctx context.Context, save world.CargoSave) error {
	select {
	case g.entered <- struct{}{}:
	default:
	}
	<-g.gate
	return g.fakeDB.SaveCargo(ctx, save)
}

// TestLoginRefusedWhileQuitSaveInFlight: the account stays in use until the
// closed session's save lands, as the legacy DBSrv keeps the account's slot
// until then. Logging in earlier would read the state from before that save.
func TestLoginRefusedWhileQuitSaveInFlight(t *testing.T) {
	db := &gatedCargoDB{fakeDB: newDB(), entered: make(chan struct{}, 8), gate: make(chan struct{})}
	addr, stop := startServer(t, db)
	defer stop()
	var once sync.Once
	open := func() { once.Do(func() { close(db.gate) }) }
	defer open() // before stop: the shutdown waits for every save

	c1 := loginOK(t, addr)
	_ = c1.Close()
	select {
	case <-db.entered: // the session is gone and its cargo save is held
	case <-time.After(2 * time.Second):
		t.Fatal("the closed session never saved its cargo")
	}

	c2 := dial(t, addr)
	defer c2.Close()
	send(t, c2, protocol.MsgAccountLogin, loginBody("tester", "secret", protocol.AppVersion))
	if h := readHeader(t, c2); h.Type != protocol.MsgAlreadyPlaying {
		t.Fatalf("login during the quit-save = %#x, want AlreadyPlaying", h.Type)
	}
	expectClosed(t, c2)

	open()
	c3 := loginUntilIn(t, addr, "tester", "secret")
	_ = c3.Close()
}

// TestLoginAlreadyPlayingFromDB: a dbServer that answers "already playing"
// itself gets the legacy TM's reply to _MSG_DBAlreadyPlaying, and the
// connection is closed rather than left stuck in USER_LOGIN.
func TestLoginAlreadyPlayingFromDB(t *testing.T) {
	addr, stop := startServer(t, newDB())
	defer stop()
	c := dial(t, addr)
	defer c.Close()
	send(t, c, protocol.MsgAccountLogin, loginBody("online", "x", protocol.AppVersion))
	if h := readHeader(t, c); h.Type != protocol.MsgAlreadyPlaying || h.ID != protocol.IDSelChar {
		t.Fatalf("got %#x id %d, want AlreadyPlaying id %d", h.Type, h.ID, protocol.IDSelChar)
	}
	expectClosed(t, c)
}
