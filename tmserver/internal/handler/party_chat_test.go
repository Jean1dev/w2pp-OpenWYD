package handler

import (
	"net"
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// partyOfTwo puts "tester" (Hero, conn 1, leader) and "tradeb" (HeroB, conn 2) in a
// party and logs "third" (HeroC, conn 3) in outside of it.
func partyOfTwo(t *testing.T) (a, b, c net.Conn, stop func()) {
	t.Helper()
	addr, stop, _ := startServerClock(t, partyDB())
	a = enterWorldAs(t, addr, "tester")
	b = enterWorldAs(t, addr, "tradeb")
	c = enterWorldAs(t, addr, "third")
	reqPartyFrame(t, a, 1, 2)
	expectPartyFrame(t, b, protocol.MsgSendReqParty)
	acceptPartyFrame(t, b, 1, "Hero")
	drainRaw(t, a)
	drainRaw(t, b)
	drainRaw(t, c)
	return a, b, c, stop
}

// expectPartyLine reads the party line a member receives: an MSG_MessageWhisper
// from the speaker's conn, with MobName rewritten to the speaker's name and the
// text kept with its '=' prefix (the client strips it, TMFieldScene.cpp:17906).
func expectPartyLine(t *testing.T, c net.Conn, fromConn uint16, fromName, text string) {
	t.Helper()
	h, p, ok := readMaybeHeader(t, c)
	if !ok || h.Type != protocol.MsgMessageWhisper {
		t.Fatalf("got %#x ok=%v, want the party line", h.Type, ok)
	}
	var body protocol.MsgWhisperBody
	if err := body.Decode(p); err != nil {
		t.Fatal(err)
	}
	if h.ID != fromConn || cstr(body.MobName[:]) != fromName || !strings.HasPrefix(string(body.String), text) {
		t.Fatalf("party line = ID %d %q %q, want ID %d %q %q", h.ID, cstr(body.MobName[:]), body.String, fromConn, fromName, text)
	}
}

func expectNothing(t *testing.T, c net.Conn, who string) {
	t.Helper()
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("%s received %#x, want nothing", who, ty)
	}
}

// TestPartyChatMemberToLeader: a member's "=" line reaches the leader, not the
// speaker and not a player outside the party (_MSG_MessageWhisper.cpp:1470-1510).
func TestPartyChatMemberToLeader(t *testing.T) {
	a, b, c, stop := partyOfTwo(t)
	defer stop()
	defer a.Close()
	defer b.Close()
	defer c.Close()

	whisperFrame(t, b, "", "=ola grupo")
	expectPartyLine(t, a, 2, "HeroB", "=ola grupo")
	expectNothing(t, b, "speaker")
	expectNothing(t, c, "outsider")
}

// TestPartyChatLeaderToMember: the leader's line goes to its PartyList.
func TestPartyChatLeaderToMember(t *testing.T) {
	a, b, c, stop := partyOfTwo(t)
	defer stop()
	defer a.Close()
	defer b.Close()
	defer c.Close()

	whisperFrame(t, a, "", "=vamos")
	expectPartyLine(t, b, 1, "Hero", "=vamos")
	expectNothing(t, a, "speaker")
	expectNothing(t, c, "outsider")
}

// TestPartyChatSpoofedName: the server, not the client, names the speaker.
func TestPartyChatSpoofedName(t *testing.T) {
	a, b, c, stop := partyOfTwo(t)
	defer stop()
	defer a.Close()
	defer b.Close()
	defer c.Close()

	var body protocol.MsgWhisperBody
	body.String = []byte("=sou o lider")
	send(t, b, protocol.MsgMessageWhisper, body.Encode())
	expectPartyLine(t, a, 2, "HeroB", "=sou o lider")
}

// TestPartyChatToggle: "partychat" switches the channel off for the player who
// sent it (_MSG_MessageChat.cpp:117-126) and confirms with a chat line.
func TestPartyChatToggle(t *testing.T) {
	a, b, c, stop := partyOfTwo(t)
	defer stop()
	defer a.Close()
	defer b.Close()
	defer c.Close()

	chatFrame(t, b, "partychat")
	ty, p, ok := readMaybe(t, b)
	if !ok || ty != protocol.MsgMessageChat || !strings.HasPrefix(string(p), "Party Chatting : Off") {
		t.Fatalf("got %#x %q, want \"Party Chatting : Off\"", ty, p)
	}
	whisperFrame(t, a, "", "=alguem?")
	expectNothing(t, b, "member with partychat off")

	chatFrame(t, b, "partychat")
	if ty, p, ok := readMaybe(t, b); !ok || !strings.HasPrefix(string(p), "Party Chatting : On") {
		t.Fatalf("got %#x %q, want \"Party Chatting : On\"", ty, p)
	}
	whisperFrame(t, a, "", "=de volta")
	expectPartyLine(t, b, 1, "Hero", "=de volta")
}

// TestPartyChatWithoutParty: outside a party nobody receives the line, and it is
// not mistaken for a whisper to an empty name (no "not connected" notice).
func TestPartyChatWithoutParty(t *testing.T) {
	addr, stop, _ := startServerClock(t, partyDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()
	drainRaw(t, a)
	drainRaw(t, b)

	whisperFrame(t, a, "", "=sozinho")
	expectNothing(t, a, "speaker")
	expectNothing(t, b, "player outside the party")
}

// TestPartyChatAfterLeave: once B leaves, the leader's line no longer reaches B.
func TestPartyChatAfterLeave(t *testing.T) {
	a, b, c, stop := partyOfTwo(t)
	defer stop()
	defer a.Close()
	defer b.Close()
	defer c.Close()

	removePartyFrame(t, b, 2)
	expectRemoveParty(t, b, 0)
	expectRemoveParty(t, a, 2)
	whisperFrame(t, a, "", "=ainda ai?")
	expectNothing(t, b, "former member")
}
