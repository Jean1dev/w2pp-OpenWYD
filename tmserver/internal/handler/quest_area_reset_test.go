package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// startOnePlayer boots a world with one player in USER_PLAY and returns the
// dispatcher, world and that player's session/entity (loop-owned: touch them
// only inside runInLoop).
func startOnePlayer(t *testing.T) (*Dispatcher, *world.World, *world.Session, *world.Entity) {
	t.Helper()
	addr, d, w, _ := startKefraServer(t, tradeDB())
	c := enterWorldAs(t, addr, "tester")
	t.Cleanup(func() { _ = c.Close() })
	var s *world.Session
	var e *world.Entity
	runInLoop(t, w, func() { s, e = w.SessionByName("Hero") })
	if s == nil {
		t.Fatal("player not in world")
	}
	return d, w, s, e
}

func TestResetQuestAreas(t *testing.T) {
	d, w, s, e := startOnePlayer(t)
	coveiro := questResetAreas[0]
	for _, tc := range []struct {
		name         string
		x, y         int16
		hp           int32
		tick         int
		wantRecalled bool
		wantFlag     uint8
	}{
		{"before the 10 min mark", 2400, 2100, 100, questAreaResetPeriod - 1, false, 1},
		{"inside Coveiro", 2400, 2100, 100, questAreaResetPeriod, true, 0},
		{"inclusive corner", coveiro.x1, coveiro.y1, 100, questAreaResetPeriod, true, 0},
		{"dead inside is revived", 2400, 2100, 0, 2 * questAreaResetPeriod, true, 0},
		{"outside keeps place, loses flag", 2200, 2200, 100, questAreaResetPeriod, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runInLoop(t, w, func() {
				w.SetEntityPos(s.Conn, tc.x, tc.y)
				e.HP, e.QuestFlag, e.LastCity = tc.hp, 1, 0
				scores := w.SentOfType(s, protocol.MsgUpdateScore)
				d.tickCount = tc.tick
				d.tickQuestAreaReset(w)

				if moved := e.X != tc.x || e.Y != tc.y; moved != tc.wantRecalled {
					t.Errorf("recalled = %v (at %d,%d), want %v", moved, e.X, e.Y, tc.wantRecalled)
				}
				if tc.wantRecalled && coveiro.contains(e.X, e.Y) {
					t.Errorf("still inside the arena at %d,%d", e.X, e.Y)
				}
				if e.QuestFlag != tc.wantFlag {
					t.Errorf("QuestFlag = %d, want %d", e.QuestFlag, tc.wantFlag)
				}
				if tc.hp <= 0 {
					if e.HP != 2 {
						t.Errorf("HP = %d, want 2", e.HP)
					}
					if w.SentOfType(s, protocol.MsgUpdateScore) == scores {
						t.Error("revive sent no score")
					}
				}
			})
		})
	}
}
