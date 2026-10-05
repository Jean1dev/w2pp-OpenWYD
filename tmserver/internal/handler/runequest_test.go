package handler

import (
	"os"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func uxmalTemplate(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("../../../Release/TMsrv/run/npc/Uxmal")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func runeQuestTestTime(minute, second int) time.Time {
	return time.Date(2026, 9, 24, 12, minute, second, 0, time.Local)
}

func runeTicket(sanc uint8) world.Item {
	it := world.Item{Index: itemPistaDaRunas}
	if sanc > 0 {
		it.Effects[0] = world.Effect{Effect: efSanc, Value: sanc}
	}
	return it
}

// installRuneQuestGenerators gives the Lich room generators one-leader recipes
// at their NPCGener.txt positions.
func installRuneQuestGenerators(w *world.World) {
	gens := make([]*world.Generator, 5976)
	gens[5653] = &world.Generator{LeaderTmpl: kefraBossMob(0), MaxNumMob: 1, MinuteGenerate: -1, SegX: [5]int16{3419}, SegY: [5]int16{1629}}
	gens[5654] = &world.Generator{LeaderTmpl: kefraBossMob(0), MaxNumMob: 1, MinuteGenerate: -1, SegX: [5]int16{3355}, SegY: [5]int16{1637}}
	w.RegisterGenerators(gens)
}

func TestUxmalNPCShapes(t *testing.T) {
	uxmal := uxmalTemplate(t)
	if uxmal[17] != uxmalMerchant || uxmal[92+12] != 104 {
		t.Fatalf("Uxmal template merchant bytes = %d/%d, want 72/104", uxmal[17], uxmal[92+12])
	}
	trainer := make([]byte, 816)
	trainer[92+12] = 104
	for _, tc := range []struct {
		name string
		npc  *world.Entity
		want bool
	}{
		{"nil", nil, false},
		{"real template", &world.Entity{Merchant: 104, Template: uxmal}, true},
		{"trainer sharing merchant 104", &world.Entity{Merchant: 104, Template: trainer}, false},
		{"short template", &world.Entity{Template: make([]byte, 17)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isUxmalNPC(tc.npc); got != tc.want {
				t.Errorf("isUxmalNPC=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestUxmalRegistration(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, perzenDB(0))
	c := enterWorld(t, addr)
	defer c.Close()
	var uxmalID int
	runInLoop(t, w, func() {
		uxmalID = w.SpawnMob(uxmalTemplate(t), 3293, 1693)
		if uxmalID < world.MaxUser {
			t.Fatal("spawn Uxmal failed")
		}
	})
	full := world.RuneQuestParty{LeaderID: 900, LeaderName: "Other"}
	for _, tc := range []struct {
		name     string
		minute   int
		leader   int
		ticket   world.Item
		room     int // pre-filled room, -1 for none
		filled   int // slots pre-filled in that room
		self     bool
		wantSala int // registered room, -1 for rejection
	}{
		{"closed window", 5, 0, runeTicket(0), -1, 0, false, -1},
		{"closed at exit minute", 15, 0, runeTicket(0), -1, 0, false, -1},
		{"party member", 17, 10, runeTicket(0), -1, 0, false, -1},
		{"no ticket", 17, 0, world.Item{Index: 1}, -1, 0, false, -1},
		{"room 0", 16, 0, runeTicket(0), -1, 0, false, 0},
		{"solo leader -1", 19, -1, runeTicket(0), -1, 0, false, 0},
		{"room 2", 37, 0, runeTicket(2), -1, 0, false, 2},
		{"sanc above 6 capped", 59, 0, runeTicket(9), -1, 0, false, 6},
		{"room 0 full with two", 17, 0, runeTicket(0), 0, 2, false, -1},
		{"room 1 has a third slot", 17, 0, runeTicket(1), 1, 2, false, 1},
		{"already registered", 17, 0, runeTicket(3), 3, 1, true, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runInLoop(t, w, func() {
				s, e := w.SessionByName("Hero")
				now := runeQuestTestTime(tc.minute, 0)
				d.now = func() time.Time { return now }
				st := world.RuneQuestState{Initialized: true, PeriodUnix: runeQuestPeriod(now).Unix(), Entered: true, Exited: tc.minute%20 >= 15}
				if tc.room >= 0 {
					for j := 0; j < tc.filled; j++ {
						st.Rooms[tc.room].Party[j] = full
					}
					if tc.self {
						st.Rooms[tc.room].Party[0].LeaderID = s.Conn
					}
				}
				w.SetRuneQuestState(st)
				w.SetEntityPos(s.Conn, 3294, 1694)
				e.Leader = tc.leader
				e.Carry[0] = world.Item{}
				e.Carry[1] = tc.ticket
				panels := w.SentOfType(s, protocol.MsgMessagePanel)
				items := w.SentOfType(s, protocol.MsgSendItem)

				d.quest(w, s, protocol.Header{}, protocol.EncodeStandardParm2(int32(uxmalID), 0))

				if got := w.SentOfType(s, protocol.MsgMessagePanel) - panels; got != 1 {
					t.Errorf("message panels = %d, want 1", got)
				}
				got := w.RuneQuestState()
				if tc.wantSala < 0 {
					if e.Carry[1] != tc.ticket {
						t.Errorf("rejected registration changed the ticket: %+v", e.Carry[1])
					}
					if w.SentOfType(s, protocol.MsgSendItem) != items {
						t.Error("rejected registration sent an item update")
					}
					if !tc.self && tc.ticket.Index == itemPistaDaRunas {
						for sala := range got.Rooms {
							for _, p := range got.Rooms[sala].Party {
								if p.LeaderID == s.Conn {
									t.Errorf("rejected registration stored in room %d", sala)
								}
							}
						}
					}
					return
				}
				if e.Carry[1] != (world.Item{}) {
					t.Errorf("ticket not consumed: %+v", e.Carry[1])
				}
				if w.SentOfType(s, protocol.MsgSendItem) != items+1 {
					t.Error("consumed slot not sent to the client")
				}
				slot := tc.filled
				if tc.room != tc.wantSala {
					slot = 0
				}
				p := got.Rooms[tc.wantSala].Party[slot]
				if p.LeaderID != s.Conn || p.LeaderName != "Hero" || p.Sala != tc.wantSala || p.MobCount != 0 {
					t.Errorf("room %d slot %d = %+v", tc.wantSala, slot, p)
				}
			})
		})
	}
}

func TestRuneQuestEntryAndExit(t *testing.T) {
	db := tradeDB()
	member := db.loads[11]
	member.Name = "HeroB"
	db.loads[11] = member
	addr, d, w, _ := startKefraServer(t, db)
	cl := enterWorldAs(t, addr, "tester")
	defer cl.Close()
	cm := enterWorldAs(t, addr, "tradeb")
	defer cm.Close()

	runInLoop(t, w, func() {
		installRuneQuestGenerators(w)
		ls, leader := w.SessionByName("Hero")
		ms, mate := w.SessionByName("HeroB")
		if ls == nil || ms == nil {
			t.Fatal("players not in world")
		}
		now := runeQuestTestTime(17, 0)
		d.now = func() time.Time { return now }
		w.SetRuneQuestState(world.RuneQuestState{})
		d.tickRuneQuest(w)

		leader.Leader, mate.Leader = 0, ls.Conn
		leader.PartyList[0] = ms.Conn
		w.SetEntityPos(ls.Conn, 3294, 1694)
		w.SetEntityPos(ms.Conn, 3296, 1694)
		st := w.RuneQuestState()
		st.Rooms[0].Party[0] = world.RuneQuestParty{LeaderID: ls.Conn, LeaderName: "Hero"}
		// A stale slot whose conn now belongs to someone else is not admitted.
		st.Rooms[0].Party[1] = world.RuneQuestParty{LeaderID: ms.Conn, LeaderName: "Gone"}
		w.SetRuneQuestState(st)

		now = runeQuestTestTime(20, 0)
		d.tickRuneQuest(w)
		want := world.RuneQuestEntryPos[0][0]
		if leader.X != want[0] || leader.Y != want[1] {
			t.Errorf("leader at %d,%d, want %v", leader.X, leader.Y, want)
		}
		if mate.X != want[0] || mate.Y != want[1] {
			t.Errorf("member at %d,%d, want %v", mate.X, mate.Y, want)
		}
		if g := w.GeneratorAt(5654); g.CurrentNumMob == 0 {
			t.Error("Lich group 1 generator did not spawn")
		}
		if g := w.GeneratorAt(5653); g.CurrentNumMob != 0 {
			t.Error("stale group 2 spawned its Lich")
		}

		// Entry runs once per round.
		w.SetEntityPos(ls.Conn, 3294, 1694)
		now = runeQuestTestTime(20, 30)
		d.tickRuneQuest(w)
		if leader.X != 3294 {
			t.Error("entry repeated within the round")
		}

		w.SetEntityPos(ls.Conn, want[0], want[1])
		mate.HP = 0
		now = runeQuestTestTime(35, 0)
		d.tickRuneQuest(w)
		for _, e := range []*world.Entity{leader, mate} {
			if e.X != runeQuestExitX || e.Y != runeQuestExitY {
				t.Errorf("%s at %d,%d after exit", e.Name, e.X, e.Y)
			}
		}
		if mate.HP != 1 {
			t.Errorf("dead member HP = %d, want 1", mate.HP)
		}
		if w.GeneratorAt(5654).CurrentNumMob != 0 {
			t.Error("exit left rune track mobs alive")
		}
		w.ForEachMob(func(_ int, e *world.Entity) {
			if world.RuneQuestArea(e.X, e.Y) {
				t.Errorf("mob %s left in the rooms at %d,%d", e.Name, e.X, e.Y)
			}
		})
		if got := w.RuneQuestState().Rooms; got != ([world.RuneQuestRooms]world.RuneQuestRoom{}) {
			t.Errorf("registrations not cleared: %+v", got)
		}
	})
}

func TestRuneQuestEntryRequiresLobby(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, perzenDB(0))
	c := enterWorld(t, addr)
	defer c.Close()
	for _, tc := range []struct {
		name   string
		x, y   int16
		leader int
		moved  bool
	}{
		{"in lobby", 3294, 1694, 0, true},
		{"left the lobby", 2100, 2100, 0, false},
		{"joined another party", 3294, 1694, 10, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runInLoop(t, w, func() {
				installRuneQuestGenerators(w)
				s, e := w.SessionByName("Hero")
				now := runeQuestTestTime(19, 0)
				d.now = func() time.Time { return now }
				w.SetRuneQuestState(world.RuneQuestState{})
				d.tickRuneQuest(w)
				st := w.RuneQuestState()
				st.Rooms[2].Party[1] = world.RuneQuestParty{LeaderID: s.Conn, LeaderName: "Hero", Sala: 2}
				w.SetRuneQuestState(st)
				w.SetEntityPos(s.Conn, tc.x, tc.y)
				e.Leader = tc.leader

				now = runeQuestTestTime(40, 1)
				d.tickRuneQuest(w)
				want := world.RuneQuestEntryPos[2][1]
				if moved := e.X == want[0] && e.Y == want[1]; moved != tc.moved {
					t.Errorf("teleported=%v, want %v (at %d,%d)", moved, tc.moved, e.X, e.Y)
				}
				e.Leader = 0
			})
		})
	}
}

func TestRuneQuestRestartMidRound(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, perzenDB(0))
	c := enterWorld(t, addr)
	defer c.Close()
	runInLoop(t, w, func() {
		s, e := w.SessionByName("Hero")
		w.SetEntityPos(s.Conn, 3400, 1500)
		now := runeQuestTestTime(25, 0)
		d.now = func() time.Time { return now }
		st := world.RuneQuestState{}
		st.Rooms[0].Party[0] = world.RuneQuestParty{LeaderID: s.Conn, LeaderName: "Hero"}
		w.SetRuneQuestState(st)
		d.tickRuneQuest(w)
		if e.X != 3400 || e.Y != 1500 {
			t.Error("restart mid-round ran the entry or exit")
		}
		now = runeQuestTestTime(35, 0)
		d.tickRuneQuest(w)
		if e.X != runeQuestExitX || e.Y != runeQuestExitY {
			t.Error("exit did not run after a mid-round restart")
		}
		// A backward clock does not replay the exit.
		w.SetEntityPos(s.Conn, 3400, 1500)
		now = runeQuestTestTime(15, 0)
		d.tickRuneQuest(w)
		if e.X != 3400 {
			t.Error("backward clock replayed a round")
		}
	})
}

func TestIsRuneQuestGenerator(t *testing.T) {
	for _, tc := range []struct {
		idx  int
		want bool
	}{
		{5652, false}, // Tauron
		{5653, true},  // Lich
		{5656, false}, // permanent elf population (MinuteGenerate 1)
		{5706, true},  // Torre
		{5764, true},
		{5766, false}, // Tauron
		{5789, true},  // Amon boss
		{5790, false}, // permanent Amon population
		{5899, true},  // Balrog
		{5972, false}, // permanent Sulrang
	} {
		if got := world.IsRuneQuestGenerator(tc.idx); got != tc.want {
			t.Errorf("IsRuneQuestGenerator(%d)=%v, want %v", tc.idx, got, tc.want)
		}
	}
}
