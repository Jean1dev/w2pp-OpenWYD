package handler

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// at builds a wall clock reading for a given minute and second of some hour. The
// hour never matters: the whole Pesadelo schedule is modulo twenty minutes.
func at(minute, sec int) time.Time {
	return time.Date(2026, 9, 5, 13, minute, sec, 0, time.UTC)
}

// TestPesadeloWindowSchedule is the parity-critical table. The legacy states the
// schedule as three disjunctions of REJECTED minute ranges per tier
// (_MSG_UseItem.cpp:2580/2677/2781); this asserts the complement it implies —
// N open at :00-:03, M at :05-:08, A at :10-:13, and the same twenty minutes
// later, twice.
func TestPesadeloWindowSchedule(t *testing.T) {
	tests := []struct {
		name string
		tier int
		min  int
		open bool
	}{
		// N: :00-:03, :20-:23, :40-:43.
		{"N abre em :00", pesaN, 0, true},
		{"N ainda aberto em :03", pesaN, 3, true},
		{"N fecha em :04", pesaN, 4, false},
		{"N fechado em :19", pesaN, 19, false},
		{"N reabre em :20", pesaN, 20, true},
		{"N aberto em :43", pesaN, 43, true},
		{"N fechado em :44", pesaN, 44, false},
		{"N fechado em :59", pesaN, 59, false},

		// M: :05-:08, :25-:28, :45-:48.
		{"M fechado em :04", pesaM, 4, false},
		{"M abre em :05", pesaM, 5, true},
		{"M aberto em :08", pesaM, 8, true},
		{"M fecha em :09", pesaM, 9, false},
		{"M reabre em :25", pesaM, 25, true},
		{"M aberto em :48", pesaM, 48, true},
		{"M fechado em :49", pesaM, 49, false},

		// A: :10-:13, :30-:33, :50-:53.
		{"A fechado em :09", pesaA, 9, false},
		{"A abre em :10", pesaA, 10, true},
		{"A aberto em :13", pesaA, 13, true},
		{"A fecha em :14", pesaA, 14, false},
		{"A reabre em :30", pesaA, 30, true},
		{"A aberto em :53", pesaA, 53, true},
		{"A fechado em :54", pesaA, 54, false},

		// The tiers must never overlap: at any minute at most one is open.
		{"em :05 o N ja fechou", pesaN, 5, false},
		{"em :10 o M ja fechou", pesaM, 10, false},
		{"em :00 o A esta fechado", pesaA, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, open := pesaTierTable[tc.tier].window(at(tc.min, 0))
			if open != tc.open {
				t.Errorf("window(min %d) open = %v, want %v", tc.min, open, tc.open)
			}
		})
	}
}

// No two tiers may be open at the same second, or a party could be admitted to
// two instances from one schedule.
func TestPesadeloTiersNeverOverlap(t *testing.T) {
	for minute := 0; minute < 60; minute++ {
		for sec := 0; sec < 60; sec += 17 {
			openCount := 0
			for tier := range pesaTierTable {
				if _, open := pesaTierTable[tier].window(at(minute, sec)); open {
					openCount++
				}
			}
			if openCount > 1 {
				t.Fatalf("%d tiers open at :%02d:%02d, want at most 1", openCount, minute, sec)
			}
		}
	}
}

// The countdown is what is LEFT of the four-minute window, not a fresh four
// minutes: enter late and you get less (_MSG_UseItem.cpp:2604-2614).
func TestPesadeloCountdownIsRemainingWindow(t *testing.T) {
	tests := []struct {
		name     string
		tier     int
		min, sec int
		want     int
	}{
		{"N no instante da abertura", pesaN, 0, 0, 240},
		{"N trinta segundos depois", pesaN, 0, 30, 210},
		{"N no ultimo segundo util", pesaN, 3, 59, 1},
		{"N na terceira janela", pesaN, 40, 10, 230},
		{"M um minuto dentro", pesaM, 6, 0, 180},
		{"A meio da janela", pesaA, 32, 0, 120},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			left, open := pesaTierTable[tc.tier].window(at(tc.min, tc.sec))
			if !open {
				t.Fatalf("window(:%02d:%02d) closed, want open", tc.min, tc.sec)
			}
			if left != tc.want {
				t.Errorf("secondsLeft = %d, want %d", left, tc.want)
			}
		})
	}
}

func TestPesadeloNextWindow(t *testing.T) {
	tests := []struct {
		name     string
		tier     int
		min, sec int
		want     time.Duration
	}{
		{"N fechado logo apos a janela", pesaN, 4, 0, 16 * time.Minute},
		{"N um minuto antes de reabrir", pesaN, 19, 0, time.Minute},
		{"N com segundos quebrados", pesaN, 19, 30, 30 * time.Second},
		{"M fechado em :00", pesaM, 0, 0, 5 * time.Minute},
		{"A fechado em :00", pesaA, 0, 0, 10 * time.Minute},
		// An open tier reports zero rather than the time to its NEXT window.
		{"N aberto reporta zero", pesaN, 1, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := pesaTierTable[tc.tier].nextWindow(at(tc.min, tc.sec)); got != tc.want {
				t.Errorf("nextWindow(:%02d:%02d) = %v, want %v", tc.min, tc.sec, got, tc.want)
			}
		})
	}
}

// The wipe runs one minute before each opening (ProcessSecMinTimer.cpp:1006-1035:
// A at :09/:29/:49, M at :04/:24/:44, N at :19/:39/:59).
func TestPesadeloWipeMinutes(t *testing.T) {
	want := map[int][]int{
		pesaN: {19, 39, 59},
		pesaM: {4, 24, 44},
		pesaA: {9, 29, 49},
	}
	for tier, minutes := range want {
		got := []int{}
		for minute := 0; minute < 60; minute++ {
			if pesaTierTable[tier].wipeMinute(minute) {
				got = append(got, minute)
			}
		}
		if len(got) != len(minutes) {
			t.Errorf("tier %s wipes at %v, want %v", pesaTierTable[tier].name, got, minutes)
			continue
		}
		for i := range got {
			if got[i] != minutes[i] {
				t.Errorf("tier %s wipes at %v, want %v", pesaTierTable[tier].name, got, minutes)
				break
			}
		}
	}
}

// A wipe must land in the minute BEFORE its tier reopens, so the instance is
// empty when the window starts. Tying the two together catches a schedule edit
// that moves one without the other.
func TestPesadeloWipePrecedesEachOpening(t *testing.T) {
	for tier := range pesaTierTable {
		tr := pesaTierTable[tier]
		for minute := 0; minute < 60; minute++ {
			if !tr.wipeMinute(minute) {
				continue
			}
			next := (minute + 1) % 60
			if _, open := tr.window(at(next, 0)); !open {
				t.Errorf("tier %s wipes at :%02d but does not open at :%02d", tr.name, minute, next)
			}
		}
	}
}

func TestPesadeloTierForVolatile(t *testing.T) {
	tests := []struct {
		vol  int
		tier int
		ok   bool
	}{
		{173, pesaN, true},
		{174, pesaM, true},
		{175, pesaA, true},
		{172, 0, false},
		{176, 0, false},
		{212, 0, false}, // the ticket book, not an entry scroll
		{0, 0, false},
	}
	for _, tc := range tests {
		tier, ok := pesadeloTierForVolatile(tc.vol)
		if ok != tc.ok {
			t.Errorf("pesadeloTierForVolatile(%d) ok = %v, want %v", tc.vol, ok, tc.ok)
			continue
		}
		if ok && tier != tc.tier {
			t.Errorf("pesadeloTierForVolatile(%d) = %d, want %d", tc.vol, tier, tc.tier)
		}
		if isPesadeloVolatile(tc.vol) != tc.ok {
			t.Errorf("isPesadeloVolatile(%d) = %v, want %v", tc.vol, !tc.ok, tc.ok)
		}
	}
}

// The staging tiles are the legacy TargetX/128 gates: N (19,15), M (16,16),
// A (19,13). Each tier stages from a different city, so mixing them up would
// send players to the wrong door.
func TestPesadeloStagingTiles(t *testing.T) {
	tests := []struct {
		name string
		tier int
		x, y int16
		want bool
	}{
		{"N no centro do tile", pesaN, 19*128 + 64, 15*128 + 64, true},
		{"N na borda inferior", pesaN, 19 * 128, 15 * 128, true},
		{"N na borda superior", pesaN, 19*128 + 127, 15*128 + 127, true},
		{"N um tile a leste", pesaN, 20 * 128, 15 * 128, false},
		{"N no tile do A", pesaN, 19*128 + 10, 13*128 + 10, false},
		{"M no proprio tile", pesaM, 16*128 + 10, 16*128 + 10, true},
		{"A no proprio tile", pesaA, 19*128 + 10, 13*128 + 10, true},
		{"A no tile do N", pesaA, 19*128 + 10, 15*128 + 10, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := pesaTierTable[tc.tier].staging(tc.x, tc.y); got != tc.want {
				t.Errorf("staging(%d,%d) = %v, want %v", tc.x, tc.y, got, tc.want)
			}
		})
	}
}

// The dungeon segments are Regions.txt rows 4-6, and they must agree with the
// ones inPesadelo already uses for the Gema Estelar carve-out — the two would
// otherwise disagree about where the dungeon is.
func TestPesadeloSegmentsMatchRegions(t *testing.T) {
	// Regions.txt: Pesadelo_N 1285,268-1332,367; Pesadelo_M 1026,260-1158,382;
	// Pesadelo_A 1155,130-1290,225.
	tests := []struct {
		name string
		tier int
		x, y int16
	}{
		{"N canto declarado em Regions.txt", pesaN, 1285, 268},
		{"M canto declarado em Regions.txt", pesaM, 1026, 260},
		{"A canto declarado em Regions.txt", pesaA, 1204, 152},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !pesaTierTable[tc.tier].segment(tc.x, tc.y) {
				t.Errorf("segment(%d,%d) = false for tier %s", tc.x, tc.y, pesaTierTable[tc.tier].name)
			}
		})
	}

	// inPesadelo (item.go) must cover all three segments, N included. The legacy
	// lists only M and A, and the gap is a way in: nothing stops a Gema Estelar
	// inside N, and a Pergaminho de Portal then re-enters the instance past the
	// window, the leader gate, the class ladder and the run cap.
	for tier := range pesaTierTable {
		tr := pesaTierTable[tier]
		x, y := tr.segX*128+40, tr.segY*128+40
		if !inPesadelo(x, y) {
			t.Errorf("inPesadelo(%d,%d) = false for tier %s — o Portal volta para dentro dela",
				x, y, tr.name)
		}
	}
	// And nowhere else: a blanket true would block the Portal everywhere.
	if inPesadelo(2086, 2093) {
		t.Error("inPesadelo cobre Armia; o Portal pararia de funcionar no mundo inteiro")
	}
}

// TestUsePortalScrollBlockedFromPesadeloNormal is the hole itself, end to end.
// The Arcano case is already covered in item_test.go; this is the Normal
// segment, the one the legacy leaves open. Save a point inside it and the
// Pergaminho de Portal walks back in whenever it likes — past the four-minute
// window, the leader gate, the class ladder and the run cap.
func TestUsePortalScrollBlockedFromPesadeloNormal(t *testing.T) {
	const portal = 776
	// (1304, 335) is the N arrival point, inside segment (10,2).
	db := gemaEstelarDB(5, 5, 1304, 335, world.Item{Index: portal})
	addr, stop := startServerClockVol(t, db, map[int]int{portal: volPortalScroll})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useItemFrame(t, c, 0)
	if ty, p, ok := readMaybe(t, c); !ok || ty != protocol.MsgMessageBoxOk || noticeCode(t, p) != NoticeCantUseHere {
		t.Fatalf("notice = %#x/%v ok=%v, want NoticeCantUseHere", ty, noticeCode(t, p), ok)
	}
	if got := le16(expect(t, c, protocol.MsgSendItem)[4:6]); got != portal {
		t.Errorf("slot 0 = %d, want o pergaminho devolvido intacto (%d)", got, portal)
	}
}

// TestPesadeloSegmentsMatchExpZones ties the dungeon to the EXP table that pays
// inside it. They are two tables in two packages that have to agree on the same
// six numbers: move a tier here and its kills silently start paying open-field
// rates, which is exactly the bug the zone port existed to fix.
func TestPesadeloSegmentsMatchExpZones(t *testing.T) {
	want := map[int]level.Zone{
		pesaN: level.ZonePesadeloNormal,
		pesaM: level.ZonePesadeloMistico,
		pesaA: level.ZonePesadeloArcano,
	}
	for tier, zone := range want {
		tr := pesaTierTable[tier]
		x, y := int32(tr.segX)*128+40, int32(tr.segY)*128+40
		if got := level.ZoneForTile(x, y); got != zone {
			t.Errorf("o segmento do Pesadelo %s cai na zona de XP %q, quero %q",
				tr.name, got.Name(), zone.Name())
		}
	}
}

// Every landing slot must be inside its own dungeon segment, or entry would drop
// the party outside the instance the wipe later clears.
func TestPesadeloSpawnsAreInsideTheirSegment(t *testing.T) {
	for tier := range pesaTierTable {
		tr := pesaTierTable[tier]
		for slot, pos := range tr.spawn {
			if !tr.segment(pos[0], pos[1]) {
				t.Errorf("tier %s slot %d lands at (%d,%d), outside its segment",
					tr.name, slot, pos[0], pos[1])
			}
		}
	}
}

func TestPesadeloBoxCoversSegment(t *testing.T) {
	b := pesaTierTable[pesaA].box()
	if b.x1 != 9*128 || b.y1 != 1*128 || b.x2 != 9*128+127 || b.y2 != 1*128+127 {
		t.Errorf("box = %+v, want the full (9,1) segment", b)
	}
	if !b.contains(1204, 152) {
		t.Error("box should contain the A landing point")
	}
	if b.contains(1304, 335) {
		t.Error("box should not reach into the N segment")
	}
}

// startPesadeloServer is startServerClockVolGrid with a frozen clock, so the
// schedule under test is the one the assertion names.
func startPesadeloServer(t *testing.T, persist world.Persistence, vols map[int]int, now time.Time) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, ItemVolatiles: vols, Now: func() time.Time { return now }})
	w := world.New(world.Config{GridDim: 2600}, log, persist, d.Handle)
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

// pesadeloDB seeds a character standing at (x,y) holding the scroll in slot 0.
func pesadeloDB(x, y int16, classMaster uint8, scroll int16) *fakeDB {
	db := newDB()
	st := world.CharacterState{
		Slot: 0, Name: "Hero", X: x, Y: y, HP: 1000, MaxHP: 1000,
		ClassMaster: classMaster,
	}
	st.Carry[0] = world.Item{Index: scroll}
	db.loadResult = st
	return db
}

// The N staging tile, well inside the (19,15) segment.
const stageNX, stageNY = 19*128 + 40, 15*128 + 40

// A refused scroll must come back: the client optimistically removes it, so the
// slot resend is what stops it looking eaten.
func TestUsePesadeloScrollRefusedOutsideStagingArea(t *testing.T) {
	db := pesadeloDB(2100, 2100, classMasterMortal, itemPesadeloGrupoN)
	addr, stop := startPesadeloServer(t, db,
		map[int]int{itemPesadeloGrupoN: volPesadeloN}, at(0, 10))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useItemFrame(t, c, 0)
	if got := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); got != NoticeCantUseHere {
		t.Errorf("notice = %v, want NoticeCantUseHere", got)
	}
	if got := le16(expect(t, c, protocol.MsgSendItem)[4:6]); got != itemPesadeloGrupoN {
		t.Errorf("slot = %d, want the scroll returned uneaten (%d)", got, itemPesadeloGrupoN)
	}
}

func TestUsePesadeloScrollRefusedOutsideWindow(t *testing.T) {
	db := pesadeloDB(stageNX, stageNY, classMasterMortal, itemPesadeloGrupoN)
	// :10 is inside the A window, so N is firmly closed.
	addr, stop := startPesadeloServer(t, db,
		map[int]int{itemPesadeloGrupoN: volPesadeloN}, at(10, 0))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useItemFrame(t, c, 0)
	if got := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); got != NoticePesadeloClosed {
		t.Errorf("notice = %v, want NoticePesadeloClosed", got)
	}
	// The notice this rides on is the legacy's own text, "Pesadelo disponível
	// entre 18h e 24h." — an hour-of-day rule this server does not have. At
	// :10:00 the N door reopens at :20, so the line beside it has to say ten
	// minutes, which is the only part a player can act on.
	if got, want := decodePanel(expect(t, c, protocol.MsgMessagePanel)),
		"Pesadelo N fechado. Abre em 10m00s. A janela dura 4 min e volta a cada 20."; got != want {
		t.Errorf("panel = %q, want %q", got, want)
	}
	if got := le16(expect(t, c, protocol.MsgSendItem)[4:6]); got != itemPesadeloGrupoN {
		t.Errorf("slot = %d, want the scroll returned uneaten", got)
	}
}

// TestPesadeloClosedLineFitsThePanel guards the one way this line can fail in
// production and nowhere else: MSG_MessagePanel truncates at 94 bytes, and the
// tier name plus a two-digit countdown is the longest it gets.
func TestPesadeloClosedLineFitsThePanel(t *testing.T) {
	for tier := range pesaTierTable {
		linha := fmt.Sprintf(
			"Pesadelo %s fechado. Abre em %dm%02ds. A janela dura %d min e volta a cada %d.",
			pesaTierTable[tier].name, 19, 59, pesaWindowMinutes, pesaWindowStride)
		if n := len(protocol.ClientText(linha)); n > 94 {
			t.Errorf("a linha do tier %s ocupa %d bytes, e o painel corta em 94: %q",
				pesaTierTable[tier].name, n, linha)
		}
	}
}

func TestUsePesadeloScrollRefusedWrongTier(t *testing.T) {
	// An Arch on the N staging tile during the N window: right place, right time,
	// wrong progression tier.
	db := pesadeloDB(stageNX, stageNY, classMasterArch, itemPesadeloGrupoN)
	addr, stop := startPesadeloServer(t, db,
		map[int]int{itemPesadeloGrupoN: volPesadeloN}, at(0, 10))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useItemFrame(t, c, 0)
	if got := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); got != NoticePesadeloClassNotAllowed {
		t.Errorf("notice = %v, want NoticePesadeloClassNotAllowed", got)
	}
	// The notice code alone renders as nothing on the client; the line beside it
	// is what tells the player which door refused them.
	if got := decodePanel(expect(t, c, protocol.MsgMessagePanel)); got != "Entrada permitida somente à Mortais" {
		t.Errorf("panel = %q, want the N tier refusal", got)
	}
	if got := le16(expect(t, c, protocol.MsgSendItem)[4:6]); got != itemPesadeloGrupoN {
		t.Errorf("slot = %d, want the scroll returned uneaten", got)
	}
}

// A Celestial with no entries is refused at the Arcano door even when everything
// else lines up — the /nt count is the last gate (_MSG_UseItem.cpp:2789).
func TestUsePesadeloScrollRefusedWithoutEntries(t *testing.T) {
	db := pesadeloDB(19*128+40, 13*128+40, classMasterCelestial, itemPesadeloGrupoA)
	addr, stop := startPesadeloServer(t, db,
		map[int]int{itemPesadeloGrupoA: volPesadeloA}, at(10, 0))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useItemFrame(t, c, 0)
	if got := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); got != NoticePesadeloNoEntries {
		t.Errorf("notice = %v, want NoticePesadeloNoEntries", got)
	}
}

// The happy path: the player is jumped to the tier's landing point, the client
// gets the remaining window as a countdown, and the scroll is consumed.
func TestUsePesadeloScrollEntersAndConsumes(t *testing.T) {
	db := pesadeloDB(stageNX, stageNY, classMasterMortal, itemPesadeloGrupoN)
	addr, stop := startPesadeloServer(t, db,
		map[int]int{itemPesadeloGrupoN: volPesadeloN}, at(0, 30))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useItemFrame(t, c, 0)

	// doTeleport jumps the avatar with MSG_Action carrying the destination.
	action := expect(t, c, protocol.MsgAction)
	wantX, wantY := pesaTierTable[pesaN].spawn[0][0], pesaTierTable[pesaN].spawn[0][1]
	// MsgActionBody: PosX(2) PosY(2) Effect(4) Speed(4) Route(24) TargetX(2) TargetY(2).
	if gotX, gotY := int16(le16(action[36:38])), int16(le16(action[38:40])); gotX != wantX || gotY != wantY {
		t.Errorf("teleport target = (%d,%d), want (%d,%d)", gotX, gotY, wantX, wantY)
	}

	// Entering 30s into the window leaves 210s, not a fresh 240.
	if got := int32(binary.LittleEndian.Uint32(expect(t, c, protocol.MsgStartTime)[0:4])); got != 210 {
		t.Errorf("countdown = %d, want 210", got)
	}

	if got := le16(expect(t, c, protocol.MsgSendItem)[4:6]); got != 0 {
		t.Errorf("slot = %d, want empty after a successful entry", got)
	}
}

// The ticket book tops a Celestial up by thirteen entries and is consumed.
func TestUseEscrituraPesadeloGrantsEntries(t *testing.T) {
	db := pesadeloDB(2100, 2100, classMasterCelestial, itemEscrituraPesadelo)
	addr, stop := startPesadeloServer(t, db,
		map[int]int{itemEscrituraPesadelo: volEscrituraPesadelo}, at(0, 0))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useItemFrame(t, c, 0)
	if got := le16(expect(t, c, protocol.MsgSendItem)[4:6]); got != 0 {
		t.Errorf("slot = %d, want the escritura consumed", got)
	}
}

// newTickDispatcher builds a dispatcher and an unserved world so the wipe can be
// driven directly. With no sessions, clearArea is a no-op and only the counter
// bookkeeping is under test.
func newTickDispatcher(t *testing.T, now func() time.Time) (*Dispatcher, *world.World) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, Now: now})
	w := world.New(world.Config{GridDim: 2600}, log, newDB(), d.Handle)
	return d, w
}

// The wipe resets its own tier's run counter and leaves the other two alone —
// they are on different schedules and a shared reset would hand out free runs.
func TestTickPesadeloResetsOnlyItsOwnTier(t *testing.T) {
	now := at(19, 0) // N's wipe minute
	d, w := newTickDispatcher(t, func() time.Time { return now })
	d.events.pesaRuns = [pesaTiers]int{pesaN: 3, pesaM: 2, pesaA: 1}

	d.tickPesadelo(w)

	if d.events.pesaRuns[pesaN] != 0 {
		t.Errorf("N runs = %d, want 0 after its wipe", d.events.pesaRuns[pesaN])
	}
	if d.events.pesaRuns[pesaM] != 2 || d.events.pesaRuns[pesaA] != 1 {
		t.Errorf("M/A runs = %d/%d, want 2/1 untouched",
			d.events.pesaRuns[pesaM], d.events.pesaRuns[pesaA])
	}
}

// The tick fires many times a minute. Without the lastWipe guard the wipe would
// repeat for a whole minute, evicting anyone who walked back in and zeroing a
// counter that had already started counting the next window.
func TestTickPesadeloWipesOncePerMinute(t *testing.T) {
	now := at(19, 0)
	d, w := newTickDispatcher(t, func() time.Time { return now })

	d.events.pesaRuns[pesaN] = 3
	d.tickPesadelo(w)
	if d.events.pesaRuns[pesaN] != 0 {
		t.Fatalf("first tick did not wipe: runs = %d", d.events.pesaRuns[pesaN])
	}

	// A run started later in the same minute must survive the remaining ticks.
	d.events.pesaRuns[pesaN] = 1
	now = at(19, 30)
	d.tickPesadelo(w)
	if d.events.pesaRuns[pesaN] != 1 {
		t.Errorf("runs = %d, want the second wipe in the same minute suppressed", d.events.pesaRuns[pesaN])
	}

	// The next wipe minute wipes again.
	d.events.pesaRuns[pesaN] = 2
	now = at(39, 0)
	d.tickPesadelo(w)
	if d.events.pesaRuns[pesaN] != 0 {
		t.Errorf("runs = %d, want 0 at the next wipe minute", d.events.pesaRuns[pesaN])
	}
}

// Outside a wipe minute nothing happens.
func TestTickPesadeloIdleOutsideWipeMinutes(t *testing.T) {
	now := at(1, 0) // N is open, nobody wipes
	d, w := newTickDispatcher(t, func() time.Time { return now })
	d.events.pesaRuns = [pesaTiers]int{pesaN: 1, pesaM: 2, pesaA: 3}

	d.tickPesadelo(w)

	if d.events.pesaRuns != [pesaTiers]int{pesaN: 1, pesaM: 2, pesaA: 3} {
		t.Errorf("runs = %v, want them untouched at a non-wipe minute", d.events.pesaRuns)
	}
}

// The run cap defaults to the legacy maxNightmare of 3, and a configured value
// wins.
func TestMaxNightmareDefault(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if d := New(Config{Log: log}); d.maxNightmare != defaultMaxNightmare {
		t.Errorf("maxNightmare = %d, want the default %d", d.maxNightmare, defaultMaxNightmare)
	}
	if d := New(Config{Log: log, MaxNightmare: 7}); d.maxNightmare != 7 {
		t.Errorf("maxNightmare = %d, want the configured 7", d.maxNightmare)
	}
}

// pesadeloMobTemplate builds an 816-byte STRUCT_MOB. merchant != 0 makes it an
// NPC (shopkeeper), which the wipe must leave standing.
func pesadeloMobTemplate(name string, merchant uint8) []byte {
	b := make([]byte, 816)
	copy(b[0:16], name)
	const cs = 92                                  // CurrentScore
	binary.LittleEndian.PutUint32(b[cs+0:], 10)    // Level
	binary.LittleEndian.PutUint32(b[cs+16:], 1000) // MaxHp
	binary.LittleEndian.PutUint32(b[cs+24:], 1000) // Hp
	b[cs+12] = merchant                            // CurrentScore.Merchant
	return b
}

// The wipe must clear the instance's monsters — that is the whole point of it —
// but leave the dungeon's own shopkeepers alone. DespawnMob only queues a
// respawn for combat monsters, so despawning an NPC would delete it until the
// next server restart, and both the N and M segments hold a row of shops.
func TestDespawnPesadeloMobsSparesNPCs(t *testing.T) {
	d, w := newTickDispatcher(t, func() time.Time { return at(19, 0) })

	// Inside Pesadelo N (segment 10,2).
	monster := w.SpawnMob(pesadeloMobTemplate("Monstro", 0), 1304, 335)
	shop := w.SpawnMob(pesadeloMobTemplate("Martin", 1), 1317, 346)
	// Inside Pesadelo M (segment 8,2) — a different tier, must be untouched.
	otherTier := w.SpawnMob(pesadeloMobTemplate("Outro", 0), 1083, 308)
	// Out in the open world.
	outside := w.SpawnMob(pesadeloMobTemplate("Campo", 0), 2100, 2100)
	for name, id := range map[string]int{"monster": monster, "shop": shop, "otherTier": otherTier, "outside": outside} {
		if id < 0 {
			t.Fatalf("could not spawn %s", name)
		}
	}

	if got := d.despawnPesadeloMobs(w, pesaN); got != 1 {
		t.Errorf("despawned %d entities, want only the monster inside Pesadelo N", got)
	}
	if w.Entity(monster) != nil {
		t.Error("the monster inside the instance survived the wipe")
	}
	if w.Entity(shop) == nil {
		t.Error("the dungeon shopkeeper was despawned; DespawnMob never brings an NPC back")
	}
	if w.Entity(otherTier) == nil {
		t.Error("a monster in the Místico segment was despawned by the Normal wipe")
	}
	if w.Entity(outside) == nil {
		t.Error("a monster outside every instance was despawned")
	}
}

// The per-minute wipe drives the despawn, not just the counter reset.
func TestTickPesadeloDespawnsMonsters(t *testing.T) {
	now := at(19, 0) // N's wipe minute
	d, w := newTickDispatcher(t, func() time.Time { return now })
	monster := w.SpawnMob(pesadeloMobTemplate("Monstro", 0), 1304, 335)
	if monster < 0 {
		t.Fatal("could not spawn the monster")
	}

	d.tickPesadelo(w)

	if w.Entity(monster) != nil {
		t.Error("tickPesadelo left the instance populated")
	}
}

// The entry ladder, class and level together. The legacy gates on class alone;
// the caps are a server rule mirroring the Pergaminho da Água, so this table IS
// the specification — Mortal runs N to 400, Arch runs M to 400, a Celestial
// borrows M only while under 40, and Arcano is Celestial-only up to 150, after
// which the progression moves to the Água A chain.
func TestPesadeloEntryLadder(t *testing.T) {
	tests := []struct {
		name        string
		tier        int
		classMaster uint8
		level       int32
		want        bool
	}{
		// N — Mortal, uncapped in practice (MaxLevel is 399).
		{"N: Mortal nivel 1", pesaN, classMasterMortal, 1, true},
		{"N: Mortal no teto do servidor", pesaN, classMasterMortal, 399, true},
		{"N: Arch nao entra", pesaN, classMasterArch, 1, false},
		{"N: Celestial nao entra", pesaN, classMasterCelestial, 1, false},

		// M — Arch uncapped, plus a Celestial still under 40.
		{"M: Arch nivel 1", pesaM, classMasterArch, 1, true},
		{"M: Arch no teto do servidor", pesaM, classMasterArch, 399, true},
		{"M: Celestial abaixo do teto", pesaM, classMasterCelestial, 39, true},
		{"M: Celestial exatamente no teto", pesaM, classMasterCelestial, 40, true},
		{"M: Celestial acima do teto", pesaM, classMasterCelestial, 41, false},
		{"M: Mortal nao entra", pesaM, classMasterMortal, 1, false},
		// The sub-celestial tiers are past this rung entirely.
		{"M: CelestialCS nao entra", pesaM, classMasterCelestialCS, 10, false},
		{"M: SCelestial nao entra", pesaM, classMasterSCelestial, 10, false},

		// A — every celestial tier, up to 150.
		{"A: Celestial abaixo do teto", pesaA, classMasterCelestial, 149, true},
		{"A: Celestial exatamente no teto", pesaA, classMasterCelestial, 150, true},
		{"A: Celestial acima do teto vai para a Agua A", pesaA, classMasterCelestial, 151, false},
		{"A: CelestialCS dentro do teto", pesaA, classMasterCelestialCS, 100, true},
		{"A: SCelestial dentro do teto", pesaA, classMasterSCelestial, 100, true},
		{"A: SCelestial acima do teto", pesaA, classMasterSCelestial, 200, false},
		{"A: Mortal nao entra", pesaA, classMasterMortal, 1, false},
		{"A: Arch nao entra", pesaA, classMasterArch, 1, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := pesadeloAllowed(tc.tier, tc.classMaster, tc.level); got != tc.want {
				t.Errorf("pesadeloAllowed(tier %s, class %d, level %d) = %v, want %v",
					pesaTierTable[tc.tier].name, tc.classMaster, tc.level, got, tc.want)
			}
		})
	}
}

// Class and level are separate gates so a refusal can say which one closed:
// "wrong dungeon" and "you outgrew this one" need different fixes from the
// player. A Celestial at 200 passes the class gate for Arcano and fails only on
// level.
func TestPesadeloClassAndLevelGatesAreDistinct(t *testing.T) {
	if !pesadeloTierClass(pesaA, classMasterCelestial) {
		t.Error("Celestial should pass the Arcano class gate at any level")
	}
	if maxLevel := pesadeloLevelCap(pesaA, classMasterCelestial); maxLevel != pesaACelestialMaxLevel {
		t.Errorf("Arcano cap = %d, want %d", maxLevel, pesaACelestialMaxLevel)
	}
	if pesadeloAllowed(pesaA, classMasterCelestial, 200) {
		t.Error("a Celestial at 200 must be refused by the level cap, not admitted")
	}

	// A Mortal fails the class gate for Arcano, so the level cap never applies.
	if pesadeloTierClass(pesaA, classMasterMortal) {
		t.Error("Mortal must fail the Arcano class gate")
	}
}

// The Água chain is the next rung: what a Celestial outgrows in Pesadelo A it
// continues there, which is why Água A has no cap of its own. Pinning the two
// together documents the ladder as one rule rather than two coincidences.
func TestPesadeloAndWaterLaddersLineUp(t *testing.T) {
	// The Místico cap is the same number in both dungeons.
	if pesaMCelestialMaxLevel != waterMCelestialMaxLevel {
		t.Errorf("Pesadelo M cap = %d, Água M cap = %d; the ladder expects one number",
			pesaMCelestialMaxLevel, waterMCelestialMaxLevel)
	}
	// And a Celestial past the Arcano cap must still be welcome in Água A.
	if !waterClassAllowed(waterA, classMasterCelestial, pesaACelestialMaxLevel+1) {
		t.Error("a Celestial past the Pesadelo A cap has nowhere to go: Água A refused them too")
	}
}

// End to end: a Celestial who outgrew Arcano is refused at the door with the
// level notice, and keeps the scroll.
func TestUsePesadeloScrollRefusedAboveLevelCap(t *testing.T) {
	db := newDB()
	st := world.CharacterState{
		Slot: 0, Name: "Hero", X: 19*128 + 40, Y: 13*128 + 40, HP: 1000, MaxHP: 1000,
		ClassMaster: classMasterCelestial,
		Level:       pesaACelestialMaxLevel + 1,
	}
	st.Carry[0] = world.Item{Index: itemPesadeloGrupoA}
	db.loadResult = st

	addr, stop := startPesadeloServer(t, db,
		map[int]int{itemPesadeloGrupoA: volPesadeloA}, at(10, 0))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useItemFrame(t, c, 0)
	if got := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); got != NoticePesadeloLevelTooHigh {
		t.Errorf("notice = %v, want NoticePesadeloLevelTooHigh", got)
	}
	if got := le16(expect(t, c, protocol.MsgSendItem)[4:6]); got != itemPesadeloGrupoA {
		t.Errorf("slot = %d, want the scroll returned uneaten", got)
	}
}

// TestNigDizOEstadoAntesDoNumero pins the wording of /nig, which is the whole
// point of the line existing.
//
// "abre em 5m32s" read alone is a countdown, and the client's own timer widget
// sits at 0:0 whenever nothing is running. Put together, a closed door looks
// broken: the counter is at zero, so surely it should be open. The state has to
// come first, and the open line has to say the number is time to ENTER — it is
// what remains of the entry window, not how long a run lasts.
func TestNigDizOEstadoAntesDoNumero(t *testing.T) {
	// :14:28 — the exact clock from a player's report, where /nig read
	// "N 5m32s / M 10m32s / A 15m32s" and the three doors were shut.
	db := pesadeloDB(stageNX, stageNY, classMasterMortal, itemPesadeloGrupoN)
	addr, stop := startPesadeloServer(t, db,
		map[int]int{itemPesadeloGrupoN: volPesadeloN}, at(14, 28))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	whisperFrame(t, c, "nig", "")
	want := []string{
		"Pesadelo N: FECHADO, abre em 5m32s",
		"Pesadelo M: FECHADO, abre em 10m32s",
		"Pesadelo A: FECHADO, abre em 15m32s",
	}
	for _, quero := range want {
		if got := decodePanel(expect(t, c, protocol.MsgMessagePanel)); got != quero {
			t.Errorf("linha = %q, quero %q", got, quero)
		}
	}
}

// And an open tier reports what is left of the ENTRY window, said as such.
func TestNigDizQuantoFaltaParaEntrar(t *testing.T) {
	db := pesadeloDB(stageNX, stageNY, classMasterMortal, itemPesadeloGrupoN)
	addr, stop := startPesadeloServer(t, db,
		map[int]int{itemPesadeloGrupoN: volPesadeloN}, at(20, 30))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	whisperFrame(t, c, "nig", "")
	if got, quero := decodePanel(expect(t, c, protocol.MsgMessagePanel)),
		"Pesadelo N: ABERTO, 210s para entrar"; got != quero {
		t.Errorf("linha = %q, quero %q", got, quero)
	}
}
