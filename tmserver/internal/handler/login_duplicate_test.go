package handler

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// expectKicked reads the previous session's last frames: the
// _NN_Your_Account_From_Others panel, then the socket close.
func expectKicked(t *testing.T, c net.Conn) {
	t.Helper()
	ty, p, ok := readMaybe(t, c)
	for ok && ty != protocol.MsgMessagePanel {
		ty, p, ok = readMaybe(t, c)
	}
	if !ok {
		t.Fatal("previous session got no message before the close")
	}
	if got := cstr(p); got != accountFromOthers {
		t.Errorf("previous session message = %q, want %q", got, accountFromOthers)
	}
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	var b [1]byte
	for {
		if _, err := c.Read(b[:]); err != nil {
			// EOF or reset: closed by the server. A timeout means it stayed open.
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				t.Error("previous session still open")
			}
			return
		}
	}
}

func tryLogin(t *testing.T, c net.Conn) protocol.Type {
	t.Helper()
	send(t, c, protocol.MsgAccountLogin, loginBody("tester", "secret", protocol.AppVersion))
	ty, _ := read(t, c)
	return ty
}

// TestDuplicateLoginKicksPreviousAndRefusesNew: a second login of an account at
// character selection closes the first session with the legacy message and
// refuses the new one with _MSG_StillPlaying, which may then retry on the same
// connection once the previous session is saved.
func TestDuplicateLoginKicksPreviousAndRefusesNew(t *testing.T) {
	db := newDB()
	db.accounts["tester"].cargo = world.CargoState{Coin: 777}
	addr, stop := startServer(t, db)
	defer stop()
	a := loginAndSelect(t, addr)
	defer a.Close()
	b := dial(t, addr)
	defer b.Close()

	if ty := tryLogin(t, b); ty != protocol.MsgStillPlaying {
		t.Fatalf("second login = %#x, want StillPlaying (0x011D)", ty)
	}
	expectKicked(t, a)

	deadline := time.Now().Add(3 * time.Second)
	for {
		ty := tryLogin(t, b)
		if ty == protocol.MsgCNFAccountLogin {
			break
		}
		if ty != protocol.MsgAlreadyPlaying || time.Now().After(deadline) {
			t.Fatalf("retry = %#x, want AlreadyPlaying while saving, then CNFAccountLogin", ty)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if save, n := db.lastSavedCargo(); n != 1 || save.Coin != 777 {
		t.Errorf("previous session cargo saves = %d (coin %d), want 1 with 777", n, save.Coin)
	}
}

// TestDuplicateLoginWaitsForSave: while the kicked session's save is in flight
// every retry gets _MSG_AlreadyPlaying; the login only completes after the save
// returned, so it cannot load rows older than the previous session's state.
func TestDuplicateLoginWaitsForSave(t *testing.T) {
	db := newDB()
	db.accounts["tester"].cargo = world.CargoState{Coin: 777}
	db.cargoGate = make(chan struct{})
	addr, stop := startServer(t, db)
	defer stop()
	a := loginAndSelect(t, addr)
	defer a.Close()
	b := dial(t, addr)
	defer b.Close()

	if ty := tryLogin(t, b); ty != protocol.MsgStillPlaying {
		t.Fatalf("second login = %#x, want StillPlaying", ty)
	}
	expectKicked(t, a)
	for range 3 {
		if ty := tryLogin(t, b); ty != protocol.MsgAlreadyPlaying {
			t.Fatalf("retry during save = %#x, want AlreadyPlaying (0x011C)", ty)
		}
	}
	if _, n := db.lastSavedCargo(); n != 0 {
		t.Fatalf("cargo saved %d times before the gate opened", n)
	}

	close(db.cargoGate)
	deadline := time.Now().Add(3 * time.Second)
	for tryLogin(t, b) != protocol.MsgCNFAccountLogin {
		if time.Now().After(deadline) {
			t.Fatal("login never completed after the save")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, n := db.lastSavedCargo(); n != 1 {
		t.Errorf("cargo saves = %d, want 1 before the new login", n)
	}
}

// TestDuplicateLoginInWorld: a session in play is saved (character and cargo)
// when the same account logs in elsewhere.
func TestDuplicateLoginInWorld(t *testing.T) {
	db := chatDB()
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := dial(t, addr)
	defer b.Close()

	if ty := tryLogin(t, b); ty != protocol.MsgStillPlaying {
		t.Fatalf("second login = %#x, want StillPlaying", ty)
	}
	expectKicked(t, a)
	deadline := time.Now().Add(3 * time.Second)
	for {
		if save, n := db.lastSavedChar(); n == 1 && save.AccountID == 7 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("kicked character was not saved")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestOtherAccountsUnaffected: logins of different accounts never refuse each other.
func TestOtherAccountsUnaffected(t *testing.T) {
	addr, stop := startServer(t, newDB())
	defer stop()
	a := loginAndSelect(t, addr)
	defer a.Close()
	b := dial(t, addr)
	defer b.Close()
	send(t, b, protocol.MsgAccountLogin, loginBody("tradeb", "secret", protocol.AppVersion))
	if ty, _ := read(t, b); ty != protocol.MsgCNFAccountLogin {
		t.Fatalf("other account login = %#x, want CNFAccountLogin", ty)
	}
	if ty, _, ok := readMaybe(t, a); ok {
		t.Errorf("first session got %#x, want nothing", ty)
	}
}
