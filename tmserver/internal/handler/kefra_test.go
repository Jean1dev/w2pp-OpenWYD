package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"os"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/rng"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func startKefraServer(t *testing.T, db *fakeDB) (string, *Dispatcher, *world.World, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.DiscardHandler)
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 4096}, log, db, d.Handle)
	template, err := os.ReadFile("../../../Release/TMsrv/run/npc/Sobrevivente")
	if err != nil {
		t.Fatal(err)
	}
	npcID := w.SpawnMob(template, 5, 5)
	if npcID < world.MaxUser {
		t.Fatal("spawn survivor failed")
	}
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
	return ln.Addr().String(), d, w, npcID
}

func TestSurvivorExchange(t *testing.T) {
	db := perzenDB(4127)
	db.loadResult.KefraTicket = 37
	addr, d, w, npcID := startKefraServer(t, db)
	c := enterWorld(t, addr)
	defer c.Close()
	var s *world.Session
	runInLoop(t, w, func() {
		s, _ = w.SessionByName("Hero")
		if s == nil || w.Entity(s.Conn).KefraTicket != 37 {
			t.Error("persisted Kefra balance was not loaded")
		}
	})
	if s == nil {
		t.Fatal("missing session")
	}

	for _, tc := range []struct {
		name                string
		balance             int32
		slot                int
		confirm             int32
		bag, full, overflow bool
	}{
		{name: "zero", slot: 0},
		{name: "accumulated and confirmed", balance: 37, slot: 29, confirm: 1},
		{name: "full inventory", balance: 100, slot: 12, full: true},
		{name: "locked bag slot", slot: 30},
		{name: "unlocked bag slot", slot: 44, bag: true},
		{name: "last representable balance", balance: math.MaxInt32 - 100, slot: 0},
		{name: "overflow", balance: math.MaxInt32 - 99, slot: 0, overflow: true},
		{name: "missing item", slot: -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			accepted := tc.slot >= 0 && (tc.slot < 30 || tc.bag) && !tc.overflow
			runInLoop(t, w, func() {
				e := w.Entity(s.Conn)
				e.KefraTicket = tc.balance
				e.Carry = [world.MaxCarry]world.Item{}
				if tc.full {
					for i := 0; i < 30; i++ {
						e.Carry[i] = world.Item{Index: 1100}
					}
				}
				if tc.bag {
					e.Carry[60] = world.Item{Index: itemWandererBag}
				}
				if tc.slot >= 0 {
					// Even a stack-shaped legacy item clears the whole selected slot.
					e.Carry[tc.slot] = world.Item{Index: 4127, Effects: [3]world.Effect{{Effect: 61, Value: 3}}}
				}
				before := e.Carry
				packets := kefraPacketCount(w, s)
				d.quest(w, s, protocol.Header{}, protocol.EncodeStandardParm2(int32(npcID), tc.confirm))
				wantBalance := tc.balance
				if accepted {
					wantBalance += 100
					before[tc.slot] = world.Item{}
				}
				if e.KefraTicket != wantBalance || e.Carry != before {
					t.Errorf("exchange balance=%d, want %d; inventory matches=%v", e.KefraTicket, wantBalance, e.Carry == before)
				}
				if !accepted && kefraPacketCount(w, s) != packets {
					t.Error("rejected exchange sent packets")
				}
			})
			if accepted {
				ty, payload := read(t, c)
				want := protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, tc.slot, protocol.SelItem{})
				if ty != protocol.MsgSendItem || !bytes.Equal(payload, want) {
					t.Fatalf("missing cleared slot: type=%x payload=%x", ty, payload)
				}
				assertKefraBalance(t, c, tc.balance+100)
			}
		})
	}

	t.Run("one scroll per interaction and snapshot", func(t *testing.T) {
		runInLoop(t, w, func() {
			e := w.Entity(s.Conn)
			e.KefraTicket = 0
			e.Carry = [world.MaxCarry]world.Item{}
			e.Carry[2], e.Carry[5] = world.Item{Index: 4127}, world.Item{Index: 4127}
			for i := 0; i < 3; i++ {
				d.quest(w, s, protocol.Header{}, protocol.EncodeStandardParm2(int32(npcID), 0))
				if i == 0 && e.Carry[5].Index != 4127 {
					t.Error("consumed second scroll on first interaction")
				}
			}
			save := w.CharacterSaveFor(s, e)
			if save.KefraTicket != 200 || len(save.Carry) != 0 {
				t.Errorf("incorrect snapshot: balance=%d carry=%v", save.KefraTicket, save.Carry)
			}
		})
		for _, balance := range []int32{100, 200} {
			ty, _ := read(t, c)
			if ty != protocol.MsgSendItem {
				t.Fatalf("got %x, want SendItem", ty)
			}
			assertKefraBalance(t, c, balance)
		}
	})

	t.Run("invalid interactions", func(t *testing.T) {
		runInLoop(t, w, func() {
			e, npc := w.Entity(s.Conn), w.Entity(npcID)
			e.Carry[0] = world.Item{Index: 4127}
			balance, packets := e.KefraTicket, kefraPacketCount(w, s)
			body := protocol.EncodeStandardParm2(int32(npcID), 0)
			e.HP = 0
			d.quest(w, s, protocol.Header{}, body)
			e.HP = 100
			s.Mode = world.UserSelChar
			d.quest(w, s, protocol.Header{}, body)
			s.Mode = world.UserPlay
			d.quest(w, s, protocol.Header{}, nil)
			d.quest(w, s, protocol.Header{}, protocol.EncodeStandardParm2(int32(s.Conn), 0))
			npc.Grade = 21
			d.quest(w, s, protocol.Header{}, body)
			npc.Grade, npc.Merchant = 22, 99
			npc.Template[17] = 99
			d.quest(w, s, protocol.Header{}, body)
			npc.Merchant, npc.Template[17] = 68, 100
			if e.KefraTicket != balance || e.Carry[0].Index != 4127 || kefraPacketCount(w, s) != packets {
				t.Error("invalid interaction changed state or sent packets")
			}
		})
	})
}

func assertKefraBalance(t *testing.T, c net.Conn, balance int32) {
	t.Helper()
	ty, payload := read(t, c)
	want := protocol.EncodeMessagePanelBody(fmt.Sprintf("Você pode usar isto %d vezes.", balance))
	if ty != protocol.MsgMessagePanel || !bytes.Equal(payload, want) {
		t.Fatalf("balance message type=%x payload=%q, want %d", ty, payload, balance)
	}
}

func TestKefraLogoutReload(t *testing.T) {
	var saved world.CharacterSave
	t.Run("convert and logout", func(t *testing.T) {
		db := perzenDB(4127)
		db.loadResult.KefraTicket = 37
		addr, _, _, npcID := startKefraServer(t, db)
		c := enterWorld(t, addr)
		defer c.Close()
		questFrame(t, c, npcID)
		expect(t, c, protocol.MsgSendItem)
		assertKefraBalance(t, c, 137)
		send(t, c, protocol.MsgCharacterLogout, nil)
		expect(t, c, protocol.MsgCNFCharacterLogout)
		var count int
		saved, count = db.lastSavedChar()
		if count != 1 || saved.KefraTicket != 137 || len(saved.Carry) != 0 {
			t.Fatalf("logout saves=%d, balance=%d carry=%v", count, saved.KefraTicket, saved.Carry)
		}
	})
	t.Run("reload spend and logout", func(t *testing.T) {
		db := perzenDB(0)
		// Rehydrate the last committed snapshot in a fresh session. SQL and RPC
		// round trips are covered separately by the persistence-layer tests.
		db.loadResult.KefraTicket = saved.KefraTicket
		addr, _, w, _ := startKefraServer(t, db)
		c := enterWorld(t, addr)
		defer c.Close()
		runInLoop(t, w, func() {
			s, _ := w.SessionByName("Hero")
			if s == nil {
				t.Error("missing player after reload")
				return
			}
			if e := w.Entity(s.Conn); e.KefraTicket != 137 || e.Carry[0].Index != 0 {
				t.Error("reload lost balance or restored consumed scroll")
			}
			w.SetEntityPos(s.Conn, 2364, 3892)
		})
		send(t, c, protocol.MsgReqTeleport, nil)
		assertKefraBalance(t, c, 136)
		expect(t, c, protocol.MsgAction)
		send(t, c, protocol.MsgCharacterLogout, nil)
		expect(t, c, protocol.MsgCNFCharacterLogout)
		last, count := db.lastSavedChar()
		if count != 1 || last.KefraTicket != 136 {
			t.Fatalf("spent entry not saved: count=%d balance=%d", count, last.KefraTicket)
		}
	})
}

func TestKefraHallEntry(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, perzenDB(0))
	c := enterWorld(t, addr)
	defer c.Close()
	var s *world.Session
	runInLoop(t, w, func() { s, _ = w.SessionByName("Hero") })
	if s == nil {
		t.Fatal("missing session")
	}
	t.Run("invalid player or origin", func(t *testing.T) {
		runInLoop(t, w, func() {
			e := w.Entity(s.Conn)
			e.KefraTicket = 100
			w.SetEntityPos(s.Conn, 2364, 3892)
			before := *w.Rand()
			packets := kefraPacketCount(w, s)
			for _, hp := range []int32{0, -1} {
				e.HP = hp
				d.reqTeleport(w, s, protocol.Header{}, nil)
			}
			e.HP = 100
			s.Mode = world.UserSelChar
			d.reqTeleport(w, s, protocol.Header{}, nil)
			s.Mode = world.UserPlay
			if e.X != 2364 || e.Y != 3892 {
				t.Error("invalid player teleported")
			}
			w.SetEntityPos(s.Conn, 2360, 3892)
			d.reqTeleport(w, s, protocol.Header{}, nil)
			if e.X != 2360 || e.Y != 3892 || e.KefraTicket != 100 || *w.Rand() != before || kefraPacketCount(w, s) != packets {
				t.Error("rejected entry changed position, balance, RNG or packets")
			}
		})
	})
	for _, balance := range []int32{-1, 0, 1, 100} {
		for _, offset := range []int16{0, 3} {
			t.Run(fmt.Sprintf("balance%d/offset%d", balance, offset), func(t *testing.T) {
				var x, y int16
				runInLoop(t, w, func() {
					e := w.Entity(s.Conn)
					w.SetEntityPos(s.Conn, 2364+offset, 3892+offset)
					e.KefraTicket, e.Coin = balance, 777
					*w.Rand() = *rng.NewSeeded(17)
					expected := rng.NewSeeded(17)
					packets := kefraPacketCount(w, s)
					d.reqTeleport(w, s, protocol.Header{}, nil)
					x, y = e.X, e.Y
					if balance > 0 {
						wantX, wantY := 2364+int16(expected.Intn(3)), 3906+int16(expected.Intn(3))
						if x != wantX || y != wantY || e.KefraTicket != balance-1 {
							t.Errorf("entry=(%d,%d) balance=%d; want (%d,%d) balance=%d", x, y, e.KefraTicket, wantX, wantY, balance-1)
						}
						// A duplicate request at the destination must not spend again.
						packets = kefraPacketCount(w, s)
						d.reqTeleport(w, s, protocol.Header{}, nil)
						if e.KefraTicket != balance-1 || kefraPacketCount(w, s) != packets {
							t.Error("duplicate request spent another entry")
						}
						if w.CharacterSaveFor(s, e).KefraTicket != balance-1 {
							t.Error("snapshot lost spent entry")
						}
					} else if x != 2364+offset || y != 3892+offset || e.KefraTicket != balance || kefraPacketCount(w, s) != packets {
						t.Error("entry without balance changed state")
					}
					if *w.Rand() != *expected {
						t.Error("wrong MSVC RNG call count")
					}
					if e.Coin != 777 {
						t.Error("Hall entry changed gold")
					}
				})
				if balance > 0 {
					assertKefraBalance(t, c, balance-1)
					ty, payload := read(t, c)
					var action protocol.MsgActionBody
					if err := action.Decode(payload); err != nil {
						t.Fatal(err)
					}
					if ty != protocol.MsgAction || action.Effect != 1 || action.TargetX != x || action.TargetY != y {
						t.Fatalf("incorrect teleport packet: type=%x action=%+v", ty, action)
					}
				}
			})
		}
	}
}

func TestSurvivorNPCShapes(t *testing.T) {
	for _, tc := range []struct {
		name string
		npc  *world.Entity
		want bool
	}{
		{"nil", nil, false},
		{"current merchant", &world.Entity{Merchant: 100, Grade: 22}, true},
		{"wrong grade", &world.Entity{Merchant: 100, Grade: 21}, false},
		{"merchant 68 alone", &world.Entity{Merchant: 68, Grade: 22}, false},
		{"short template", &world.Entity{Grade: 22, Template: make([]byte, 17)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isSurvivorNPC(tc.npc); got != tc.want {
				t.Errorf("isSurvivorNPC=%v, want %v", got, tc.want)
			}
		})
	}
}

// Count the response types that conversion and entry can emit.
func kefraPacketCount(w *world.World, s *world.Session) uint32 {
	return w.SentOfType(s, protocol.MsgSendItem) + w.SentOfType(s, protocol.MsgMessagePanel) + w.SentOfType(s, protocol.MsgAction)
}
