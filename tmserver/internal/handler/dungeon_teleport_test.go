package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// The real dungeon coordinates require a full-sized grid. All fixture changes
// and dispatcher calls run on the owner loop, including rejected requests.
func TestReqTeleportDungeon(t *testing.T) {
	db := newDB()
	db.accounts["observer"] = &fakeAccount{id: 12, pass: "secret", chars: []world.CharSummary{{Slot: 0, Name: "Observer"}}}
	db.loads = map[int64]world.CharacterState{
		7:  {Slot: 0, Name: "Hero", X: 20, Y: 20, HP: 1000, MaxHP: 1000},
		11: {Slot: 0, Name: "HeroB", X: 50, Y: 50, HP: 1000, MaxHP: 1000},
		12: {Slot: 0, Name: "Observer", X: 80, Y: 80, HP: 1000, MaxHP: 1000},
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.DiscardHandler)
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 4096}, log, db, d.Handle)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := w.Serve(ctx, ln); err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("serve: %v", err)
		}
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	})
	c := enterWorldAs(t, ln.Addr().String(), "tester")
	defer c.Close()
	var s *world.Session
	runInLoop(t, w, func() { s, _ = w.SessionByName("Hero") })
	if s == nil {
		t.Fatal("missing player session")
	}

	request := func(t *testing.T, dx, dy int16) {
		t.Helper()
		var oldX, oldY, x, y int16
		var coinBefore, coinAfter int32
		runInLoop(t, w, func() {
			e := w.Entity(s.Conn)
			oldX, oldY, coinBefore = e.X, e.Y, e.Coin
			d.Handle(w, s, protocol.Header{Type: protocol.MsgReqTeleport, ID: uint16(s.Conn)}, nil)
			x, y, coinAfter = e.X, e.Y, e.Coin
		})
		ty, payload := read(t, c)
		if ty != protocol.MsgAction {
			t.Fatalf("got %#x, want teleport action", ty)
		}
		var jump protocol.MsgActionBody
		if err := jump.Decode(payload); err != nil {
			t.Fatal(err)
		}
		if jump.Effect != 1 || jump.PosX != oldX || jump.PosY != oldY || jump.TargetX != x || jump.TargetY != y {
			t.Errorf("jump %+v disagrees with (%d,%d) -> (%d,%d)", jump, oldX, oldY, x, y)
		}
		if x < dx || x > dx+2 || y < dy || y > dy+2 || coinAfter != coinBefore {
			t.Errorf("landed (%d,%d), gold %d -> %d; want (%d..%d,%d..%d), unchanged gold", x, y, coinBefore, coinAfter, dx, dx+2, dy, dy+2)
		}
	}

	for _, r := range []struct{ x, y, dx, dy int16 }{
		{147, 3783, 1004, 4028}, {148, 3780, 1004, 4028},
		{1007, 4031, 148, 3780}, {411, 4075, 1004, 4064},
		{1007, 4067, 408, 4072},
	} {
		for _, coin := range []int32{0, 700} {
			t.Run(fmt.Sprintf("%d,%d/gold%d", r.x, r.y, coin), func(t *testing.T) {
				runInLoop(t, w, func() {
					w.SetEntityPos(s.Conn, r.x, r.y)
					w.Entity(s.Conn).Coin = coin
				})
				request(t, r.dx, r.dy)
			})
		}
	}
	t.Run("return-and-reenter", func(t *testing.T) {
		runInLoop(t, w, func() { w.SetEntityPos(s.Conn, 144, 3780) })
		request(t, 1004, 4028)
		request(t, 148, 3780)
		request(t, 1004, 4028)
	})
	for _, name := range []string{"dead", "not-playing", "outside-portal"} {
		t.Run(name, func(t *testing.T) {
			var changed bool
			runInLoop(t, w, func() {
				e := w.Entity(s.Conn)
				w.SetEntityPos(s.Conn, 144, 3780)
				mode, hp := s.Mode, e.HP
				switch name {
				case "dead":
					e.HP = 0
				case "not-playing":
					s.Mode = world.UserSelChar
				case "outside-portal":
					w.SetEntityPos(s.Conn, 152, 3780)
				}
				x, y, coin := e.X, e.Y, e.Coin
				jumps := w.SentOfType(s, protocol.MsgAction)
				d.Handle(w, s, protocol.Header{Type: protocol.MsgReqTeleport}, nil)
				changed = e.X != x || e.Y != y || e.Coin != coin || w.SentOfType(s, protocol.MsgAction) != jumps
				s.Mode, e.HP = mode, hp
			})
			if changed {
				t.Fatal("rejected request changed position/gold or sent a jump")
			}
		})
	}

	t.Run("visibility", func(t *testing.T) {
		oldClient := enterWorldAs(t, ln.Addr().String(), "tradeb")
		defer oldClient.Close()
		newClient := enterWorldAs(t, ln.Addr().String(), "observer")
		defer newClient.Close()
		var oldS, newS *world.Session
		var oldRemoves, newCreates, moverRemoves, moverCreates uint32
		runInLoop(t, w, func() {
			oldS, _ = w.SessionByName("HeroB")
			newS, _ = w.SessionByName("Observer")
			w.SetEntityPos(s.Conn, 148, 3780)
			w.SetEntityPos(oldS.Conn, 155, 3780)
			w.SetEntityPos(newS.Conn, 1012, 4028)
			w.MarkSeen(s, oldS.Conn)
			w.MarkSeen(oldS, s.Conn)
			oldRemoves = w.SentOfType(oldS, protocol.MsgRemoveMob)
			newCreates = w.SentOfType(newS, protocol.MsgCreateMob)
			moverRemoves = w.SentOfType(s, protocol.MsgRemoveMob)
			moverCreates = w.SentOfType(s, protocol.MsgCreateMob)
		})
		request(t, 1004, 4028)
		var reconciled bool
		runInLoop(t, w, func() {
			reconciled = w.SentOfType(oldS, protocol.MsgRemoveMob) == oldRemoves+1 &&
				w.SentOfType(newS, protocol.MsgCreateMob) == newCreates+1 &&
				w.SentOfType(s, protocol.MsgRemoveMob) == moverRemoves+1 &&
				w.SentOfType(s, protocol.MsgCreateMob) == moverCreates+1
		})
		if !reconciled {
			t.Error("teleport did not remove old and reveal new observers in both directions")
		}
	})
}
