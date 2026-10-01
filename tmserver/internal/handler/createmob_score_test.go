package handler

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// readAny is read without the visibility filter: the self-CreateMob is the
// packet under test here.
func readAny(t *testing.T, c net.Conn) (protocol.Type, []byte) {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	var sz [2]byte
	if _, err := io.ReadFull(c, sz[:]); err != nil {
		t.Fatalf("read size: %v", err)
	}
	buf := make([]byte, binary.LittleEndian.Uint16(sz[:]))
	copy(buf, sz[:])
	if _, err := io.ReadFull(c, buf[2:]); err != nil {
		t.Fatalf("read body: %v", err)
	}
	h, payload, _, err := protocol.Decode(buf)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return h.Type, payload
}

// A player's CreateMob carries the same score as its UpdateScore, as the legacy
// copies MOB.CurrentScore (GetCreateMob, GetFunc.cpp:1111). The own client
// applies the self-CreateMob after the login UpdateScore, so a flat Damage there
// (without the weapon's EF_DAMAGE) showed a weaponless score until the next push.
func TestEnterWorldSelfCreateMobCarriesTheScore(t *testing.T) {
	rel := filepath.Join("..", "..", "..", "Release")
	items, err := content.LoadItemList(filepath.Join(rel, "Common", "ItemList.csv"))
	if err != nil {
		t.Fatalf("load ItemList: %v", err)
	}
	baseMobs, err := content.LoadBaseMobs(rel)
	if err != nil {
		t.Fatalf("load BaseMobs: %v", err)
	}
	tk := protocol.ParseMobBasics(baseMobs[0])
	db := newDB()
	// A fresh Transknight: the class starter gear includes the Adaga (861,
	// EF_DAMAGE 2) in the right hand.
	db.loadResult = world.CharacterState{Slot: 0, Name: "Hero", Class: 0, Level: 1,
		HP: 105, MaxHP: 100, MP: 105, MaxMP: 100, Str: tk.Str, Int: tk.Int, Dex: tk.Dex, Con: tk.Con}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, BaseMobs: baseMobs, ItemPrices: items.Prices(), ItemEffects: items.BaseEffects(),
		ItemReqs: items.Requirements(), ItemPos: items.Positions(), ItemUnique: items.Uniques()})
	w := world.New(world.Config{}, log, db, d.Handle)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	defer func() { cancel(); <-done }()
	c := loginAndSelect(t, ln.Addr().String())
	defer c.Close()

	var body protocol.MsgCharacterLoginBody
	send(t, c, protocol.MsgCharacterLogin, body.Encode())
	le := binary.LittleEndian
	var score, self []byte
	for i := 0; i < 20 && (score == nil || self == nil); i++ {
		ty, p := readAny(t, c)
		switch {
		case ty == protocol.MsgCNFCharacterLogin:
			if got := le.Uint16(p[4+140+6*8:]); got != 861 {
				t.Fatalf("right-hand weapon = %d, want the starter Adaga 861", got)
			}
		case ty == protocol.MsgUpdateScore && score == nil:
			score = p
		case ty == protocol.MsgCreateMob && score != nil && self == nil:
			self = p[124:] // CreateMob Score @body124
		}
	}
	if score == nil || self == nil {
		t.Fatalf("login stream: UpdateScore %v, self CreateMob after it %v", score != nil, self != nil)
	}
	for _, f := range []struct {
		name string
		at   int
	}{{"Level", 0}, {"Ac", 4}, {"Damage", 8}} {
		if got, want := int32(le.Uint32(self[f.at:])), int32(le.Uint32(score[f.at:])); got != want {
			t.Errorf("self CreateMob %s = %d, UpdateScore says %d", f.name, got, want)
		}
	}
	for i, name := range []string{"Str", "Int", "Dex", "Con"} {
		if got, want := le.Uint16(self[32+i*2:]), le.Uint16(score[32+i*2:]); got != want {
			t.Errorf("self CreateMob %s = %d, UpdateScore says %d", name, got, want)
		}
	}
}
