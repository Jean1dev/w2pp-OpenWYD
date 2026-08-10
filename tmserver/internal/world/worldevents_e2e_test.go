//go:build e2e

// End-to-end validation of the WORLD EVENTS (issue #116) against a LIVE stack,
// driven by a headless CPSock client that speaks exactly what WYD.exe speaks.
//
// Why this exists: the world events are timer-driven and their only observable
// contract is the S→C frame the client receives — a server that computes the
// right state but ships the wrong packet is indistinguishable from a broken one
// at the unit-test level (SESSION-PRIMER §5.1, "servidor-correto ≠ cliente-correto").
// These tests assert the bytes on the wire, from a real socket, against a real
// tmServer → dbServer → PostgreSQL chain.
//
// The client here is deliberately dumb: it performs the INITCODE handshake, a
// real MSG_AccountLogin, MSG_CreateCharacter and MSG_CharacterLogin, then reads
// frames. It renders nothing, so it validates the SERVER half of "what WYD.exe
// would see" — the rendering half still needs a human at the real client, and
// the events that have no S→C frame at all are listed in
// docs/migration/worldevents-e2e-report.md rather than silently skipped.
//
// Run against a stack brought up by scripts/run-local.sh:
//
//	W2PP_E2E_ADDR=127.0.0.1:8281 W2PP_E2E_ACCOUNT=test W2PP_E2E_PASSWORD=test123 \
//	  W2PP_E2E_VERSION=12000 go test -tags=e2e -run TestE2EWorldEvent ./tmserver/internal/world/ -v
//
// The GM-driven cases need the account promoted to a moderator/admin role:
//
//	docker compose exec db psql -U postgres -c "UPDATE account SET role='admin' WHERE name='test'"
package world

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// clientKeyWord seeds the C→S obfuscation. Any value works (the server reads it
// out of byte 2); 7 is what the smoke test uses.
const clientKeyWord = 7

// cnfCharLoginWeatherOffset is where CurrentWeather rides the login snapshot:
// MSG_CNFCharacterLogin body @1032, an unsigned short (protocol/mob.go
// EncodeCNFCharacterLoginRaw, legacy sm.Weather at ProcessDBMessage.cpp:834).
// Weather is NOT sent as its own packet at login — if this offset is wrong the
// client shows the wrong sky until the next broadcast.
const cnfCharLoginWeatherOffset = 1032

// liveClient is a headless stand-in for WYD.exe: one CPSock connection plus a
// backlog of frames already pulled off the socket.
//
// The backlog matters. Entering the world triggers a burst of unrelated frames
// (CreateMob for every visible entity, affects, item slots), and a world-event
// frame arrives interleaved with them. Every wait therefore scans forward and
// keeps what it did not want, so a later wait can still find it.
type liveClient struct {
	t       *testing.T
	conn    net.Conn
	account string
	backlog []protocol.Frame
	connID  uint16
	// snapshot is the MSG_CNFCharacterLogin this client entered the world with.
	// Weather rides it, so tests read it instead of re-deriving state.
	snapshot protocol.Frame
}

// dialLive opens a connection and completes the INITCODE handshake.
func dialLive(t *testing.T, addr, account string) *liveClient {
	t.Helper()
	return &liveClient{t: t, conn: liveDial(t, addr), account: account}
}

func (c *liveClient) close() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
}

// send encodes and writes one C→S frame.
func (c *liveClient) send(typ protocol.Type, id uint16, body []byte) {
	c.t.Helper()
	wire, err := protocol.Encode(protocol.Header{Type: typ, ID: id}, body, clientKeyWord)
	if err != nil {
		c.t.Fatalf("encode %#x: %v", typ, err)
	}
	if _, err := c.conn.Write(wire); err != nil {
		c.t.Fatalf("write %#x: %v", typ, err)
	}
}

// readFrame pulls exactly one frame off the socket, honouring deadline.
func (c *liveClient) readFrame(deadline time.Time) (protocol.Frame, error) {
	if err := c.conn.SetReadDeadline(deadline); err != nil {
		return protocol.Frame{}, err
	}
	var sz [2]byte
	if _, err := io.ReadFull(c.conn, sz[:]); err != nil {
		return protocol.Frame{}, err
	}
	size := int(binary.LittleEndian.Uint16(sz[:]))
	if size < protocol.HeaderSize {
		return protocol.Frame{}, fmt.Errorf("frame size %d below header size", size)
	}
	buf := make([]byte, size)
	copy(buf, sz[:])
	if _, err := io.ReadFull(c.conn, buf[2:]); err != nil {
		return protocol.Frame{}, err
	}
	h, payload, _, err := protocol.Decode(buf)
	if err != nil {
		return protocol.Frame{}, err
	}
	return protocol.Frame{Header: h, Payload: payload}, nil
}

// waitFor scans the backlog and then the socket for the first frame of type typ.
// Frames of other types are retained in the backlog, never dropped.
func (c *liveClient) waitFor(typ protocol.Type, within time.Duration) (protocol.Frame, bool) {
	c.t.Helper()
	for i, f := range c.backlog {
		if f.Header.Type == typ {
			c.backlog = append(c.backlog[:i], c.backlog[i+1:]...)
			return f, true
		}
	}
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		f, err := c.readFrame(deadline)
		if err != nil {
			return protocol.Frame{}, false
		}
		if f.Header.Type == typ {
			return f, true
		}
		c.backlog = append(c.backlog, f)
	}
	return protocol.Frame{}, false
}

// waitForAny is waitFor over a set of types, for the branches where the server
// legitimately answers one of several frames. Waiting for them one at a time
// would burn a full timeout on the branch that did not happen.
func (c *liveClient) waitForAny(types []protocol.Type, within time.Duration) (protocol.Frame, bool) {
	c.t.Helper()
	match := func(t protocol.Type) bool {
		for _, want := range types {
			if t == want {
				return true
			}
		}
		return false
	}
	for i, f := range c.backlog {
		if match(f.Header.Type) {
			c.backlog = append(c.backlog[:i], c.backlog[i+1:]...)
			return f, true
		}
	}
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		f, err := c.readFrame(deadline)
		if err != nil {
			return protocol.Frame{}, false
		}
		if match(f.Header.Type) {
			return f, true
		}
		c.backlog = append(c.backlog, f)
	}
	return protocol.Frame{}, false
}

// waitForChat scans for a MSG_MessageChat whose text contains substr. World
// event announcements (tower, castle, event drops) all ride this frame.
func (c *liveClient) waitForChat(substr string, within time.Duration) (string, bool) {
	c.t.Helper()
	deadline := time.Now().Add(within)
	// Backlog first, then the socket, with the same matching rule.
	for i := 0; i < len(c.backlog); i++ {
		if text, ok := chatText(c.backlog[i]); ok && strings.Contains(text, substr) {
			c.backlog = append(c.backlog[:i], c.backlog[i+1:]...)
			return text, true
		}
	}
	for time.Now().Before(deadline) {
		f, err := c.readFrame(deadline)
		if err != nil {
			return "", false
		}
		if text, ok := chatText(f); ok && strings.Contains(text, substr) {
			return text, true
		}
		c.backlog = append(c.backlog, f)
	}
	return "", false
}

// chatText extracts the NUL-terminated text of a MSG_MessageChat frame.
func chatText(f protocol.Frame) (string, bool) {
	if f.Header.Type != protocol.MsgMessageChat {
		return "", false
	}
	return cstring(f.Payload), true
}

// cstring reads a NUL-terminated string from b.
func cstring(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}

// quiet drains whatever the server pushes for d, so a later waitFor cannot match
// a frame the PREVIOUS step caused. Retains nothing: callers use it as a barrier.
func (c *liveClient) quiet(d time.Duration) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if _, err := c.readFrame(deadline); err != nil {
			return
		}
	}
}

// login performs MSG_AccountLogin and waits for the character-selection reply.
func (c *liveClient) login(password string) {
	c.t.Helper()
	if _, err := c.conn.Write(loginFrame(c.account, password)); err != nil {
		c.t.Fatalf("write login: %v", err)
	}
	f, ok := c.waitFor(protocol.MsgCNFAccountLogin, 10*time.Second)
	if !ok {
		c.t.Fatalf("account %q: no CNFAccountLogin (backlog: %s)", c.account, c.backlogTypes())
	}
	c.connID = f.Header.ID
}

// enterWorld creates the character if it does not exist yet and selects slot 0,
// leaving the session in USER_PLAY. Returns the CNFCharacterLogin snapshot.
//
// Creation is attempted unconditionally: on a re-run the name is taken and the
// server answers MSG_NewCharacterFail, which is the same "slot 0 is ready"
// state. That keeps the test idempotent without a delete/recreate cycle.
func (c *liveClient) enterWorld(charName string, class int32) protocol.Frame {
	c.t.Helper()
	create := protocol.MsgCreateCharacterBody{Slot: 0, MobClass: class}
	copy(create.MobName[:], charName)
	c.send(protocol.MsgCreateCharacter, 0, create.Encode())
	// Either outcome is fine; both mean slot 0 is settled.
	if _, ok := c.waitForAny([]protocol.Type{protocol.MsgCNFNewCharacter, protocol.MsgNewCharacterFail},
		8*time.Second); !ok {
		c.t.Fatalf("create char %q: neither CNFNewCharacter nor NewCharacterFail", charName)
	}

	login := protocol.MsgCharacterLoginBody{Slot: 0}
	c.send(protocol.MsgCharacterLogin, 0, login.Encode())
	f, ok := c.waitFor(protocol.MsgCNFCharacterLogin, 15*time.Second)
	if !ok {
		c.t.Fatalf("char login: no CNFCharacterLogin (backlog: %s)", c.backlogTypes())
	}
	if len(f.Payload) < cnfCharLoginWeatherOffset+2 {
		c.t.Fatalf("CNFCharacterLogin body too short: %d bytes", len(f.Payload))
	}
	c.snapshot = f
	return f
}

// gm sends a "/gm <line>" command. The client encodes slash commands as a
// whisper whose TARGET NAME is the keyword ("gm") and whose text is the rest of
// the line — the legacy _MSG_MessageWhisper quirk (handler/chat.go runCommand).
func (c *liveClient) gm(line string) {
	c.t.Helper()
	body := protocol.MsgWhisperBody{String: append([]byte(line), 0)}
	copy(body.MobName[:], "gm")
	c.send(protocol.MsgMessageWhisper, c.connID, body.Encode())
}

func (c *liveClient) backlogTypes() string {
	var b strings.Builder
	for i, f := range c.backlog {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, "%#x", f.Header.Type)
	}
	if b.Len() == 0 {
		return "(empty)"
	}
	return b.String()
}

// snapshotWeather reads CurrentWeather out of a CNFCharacterLogin body.
func snapshotWeather(f protocol.Frame) int32 {
	return int32(binary.LittleEndian.Uint16(f.Payload[cnfCharLoginWeatherOffset:]))
}

// newPlayer is the common arrangement: connect, log in, enter the world.
func newPlayer(t *testing.T, account, password, charName string) *liveClient {
	t.Helper()
	c := dialLive(t, env("W2PP_E2E_ADDR", "127.0.0.1:8281"), account)
	t.Cleanup(c.close)
	c.login(password)
	c.enterWorld(charName, 0) // class 0 = TransKnight
	return c
}

// gmPlayer is newPlayer for the privileged account under test.
func gmPlayer(t *testing.T) *liveClient {
	t.Helper()
	return newPlayer(t,
		env("W2PP_E2E_ACCOUNT", "test"),
		env("W2PP_E2E_PASSWORD", "test123"),
		env("W2PP_E2E_CHAR", "EvtProbe"))
}

// ---------------------------------------------------------------------------
// Weather (issue #116 fase 1)
// ---------------------------------------------------------------------------

// TestE2EWorldEventWeatherBroadcast is the core weather contract: a GM forcing a
// value must produce ONE MSG_UpdateWeather (0x018B) per in-play client, carrying
// that value as an int32, with HEADER.ID = ESCENE_FIELD (30000).
//
// Both halves matter to the real client: a wrong Type is ignored, and a wrong
// HEADER.ID routes the packet to the wrong scene handler (SendFunc.cpp:1669-1696).
func TestE2EWorldEventWeatherBroadcast(t *testing.T) {
	c := gmPlayer(t)
	defer c.gm("weather auto") // leave the server on automatic rolls

	for _, want := range []int32{1, 2, 0} {
		c.quiet(300 * time.Millisecond)
		c.gm(fmt.Sprintf("weather %d", want))

		f, ok := c.waitFor(protocol.MsgUpdateWeather, 8*time.Second)
		if !ok {
			t.Fatalf("weather %d: no MSG_UpdateWeather (%#x) reached the client",
				want, protocol.MsgUpdateWeather)
		}
		if len(f.Payload) != protocol.MsgUpdateWeatherBodySize {
			t.Errorf("weather %d: body = %d bytes, want %d",
				want, len(f.Payload), protocol.MsgUpdateWeatherBodySize)
		}
		if got := int32(binary.LittleEndian.Uint32(f.Payload)); got != want {
			t.Errorf("weather payload = %d, want %d", got, want)
		}
		if f.Header.ID != protocol.IDScene {
			t.Errorf("weather %d: HEADER.ID = %d, want ESCENE_FIELD (%d)",
				want, f.Header.ID, protocol.IDScene)
		}
		t.Logf("weather %d → MSG_UpdateWeather ok (ID=%d, %d-byte body)",
			want, f.Header.ID, len(f.Payload))
	}
}

// TestE2EWorldEventWeatherIdempotent pins the legacy's
// `ForceWeather != CurrentWeather` guard: re-forcing the value that is already
// live must NOT put a second packet on the wire. A client that receives a
// redundant weather change restarts the sky transition, so the guard is visible.
func TestE2EWorldEventWeatherIdempotent(t *testing.T) {
	c := gmPlayer(t)
	defer c.gm("weather auto")

	c.gm("weather 1")
	if _, ok := c.waitFor(protocol.MsgUpdateWeather, 8*time.Second); !ok {
		t.Fatalf("setup: first /gm weather 1 produced no MSG_UpdateWeather")
	}
	c.quiet(500 * time.Millisecond)

	c.gm("weather 1") // already 1 — must be a no-op on the wire
	if f, ok := c.waitFor(protocol.MsgUpdateWeather, 3*time.Second); ok {
		t.Errorf("re-forcing the live weather re-broadcast it: payload=%d",
			int32(binary.LittleEndian.Uint32(f.Payload)))
	} else {
		t.Log("re-forcing the live weather is silent, as the legacy guard requires")
	}
}

// TestE2EWorldEventWeatherRejectsInvalid covers the deliberate divergence from
// the legacy: imple.cpp:1122-1129 shipped ANY int straight to the client, which
// is how a bad /weather bricked the sky. The port validates and answers with an
// error line instead of broadcasting.
func TestE2EWorldEventWeatherRejectsInvalid(t *testing.T) {
	c := gmPlayer(t)
	defer c.gm("weather auto")

	c.quiet(300 * time.Millisecond)
	c.gm("weather 9")

	if _, ok := c.waitForChat("clima invalido", 5*time.Second); !ok {
		t.Errorf("no rejection message for /gm weather 9")
	}
	if f, ok := c.waitFor(protocol.MsgUpdateWeather, 2*time.Second); ok {
		t.Errorf("invalid weather was BROADCAST: payload=%d",
			int32(binary.LittleEndian.Uint32(f.Payload)))
	}
}

// TestE2EWorldEventWeatherLoginSnapshot proves the second delivery path: a
// client that logs in AFTER a weather change learns the current value from the
// MSG_CNFCharacterLogin snapshot (@1032), not from a broadcast it never saw.
// Without this a relogging player renders clear skies during a storm.
//
// Needs a second seeded account so the first session can stay in play:
//
//	docker compose run --rm dbserver seed-account -name test2 -pass test123
func TestE2EWorldEventWeatherLoginSnapshot(t *testing.T) {
	second := os.Getenv("W2PP_E2E_ACCOUNT2")
	if second == "" {
		t.Skip("set W2PP_E2E_ACCOUNT2 (a second seeded account) to run the login-snapshot check")
	}
	gm := gmPlayer(t)
	defer gm.gm("weather auto")

	const want = int32(2)
	gm.gm(fmt.Sprintf("weather %d", want))
	if _, ok := gm.waitFor(protocol.MsgUpdateWeather, 8*time.Second); !ok {
		t.Fatalf("setup: /gm weather %d produced no broadcast", want)
	}

	late := newPlayer(t, second,
		env("W2PP_E2E_PASSWORD2", "test123"),
		env("W2PP_E2E_CHAR2", "EvtProbe2"))
	if got := snapshotWeather(late.snapshot); got != want {
		t.Errorf("CNFCharacterLogin weather @%d = %d, want %d — a relogging client would render the wrong sky",
			cnfCharLoginWeatherOffset, got, want)
	} else {
		t.Logf("late joiner received weather %d in its login snapshot", got)
	}
}

// ---------------------------------------------------------------------------
// Newbie event (issue #116 fase 2)
// ---------------------------------------------------------------------------

// createMobScoreOffset is where STRUCT_SCORE starts inside a MSG_CreateMob body
// (protocol/createmob.go: Score @abs136 → body124). Level sits at +0, MaxHp at
// +16 and Hp at +24 within it.
const createMobScoreOffset = 124

type mobView struct {
	id        int
	level     int32
	maxHP, hp int32
}

// parseCreateMob reads the fields the newbie handicap is visible in.
func parseCreateMob(f protocol.Frame) (mobView, bool) {
	const need = createMobScoreOffset + 32
	if f.Header.Type != protocol.MsgCreateMob || len(f.Payload) < need {
		return mobView{}, false
	}
	s := f.Payload[createMobScoreOffset:]
	return mobView{
		id:    int(binary.LittleEndian.Uint16(f.Payload[4:])),
		level: int32(binary.LittleEndian.Uint32(s[0:])),
		maxHP: int32(binary.LittleEndian.Uint32(s[16:])),
		hp:    int32(binary.LittleEndian.Uint32(s[24:])),
	}, true
}

// spawnAndInspect runs `/gm spawn tmpl` and returns the CreateMob the server
// pushes for the freshly spawned mob (revealSpawned).
func (c *liveClient) spawnAndInspect(tmpl int, within time.Duration) (mobView, bool) {
	c.t.Helper()
	c.quiet(time.Second) // let the entering-the-world burst finish first
	c.backlog = nil
	c.gm(fmt.Sprintf("spawn %d", tmpl))
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		f, err := c.readFrame(deadline)
		if err != nil {
			return mobView{}, false
		}
		m, ok := parseCreateMob(f)
		if !ok || m.id < MaxUser { // players share the id space below MaxUser
			continue
		}
		return m, true
	}
	return mobView{}, false
}

// TestE2EWorldEventNewbieMobHandicap validates fase 2's one CLIENT-VISIBLE
// effect: while NewbieEventServer is on, a sub-120 monster spawns at 3/4 HP
// (Server.cpp:3326-3327 → world.applyNewbieHandicap). MaxHp is deliberately left
// alone, so the client renders a monster whose health bar starts three-quarters
// full — that difference is the whole observable contract.
//
// The EXP side of the same flag has no S→C frame of its own and is not
// observable from a client; it is covered by internal/level's unit tests.
//
// The flag is driven by the PORTAL config, not the -newbie-event boot flag:
// ApplyWorldEventConfigBoot calls setNewbieEvent with whatever
// world_event_config.newbie_event_enabled says, so the DB value wins over the
// CLI flag at boot. The runner therefore flips the DB row (bumping
// world_event_meta.version so the 15-tick poll reloads) and runs this test twice:
//
//	W2PP_E2E_NEWBIE_EXPECT=full       # row false → mobs spawn at MaxHp
//	W2PP_E2E_NEWBIE_EXPECT=handicap   # row true  → mobs spawn at 3/4 MaxHp
func TestE2EWorldEventNewbieMobHandicap(t *testing.T) {
	expect := os.Getenv("W2PP_E2E_NEWBIE_EXPECT")
	if expect != "full" && expect != "handicap" {
		t.Skip("set W2PP_E2E_NEWBIE_EXPECT to 'full' or 'handicap' (see scripts/e2e-worldevents.sh)")
	}
	tmpl := 1
	if v := os.Getenv("W2PP_E2E_SUMMON_TEMPLATE"); v != "" {
		fmt.Sscanf(v, "%d", &tmpl)
	}

	c := gmPlayer(t)
	got, ok := c.spawnAndInspect(tmpl, 15*time.Second)
	if !ok {
		t.Fatalf("no CreateMob after /gm spawn %d", tmpl)
	}
	if got.maxHP <= 0 {
		t.Fatalf("spawned mob has MaxHp %d — template %d is not usable for this check", got.maxHP, tmpl)
	}
	if got.level >= 120 {
		t.Skipf("summon template %d is level %d (>=120): the handicap only applies below 120; set W2PP_E2E_SUMMON_TEMPLATE",
			tmpl, got.level)
	}

	want := got.maxHP
	if expect == "handicap" {
		want = 3 * got.maxHP / 4
	}
	if got.hp != want {
		t.Errorf("newbie event %s: level %d mob spawned at HP %d/%d, want %d — the client would draw the wrong health bar",
			expect, got.level, got.hp, got.maxHP, want)
		return
	}
	t.Logf("newbie event %s: level %d mob spawned at %d/%d HP as expected",
		expect, got.level, got.hp, got.maxHP)
}

// ---------------------------------------------------------------------------
// Tower capture war (issue #116 fase 4)
// ---------------------------------------------------------------------------

// TestE2EWorldEventTowerWar validates the calendar-gated tower window end to end:
// the announce notice at minute ≤5 and the start notice at minute ≥6, both as
// MSG_MessageChat lines every in-play client receives.
//
// The window is a weekday at hour 20 with NewbieEventServer on (CWarTower.cpp:203,
// handler/towerwar.go). Rather than wait for a real Tuesday evening, the runner
// starts a SECOND tmserver whose TZ places it inside the window — the event reads
// time.Now(), so the container's zone is the whole gate. See
// scripts/e2e-worldevents.sh, which computes the zone and exports the address.
func TestE2EWorldEventTowerWar(t *testing.T) {
	addr := os.Getenv("W2PP_E2E_TOWER_ADDR")
	if addr == "" {
		t.Skip("set W2PP_E2E_TOWER_ADDR to a tmserver inside the tower window (see scripts/e2e-worldevents.sh)")
	}
	c := dialLive(t, addr, env("W2PP_E2E_ACCOUNT", "test"))
	t.Cleanup(c.close)
	c.login(env("W2PP_E2E_PASSWORD", "test123"))
	c.enterWorld(env("W2PP_E2E_CHAR", "EvtProbe"), 0)

	// tickTowerWar runs one pass per minute, so the announce lands within ~2
	// minutes of boot when the server started inside the window.
	if text, ok := c.waitForChat("[Torre]", 3*time.Minute); ok {
		t.Logf("tower announce reached the client: %q", text)
	} else {
		t.Fatalf("no [Torre] notice within the window — the client would never learn the war started")
	}
	// The start transition needs minute >= 6, so from an announce at minute 0-5 it
	// can be a full six minutes away. Waiting less than that would report a
	// working event as broken.
	if text, ok := c.waitForChat("comecou", 8*time.Minute); ok {
		t.Logf("tower start reached the client: %q", text)
	} else {
		t.Errorf("announce arrived but the START notice did not")
	}
}
