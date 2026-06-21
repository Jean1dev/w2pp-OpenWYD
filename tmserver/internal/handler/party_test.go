package handler

import (
	"net"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func partyDB() *fakeDB {
	db := newDB()
	mk := func() world.CharacterState {
		return world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Level: 50}
	}
	db.loads = map[int64]world.CharacterState{7: mk(), 11: mk()}
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
	body := protocol.MsgSendReqPartyBody{PartyID: int32(partyID), Target: int32(target)}
	send(t, c, protocol.MsgSendReqParty, body.Encode())
}

func acceptPartyFrame(t *testing.T, c net.Conn, leaderID int) {
	t.Helper()
	body := protocol.MsgAcceptPartyBody{LeaderID: int32(leaderID)}
	send(t, c, protocol.MsgAcceptParty, body.Encode())
}

func TestPartyInviteAndAccept(t *testing.T) {
	addr, stop, _ := startServerClock(t, partyDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester") // conn 1 (leader)
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb") // conn 2 (invitee)
	defer b.Close()

	reqPartyFrame(t, a, 1, 2)
	if ty, _, ok := readMaybe(t, b); !ok || ty != protocol.MsgSendReqParty {
		t.Fatalf("invitee got %#x ok=%v, want SendReqParty", ty, ok)
	}

	acceptPartyFrame(t, b, 1)
	// Both members receive the party-list sync.
	if ty, _, ok := readMaybe(t, a); !ok || ty != protocol.MsgAcceptParty {
		t.Errorf("leader got %#x ok=%v, want AcceptParty sync", ty, ok)
	}
	if ty, _, ok := readMaybe(t, b); !ok || ty != protocol.MsgAcceptParty {
		t.Errorf("invitee got %#x ok=%v, want AcceptParty sync", ty, ok)
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
	acceptPartyFrame(t, b, 1)
	if ty, _, ok := readMaybe(t, b); ok {
		t.Errorf("forged accept produced %#x; should be rejected (PARTYHACK)", ty)
	}
	if ty, _, ok := readMaybe(t, a); ok {
		t.Errorf("leader received %#x for a forged accept; should be none", ty)
	}
}

func TestGuildInvite(t *testing.T) {
	addr, stop, _ := startServerClock(t, guildDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester") // conn 1 (guild leader)
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb") // conn 2 (recruit, same clan, no guild)
	defer b.Close()

	send(t, a, protocol.MsgInviteGuild, protocol.EncodeStandardParm2(2, 0)) // target conn 2, type 0
	// The recruit is welcomed; the leader sees the recruit's refreshed tag.
	if ty, _, ok := readMaybe(t, b); !ok || ty != protocol.MsgMessagePanel {
		t.Errorf("recruit got %#x ok=%v, want MessagePanel welcome", ty, ok)
	}
	if ty, _, ok := readMaybe(t, a); !ok || ty != protocol.MsgCreateMob {
		t.Errorf("leader got %#x ok=%v, want CreateMob (tag refresh)", ty, ok)
	}
}
