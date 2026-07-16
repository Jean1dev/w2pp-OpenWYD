package handler

import (
	"net"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestParseAccess covers the account.role → AccessLevel mapping (fail-closed:
// anything unrecognized is a plain player).
func TestParseAccess(t *testing.T) {
	cases := []struct {
		role string
		want world.AccessLevel
	}{
		{"admin", world.AccessAdmin},
		{"moderator", world.AccessModerator},
		{"player", world.AccessPlayer},
		{"", world.AccessPlayer},
		{"MODERATOR", world.AccessPlayer}, // case-sensitive: DB stores lowercase
		{"gm", world.AccessPlayer},
	}
	for _, c := range cases {
		if got := world.ParseAccess(c.role); got != c.want {
			t.Errorf("ParseAccess(%q) = %v, want %v", c.role, got, c.want)
		}
	}
}

// gmDB seeds a moderator, a second moderator, a plain player and a victim, all at
// (5,5) so they share a view. Passwords are "secret" (enterWorldAs default).
func gmDB() *fakeDB {
	db := &fakeDB{accounts: map[string]*fakeAccount{
		"mod":    {id: 20, pass: "secret", role: "moderator", chars: []world.CharSummary{{Slot: 0, Name: "Mod"}}},
		"mod2":   {id: 21, pass: "secret", role: "moderator", chars: []world.CharSummary{{Slot: 0, Name: "Mod2"}}},
		"player": {id: 22, pass: "secret", role: "player", chars: []world.CharSummary{{Slot: 0, Name: "Player"}}},
		"victim": {id: 23, pass: "secret", role: "player", chars: []world.CharSummary{{Slot: 0, Name: "Victim"}}},
	}}
	db.loads = map[int64]world.CharacterState{
		20: {Slot: 0, Name: "Mod", Class: 0, Level: 10, X: 5, Y: 5, HP: 1000, MaxHP: 1000},
		21: {Slot: 0, Name: "Mod2", Class: 0, Level: 10, X: 5, Y: 5, HP: 1000, MaxHP: 1000},
		22: {Slot: 0, Name: "Player", Class: 0, Level: 10, X: 5, Y: 5, HP: 1000, MaxHP: 1000},
		23: {Slot: 0, Name: "Victim", Class: 0, Level: 10, X: 5, Y: 5, HP: 1000, MaxHP: 1000},
	}
	return db
}

// gmFrame sends "/gm <line>" the way the client does: a whisper to target "gm"
// whose String is the rest of the command line.
func gmFrame(t *testing.T, c net.Conn, line string) {
	t.Helper()
	whisperFrame(t, c, "gm", line)
}

// TestGMGateDeniesPlayer verifies a plain player is silently refused the bus.
func TestGMGateDeniesPlayer(t *testing.T) {
	addr, stop, _ := startServerClock(t, gmDB())
	defer stop()
	c := enterWorldAs(t, addr, "player")
	defer c.Close()

	gmFrame(t, c, "setgold 999999")
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("player got %#x; a non-GM must be silently denied", ty)
	}
}

// TestGMSetGold verifies a moderator's /gm setgold mutates gold and pushes UpdateEtc.
func TestGMSetGold(t *testing.T) {
	addr, stop, _ := startServerClock(t, gmDB())
	defer stop()
	c := enterWorldAs(t, addr, "mod")
	defer c.Close()

	gmFrame(t, c, "setgold 424242")
	etc := expect(t, c, protocol.MsgUpdateEtc)
	if got := int32(le(etc[28:32])); got != 424242 { // char coin @body28
		t.Errorf("gold = %d, want 424242", got)
	}
}

// TestGMItem verifies /gm item grants an item into the first free inventory slot.
func TestGMItem(t *testing.T) {
	addr, stop, _ := startServerClock(t, gmDB())
	defer stop()
	c := enterWorldAs(t, addr, "mod")
	defer c.Close()

	gmFrame(t, c, "item 1234")
	got := expect(t, c, protocol.MsgCNFGetItem)
	if slot := le(got[0:4]); slot != 0 {
		t.Errorf("granted slot = %d, want 0 (first free)", slot)
	}
}

// TestGMSetLevel verifies /gm setlevel raises the moderator to the target level.
func TestGMSetLevel(t *testing.T) {
	addr, stop, _ := startServerClock(t, gmDB())
	defer stop()
	c := enterWorldAs(t, addr, "mod")
	defer c.Close()

	gmFrame(t, c, "setlevel 55")
	// Level-up pushes UpdateScore + UpdateEtc + a level-up motion; assert one arrives.
	if ty, _, ok := readMaybe(t, c); !ok || (ty != protocol.MsgUpdateScore && ty != protocol.MsgUpdateEtc) {
		t.Errorf("got %#x ok=%v, want a level-up score/etc packet", ty, ok)
	}
}

// TestGMNotice verifies /gm notice reaches every player as a chat line, prefixed.
func TestGMNotice(t *testing.T) {
	addr, stop, _ := startServerClock(t, gmDB())
	defer stop()
	mod := enterWorldAs(t, addr, "mod")
	defer mod.Close()
	other := enterWorldAs(t, addr, "player")
	defer other.Close()

	gmFrame(t, mod, "notice server restarting")
	p := expect(t, other, protocol.MsgMessageChat)
	if got := cstr(p); got != "[GM] server restarting" {
		t.Errorf("notice text = %q, want %q", got, "[GM] server restarting")
	}
}

// TestGMGoto verifies /gm goto teleports the caller (MsgAction jump).
func TestGMGoto(t *testing.T) {
	addr, stop, _ := startServerClock(t, gmDB())
	defer stop()
	mod := enterWorldAs(t, addr, "mod")
	defer mod.Close()
	victim := enterWorldAs(t, addr, "victim")
	defer victim.Close()

	gmFrame(t, mod, "goto Victim")
	if ty, _, ok := readMaybe(t, mod); !ok || ty != protocol.MsgAction {
		t.Errorf("got %#x ok=%v, want MsgAction (teleport to player)", ty, ok)
	}
}

// TestGMSummon verifies /gm summon pulls the target (they receive the MsgAction jump).
func TestGMSummon(t *testing.T) {
	addr, stop, _ := startServerClock(t, gmDB())
	defer stop()
	mod := enterWorldAs(t, addr, "mod")
	defer mod.Close()
	victim := enterWorldAs(t, addr, "victim")
	defer victim.Close()

	gmFrame(t, mod, "summon Victim")
	if ty, _, ok := readMaybe(t, victim); !ok || ty != protocol.MsgAction {
		t.Errorf("victim got %#x ok=%v, want MsgAction (summoned)", ty, ok)
	}
}

// TestGMKick verifies a moderator can disconnect a lower-tier player.
func TestGMKick(t *testing.T) {
	addr, stop, _ := startServerClock(t, gmDB())
	defer stop()
	mod := enterWorldAs(t, addr, "mod")
	defer mod.Close()
	victim := enterWorldAs(t, addr, "victim")
	defer victim.Close()

	gmFrame(t, mod, "kick Victim")
	expectDisconnect(t, victim)
}

// TestGMKickGuard verifies a moderator cannot kick an equal-or-higher tier admin.
func TestGMKickGuard(t *testing.T) {
	addr, stop, _ := startServerClock(t, gmDB())
	defer stop()
	mod := enterWorldAs(t, addr, "mod")
	defer mod.Close()
	mod2 := enterWorldAs(t, addr, "mod2")
	defer mod2.Close()

	gmFrame(t, mod, "kick Mod2")
	expectStillConnected(t, mod2)
}

// TestGMBan verifies /gm ban persists the block and kicks the online player.
func TestGMBan(t *testing.T) {
	db := gmDB()
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	mod := enterWorldAs(t, addr, "mod")
	defer mod.Close()
	victim := enterWorldAs(t, addr, "victim")
	defer victim.Close()

	gmFrame(t, mod, "ban Victim")
	expectDisconnect(t, victim)
	blocked, ok := db.blockedState(t, "victim")
	if !ok || !blocked {
		t.Errorf("ban recorded blocked=%v ok=%v, want blocked=true for account 'victim'", blocked, ok)
	}
}

// TestGMUnban verifies /gm unban persists blocked=false (by account name; the
// target need not be online).
func TestGMUnban(t *testing.T) {
	db := gmDB()
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	mod := enterWorldAs(t, addr, "mod")
	defer mod.Close()

	gmFrame(t, mod, "unban victim")
	blocked, ok := db.blockedState(t, "victim")
	if !ok || blocked {
		t.Errorf("unban recorded blocked=%v ok=%v, want blocked=false", blocked, ok)
	}
}

// expectDisconnect drains any queued frames then asserts the socket was closed by
// the server (a non-timeout read error), i.e. the client was kicked.
func expectDisconnect(t *testing.T, c net.Conn) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		_ = c.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		var b [256]byte
		_, err := c.Read(b[:])
		if err == nil {
			if time.Now().After(deadline) {
				t.Fatal("still connected after kick (kept receiving data)")
			}
			continue // drain frames flushed just before close
		}
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			t.Fatal("expected disconnect, got read timeout (still connected)")
		}
		return // EOF / connection reset = disconnected as expected
	}
}

// expectStillConnected asserts the socket stays open (an idle read times out
// rather than returning EOF).
func expectStillConnected(t *testing.T, c net.Conn) {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	var b [1]byte
	_, err := c.Read(b[:])
	if err == nil {
		return // received data → still connected
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return // idle but open
	}
	t.Fatalf("expected still-connected, got %v (disconnected)", err)
}
