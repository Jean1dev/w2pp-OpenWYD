package handler

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/rng"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// startServerWeather is a serving harness that hands the Dispatcher and World
// back, so tests can seed the world-event state and drive Tick directly.
func startServerWeather(t *testing.T, persist world.Persistence, baseMobs map[int][]byte) (string, func(), *Dispatcher, *world.World) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, BaseMobs: baseMobs})
	w := world.New(world.Config{GridDim: 16}, log, persist, d.Handle)
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
	}, d, w
}

// weatherTemplate is a minimal 816-byte BaseMob so character login takes the
// template branch (the other branch is the relational fallback).
func weatherTemplate() []byte {
	tmpl := make([]byte, content.BaseMobSize)
	copy(tmpl[0:16], "Template")
	binary.LittleEndian.PutUint16(tmpl[40:], 2112)
	binary.LittleEndian.PutUint16(tmpl[42:], 2112)
	return tmpl
}

// TestLoginSnapshotCarriesWeather covers BOTH CNFCharacterLogin builders: the
// legacy ships CurrentWeather inside the login blob rather than as its own
// packet (sm.Weather = CurrentWeather, ProcessDBMessage.cpp:834), and until
// issue #116 both call sites hardcoded 0.
func TestLoginSnapshotCarriesWeather(t *testing.T) {
	tests := []struct {
		name     string
		baseMobs map[int][]byte
	}{
		{"template path", map[int][]byte{1: weatherTemplate()}},
		{"relational fallback", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newDB()
			db.loadResult = world.CharacterState{Slot: 0, Name: "Hero", Class: 1, Level: 50, HP: 1200, MaxHP: 1200}
			addr, stop, d, w := startServerWeather(t, db, tt.baseMobs)
			defer stop()

			// Seed the weather from inside the loop, so it is set before login.
			setWeatherInLoop(t, w, d, 2)

			c := loginAndSelect(t, addr)
			defer c.Close()
			var body protocol.MsgCharacterLoginBody
			send(t, c, protocol.MsgCharacterLogin, body.Encode())
			ty, payload := read(t, c)
			if ty != protocol.MsgCNFCharacterLogin {
				t.Fatalf("got %#x, want CNFCharacterLogin", ty)
			}
			if got := binary.LittleEndian.Uint16(payload[1032:]); got != 2 {
				t.Fatalf("Weather @body1032 = %d, want 2", got)
			}
		})
	}
}

// setWeatherInLoop mutates the loop-owned weather state from the loop goroutine
// (world state is lock-free precisely because only the loop touches it).
func setWeatherInLoop(t *testing.T, w *world.World, d *Dispatcher, weather int32) {
	t.Helper()
	done := make(chan struct{})
	w.GoDetached(func() func(*world.World) {
		return func(*world.World) {
			d.events.weather = weather
			close(done)
		}
	})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("loop callback did not run")
	}
}

// TestTickWeatherBroadcastsChange drives the minute gate with a seeded stream
// and asserts the global MSG_UpdateWeather reaches an in-play client with
// HEADER.ID = ESCENE_FIELD (SendWeather, SendFunc.cpp:1669-1696).
func TestTickWeatherBroadcastsChange(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{Slot: 0, Name: "Hero", Class: 1, Level: 50, X: 5, Y: 5, HP: 1200, MaxHP: 1200}
	addr, stop, d, w := startServerWeather(t, db, nil)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	// Pin the roll: an eventRNG whose first draw lands in the weather-1 band,
	// and a tickCount that puts the next Tick exactly on the minute gate.
	forceWeatherRoll(t, w, d, 35)

	h, payload, ok := readMaybeHeader(t, c)
	if !ok {
		t.Fatal("no weather broadcast after the minute tick")
	}
	if h.Type != protocol.MsgUpdateWeather {
		t.Fatalf("got %#x, want MsgUpdateWeather", h.Type)
	}
	if h.ID != protocol.IDScene {
		t.Fatalf("HEADER.ID = %d, want IDScene (%d)", h.ID, protocol.IDScene)
	}
	if got := int32(binary.LittleEndian.Uint32(payload)); got != 1 {
		t.Fatalf("CurrentWeather = %d, want 1", got)
	}
}

// TestTickWeatherHonoursForceOverride pins the GM override semantics: the roll
// is still consumed every minute (the legacy rolls before it reads
// ForceWeather), but the forced value is what reaches the client, and only once.
func TestTickWeatherHonoursForceOverride(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{Slot: 0, Name: "Hero", Class: 1, Level: 50, X: 5, Y: 5, HP: 1200, MaxHP: 1200}
	addr, stop, d, w := startServerWeather(t, db, nil)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	// Seed a stream whose first draw would otherwise select weather 1, so the
	// assertion below proves the override beat the roll rather than agreeing
	// with it by luck.
	runInLoop(t, w, func() {
		d.events.forceWeather = 2
		d.eventRNG = seededForRoll(35) // band → weather 1
		d.tickCount = weatherTickPeriod - 1
	})
	runInLoop(t, w, func() { d.Tick(w) })

	payload, ok := readUntilType(t, c, protocol.MsgUpdateWeather)
	if !ok {
		t.Fatal("forced weather did not broadcast")
	}
	if got := int32(binary.LittleEndian.Uint32(payload)); got != 2 {
		t.Fatalf("CurrentWeather = %d, want the forced 2 (the roll wanted 1)", got)
	}

	// A second minute must still consume a draw — the legacy rolls before it
	// looks at ForceWeather, so the stream advances at the same rate either way
	// — but with the value already matching the override nothing goes on the wire.
	counter := &countingRand{inner: seededForRoll(35)}
	runInLoop(t, w, func() {
		d.eventRNG = counter
		d.tickCount = weatherTickPeriod - 1
	})
	runInLoop(t, w, func() { d.Tick(w) })

	var draws int
	runInLoop(t, w, func() { draws = counter.draws })
	if draws != 1 {
		t.Errorf("draws while an override was active = %d, want 1 (the stream must still advance)", draws)
	}
	if h, _, ok := readMaybeHeader(t, c); ok && h.Type == protocol.MsgUpdateWeather {
		t.Error("weather rebroadcast while the value was unchanged")
	}
}

// countingRand counts draws so tests can assert the stream advanced (or did not).
type countingRand struct {
	inner *rng.MSVC
	draws int
}

func (c *countingRand) Intn(n int) int {
	c.draws++
	return c.inner.Intn(n)
}

// forceWeatherRoll seeds eventRNG so the next RollWeather draw returns `roll`,
// parks tickCount one short of the minute gate, and runs a Tick.
func forceWeatherRoll(t *testing.T, w *world.World, d *Dispatcher, roll int) {
	t.Helper()
	runInLoop(t, w, func() {
		d.eventRNG = seededForRoll(roll)
		d.tickCount = weatherTickPeriod - 1
	})
	runInLoop(t, w, func() { d.Tick(w) })
}

// seededForRoll finds an MSVC seed whose first Intn(1200) equals roll. The LCG
// is cheap and surjective over the modulus, so a short scan always succeeds.
func seededForRoll(roll int) *rng.MSVC {
	for seed := uint32(1); seed < 100000; seed++ {
		r := rng.NewSeeded(seed)
		if r.Intn(1200) == roll {
			return rng.NewSeeded(seed)
		}
	}
	panic("no seed produces the requested roll")
}

// runInLoop executes fn on the world's loop goroutine and waits for it.
func runInLoop(t *testing.T, w *world.World, fn func()) {
	t.Helper()
	done := make(chan struct{})
	w.GoDetached(func() func(*world.World) {
		return func(*world.World) {
			fn()
			close(done)
		}
	})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("loop callback did not run")
	}
}

// TestGMWeatherRequiresModerator: the bus already gates on AccessLevel, so a
// plain player's /gm weather must be silently dropped and change nothing.
func TestGMWeatherRequiresModerator(t *testing.T) {
	addr, stop, d, w := startServerWeather(t, gmDB(), nil)
	defer stop()
	c := enterWorldAs(t, addr, "player")
	defer c.Close()

	gmFrame(t, c, "weather 2")
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("player got %#x; a non-GM must be silently denied", ty)
	}
	var weather int32
	runInLoop(t, w, func() { weather = d.events.weather })
	if weather != 0 {
		t.Errorf("weather = %d after a denied command, want 0", weather)
	}
}

// TestGMWeatherSetsAndRestoresAuto covers the three improvements over the legacy
// /weather (imple.cpp:1122-1129): it validates the value, it broadcasts the
// change, and "auto" restores ForceWeather = -1 (the legacy had no way back).
func TestGMWeatherSetsAndRestoresAuto(t *testing.T) {
	addr, stop, d, w := startServerWeather(t, gmDB(), nil)
	defer stop()
	c := enterWorldAs(t, addr, "mod")
	defer c.Close()

	// Every /gm weather replies with a chat line; draining up to it is what makes
	// the subsequent state read race-free (the command travels over the socket
	// and is applied on the loop goroutine).
	gmWeatherCmd(t, c, "weather 2")
	var weather, force int32
	runInLoop(t, w, func() { weather, force = d.events.weather, d.events.forceWeather })
	if weather != 2 || force != 2 {
		t.Fatalf("weather/force = %d/%d, want 2/2", weather, force)
	}

	gmWeatherCmd(t, c, "weather auto")
	runInLoop(t, w, func() { force = d.events.forceWeather })
	if force != weatherAuto {
		t.Fatalf("forceWeather = %d after /gm weather auto, want %d", force, weatherAuto)
	}

	// Invalid values are rejected without touching state.
	gmWeatherCmd(t, c, "weather 9")
	runInLoop(t, w, func() { weather, force = d.events.weather, d.events.forceWeather })
	if weather != 2 || force != weatherAuto {
		t.Fatalf("invalid /gm weather 9 changed state: weather=%d force=%d", weather, force)
	}
}

// gmWeatherCmd sends a /gm weather line and waits for its chat acknowledgement,
// so the caller can then read the loop-owned state without racing the handler.
func gmWeatherCmd(t *testing.T, c net.Conn, line string) {
	t.Helper()
	gmFrame(t, c, line)
	if _, ok := readUntilType(t, c, protocol.MsgMessageChat); !ok {
		t.Fatalf("no chat acknowledgement for /gm %s", line)
	}
}

// readUntilType drains frames until `want` shows up or the connection goes quiet.
func readUntilType(t *testing.T, c net.Conn, want protocol.Type) ([]byte, bool) {
	t.Helper()
	for {
		h, payload, ok := readMaybeHeader(t, c)
		if !ok {
			return nil, false
		}
		if h.Type == want {
			return payload, true
		}
	}
}
