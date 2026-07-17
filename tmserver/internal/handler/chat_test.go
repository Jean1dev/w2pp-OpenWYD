package handler

import (
	"net"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func chatDB() *fakeDB {
	db := newDB()
	db.loads = map[int64]world.CharacterState{
		7:  {Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000},
		11: {Slot: 0, Name: "HeroB", X: 5, Y: 5, HP: 1000, MaxHP: 1000},
	}
	return db
}

func chatFrame(t *testing.T, c net.Conn, text string) {
	t.Helper()
	send(t, c, protocol.MsgMessageChat, []byte(text))
}

func whisperFrame(t *testing.T, c net.Conn, target, text string) {
	t.Helper()
	var body protocol.MsgWhisperBody
	copy(body.MobName[:], target)
	body.String = []byte(text)
	send(t, c, protocol.MsgMessageWhisper, body.Encode())
}

// TestCommandTeleport verifies a /city command (delivered as a whisper to the city
// name) teleports the player (MSG_Action jump) instead of being a whisper.
func TestCommandTeleport(t *testing.T) {
	addr, stop, _ := startServerClock(t, chatDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()

	whisperFrame(t, a, "armia", "")
	if ty, _, ok := readMaybe(t, a); !ok || ty != protocol.MsgAction {
		t.Errorf("got %#x ok=%v, want MsgAction (teleport)", ty, ok)
	}
}

// TestCommandNoatunTeleport verifies /noatun teleports to Noatun with the same
// rand%3 spread used by the other free teleport commands.
func TestCommandNoatunTeleport(t *testing.T) {
	addr, stop, _ := startServerClock(t, chatDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()

	whisperFrame(t, a, "noatun", "")
	ty, payload, ok := readMaybe(t, a)
	if !ok || ty != protocol.MsgAction {
		t.Fatalf("got %#x ok=%v, want MsgAction (teleport)", ty, ok)
	}
	var body protocol.MsgActionBody
	if err := body.Decode(payload); err != nil {
		t.Fatalf("decode MsgAction: %v", err)
	}
	if body.TargetX < 1052 || body.TargetX > 1054 || body.TargetY < 1726 || body.TargetY > 1728 {
		t.Errorf("/noatun target = %d,%d, want within 1052..1054,1726..1728", body.TargetX, body.TargetY)
	}
}

// TestCommandBuffs verifies /buffs clears affects and pushes a fresh score.
func TestCommandBuffs(t *testing.T) {
	addr, stop, _ := startServerClock(t, chatDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()

	whisperFrame(t, a, "buffs", "")
	if ty, _, ok := readMaybe(t, a); !ok || ty != protocol.MsgUpdateScore {
		t.Errorf("got %#x ok=%v, want MsgUpdateScore after /buffs", ty, ok)
	}
}

// TestCommandSair verifies /sair is handled as a command (not delivered as a whisper
// to a missing player): a guilded player leaving with no one in view produces no packet.
func TestCommandSair(t *testing.T) {
	db := newDB()
	db.loads = map[int64]world.CharacterState{
		7: {Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000, GuildID: 5},
	}
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()

	whisperFrame(t, a, "sair", "")
	// Handled as a command → no NoticeNotConnected (which a real whisper to "sair" would
	// trigger). With no in-view players, the guild-tag broadcast reaches no one.
	if ty, _, ok := readMaybe(t, a); ok {
		t.Errorf("/sair produced %#x; want a silent handled command", ty)
	}
}

func TestGuildCreateCommand(t *testing.T) {
	db := newDB()
	db.loads = map[int64]world.CharacterState{
		7: {Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Clan: 7, Citizen: 1, Coin: 100_000_000},
	}
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	drainRaw(t, a)

	whisperFrame(t, a, "create", "Knights")
	if ty, _, ok := readMaybe(t, a); !ok || ty != protocol.MsgUpdateEtc {
		t.Fatalf("/create got %#x ok=%v, want UpdateEtc", ty, ok)
	}
	if ty, _, ok := readMaybeRaw(t, a); !ok || ty != protocol.MsgCreateMob {
		t.Fatalf("/create got %#x ok=%v, want CreateMob tag refresh", ty, ok)
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	if len(db.createdGuilds) != 1 || db.createdGuilds[0].Name != "Knights" {
		t.Fatalf("created guilds = %+v, want Knights", db.createdGuilds)
	}
}

func TestGuildSubcreateCommand(t *testing.T) {
	db := newDB()
	db.loads = map[int64]world.CharacterState{
		7:  {Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Clan: 1, GuildID: 5, GuildLevel: 9, Coin: 100_000_000},
		11: {Slot: 0, Name: "HeroB", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Clan: 1, GuildID: 5},
	}
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()
	drainRaw(t, a)
	drainRaw(t, b)

	whisperFrame(t, a, "subcreate", "HeroB Sub")
	if ty, _, ok := readMaybe(t, a); !ok || ty != protocol.MsgUpdateEtc {
		t.Fatalf("/subcreate got %#x ok=%v, want UpdateEtc", ty, ok)
	}
	if ty, _, ok := readMaybeRaw(t, a); !ok || ty != protocol.MsgCreateMob {
		t.Fatalf("/subcreate leader got %#x ok=%v, want CreateMob tag refresh", ty, ok)
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	if len(db.promoted) != 1 || db.promoted[0] != 6 {
		t.Fatalf("promoted = %v, want [6]", db.promoted)
	}
}

func TestGuildHandoverCommand(t *testing.T) {
	db := newDB()
	db.loads = map[int64]world.CharacterState{
		7:  {Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Clan: 1, GuildID: 5, GuildLevel: 9},
		11: {Slot: 0, Name: "HeroB", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Clan: 1, GuildID: 5},
	}
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()
	drainRaw(t, a)
	drainRaw(t, b)

	whisperFrame(t, a, "handover", "HeroB")
	if ty, _, ok := readMaybeRaw(t, a); !ok || ty != protocol.MsgCreateMob {
		t.Fatalf("/handover leader got %#x ok=%v, want CreateMob tag refresh", ty, ok)
	}
	if ty, _, ok := readMaybeRaw(t, b); !ok || ty != protocol.MsgCreateMob {
		t.Fatalf("/handover target got %#x ok=%v, want CreateMob tag refresh", ty, ok)
	}
}

func TestChatPublicBroadcast(t *testing.T) {
	addr, stop, _ := startServerClock(t, chatDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	chatFrame(t, a, "hello world")
	ty, p, ok := readMaybe(t, b)
	if !ok || ty != protocol.MsgMessageChat {
		t.Fatalf("got %#x ok=%v, want MessageChat broadcast", ty, ok)
	}
	if string(p[:len("hello world")]) != "hello world" {
		t.Errorf("chat text = %q", p)
	}
}

func TestWhisperDeliver(t *testing.T) {
	addr, stop, _ := startServerClock(t, chatDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb") // name "HeroB"
	defer b.Close()

	whisperFrame(t, a, "HeroB", "psst")
	if ty, _, ok := readMaybe(t, b); !ok || ty != protocol.MsgMessageWhisper {
		t.Errorf("got %#x ok=%v, want MessageWhisper delivered", ty, ok)
	}
}

func TestWhisperOffline(t *testing.T) {
	addr, stop, _ := startServerClock(t, chatDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()

	whisperFrame(t, a, "Ghost", "anyone?")
	if ty, p, ok := readMaybe(t, a); !ok || ty != protocol.MsgMessageBoxOk || noticeCode(t, p) != NoticeNotConnected {
		t.Errorf("got %#x/%d, want not-connected notice", ty, noticeCode(t, p))
	}
}

func TestWhisperBlocked(t *testing.T) {
	addr, stop, _ := startServerClock(t, chatDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	// B blocks whispers, then chats publicly so A's read confirms the toggle ran.
	chatFrame(t, b, "whisper")
	chatFrame(t, b, "ping")
	if ty, _, ok := readMaybe(t, a); !ok || ty != protocol.MsgMessageChat {
		t.Fatalf("expected B's public ping, got %#x ok=%v", ty, ok)
	}

	whisperFrame(t, a, "HeroB", "hi")
	if ty, p, ok := readMaybe(t, a); !ok || ty != protocol.MsgMessageBoxOk || noticeCode(t, p) != NoticeDenyWhisper {
		t.Errorf("got %#x/%d, want deny-whisper notice", ty, noticeCode(t, p))
	}
}

// TestCommandNickGuildless verifies /nick on a guildless target reports "Sem
// guilda" plus its (zeroed) citizenship/fame.
func TestCommandNickGuildless(t *testing.T) {
	addr, stop, _ := startServerClock(t, chatDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb") // name "HeroB"
	defer b.Close()

	whisperFrame(t, a, "nick", "HeroB")
	p := expect(t, a, protocol.MsgMessageChat)
	want := "HeroB [Sem guilda] Cidadania: 0 / Fama: 0"
	if got := cstr(p); got != want {
		t.Errorf("/nick text = %q, want %q", got, want)
	}
}

// TestCommandNickUnregisteredGuild verifies a target with a nonzero guild id
// that was never registered via /gm guildname|guildfame shows a placeholder.
func TestCommandNickUnregisteredGuild(t *testing.T) {
	db := newDB()
	db.loads = map[int64]world.CharacterState{
		7:  {Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000},
		11: {Slot: 0, Name: "HeroB", X: 5, Y: 5, HP: 1000, MaxHP: 1000, GuildID: 9, Citizen: 2, Fame: 150},
	}
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	whisperFrame(t, a, "nick", "HeroB")
	p := expect(t, a, protocol.MsgMessageChat)
	want := "HeroB [Guild #9 (nao registrada)] Cidadania: 2 / Fama: 150"
	if got := cstr(p); got != want {
		t.Errorf("/nick text = %q, want %q", got, want)
	}
}

// TestCommandNickRegisteredGuild verifies /nick shows the registered guild
// name/fame (set via /gm guildname and /gm guildfame) alongside citizenship
// and character fame.
func TestCommandNickRegisteredGuild(t *testing.T) {
	db := gmDB()
	db.loads[23] = world.CharacterState{ // account id 23 = "victim" (see gmDB)
		Slot: 0, Name: "Victim", Class: 0, Level: 10, X: 5, Y: 5, HP: 1000, MaxHP: 1000,
		GuildID: 9, Citizen: 3, Fame: 777,
	}
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	mod := enterWorldAs(t, addr, "mod")
	defer mod.Close()
	victim := enterWorldAs(t, addr, "victim")
	defer victim.Close()

	gmFrame(t, mod, "guildname 9 Dragoes")
	gmFrame(t, mod, "guildfame 9 4242")
	if ty, _, ok := readMaybe(t, mod); ok {
		t.Errorf("guildname/guildfame produced %#x; want no client-visible packet", ty)
	}

	whisperFrame(t, mod, "nick", "Victim")
	p := expect(t, mod, protocol.MsgMessageChat)
	want := "Victim [Dragoes (Fama guilda: 4242)] Cidadania: 3 / Fama: 777"
	if got := cstr(p); got != want {
		t.Errorf("/nick text = %q, want %q", got, want)
	}
}

// TestCommandNickOffline verifies /nick on an unknown name behaves like a
// normal offline whisper (the existing not-connected notice).
func TestCommandNickOffline(t *testing.T) {
	addr, stop, _ := startServerClock(t, chatDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()

	whisperFrame(t, a, "nick", "Ghost")
	if ty, p, ok := readMaybe(t, a); !ok || ty != protocol.MsgMessageBoxOk || noticeCode(t, p) != NoticeNotConnected {
		t.Errorf("got %#x/%d, want not-connected notice", ty, noticeCode(t, p))
	}
}

func TestApplyBonusScore(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Hero", Class: 0, ClassMaster: classMasterMortal, X: 5, Y: 5,
		HP: 1000, MaxHP: 1000, Str: 11, Dex: 10, Level: 10,
		Damage:     50 + 11/2 + 10/3 + 10,
		ScoreBonus: 5,
		Equip:      [world.MaxEquip]world.Item{{Index: 11}},
	}
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgApplyBonusBody{BonusType: protocol.BonusScore, Detail: protocol.DetailStr}
	send(t, c, protocol.MsgApplyBonus, body.Encode())
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgUpdateEtc {
		t.Errorf("got %#x ok=%v, want UpdateEtc first", ty, ok)
	}
	ty, payload, ok := readMaybe(t, c)
	if !ok || ty != protocol.MsgUpdateScore {
		t.Errorf("got %#x ok=%v, want UpdateScore second", ty, ok)
	}
	if dmg := scoreDamage(payload); dmg != 69 {
		t.Errorf("UpdateScore Damage = %d, want 69 after +1 STR (11→12)", dmg)
	}
}
