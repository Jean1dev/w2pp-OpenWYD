package handler

import (
	"errors"
	"net"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func secureFrame(t *testing.T, c net.Conn, pin string, change int32) {
	t.Helper()
	var body protocol.MsgAccountSecureBody
	copy(body.NumericToken[:], pin)
	body.ChangeNumeric = change
	send(t, c, protocol.MsgAccountSecure, body.Encode())
}

func TestAccountSecureVerifyOK(t *testing.T) {
	db := newDB()
	db.pinVerify = world.PinOK
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	secureFrame(t, c, "1234", 0)
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgAccountSecure {
		t.Errorf("verify OK got %#x ok=%v, want MsgAccountSecure ack", ty, ok)
	}
}

func TestAccountSecureVerifyBad(t *testing.T) {
	db := newDB()
	db.pinVerify = world.PinBadPin
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	secureFrame(t, c, "0000", 0)
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgAccountSecureFail {
		t.Errorf("verify bad got %#x ok=%v, want MsgAccountSecureFail", ty, ok)
	}
}

// A verify against an account with no PIN yet sets it (first-time) and acks.
func TestAccountSecureFirstTimeSet(t *testing.T) {
	db := newDB()
	db.pinVerify = world.PinNotSet
	db.pinSetOK = true
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	secureFrame(t, c, "4321", 0)
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgAccountSecure {
		t.Errorf("first-time set got %#x ok=%v, want MsgAccountSecure ack", ty, ok)
	}
	if got := db.setPinCount(); got != 1 {
		t.Errorf("SetPin called %d times, want 1 (first-time set)", got)
	}
}

func TestAccountSecureChange(t *testing.T) {
	db := newDB()
	db.pinSetOK = true
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	secureFrame(t, c, "9999", 1) // ChangeNumeric = 1 → set/change
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgAccountSecure {
		t.Errorf("change got %#x ok=%v, want MsgAccountSecure ack", ty, ok)
	}
	if got := db.setPinCount(); got != 1 {
		t.Errorf("SetPin called %d times, want 1 (change)", got)
	}
}

// A backend error (e.g. no dbServer) degrades to allow-all so bring-up isn't
// blocked on the secure-password screen — and never leaks the PIN.
func TestAccountSecureErrorDegrades(t *testing.T) {
	db := newDB()
	db.pinVerifyErr = errTestBackend
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	secureFrame(t, c, "1234", 0)
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgAccountSecure {
		t.Errorf("backend-error got %#x ok=%v, want MsgAccountSecure ack (degrade)", ty, ok)
	}
}

var errTestBackend = errors.New("backend down")
