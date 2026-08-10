package handler

import (
	"net"
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func partyDB() *fakeDB {
	db := newDB()
	mk := func(name string) world.CharacterState {
		return world.CharacterState{Slot: 0, Name: name, X: 5, Y: 5, HP: 1000, MaxHP: 1000, Level: 50}
	}
	db.accounts["third"] = &fakeAccount{id: 13, pass: "secret", chars: []world.CharSummary{{Slot: 0, Name: "HeroC", Class: 1, Level: 50}}}
	db.loads = map[int64]world.CharacterState{7: mk("Hero"), 11: mk("HeroB"), 13: mk("HeroC")}
	return db
}

func guildDB() *fakeDB {
	db := newDB()
	db.loads = map[int64]world.CharacterState{
		7:  {Slot: 0, Name: "Leader", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Level: 50, Clan: 1, GuildID: 5, GuildLevel: 9, Coin: 5_000_000},
		11: {Slot: 0, Name: "Recruit", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Level: 50, Clan: 1},
	}
	return db
}

func reqPartyFrame(t *testing.T, c net.Conn, partyID, target int) {
	t.Helper()
	body := protocol.MsgSendReqPartyBody{PartyID: int16(partyID), Unk: int32(target)}
	send(t, c, protocol.MsgSendReqParty, body.Encode())
}

func acceptPartyFrame(t *testing.T, c net.Conn, leaderID int, leaderName string) {
	t.Helper()
	body := protocol.MsgAcceptPartyBody{LeaderID: int16(leaderID)}
	copy(body.MobName[:], leaderName)
	send(t, c, protocol.MsgAcceptParty, body.Encode())
}

func removePartyFrame(t *testing.T, c net.Conn, target int) {
	t.Helper()
	send(t, c, protocol.MsgRemoveParty, protocol.EncodeStandardParm(int32(target)))
}

func expectPartyFrame(t *testing.T, c net.Conn, want protocol.Type) []byte {
	t.Helper()
	ty, payload, ok := readMaybe(t, c)
	if !ok || ty != want {
		t.Fatalf("got %#x ok=%v, want %#x", ty, ok, want)
	}
	return payload
}

func expectCNFAddParty(t *testing.T, c net.Conn) protocol.MsgCNFAddPartyBody {
	t.Helper()
	payload := expectPartyFrame(t, c, protocol.MsgCNFAddParty)
	if len(payload) != protocol.MsgCNFAddPartyBodySize {
		t.Fatalf("CNFAddParty payload = %d, want %d", len(payload), protocol.MsgCNFAddPartyBodySize)
	}
	return protocol.MsgCNFAddPartyBody{
		LeaderConn: protocolLe16(payload[0:2]),
		PartyID:    protocolLe16(payload[8:10]),
		Target:     protocolLe16(payload[26:28]),
	}
}

func expectRemoveParty(t *testing.T, c net.Conn, connExit uint16) {
	t.Helper()
	payload := expectPartyFrame(t, c, protocol.MsgRemoveParty)
	if len(payload) != protocol.MsgRemovePartyBodySize {
		t.Fatalf("RemoveParty payload = %d, want %d", len(payload), protocol.MsgRemovePartyBodySize)
	}
	if got := protocolLe16(payload[0:2]); got != connExit {
		t.Fatalf("RemoveParty Leaderconn = %d, want %d", got, connExit)
	}
}

func protocolLe16(b []byte) uint16 {
	return uint16(b[0]) | uint16(b[1])<<8
}

func TestPartyInviteAndAccept(t *testing.T) {
	addr, stop, _ := startServerClock(t, partyDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester") // conn 1 (leader)
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb") // conn 2 (invitee)
	defer b.Close()

	reqPartyFrame(t, a, 1, 2)
	payload := expectPartyFrame(t, b, protocol.MsgSendReqParty)
	var invite protocol.MsgSendReqPartyBody
	if err := invite.Decode(payload); err != nil {
		t.Fatal(err)
	}
	if invite.PartyID != 1 || strings.TrimRight(string(invite.MobName[:]), "\x00") != "Hero" {
		t.Fatalf("invite payload = %+v, want leader party/name", invite)
	}

	acceptPartyFrame(t, b, 1, "Hero")
	leaderSlot := expectCNFAddParty(t, a)
	if leaderSlot.LeaderConn != 1 || leaderSlot.PartyID != 1 {
		t.Fatalf("leader first CNFAddParty = %+v, want leader slot", leaderSlot)
	}
	memberSlot := expectCNFAddParty(t, b)
	if memberSlot.PartyID != 2 || memberSlot.Target != 52428 {
		t.Fatalf("invitee first CNFAddParty = %+v, want member slot", memberSlot)
	}
}

// TestPartyAcceptWithoutInvite: the LastReqParty gate blocks a forged accept.
func TestPartyAcceptWithoutInvite(t *testing.T) {
	addr, stop, _ := startServerClock(t, partyDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	// B accepts a party it was never invited to → rejected, no sync.
	acceptPartyFrame(t, b, 1, "Hero")
	if ty, _, ok := readMaybe(t, b); ok {
		t.Errorf("forged accept produced %#x; should be rejected (PARTYHACK)", ty)
	}
	if ty, _, ok := readMaybe(t, a); ok {
		t.Errorf("leader received %#x for a forged accept; should be none", ty)
	}
}

func TestPartyAddMemberFull(t *testing.T) {
	leader := &world.Entity{}
	for i := range leader.PartyList {
		leader.PartyList[i] = i + 1
	}
	if _, ok := addMember(leader, 99); ok {
		t.Fatal("addMember succeeded for a full party")
	}
	if hasMember(leader, 99) {
		t.Fatal("full party mutated with the rejected member")
	}
}

func TestPartyMemberLeave(t *testing.T) {
	addr, stop, _ := startServerClock(t, partyDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	reqPartyFrame(t, a, 1, 2)
	expectPartyFrame(t, b, protocol.MsgSendReqParty)
	acceptPartyFrame(t, b, 1, "Hero")
	drainRaw(t, a)
	drainRaw(t, b)

	removePartyFrame(t, b, 2)
	expectRemoveParty(t, b, 0)
	expectRemoveParty(t, a, 2)
}

func TestPartyLeaderKicksMember(t *testing.T) {
	addr, stop, _ := startServerClock(t, partyDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	reqPartyFrame(t, a, 1, 2)
	expectPartyFrame(t, b, protocol.MsgSendReqParty)
	acceptPartyFrame(t, b, 1, "Hero")
	drainRaw(t, a)
	drainRaw(t, b)

	removePartyFrame(t, a, 2)
	expectRemoveParty(t, b, 0)
	expectRemoveParty(t, a, 2)
}

func TestPartyLeaderLeavePromotesNextMember(t *testing.T) {
	addr, stop, _ := startServerClock(t, partyDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()
	c := enterWorldAs(t, addr, "third")
	defer c.Close()

	reqPartyFrame(t, a, 1, 2)
	expectPartyFrame(t, b, protocol.MsgSendReqParty)
	acceptPartyFrame(t, b, 1, "Hero")
	drainRaw(t, a)
	drainRaw(t, b)
	reqPartyFrame(t, a, 1, 3)
	expectPartyFrame(t, c, protocol.MsgSendReqParty)
	acceptPartyFrame(t, c, 1, "Hero")
	drainRaw(t, a)
	drainRaw(t, b)
	drainRaw(t, c)

	removePartyFrame(t, a, 1)
	expectRemoveParty(t, a, 0)
	expectRemoveParty(t, b, 0)
	expectRemoveParty(t, c, 0)
	if slot := expectCNFAddParty(t, b); slot.LeaderConn != 2 || slot.PartyID != 2 {
		t.Fatalf("promoted member first sync = %+v, want new leader slot", slot)
	}
	if slot := expectCNFAddParty(t, c); slot.PartyID != 3 {
		t.Fatalf("remaining member first sync = %+v, want own slot", slot)
	}
}

// TestPartyMemberLogoutLeavesParty: the port never ran the legacy
// RemoveParty(conn) on logout (_MSG_CharacterLogout.cpp:23), so the leaver stayed
// a ghost slot in the leader's PartyList — which made the leader read as "already
// partied" forever and unable to accept any later invite (isInParty).
func TestPartyMemberLogoutLeavesParty(t *testing.T) {
	addr, stop, _ := startServerClock(t, partyDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()
	c := enterWorldAs(t, addr, "third")
	defer c.Close()

	reqPartyFrame(t, a, 1, 2)
	expectPartyFrame(t, b, protocol.MsgSendReqParty)
	acceptPartyFrame(t, b, 1, "Hero")
	drainRaw(t, a)
	drainRaw(t, b)

	send(t, b, protocol.MsgCharacterLogout, nil)
	expectRemoveParty(t, a, 2)

	// The ex-leader is party-free again, so a fresh invite reaches it.
	drainRaw(t, a)
	drainRaw(t, c)
	reqPartyFrame(t, c, 3, 1)
	expectPartyFrame(t, a, protocol.MsgSendReqParty)
}

func TestGuildInvite(t *testing.T) {
	addr, stop, _ := startServerClock(t, guildDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester") // conn 1 (guild leader)
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb") // conn 2 (recruit, same clan, no guild)
	defer b.Close()
	drainRaw(t, a)
	drainRaw(t, b)

	send(t, a, protocol.MsgInviteGuild, protocol.EncodeStandardParm2(2, 0)) // target conn 2, type 0
	// The recruit is welcomed; the leader sees the recruit's refreshed tag.
	if ty, _, ok := readMaybe(t, b); !ok || ty != protocol.MsgMessagePanel {
		t.Errorf("recruit got %#x ok=%v, want MessagePanel welcome", ty, ok)
	}
	if ty, _, ok := readMaybeRaw(t, a); !ok || ty != protocol.MsgCreateMob {
		t.Errorf("leader got %#x ok=%v, want CreateMob (tag refresh)", ty, ok)
	}
}
