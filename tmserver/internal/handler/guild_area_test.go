package handler

import (
	"context"
	"io"
	"log/slog"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestAreaDeGuildTemAsQuatroCidades pins the table to the legacy's WarArea rows
// and keeps the city of Noatum out of it.
func TestAreaDeGuildTemAsQuatroCidades(t *testing.T) {
	dentro := [][2]int16{{197, 213}, {238, 230}, {220, 150}, {141, 213}, {182, 166}}
	for _, p := range dentro {
		if !inGuildWarArea(p[0], p[1]) {
			t.Errorf("(%d,%d) devia estar numa área de guild", p[0], p[1])
		}
	}
	fora := [][2]int16{
		{196, 213}, {239, 230}, {190, 200}, // just outside the fields
		{2086, 2093}, // Armia's city spawn
		{1050, 1720}, // the city of Noatum, inside the castle's legacy rectangle
	}
	for _, p := range fora {
		if inGuildWarArea(p[0], p[1]) {
			t.Errorf("(%d,%d) não devia ser área de guild", p[0], p[1])
		}
	}
}

// TestAreaDeGuildMandaEmboraQuemNaoEhStaff: a player standing in Armia's guild
// field is sent out with the reason; a moderator on the same tile stays.
func TestAreaDeGuildMandaEmboraQuemNaoEhStaff(t *testing.T) {
	db := &fakeDB{accounts: map[string]*fakeAccount{
		"mod":    {id: 20, pass: "secret", role: "moderator", chars: []world.CharSummary{{Slot: 0, Name: "Mod"}}},
		"player": {id: 22, pass: "secret", role: "player", chars: []world.CharSummary{{Slot: 0, Name: "Player"}}},
	}}
	db.loads = map[int64]world.CharacterState{
		20: {Slot: 0, Name: "Mod", Level: 10, X: 210, Y: 220, HP: 1000, MaxHP: 1000},
		22: {Slot: 0, Name: "Player", Level: 10, X: 212, Y: 220, HP: 1000, MaxHP: 1000},
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	clock := &atomic.Uint32{}
	clock.Store(serverTime)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, Now: func() time.Time { return time.Unix(0, 0) }})
	w := world.New(world.Config{GridDim: 4096, Now: clock.Load}, log, db, d.Handle)
	w.SetTickHandler(10*time.Millisecond, func(w *world.World) { clock.Add(100); d.Tick(w) })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}()

	mod := enterWorldAs(t, ln.Addr().String(), "mod")
	defer mod.Close()
	player := enterWorldAs(t, ln.Addr().String(), "player")
	defer player.Close()

	ouviuAviso := func(c net.Conn) bool {
		for i := 0; i < 40; i++ {
			ty, p, ok := readMaybe(t, c)
			if !ok {
				return false
			}
			if ty == protocol.MsgMessagePanel && decodePanel(p) == msgAreaDeGuild {
				return true
			}
		}
		return false
	}
	if !ouviuAviso(player) {
		t.Error("o jogador na área de guild não foi mandado embora com o aviso")
	}
	if ouviuAviso(mod) {
		t.Error("o moderador foi mandado embora da área de guild")
	}
}
