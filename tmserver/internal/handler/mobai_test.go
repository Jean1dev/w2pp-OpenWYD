package handler

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestChebyshevAndStep(t *testing.T) {
	if d := chebyshev(5, 5, 8, 6); d != 3 { // king-move distance = max(|dx|,|dy|)
		t.Errorf("chebyshev = %d, want 3", d)
	}
	if d := chebyshev(5, 5, 5, 5); d != 0 {
		t.Errorf("chebyshev same cell = %d, want 0", d)
	}
	if step(7) != 1 || step(-3) != -1 || step(0) != 0 {
		t.Errorf("step direction wrong: %d %d %d", step(7), step(-3), step(0))
	}
}

// aggressiveMob builds an 816-byte STRUCT_MOB template for a monster that always
// acts (high Int → no hesitation) and hits hard (high Damage). Merchant=0 so the
// AI treats it as a combatant, not a shopkeeper.
func aggressiveMob() []byte {
	b := make([]byte, 816)
	copy(b[0:16], "Biter")
	const cs = 92                                 // CurrentScore
	binary.LittleEndian.PutUint32(b[cs+0:], 50)   // Level
	binary.LittleEndian.PutUint32(b[cs+8:], 500)  // Damage
	binary.LittleEndian.PutUint32(b[cs+16:], 800) // MaxHp
	binary.LittleEndian.PutUint32(b[cs+24:], 800) // Hp
	binary.LittleEndian.PutUint16(b[cs+34:], 99)  // Int (never hesitates: 99 < rand%100 is never true)
	return b
}

// startServerMobAI is the combat harness plus a mob-AI tick: it spawns one
// aggressive mob at (mobX,mobY) before the loop starts and pulses the AI every
// 10ms with a frozen clock. gridDim sizes the world (use a city-sized grid to test
// town safe zones).
func startServerMobAI(t *testing.T, persist world.Persistence, gridDim int, mobX, mobY int16) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	clock := &atomic.Uint32{}
	clock.Store(serverTime)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: gridDim, Now: clock.Load}, log, persist, d.Handle)
	w.SpawnMob(aggressiveMob(), mobX, mobY) // init-time spawn (loop-safe)
	w.SetTickHandler(10*time.Millisecond, d.Tick)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	return ln.Addr().String(), func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}
}

// actionFrameAt pins the player at (x,y): _MSG_Action sets the entity position to
// the body's PosX/PosY (the route target is cosmetic).
func actionFrameAt(t *testing.T, c net.Conn, tick uint32, x, y int16) {
	t.Helper()
	body := protocol.MsgActionBody{PosX: x, PosY: y, Speed: 30, TargetX: x, TargetY: y}
	wire, err := protocol.Encode(protocol.Header{Type: protocol.MsgAction, ClientTick: tick}, body.Encode(), 9)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write(wire); err != nil {
		t.Fatal(err)
	}
}

// TestMobAttacksAdjacentPlayer is the end-to-end AI case: a mob spawned next to
// where the player stands acquires it (FindPlayerNear), and because it is adjacent
// the AI tick makes it melee — the player receives a server-authoritative
// MSG_Attack naming itself as the damaged target.
func TestMobAttacksAdjacentPlayer(t *testing.T) {
	addr, stop := startServerMobAI(t, combatDB(), 16, 6, 5) // mob one tile east of (5,5)
	defer stop()

	c := enterWorld(t, addr) // player conn 1
	defer c.Close()
	actionFrame(t, c, serverTime, 5) // pin the player at (5,5), adjacent to the mob

	// Within a few ticks the mob should melee the player.
	for i := 0; i < 20; i++ {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			continue
		}
		if ty != protocol.MsgAttack {
			continue // skip the mob's move/other noise
		}
		var body protocol.MsgAttackBody
		if err := body.Decode(payload); err != nil {
			t.Fatal(err)
		}
		if len(body.Dam) == 0 || body.Dam[0].TargetID != 1 {
			t.Fatalf("mob attack Dam = %+v, want target player conn 1", body.Dam)
		}
		if body.Dam[0].Damage <= 0 {
			t.Fatalf("mob attack damage = %d, want > 0", body.Dam[0].Damage)
		}
		return // success
	}
	t.Fatal("player never received a mob MSG_Attack")
}

func TestRegenStep(t *testing.T) {
	tests := []struct {
		name     string
		cur, max int32
		want     int32
	}{
		{"recovers ~5%+floor", 100, 1000, 152}, // 100 + (1000/20 + 2)
		{"caps at max", 990, 1000, 1000},       // 990 + 52 > max → clamped
		{"already full", 1000, 1000, 1000},
		{"no max (mp unset)", 0, 0, 0},
		{"revived 2hp climbs", 2, 1000, 54},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := regenStep(tt.cur, tt.max); got != tt.want {
				t.Errorf("regenStep(%d,%d) = %d, want %d", tt.cur, tt.max, got, tt.want)
			}
		})
	}
}

// TestMobIgnoresPlayerInCity is the respawn-safety fix: a mob spawned right next
// to a player who is standing inside a city must NOT attack — the town is a safe
// zone where a revived player recovers. (Compare TestMobAttacksAdjacentPlayer,
// where the same setup outside a city does produce an attack.)
func TestMobIgnoresPlayerInCity(t *testing.T) {
	// Armia's rectangle is (2052,2052)-(2171,2163); use a grid that fits it.
	addr, stop := startServerMobAI(t, combatDB(), 2200, 2090, 2095)
	defer stop()

	c := enterWorld(t, addr)
	defer c.Close()
	actionFrameAt(t, c, serverTime, 2089, 2095) // adjacent to the mob, inside Armia

	// With a 10ms tick a buggy mob would attack within ~50ms; a few reads (≈0.9s of
	// timeout) is a confident negative.
	for i := 0; i < 3; i++ {
		ty, _, ok := readMaybe(t, c)
		if ok && ty == protocol.MsgAttack {
			t.Fatal("mob attacked a player standing in a safe city")
		}
	}
}
