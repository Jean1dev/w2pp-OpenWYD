package handler

import (
	"net"
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// guildChatDB puts "tester" (Hero, conn 1) and "tradeb" (HeroB, conn 2) in guild 5
// and "third" (HeroC, conn 3) in guild 6. With ally set, guild 5 is allied to 6.
func guildChatDB(ally bool) *fakeDB {
	db := newDB()
	mk := func(name string, guild uint16) world.CharacterState {
		return world.CharacterState{Slot: 0, Name: name, X: 5, Y: 5, HP: 1000, MaxHP: 1000, Level: 50, Clan: 1, GuildID: guild}
	}
	db.accounts["third"] = &fakeAccount{id: 13, pass: "secret", chars: []world.CharSummary{{Slot: 0, Name: "HeroC", Class: 1, Level: 50}}}
	db.loads = map[int64]world.CharacterState{7: mk("Hero", 5), 11: mk("HeroB", 5), 13: mk("HeroC", 6)}
	if ally {
		db.guildRelations = []world.GuildRelation{{GuildID: 5, TargetGuildID: 6, Kind: world.GuildRelationAlly}}
	}
	return db
}

func guildChatWorld(t *testing.T, db *fakeDB) (a, b, c net.Conn, stop func()) {
	t.Helper()
	addr, stop, _ := startServerClock(t, db)
	a = enterWorldAs(t, addr, "tester")
	b = enterWorldAs(t, addr, "tradeb")
	c = enterWorldAs(t, addr, "third")
	drainRaw(t, a)
	drainRaw(t, b)
	drainRaw(t, c)
	return a, b, c, stop
}

// expectGuildLine reads a guild line: an MSG_MessageWhisper from the speaker's
// conn with MobName rewritten to the speaker, the text kept with its '-' prefix
// and Color 3 at the 7662 offset (the client shows it as guild chat only then).
func expectGuildLine(t *testing.T, c net.Conn, fromConn uint16, fromName, text string) {
	t.Helper()
	h, p, ok := readMaybeHeader(t, c)
	if !ok || h.Type != protocol.MsgMessageWhisper {
		t.Fatalf("got %#x ok=%v, want the guild line", h.Type, ok)
	}
	var body protocol.MsgWhisperBody
	if err := body.Decode(p); err != nil {
		t.Fatal(err)
	}
	if h.ID != fromConn || cstr(body.MobName[:]) != fromName || !strings.HasPrefix(string(body.String), text+"\x00") {
		t.Fatalf("guild line = ID %d %q %q, want ID %d %q %q", h.ID, cstr(body.MobName[:]), body.String, fromConn, fromName, text)
	}
	if len(body.String) < whisperColorOffset+2 || body.String[whisperColorOffset] != 3 || body.String[whisperColorOffset+1] != 0 {
		t.Fatalf("guild line Color bytes = %v, want [3 0] at %d", body.String[min(len(body.String), whisperColorOffset):], whisperColorOffset)
	}
}

func expectPanel(t *testing.T, c net.Conn, text string) {
	t.Helper()
	ty, p, ok := readMaybe(t, c)
	if !ok || ty != protocol.MsgMessagePanel {
		t.Fatalf("got %#x ok=%v, want MessagePanel %q", ty, ok, text)
	}
	if got := cstr(p); got != text {
		t.Fatalf("panel = %q, want %q", got, text)
	}
}

// TestGuildChatReachesGuild: a "-" line reaches the other guild member only —
// not the speaker and not the allied guild (_MSG_MessageWhisper.cpp:1426-1468).
func TestGuildChatReachesGuild(t *testing.T) {
	a, b, c, stop := guildChatWorld(t, guildChatDB(true))
	defer stop()
	defer a.Close()
	defer b.Close()
	defer c.Close()

	whisperFrame(t, a, "", "-ola guilda")
	expectGuildLine(t, b, 1, "Hero", "-ola guilda")
	expectNothing(t, a, "speaker")
	expectNothing(t, c, "allied guild on a single -")

	whisperFrame(t, b, "", "-oi")
	expectGuildLine(t, a, 2, "HeroB", "-oi")
	expectNothing(t, c, "allied guild on a single -")
}

// TestGuildChatAlly: a "--" line also reaches the allied guild.
func TestGuildChatAlly(t *testing.T) {
	a, b, c, stop := guildChatWorld(t, guildChatDB(true))
	defer stop()
	defer a.Close()
	defer b.Close()
	defer c.Close()

	whisperFrame(t, a, "", "--aliados")
	expectGuildLine(t, b, 1, "Hero", "--aliados")
	expectGuildLine(t, c, 1, "Hero", "--aliados")
	expectNothing(t, a, "speaker")
}

// TestGuildChatAllyIsDirected: g_pGuildAlly is per guild; guild 6 has no ally,
// so its "--" line reaches nobody in guild 5.
func TestGuildChatAllyIsDirected(t *testing.T) {
	a, b, c, stop := guildChatWorld(t, guildChatDB(true))
	defer stop()
	defer a.Close()
	defer b.Close()
	defer c.Close()

	whisperFrame(t, c, "", "--ninguem")
	expectNothing(t, a, "guild 5 on guild 6's --")
	expectNothing(t, b, "guild 5 on guild 6's --")
	expectNothing(t, c, "speaker")
}

// TestGuildChatWithoutAllyStaysInGuild: without an alliance "--" is guild-only.
func TestGuildChatWithoutAllyStaysInGuild(t *testing.T) {
	a, b, c, stop := guildChatWorld(t, guildChatDB(false))
	defer stop()
	defer a.Close()
	defer b.Close()
	defer c.Close()

	whisperFrame(t, a, "", "--so guilda")
	expectGuildLine(t, b, 1, "Hero", "--so guilda")
	expectNothing(t, c, "other guild without an alliance")
}

// TestGuildChatOutsideGuild: a player without a guild gets
// _NN_Only_Guild_Member_Can and nobody receives the line.
func TestGuildChatOutsideGuild(t *testing.T) {
	db := guildChatDB(false)
	st := db.loads[13]
	st.GuildID = 0
	db.loads[13] = st
	a, b, c, stop := guildChatWorld(t, db)
	defer stop()
	defer a.Close()
	defer b.Close()
	defer c.Close()

	whisperFrame(t, c, "", "-sem guilda")
	expectPanel(t, c, onlyGuildMemberCan)
	expectNothing(t, a, "guild member")
	expectNothing(t, b, "guild member")
}

// TestGuildChatToggle: "guildchat" stops the guild channel for that player and
// confirms with the legacy panel text (_MSG_MessageChat.cpp:140-150); a second
// toggle turns it back on.
func TestGuildChatToggle(t *testing.T) {
	a, b, c, stop := guildChatWorld(t, guildChatDB(false))
	defer stop()
	defer a.Close()
	defer b.Close()
	defer c.Close()

	chatFrame(t, b, "guildchat")
	expectPanel(t, b, "Guild Chatting : Off")
	whisperFrame(t, a, "", "-voce nao ouve")
	expectNothing(t, b, "member with guildchat off")

	chatFrame(t, b, "guildchat")
	expectPanel(t, b, "Guild Chatting : On")
	whisperFrame(t, a, "", "-agora ouve")
	expectGuildLine(t, b, 1, "Hero", "-agora ouve")
}

// TestGuildChatShortLineGetsColor: a body that stops before Color is padded so
// the client still sees Color 3.
func TestGuildChatShortLineGetsColor(t *testing.T) {
	a, b, c, stop := guildChatWorld(t, guildChatDB(false))
	defer stop()
	defer a.Close()
	defer b.Close()
	defer c.Close()

	var body protocol.MsgWhisperBody
	body.String = []byte("-x\x00")
	send(t, a, protocol.MsgMessageWhisper, body.Encode())
	expectGuildLine(t, b, 1, "Hero", "-x")
}
