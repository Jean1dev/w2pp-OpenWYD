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

// TestCommandReinoNoCape verifies /reino teleports a character with no cape equipped
// (issue #127).
func TestCommandReinoNoCape(t *testing.T) {
	addr, stop, _ := startServerClock(t, chatDB())
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
	if body.TargetX < 1702 || body.TargetX > 1704 || body.TargetY < 1728 || body.TargetY > 1730 {
		t.Errorf("/reino target = %d,%d, want within 1702..1704,1728..1730", body.TargetX, body.TargetY)
	}
}

// TestCommandReinoWhiteCape verifies /reino also teleports a character wearing the
// neutral Capa Branca do Monstro (#550).
func TestCommandReinoWhiteCape(t *testing.T) {
	db := newDB()
	db.loads = map[int64]world.CharacterState{
		7: {Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000,
			Equip: [world.MaxEquip]world.Item{15: {Index: capaBrancaDoMonstroIndex}}},
	}
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()

	whisperFrame(t, a, "reino", "")
	if ty, _, ok := readMaybe(t, a); !ok || ty != protocol.MsgAction {
		t.Errorf("got %#x ok=%v, want MsgAction (teleport)", ty, ok)
	}
}

// TestCommandReinoBlocked verifies /reino refuses (and notifies) a character wearing
// any other (kingdom-aligned) cape.
func TestCommandReinoBlocked(t *testing.T) {
	db := newDB()
	db.loads = map[int64]world.CharacterState{
		7: {Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000,
			Equip: [world.MaxEquip]world.Item{15: {Index: 543}}},
	}
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()

	whisperFrame(t, a, "reino", "")
	ty, payload, ok := readMaybe(t, a)
	if !ok || ty != protocol.MsgMessageBoxOk || noticeCode(t, payload) != NoticeReinoCapeRequired {
		t.Errorf("got %#x ok=%v, want MsgMessageBoxOk/NoticeReinoCapeRequired", ty, ok)
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
