package handler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const (
	itemGemaDiamante = 3900
	itemGemaGarnet   = 3903
	itemAnctBase     = 6000
	itemPlusEleven   = 6100
)

func TestBaseGemChangesAnctGradeVariant(t *testing.T) {
	gems := []struct {
		name string
		vol  int
		want int16
	}{
		{"diamante", volGemDiamond, itemAnctBase},
		{"esmeralda", volGemEmerald, itemAnctBase + 1},
		{"coral", volGemCoral, itemAnctBase + 2},
		{"garnet", volGemGarnet, itemAnctBase + 3},
	}
	for grade := 5; grade <= 8; grade++ {
		start := int16(itemAnctBase + grade - 5)
		for _, gem := range gems {
			t.Run(fmt.Sprintf("grade%d_%s", grade, gem.name), func(t *testing.T) {
				d, w, s, e := baseGemFixture(map[int]int{int(start): grade})
				e.Carry[0] = world.Item{Index: itemGemaDiamante}
				e.Equip[1] = world.Item{Index: start}

				d.useBaseGem(w, s, e, baseGemUseBody(world.ItemPlaceEquip, 1), 0, gem.vol)

				if got := e.Equip[1].Index; got != gem.want {
					t.Errorf("grade %d changed index to %d, want %d", grade, got, gem.want)
				}
				if !e.Carry[0].Empty() {
					t.Errorf("source gem not consumed: %+v", e.Carry[0])
				}
			})
		}
	}
}

func TestBaseGemChangesPackedGemOnPlusTenThroughFifteen(t *testing.T) {
	for level := 10; level <= 15; level++ {
		t.Run(fmt.Sprintf("plus%d", level), func(t *testing.T) {
			d, w, s, e := baseGemFixture(nil)
			target := world.Item{Index: itemPlusEleven, Effects: [3]world.Effect{{Effect: efSanc}}}
			refine.Set(&target, level, 0)
			e.Carry[0] = world.Item{Index: itemGemaDiamante}
			e.Equip[1] = target

			d.useBaseGem(w, s, e, baseGemUseBody(world.ItemPlaceEquip, 1), 0, volGemCoral)

			if got := refine.Level(e.Equip[1]); got != level {
				t.Errorf("level = %d, want %d", got, level)
			}
			if got := refine.Gem(e.Equip[1]); got != 2 {
				t.Errorf("gem = %d, want 2 (Coral)", got)
			}
			if e.Equip[1].Index != itemPlusEleven {
				t.Errorf("index = %d, want unchanged %d", e.Equip[1].Index, itemPlusEleven)
			}
			if !e.Carry[0].Empty() {
				t.Errorf("source gem not consumed: %+v", e.Carry[0])
			}
		})
	}
}

func TestBaseGemRefusesNonAnctBelowPlusTen(t *testing.T) {
	d, w, s, e := baseGemFixture(map[int]int{itemAnctBase: 4})
	e.Carry[0] = world.Item{Index: itemGemaDiamante}
	e.Equip[1] = world.Item{Index: itemAnctBase}

	d.useBaseGem(w, s, e, baseGemUseBody(world.ItemPlaceEquip, 1), 0, volGemGarnet)

	if e.Carry[0].Empty() {
		t.Fatal("refused gem use consumed the source")
	}
	if e.Equip[1].Index != itemAnctBase {
		t.Errorf("target index = %d, want unchanged %d", e.Equip[1].Index, itemAnctBase)
	}
}

func TestBaseGemRejectsInvalidDestination(t *testing.T) {
	d, w, s, e := baseGemFixture(map[int]int{itemAnctBase: 5})
	e.Carry[0] = world.Item{Index: itemGemaDiamante}
	e.Carry[1] = world.Item{Index: itemAnctBase}

	d.useBaseGem(w, s, e, baseGemUseBody(world.ItemPlaceCarry, 1), 0, volGemGarnet)

	if e.Carry[0].Empty() {
		t.Fatal("invalid destination consumed the source")
	}
	if e.Carry[1].Index != itemAnctBase {
		t.Errorf("carry target index = %d, want unchanged %d", e.Carry[1].Index, itemAnctBase)
	}
}

func TestBaseGemOverTheWire(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: itemGemaGarnet}
	st.Equip[1] = world.Item{Index: itemAnctBase}
	db.loadResult = st

	addr, stop := startServerClockBaseGem(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := baseGemUseBody(world.ItemPlaceEquip, 1)
	send(t, c, protocol.MsgUseItem, body.Encode())

	for i := 0; i < 8; i++ {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			t.Fatal("connection went quiet before destination item update")
		}
		if ty != protocol.MsgSendItem {
			continue
		}
		if int(le16(payload[0:2])) != world.ItemPlaceEquip || int(le16(payload[2:4])) != 1 {
			continue
		}
		if got := le16(payload[4:6]); got != itemAnctBase+3 {
			t.Fatalf("destination index = %d, want Garnet variant %d", got, itemAnctBase+3)
		}
		return
	}
	t.Fatal("no destination MsgSendItem after base gem use")
}

func baseGemFixture(grades map[int]int) (*Dispatcher, *world.World, *world.Session, *world.Entity) {
	log := slog.New(slog.DiscardHandler)
	d := New(Config{Log: log, ItemGrades: grades})
	w := world.New(world.Config{GridDim: 16}, log, nil, nil)
	return d, w, &world.Session{Conn: 1, Mode: world.UserPlay}, &world.Entity{ID: 1, HP: 100}
}

func baseGemUseBody(destType, destPos int) protocol.MsgUseItemBody {
	return protocol.MsgUseItemBody{
		SourType: world.ItemPlaceCarry,
		SourPos:  0,
		DestType: int32(destType),
		DestPos:  int32(destPos),
	}
}

func startServerClockBaseGem(t *testing.T, persist world.Persistence) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{
		Log:           log,
		ItemVolatiles: map[int]int{itemGemaGarnet: volGemGarnet},
		ItemGrades:    map[int]int{itemAnctBase: 5},
	})
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
	}
}
