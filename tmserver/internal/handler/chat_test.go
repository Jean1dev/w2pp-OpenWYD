package handler

import (
	"encoding/binary"
	"net"
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// celestialDB loads a single Celestial character (the tier the unlock commands
// require) on a STAFF account: the commands are staff-only, and these tests are
// about what they do. Level is above the gates so the commands are the only
// variable.
func celestialDB(classMaster uint8) *fakeDB {
	db := newDB()
	db.accounts["tester"].role = "moderator"
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Celeste", Class: 0, ClassMaster: classMaster,
		X: 5, Y: 5, HP: 1000, MaxHP: 1000, Level: 41,
	}
	return db
}

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

// TestCommandReino verifies /reino routes by the cape's kingdom (issue #208): a
// Hekalotia/Akelonia cape lands on that kingdom's king, every neutral cape (including
// none at all) on the kingdom city. No cape is refused any more.
func TestCommandReino(t *testing.T) {
	tests := []struct {
		name  string
		cape  int16
		wantX int16
		wantY int16
	}{
		{"no cape", 0, reinoDestNeutral[0], reinoDestNeutral[1]},
		{"capa branca", capaBrancaDoMonstroIndex, reinoDestNeutral[0], reinoDestNeutral[1]},
		{"capa verde", greenCapeItem, reinoDestNeutral[0], reinoDestNeutral[1]},
		{"hekalotia base", 545, reinoDestHekalotia[0], reinoDestHekalotia[1]},
		{"hekalotia elite", 3191, reinoDestHekalotia[0], reinoDestHekalotia[1]},
		{"akelonia base", 546, reinoDestAkelonia[0], reinoDestAkelonia[1]},
		{"akelonia elite", 3192, reinoDestAkelonia[0], reinoDestAkelonia[1]},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := newDB()
			db.loads = map[int64]world.CharacterState{
				7: {Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000,
					Equip: [world.MaxEquip]world.Item{reinoCapeSlot: {Index: tc.cape}}},
			}
			addr, stop, _ := startServerClock(t, db)
			defer stop()
			a := enterWorldAs(t, addr, "tester")
			defer a.Close()

			whisperFrame(t, a, "reino", "")
			ty, payload, ok := readMaybe(t, a)
			if !ok || ty != protocol.MsgAction {
				t.Fatalf("got %#x ok=%v, want MsgAction (teleport)", ty, ok)
			}
			var body protocol.MsgActionBody
			if err := body.Decode(payload); err != nil {
				t.Fatalf("decode MsgAction: %v", err)
			}
			// doTeleport spreads the destination by rand%3 on each axis.
			if body.TargetX < tc.wantX || body.TargetX > tc.wantX+2 ||
				body.TargetY < tc.wantY || body.TargetY > tc.wantY+2 {
				t.Errorf("/reino target = %d,%d, want within %d..%d,%d..%d",
					body.TargetX, body.TargetY, tc.wantX, tc.wantX+2, tc.wantY, tc.wantY+2)
			}
		})
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

// TestCommandChaosPoints verifies /cp reports the caller's chaos points as a
// MessageChat line. A freshly logged-in player is never guilty, so the value is 0.
func TestCommandChaosPoints(t *testing.T) {
	addr, stop, _ := startServerClock(t, chatDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()

	whisperFrame(t, a, "cp", "")
	ty, payload, ok := readMaybe(t, a)
	if !ok || ty != protocol.MsgMessageChat {
		t.Fatalf("got %#x ok=%v, want MessageChat", ty, ok)
	}
	want := "Pontos Caos atual: 0"
	if string(payload[:len(want)]) != want {
		t.Errorf("chat text = %q, want %q", payload, want)
	}
}

// trumpetDB is chatDB with "Hero" (account 7) carrying a stack of Trombetas
// Mágicas in slot 0. amount == 0 means no trumpet at all.
func trumpetDB(amount uint8) *fakeDB {
	db := chatDB()
	hero := db.loads[7]
	if amount > 0 {
		hero.Carry[0] = amountItem(itemMagicTrumpet, amount)
	}
	db.loads[7] = hero
	return db
}

// TestCommandGritar verifies /gritar burns one Trombeta Mágica and shouts to
// every player online, the shouter included (_MSG_MessageWhisper.cpp:114).
func TestCommandGritar(t *testing.T) {
	addr, stop, _ := startServerClock(t, trumpetDB(3))
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	whisperFrame(t, a, "gritar", "ola mundo")

	// The shouter's own slot is re-synced first: 3 → 2.
	_, slot, index, amount := sendItemSlotAmount(expect(t, a, protocol.MsgSendItem))
	if slot != 0 || index != itemMagicTrumpet || amount != 2 {
		t.Errorf("slot sync = (slot %d, index %d, amount %d), want (0, %d, 2)", slot, index, amount, itemMagicTrumpet)
	}

	want := "[Hero]> ola mundo"
	for _, tc := range []struct {
		name string
		c    net.Conn
	}{{"shouter", a}, {"other", b}} {
		payload := expect(t, tc.c, protocol.MsgMagicTrumpet)
		if len(payload) != protocol.MagicTrumpetBodySize {
			t.Errorf("%s: body = %d bytes, want %d", tc.name, len(payload), protocol.MagicTrumpetBodySize)
			continue
		}
		if got := cstr(payload); got != want {
			t.Errorf("%s: shout = %q, want %q", tc.name, got, want)
		}
		if payload[94] != 1 {
			t.Errorf("%s: String[94] = %d, want 1 (legacy marker byte)", tc.name, payload[94])
		}
	}
}

// TestCommandGritarLastTrumpet verifies the slot is cleared, not decremented,
// when the last unit is spent (BASE_ClearItem, _MSG_MessageWhisper.cpp:135).
func TestCommandGritarLastTrumpet(t *testing.T) {
	addr, stop, _ := startServerClock(t, trumpetDB(1))
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()

	whisperFrame(t, a, "gritar", "adeus")
	_, slot, index, _ := sendItemSlotAmount(expect(t, a, protocol.MsgSendItem))
	if slot != 0 || index != 0 {
		t.Errorf("slot sync = (slot %d, index %d), want (0, 0) — an empty slot", slot, index)
	}
}

// TestCommandGritarNoTrumpet verifies the command refuses without the item: the
// caller is told why (a deliberate deviation — the legacy returned silently at
// _MSG_MessageWhisper.cpp:139) and nobody else hears anything.
func TestCommandGritarNoTrumpet(t *testing.T) {
	addr, stop, _ := startServerClock(t, trumpetDB(0))
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	whisperFrame(t, a, "gritar", "ola mundo")
	ty, payload, ok := readMaybe(t, a)
	if !ok || ty != protocol.MsgMessageChat {
		t.Fatalf("got %#x ok=%v, want MessageChat", ty, ok)
	}
	if got := cstr(payload); got != "Você precisa de uma Trombeta Mágica para gritar." {
		t.Errorf("refusal = %q", got)
	}
	if ty, _, ok := readMaybe(t, b); ok && ty == protocol.MsgMagicTrumpet {
		t.Error("the shout reached other players without a trumpet being spent")
	}
}

// TestCommandGritarEmpty verifies a bare /gritar is a no-op: no shout and, above
// all, no trumpet burned (the legacy shouted "[Nome]> " and charged for it).
func TestCommandGritarEmpty(t *testing.T) {
	addr, stop, _ := startServerClock(t, trumpetDB(3))
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()

	whisperFrame(t, a, "gritar", "   ")
	if ty, _, ok := readMaybe(t, a); ok {
		t.Errorf("empty shout produced %#x, want silence", ty)
	}
}

// TestCommandSpkAlias verifies /spk, the legacy alias, routes to the same command.
func TestCommandSpkAlias(t *testing.T) {
	addr, stop, _ := startServerClock(t, trumpetDB(2))
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()

	whisperFrame(t, a, "spk", "oi")
	if got := cstr(expect(t, a, protocol.MsgMagicTrumpet)); got != "[Hero]> oi" {
		t.Errorf("shout = %q, want %q", got, "[Hero]> oi")
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

// TestCommandDestravar40 verifies /destravar40 on a Celestial sets the Lv40 gate
// (signalled by MsgCombineComplete) and the flag persists through a save.
func TestCommandDestravar40(t *testing.T) {
	db := celestialDB(classMasterCelestial)
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	whisperFrame(t, c, "destravar40", "")
	ty, p, ok := readMaybe(t, c)
	if !ok || ty != protocol.MsgCombineComplete {
		t.Fatalf("got %#x ok=%v, want MsgCombineComplete", ty, ok)
	}
	if parm := int16(binary.LittleEndian.Uint16(p)); parm != celestialUnlockParm {
		t.Errorf("CombineComplete parm = %d, want %d", parm, celestialUnlockParm)
	}

	// Logout flushes the character save; the gate flag must ride along.
	send(t, c, protocol.MsgCharacterLogout, nil)
	expect(t, c, protocol.MsgCNFCharacterLogout)
	char, n := db.lastSavedChar()
	if n == 0 || char.CelLv40 != 1 {
		t.Errorf("saved CelLv40 = %d (n=%d), want 1", char.CelLv40, n)
	}
}

// TestCommandDestravarNonCelestial verifies the unlock is a no-op for a non-Celestial
// (Mortal): the command is still consumed (not delivered as a whisper) but nothing
// is sent and no flag is set.
func TestCommandDestravarNonCelestial(t *testing.T) {
	db := celestialDB(classMasterMortal)
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	whisperFrame(t, c, "destravar40", "")
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("/destravar40 on a mortal produced %#x; want a silent no-op", ty)
	}
}

// TestCommandDestravar90 verifies /destravar90 grants the Cythera Mística (item
// 3502) to carry, signals the client, and persists both the item and the Lv90 gate.
func TestCommandDestravar90(t *testing.T) {
	db := celestialDB(classMasterCelestial)
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	whisperFrame(t, c, "destravar90", "")
	expect(t, c, protocol.MsgSendItem)        // Cythera Mística into carry
	expect(t, c, protocol.MsgCombineComplete) // unlock signal
	expect(t, c, protocol.MsgMotion)          // unlock emote

	send(t, c, protocol.MsgCharacterLogout, nil)
	expect(t, c, protocol.MsgCNFCharacterLogout)
	char, n := db.lastSavedChar()
	if n == 0 || char.CelLv90 != 1 {
		t.Errorf("saved CelLv90 = %d (n=%d), want 1", char.CelLv90, n)
	}
	if !hasItem(char.Carry, itemCytheraMistica) {
		t.Errorf("saved carry missing Cythera Mística %d: %+v", itemCytheraMistica, char.Carry)
	}
}

// TestCommandDestravarPlayerDenied: a player typing any of the three unlock
// commands gets nothing — no flag, no item, no signal — while the command is
// still swallowed (not delivered as a whisper to someone named "destravar90").
// Anybody could type them before, which skipped every requirement of the 90 lock
// and of the Arcana.
func TestCommandDestravarPlayerDenied(t *testing.T) {
	for _, cmd := range []string{"destravar40", "destravar90", "arcana"} {
		t.Run(cmd, func(t *testing.T) {
			db := celestialDB(classMasterCelestial)
			db.accounts["tester"].role = "player"
			addr, stop, _ := startServerClock(t, db)
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()

			whisperFrame(t, c, cmd, "")
			if ty, _, ok := readMaybe(t, c); ok {
				t.Errorf("/%s from a player produced %#x; want silence", cmd, ty)
			}
			send(t, c, protocol.MsgCharacterLogout, nil)
			expect(t, c, protocol.MsgCNFCharacterLogout)
			char, _ := db.lastSavedChar()
			if char.CelLv40 != 0 || char.CelLv90 != 0 || char.CelCircle != 0 {
				t.Errorf("/%s from a player set Lv40/Lv90/Circle = %d/%d/%d, want 0/0/0", cmd, char.CelLv40, char.CelLv90, char.CelCircle)
			}
		})
	}
}

// TestCommandArcana verifies /arcana places the reward (item 3507) in Equip[1], sets
// the Circle flag, and persists both.
func TestCommandArcana(t *testing.T) {
	db := celestialDB(classMasterCelestial)
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	whisperFrame(t, c, "arcana", "")
	expect(t, c, protocol.MsgSendItem)        // reward into Equip[1]
	expect(t, c, protocol.MsgCombineComplete) // signal
	expect(t, c, protocol.MsgMotion)          // emote

	send(t, c, protocol.MsgCharacterLogout, nil)
	expect(t, c, protocol.MsgCNFCharacterLogout)
	char, n := db.lastSavedChar()
	if n == 0 || char.CelCircle != 1 {
		t.Errorf("saved CelCircle = %d (n=%d), want 1", char.CelCircle, n)
	}
	if !hasItem(char.Equip, arcanaItemIndex) {
		t.Errorf("saved equip missing arcana item %d: %+v", arcanaItemIndex, char.Equip)
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
	p := panelText(t, a)
	want := "HeroB Cidadania: 0 / Fama: 0"
	if got := p; got != want {
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
	p := panelText(t, a)
	want := "HeroB Cidadania: 2 / Fama: 150 [Guild #9]"
	if got := p; got != want {
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
	p := panelText(t, mod)
	want := "Victim Cidadania: 3 / Fama: 777 [Dragoes]"
	if got := p; got != want {
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
	// The notice on its own renders as nothing — its wire format is still a
	// placeholder (notice.go) — so the visible chat line is what stops the
	// command from looking dead.
	if got := panelText(t, a); got != "O jogador não está conectado." {
		t.Errorf("offline text = %q, want the not-connected chat line", got)
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
	want := baseDamageChar + 12/2 + 10/3 + 10
	if dmg := scoreDamage(payload); dmg != want {
		t.Errorf("UpdateScore Damage = %d, want %d after +1 STR (11→12)", dmg, want)
	}
}

// Typing just "/NomeDoJogador" — a whisper with no text — is the legacy's
// "inspect this player": it answers with their name, citizenship and fame
// instead of delivering an empty whisper (_MSG_UseItem is not involved;
// _MSG_MessageWhisper.cpp:1616, `if (m->String[0] == 0)`).
//
// This is the bug this test was written for: the empty-message branch did not
// exist, so the server forwarded a blank whisper and the player saw nothing.
func TestWhisperEmptyMessageShowsUserInfo(t *testing.T) {
	db := newDB()
	db.loads = map[int64]world.CharacterState{
		7:  {Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000},
		11: {Slot: 0, Name: "HeroB", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Citizen: 4, Fame: 321},
	}
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb") // name "HeroB"
	defer b.Close()

	whisperFrame(t, a, "HeroB", "")

	want := "HeroB Cidadania: 4 / Fama: 321"
	if got := panelText(t, a); got != want {
		t.Errorf("info text = %q, want %q", got, want)
	}
	// And the target must NOT receive a blank whisper.
	if ty, _, ok := readMaybe(t, b); ok {
		t.Errorf("target got %#x; an inspect must not deliver a whisper", ty)
	}
}

// The guild goes in brackets at the end, and only when there is one.
func TestWhisperEmptyMessageShowsGuild(t *testing.T) {
	db := newDB()
	db.loads = map[int64]world.CharacterState{
		7:  {Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000},
		11: {Slot: 0, Name: "HeroB", X: 5, Y: 5, HP: 1000, MaxHP: 1000, GuildID: 3, Citizen: 1, Fame: 10},
	}
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	whisperFrame(t, a, "HeroB", "")

	want := "HeroB Cidadania: 1 / Fama: 10 [Guild #3]"
	if got := panelText(t, a); got != want {
		t.Errorf("info text = %q, want %q", got, want)
	}
}

// A whisper WITH text is still a whisper. The legacy tests the first byte, so
// even a single space counts as a message — inspect is strictly the empty case.
func TestWhisperWithTextStillDelivers(t *testing.T) {
	addr, stop, _ := startServerClock(t, chatDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	whisperFrame(t, a, "HeroB", "ola")

	if ty, _, ok := readMaybe(t, b); !ok || ty != protocol.MsgMessageWhisper {
		t.Errorf("target got %#x/%v, want the whisper delivered", ty, ok)
	}
	// The sender gets no info line — they sent a message, not an inspect.
	if ty, _, ok := readMaybe(t, a); ok {
		t.Errorf("sender got %#x; a delivered whisper answers nothing", ty)
	}
}

// Inspecting someone who is offline must say so out loud.
func TestWhisperEmptyMessageOfflineTarget(t *testing.T) {
	addr, stop, _ := startServerClock(t, chatDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()

	whisperFrame(t, a, "Ghost", "")

	if ty, p, ok := readMaybe(t, a); !ok || ty != protocol.MsgMessageBoxOk || noticeCode(t, p) != NoticeNotConnected {
		t.Errorf("got %#x/%d, want not-connected notice", ty, noticeCode(t, p))
	}
	if got := panelText(t, a); got != "O jogador não está conectado." {
		t.Errorf("offline text = %q, want the not-connected chat line", got)
	}
}

// "/snd <texto>" sets the status line other players see when they inspect this
// character, and echoes it back (_MSG_MessageWhisper.cpp:591).
func TestCommandSndSetsAndEchoes(t *testing.T) {
	addr, stop, _ := startServerClock(t, chatDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()

	whisperFrame(t, a, "snd", "volto ja")
	if got := panelText(t, a); got != "Mensagem: volto ja" {
		t.Errorf("/snd echo = %q, want %q", got, "Mensagem: volto ja")
	}
}

// Inspecting a character who set a status line gets it as a SECOND chat line,
// after the citizenship/fame one (_MSG_MessageWhisper.cpp:1638-1646).
func TestWhisperEmptyMessageShowsSnd(t *testing.T) {
	db := newDB()
	db.loads = map[int64]world.CharacterState{
		7:  {Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000},
		11: {Slot: 0, Name: "HeroB", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Citizen: 2, Fame: 50},
	}
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	whisperFrame(t, b, "snd", "afk almoco")
	drainRaw(t, b)

	whisperFrame(t, a, "HeroB", "")
	if got := panelText(t, a); got != "HeroB Cidadania: 2 / Fama: 50" {
		t.Errorf("first line = %q, want the info line", got)
	}
	if got := panelText(t, a); got != "Mensagem: afk almoco" {
		t.Errorf("second line = %q, want the snd line", got)
	}
}

// A character with no status line produces exactly one line — the legacy guards
// the second with `if (pMob[target].Snd[0] != 0)`.
func TestWhisperEmptyMessageWithoutSndSendsOneLine(t *testing.T) {
	addr, stop, _ := startServerClock(t, chatDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	whisperFrame(t, a, "HeroB", "")
	if got := panelText(t, a); got != "HeroB Cidadania: 0 / Fama: 0" {
		t.Errorf("info line = %q", got)
	}
	if ty, _, ok := readMaybe(t, a); ok {
		t.Errorf("got a second frame %#x; a character with no /snd sends one line", ty)
	}
}

// "/snd" with nothing after it clears the line, the way the legacy's strncpy of
// an empty string does — so the next inspect is back to one line.
func TestCommandSndEmptyClears(t *testing.T) {
	addr, stop, _ := startServerClock(t, chatDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	whisperFrame(t, b, "snd", "ocupado")
	whisperFrame(t, b, "snd", "")
	drainRaw(t, b)

	whisperFrame(t, a, "HeroB", "")
	panelText(t, a) // the info line
	if ty, _, ok := readMaybe(t, a); ok {
		t.Errorf("got %#x; an empty /snd should have cleared the status line", ty)
	}
}

// Two different caps apply, and both are the legacy's. Snd itself holds at most
// 96 bytes (strncpy, _MSG_MessageWhisper.cpp:593); the line that carries it to
// the client is then cut to the message panel's usable 94 (MESSAGE_LENGTH-2,
// SendFunc.cpp:27). So an over-long /snd comes back trimmed by the transport,
// not by setSnd.
func TestCommandSndTruncates(t *testing.T) {
	const panelTextMax = 94

	addr, stop, _ := startServerClock(t, chatDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()

	whisperFrame(t, a, "snd", strings.Repeat("x", sndMaxLen+20))

	got := panelText(t, a)
	if len(got) != panelTextMax {
		t.Errorf("/snd echo is %d bytes, want the panel maximum of %d", len(got), panelTextMax)
	}
	if !strings.HasPrefix(got, "Mensagem: x") {
		t.Errorf("/snd echo = %q, want the prefix kept and the text trimmed at the end", got)
	}
}

// panelText reads one MSG_MessagePanel frame and decodes its body back to a Go
// string. The wire form is CP1252 in a fixed 128-byte buffer, so comparing the
// raw payload against a UTF-8 literal would fail on every accent — which is the
// whole bug protocol.ClientText exists to fix.
func panelText(t *testing.T, c net.Conn) string {
	t.Helper()
	b := expect(t, c, protocol.MsgMessagePanel)
	if i := indexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	// CP1252 is Latin-1 for everything this port emits, and Latin-1 bytes are
	// their own Unicode code points.
	runes := make([]rune, len(b))
	for i, ch := range b {
		runes[i] = rune(ch)
	}
	return string(runes)
}
