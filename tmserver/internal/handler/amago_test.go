package handler

import (
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/mountrate"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const (
	itemAmagoAndaluzN = 2400 // Âmago_de_Andaluz_N
	itemCriaAndaluzN  = 2340 // Cria_de_Andaluz_N — the hatched egg
)

// amagoDB wears mount in Equip[14] and puts the âmago in carry slot 0.
func amagoDB(mount int16, level uint8, amago int16) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Level: 100}
	m := world.Item{Index: mount}
	putShort(&m.Effects[0], 20000) // fed: a starved mount refuses the âmago
	m.Effects[1].Effect = level
	st.Equip[mountEquipSlot] = m
	st.Carry[0] = world.Item{Index: amago}
	db.loadResult = st
	return db
}

func amagoFrame(t *testing.T, c net.Conn) {
	t.Helper()
	body := protocol.MsgUseItemBody{
		SourType: world.ItemPlaceCarry, SourPos: 0,
		DestType: 0, DestPos: mountEquipSlot,
	}
	send(t, c, protocol.MsgUseItem, body.Encode())
}

// equipItemFrame drains until the SendItem for the mount slot.
func equipItem(t *testing.T, c net.Conn) world.Item {
	t.Helper()
	for i := 0; i < 12; i++ {
		p := expect(t, c, protocol.MsgSendItem)
		if le16(p[0:2]) == uint16(world.ItemPlaceEquip) && le16(p[2:4]) == uint16(mountEquipSlot) {
			return decodeItem(p[4:])
		}
	}
	t.Fatal("no SendItem for the mount slot")
	return world.Item{}
}

// A cria grows on every feed — no roll — which is what makes the early mount
// levels deterministic.
func TestAmagoFeedsCria(t *testing.T) {
	vols := map[int]int{itemAmagoAndaluzN: volAmago}
	addr, stop := startServerClockVol(t, amagoDB(itemCriaAndaluzN, 5, itemAmagoAndaluzN), vols)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	amagoFrame(t, c)
	got := equipItem(t, c)
	if got.Effects[1].Effect != 6 {
		t.Errorf("mount level = %d, want 6", got.Effects[1].Effect)
	}
	if got.Index != itemCriaAndaluzN {
		t.Errorf("mount index = %d, want %d (below its growth threshold)", got.Index, itemCriaAndaluzN)
	}
}

// Reaching the threshold turns the cria into the adult, sIndex+30 — the same
// step the egg took to become the cria.
func TestAmagoGrowsCriaIntoAdult(t *testing.T) {
	vols := map[int]int{itemAmagoAndaluzN: volAmago}
	// 2340 is past 2331, so its threshold is 100.
	addr, stop := startServerClockVol(t, amagoDB(itemCriaAndaluzN, 99, itemAmagoAndaluzN), vols)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	amagoFrame(t, c)
	got := equipItem(t, c)
	if got.Index != itemCriaAndaluzN+mountRowSize {
		t.Errorf("mount index = %d, want %d (grown)", got.Index, itemCriaAndaluzN+mountRowSize)
	}
}

// The âmago row has to match the mount row: feeding an Andaluz with someone
// else's âmago is refused, and the item is NOT consumed.
func TestAmagoRefusesMismatchedMount(t *testing.T) {
	const otherAmago = 2401 // Âmago_de_Ca_s/Sela_B, a different row
	vols := map[int]int{otherAmago: volAmago}
	addr, stop := startServerClockVol(t, amagoDB(itemCriaAndaluzN, 5, otherAmago), vols)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	amagoFrame(t, c)
	if code := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); code != NoticeMountNotMatch {
		t.Errorf("notice = %d, want NoticeMountNotMatch", code)
	}
}

func TestMountAmagoSlotOwnRow(t *testing.T) {
	// Every lineage eats its own row — Svadilfari and Sleipnir included, which the
	// legacy fed with the Andaluz N's and the Unicórnio's âmago (:1583-1587).
	for _, c := range []struct {
		name  string
		mount int16
		amago int16
	}{
		{"Svadilfari", 2387, 2417},
		{"Sleipnir", 2388, 2418},
		{"Andaluz N", 2370, 2400},
		{"Unicórnio", 2381, 2411},
	} {
		if got := mountAmagoSlot(c.mount); got != int(c.amago)-amagoBase {
			t.Errorf("%s: âmago slot = %d, want %d (%d)", c.name, got, int(c.amago)-amagoBase, c.amago)
		}
	}
	// An adult sits one row up and maps to the same âmago slot as its cria.
	if mountAmagoSlot(itemCriaAndaluzN) != mountAmagoSlot(itemCriaAndaluzN+mountRowSize) {
		t.Error("cria and adult must share an âmago slot")
	}
}

// The Svadilfari eats the Âmago de Svadilfari (2417), and the Andaluz N's âmago
// (2400) that fed it in the legacy is now refused — and kept.
func TestAmagoSvadilfariEatsItsOwn(t *testing.T) {
	const criaSvadilfari, amagoSvadilfari = 2357, 2417
	t.Run("próprio", func(t *testing.T) {
		vols := map[int]int{amagoSvadilfari: volAmago}
		addr, stop := startServerClockVol(t, amagoDB(criaSvadilfari, 5, amagoSvadilfari), vols)
		defer stop()
		c := enterWorld(t, addr)
		defer c.Close()
		amagoFrame(t, c)
		if got := equipItem(t, c); got.Effects[1].Effect != 6 {
			t.Errorf("level = %d, want 6: o Svadilfari devia comer o 2417", got.Effects[1].Effect)
		}
	})
	t.Run("do Andaluz", func(t *testing.T) {
		vols := map[int]int{itemAmagoAndaluzN: volAmago}
		addr, stop := startServerClockVol(t, amagoDB(criaSvadilfari, 5, itemAmagoAndaluzN), vols)
		defer stop()
		c := enterWorld(t, addr)
		defer c.Close()
		amagoFrame(t, c)
		if code := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); code != NoticeMountNotMatch {
			t.Errorf("notice = %d, want NoticeMountNotMatch: o 2400 é só do Andaluz N agora", code)
		}
	})
}

func TestCriaGrowsAt(t *testing.T) {
	cases := []struct {
		index int16
		want  int
	}{
		{2330, criaGrowth2330},
		{2331, criaGrowth2331},
		{itemCriaAndaluzN, criaGrowthRest},
		{2359, criaGrowthRest},
		{2360, 0}, // already an adult
	}
	for _, c := range cases {
		if got := criaGrowsAt(c.index); got != c.want {
			t.Errorf("criaGrowsAt(%d) = %d, want %d", c.index, got, c.want)
		}
	}
}

// amagoDBStack is amagoDB with a stack of âmagos instead of a single one.
func amagoDBStack(mount int16, level uint8, amago int16, amount int) *fakeDB {
	db := amagoDB(mount, level, amago)
	st := db.loadResult
	it := st.Carry[0]
	setItemAmount(&it, amount)
	st.Carry[0] = it
	db.loadResult = st
	return db
}

// One click spends ten of the stack: a cria never fails, so ten feeds are ten
// levels. This is the whole reason the batch exists — growing a mount is a
// hundred levels, and one drag per level is unusable.
func TestAmagoBatchFeedsTen(t *testing.T) {
	vols := map[int]int{itemAmagoAndaluzN: volAmago}
	addr, stop := startServerClockVol(t, amagoDBStack(itemCriaAndaluzN, 5, itemAmagoAndaluzN, 30), vols)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	amagoFrame(t, c)
	// The carry slot is sent before the equip slot, so it is read first.
	bag := decodeItem(slotItem(t, c, 0))
	got := equipItem(t, c)
	if got.Effects[1].Effect != 15 {
		t.Errorf("mount level = %d, want 15 (5 + a batch of 10)", got.Effects[1].Effect)
	}
	// Exactly ten leave the stack, not the whole thing.
	if got := itemAmount(bag); got != 20 {
		t.Errorf("stack = %d, want 20 (30 - 10)", got)
	}
}

// A stack smaller than the batch spends what it has, and no more.
func TestAmagoBatchStopsAtStackSize(t *testing.T) {
	vols := map[int]int{itemAmagoAndaluzN: volAmago}
	addr, stop := startServerClockVol(t, amagoDBStack(itemCriaAndaluzN, 5, itemAmagoAndaluzN, 3), vols)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	amagoFrame(t, c)
	got := equipItem(t, c)
	if got.Effects[1].Effect != 8 {
		t.Errorf("mount level = %d, want 8 (5 + 3)", got.Effects[1].Effect)
	}
}

// Growing into the adult ends the batch: from there feeds start rolling and can
// cost a level, which is not something to walk into on the same click.
func TestAmagoBatchStopsOnGrowth(t *testing.T) {
	vols := map[int]int{itemAmagoAndaluzN: volAmago}
	// 98 + 2 reaches the 100 threshold with 8 âmagos left in the batch.
	addr, stop := startServerClockVol(t, amagoDBStack(itemCriaAndaluzN, 98, itemAmagoAndaluzN, 30), vols)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	amagoFrame(t, c)
	// The carry slot is sent before the equip slot, so it is read first.
	bag := decodeItem(slotItem(t, c, 0))
	got := equipItem(t, c)
	if got.Index != itemCriaAndaluzN+mountRowSize {
		t.Fatalf("mount index = %d, want the adult %d", got.Index, itemCriaAndaluzN+mountRowSize)
	}
	if n := itemAmount(bag); n != 28 {
		t.Errorf("stack = %d, want 28 (only the 2 feeds up to growth)", n)
	}
}

func TestAmagoTally(t *testing.T) {
	// No failures reads cleaner without a "0 falha(s)" nobody needs.
	if got := amagoTally(10, 0, 15); got != "Âmago: 10 aprimoramento(s). Montaria no nível 15." {
		t.Errorf("tally = %q", got)
	}
	if got := amagoTally(6, 4, 21); got != "Âmago: 6 aprimoramento(s), 4 falha(s). Montaria no nível 21." {
		t.Errorf("tally = %q", got)
	}
}

// The curve, not the flat default, decides an adult's odds — and it is read at
// the mount's CURRENT level, so the same mount gets harder as it climbs.
func TestAmagoUsesTheConfiguredCurve(t *testing.T) {
	curve := mountrate.UnsetCurve()
	curve[0] = 100 // 1..20: never fails
	curve[5] = 0   // 101..120: never succeeds
	rates := mountrate.Table{itemCriaAndaluzN + mountRowSize: curve}

	d := New(Config{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), MountRates: rates})
	adult := world.Item{Index: itemCriaAndaluzN + mountRowSize}

	adult.Effects[1].Effect = 5
	if got := d.amagoGrowthRate(adult); got != 100 {
		t.Errorf("rate at level 5 = %d, want 100", got)
	}
	adult.Effects[1].Effect = 110
	if got := d.amagoGrowthRate(adult); got != 0 {
		t.Errorf("rate at level 110 = %d, want 0", got)
	}
	// A band left unset falls back rather than reading as zero.
	adult.Effects[1].Effect = 50
	if got := d.amagoGrowthRate(adult); got != defaultMountGrowthRate {
		t.Errorf("rate at an unset band = %d, want the default %d", got, defaultMountGrowthRate)
	}
}

// A server with no curves configured behaves exactly as it did before they
// existed — that is what makes the migration safe to ship unseeded.
func TestAmagoDefaultsWithoutCurves(t *testing.T) {
	d := New(Config{Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	adult := world.Item{Index: itemCriaAndaluzN + mountRowSize}
	if got := d.amagoGrowthRate(adult); got != defaultMountGrowthRate {
		t.Errorf("rate = %d, want the default %d", got, defaultMountGrowthRate)
	}
}
