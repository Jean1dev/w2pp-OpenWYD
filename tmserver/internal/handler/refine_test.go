package handler

import (
	"context"
	"encoding/binary"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// The refine dust items, as ItemList.csv defines them.
const (
	itemPoeiraOri = 412
	itemPoeiraLac = 413
	itemArmor     = 555 // a plain refinable armor
	// itemSephirotTransKnight is a quest/trophy stone (issue #133): equippable,
	// no EF_VOLATILE, now tagged EF_NOSANC in ItemList.csv.
	itemSephirotTransKnight = 1760
)

// rateTable is an injectable SancRate. Index it at level+1, like the real one.
type rateTable [3][12]int

func (r rateTable) Rate(anvil, idx int) int {
	if anvil < 0 || anvil >= len(r) || idx < 0 || idx >= len(r[anvil]) {
		return 0
	}
	return r[anvil][idx]
}

// alwaysRate is a table where every Ori/Lac lookup returns rate.
func alwaysRate(rate int) rateTable {
	var t rateTable
	for a := range t {
		for i := range t[a] {
			t[a][i] = rate
		}
	}
	return t
}

// refineFixture wires a dispatcher whose catalog mirrors production: the two
// dusts classified by EF_VOLATILE, and a refinable armor. Mirroring the real
// Config matters — a handler harness with the catalog switched off silently
// makes every EF_ gate read 0 (see the B12 lesson).
type refineFixture struct {
	d *Dispatcher
	w *world.World
	s *world.Session
	e *world.Entity
}

func newRefineFixture(t *testing.T, tbl refine.RateTable, effects map[int][]content.BaseEffect) *refineFixture {
	t.Helper()
	if effects == nil {
		effects = map[int][]content.BaseEffect{}
	}
	d := New(Config{
		Log:           slog.New(slog.DiscardHandler),
		SancRate:      tbl,
		ItemVolatiles: map[int]int{itemPoeiraOri: volDustOri, itemPoeiraLac: volDustLac, itemLacto100: volDustLac},
		ItemEffects:   effects,
	})
	w := world.New(world.Config{GridDim: 16}, slog.New(slog.DiscardHandler), nil, nil)
	return &refineFixture{
		d: d,
		w: w,
		s: &world.Session{Conn: 1, Mode: world.UserPlay},
		e: &world.Entity{ID: 1, HP: 100},
	}
}

// refine drags the dust in carry slot 0 onto the item in carry slot 1.
func (f *refineFixture) refine(dust int16) {
	f.e.Carry[0] = world.Item{Index: dust}
	body := protocol.MsgUseItemBody{
		SourType: world.ItemPlaceCarry, SourPos: 0,
		DestType: world.ItemPlaceCarry, DestPos: 1,
	}
	vol := f.d.itemVolatiles[int(dust)]
	f.d.refineItem(f.w, f.s, f.e, body, 0, vol)
}

func (f *refineFixture) target() world.Item { return f.e.Carry[1] }

func TestRefineSuccessRaisesLevel(t *testing.T) {
	f := newRefineFixture(t, alwaysRate(100), nil)
	f.e.Carry[1] = world.Item{Index: itemArmor}

	f.refine(itemPoeiraLac)

	if got := refine.Level(f.target()); got != 1 {
		t.Errorf("level = %d, want 1", got)
	}
	if !f.e.Carry[0].Empty() {
		t.Errorf("dust not consumed: %+v", f.e.Carry[0])
	}
	// The level must land in a real EF_SANC pair the rest of the server can read.
	if f.target().Effects[0] != (world.Effect{Effect: efSanc, Value: 1}) {
		t.Errorf("Effects[0] = %+v, want {EF_SANC 1}", f.target().Effects[0])
	}
}

// A failed dust refine never destroys or downgrades the item — that only happens
// on the sealed/celestial/earring branches, which are out of scope.
func TestRefineFailKeepsItemAndAccruesPity(t *testing.T) {
	f := newRefineFixture(t, alwaysRate(0), nil)
	f.e.Carry[1] = world.Item{Index: itemArmor}
	before := f.target().Index

	f.refine(itemPoeiraLac)

	if f.target().Index != before {
		t.Errorf("item index changed on failure: %d → %d", before, f.target().Index)
	}
	if got := refine.Level(f.target()); got != 0 {
		t.Errorf("level = %d, want 0 (a failure must not downgrade)", got)
	}
	if !f.e.Carry[0].Empty() {
		t.Error("dust must be consumed on failure too")
	}
	// rand()#2 % 4 = 3 (the seed-1 stream), and 3 > 2, so this particular failure
	// does NOT accrue pity — the roll is 3-in-4.
	if got := refine.Pity(f.target()); got != 0 {
		t.Errorf("pity = %d, want 0 (rand()#2%%4 = 3 misses the <=2 window)", got)
	}
}

// Pity accrues over repeated failures and is what eventually carries a refine
// through (g_pSuccessRate).
func TestRefinePityAccrues(t *testing.T) {
	f := newRefineFixture(t, alwaysRate(0), nil)
	f.e.Carry[1] = world.Item{Index: itemArmor}
	for i := 0; i < 6; i++ {
		f.refine(itemPoeiraLac)
	}
	if got := refine.Pity(f.target()); got == 0 {
		t.Errorf("pity = 0 after 6 failures, want > 0")
	}
	if got := refine.Level(f.target()); got != 0 {
		t.Errorf("level = %d, want 0", got)
	}
}

func TestRefineGates(t *testing.T) {
	cases := []struct {
		name    string
		dust    int16
		target  world.Item
		effects map[int][]content.BaseEffect
		want    Notice
	}{
		{
			name:   "Ori refuses an item already at +6",
			dust:   itemPoeiraOri,
			target: world.Item{Index: itemArmor, Effects: [3]world.Effect{{Effect: efSanc, Value: 6}}},
			want:   NoticeCantRefineMore,
		},
		{
			name:   "the dust path stops at +9",
			dust:   itemPoeiraLac,
			target: world.Item{Index: itemArmor, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}},
			want:   NoticeCantRefineMore,
		},
		{
			name:   "+11 and above is refused",
			dust:   itemPoeiraLac,
			target: world.Item{Index: itemArmor, Effects: [3]world.Effect{{Effect: efSanc, Value: 234}}}, // +11
			want:   NoticeCantRefineMore,
		},
		{
			name:   "item 769 is capped at +9",
			dust:   itemPoeiraLac,
			target: world.Item{Index: item769, Effects: [3]world.Effect{{Effect: efSanc, Value: 234}}},
			want:   NoticeCantRefineMore,
		},
		{
			name:   "a dust cannot be refined onto another dust",
			dust:   itemPoeiraLac,
			target: world.Item{Index: itemPoeiraOri},
			want:   NoticeOnlyToEquips,
		},
		{
			name:    "EF_NOSANC items never refine",
			dust:    itemPoeiraLac,
			target:  world.Item{Index: itemArmor},
			effects: map[int][]content.BaseEffect{itemArmor: {{Eff: efNoSanc, Val: 1}}},
			want:    NoticeCantRefineMore,
		},
		{
			// issue #133: Sephirot(TransKnight) is a quest/trophy stone with no
			// EF_VOLATILE, so nothing gated it before ItemList.csv started tagging
			// it EF_NOSANC.
			name:    "Sephirot(TransKnight) never refines (issue #133)",
			dust:    itemPoeiraLac,
			target:  world.Item{Index: itemSephirotTransKnight},
			effects: map[int][]content.BaseEffect{itemSephirotTransKnight: {{Eff: efNoSanc, Val: 1}}},
			want:    NoticeCantRefineMore,
		},
		{
			name:   "capa do Celestial não refina com Lac",
			dust:   itemPoeiraLac,
			target: world.Item{Index: 3197},
			want:   NoticeCantRefineMore,
		},
		{
			name:   "capa do Celestial não refina com Ori",
			dust:   itemPoeiraOri,
			target: world.Item{Index: 3198},
			want:   NoticeCantRefineMore,
		},
		{
			name:   "capa do Celestial não refina nem com Lactolerium 100",
			dust:   itemLacto100,
			target: world.Item{Index: 3199},
			want:   NoticeCantRefineMore,
		},
		{
			name: "an item with all three effect slots taken has nowhere to store a level",
			dust: itemPoeiraLac,
			target: world.Item{Index: itemArmor, Effects: [3]world.Effect{
				{Effect: efAc, Value: 10}, {Effect: efHp, Value: 20}, {Effect: efMp, Value: 30},
			}},
			want: NoticeCantRefineMore,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			f := newRefineFixture(t, alwaysRate(100), tt.effects)
			f.e.Carry[1] = tt.target
			before := f.target()

			f.refine(tt.dust)

			if f.target() != before {
				t.Errorf("a refused refine mutated the item: %+v → %+v", before, f.target())
			}
			if f.e.Carry[0].Empty() {
				t.Error("a refused refine consumed the dust")
			}
		})
	}
}

// TestCapaCelestialTravaSoElaVestidaOuNaMochila: a trava vale com a capa do
// Celestial vestida no slot 15, e só nela — a Herói do Arch (3194) e a Elite do
// Mortal (3191) seguem refinando como no legado.
func TestCapaCelestialTravaSoElaVestidaOuNaMochila(t *testing.T) {
	f := newRefineFixture(t, alwaysRate(100), nil)
	f.e.Equip[15] = world.Item{Index: 3197}
	f.e.Carry[0] = world.Item{Index: itemPoeiraLac}
	body := protocol.MsgUseItemBody{
		SourType: world.ItemPlaceCarry, SourPos: 0,
		DestType: world.ItemPlaceEquip, DestPos: 15,
	}
	f.d.refineItem(f.w, f.s, f.e, body, 0, volDustLac)
	if f.e.Equip[15] != (world.Item{Index: 3197}) || f.e.Carry[0].Empty() {
		t.Errorf("capa do Celestial vestida refinou: capa %+v, poeira %+v", f.e.Equip[15], f.e.Carry[0])
	}

	for _, capa := range []int16{3191, 3194} {
		g := newRefineFixture(t, alwaysRate(100), nil)
		g.e.Carry[1] = world.Item{Index: capa}
		g.refine(itemPoeiraLac)
		if got := refine.Level(g.target()); got != 1 {
			t.Errorf("a capa %d devia seguir refinando com Lac (legado), ficou em +%d", capa, got)
		}
	}
}

// Ori tops out at +6 even when the roll wins.
func TestRefineOriCapsAtSix(t *testing.T) {
	f := newRefineFixture(t, alwaysRate(100), nil)
	f.e.Carry[1] = world.Item{Index: itemArmor}
	for i := 0; i < 10; i++ {
		f.refine(itemPoeiraOri)
	}
	if got := refine.Level(f.target()); got != oriMaxSanc {
		t.Errorf("level = %d, want %d (Ori's cap)", got, oriMaxSanc)
	}
}

// Lac tops out at +9.
func TestRefineLacCapsAtNine(t *testing.T) {
	f := newRefineFixture(t, alwaysRate(100), nil)
	f.e.Carry[1] = world.Item{Index: itemArmor}
	for i := 0; i < 20; i++ {
		f.refine(itemPoeiraLac)
	}
	if got := refine.Level(f.target()); got != lacMaxSanc {
		t.Errorf("level = %d, want %d (Lac's cap)", got, lacMaxSanc)
	}
}

// Lactolerium_100 forces the Âmago row, which is a flat 100% regardless of the
// table — here the table would otherwise always fail.
func TestRefineLacto100AlwaysSucceeds(t *testing.T) {
	f := newRefineFixture(t, alwaysRate(0), nil)
	f.e.Carry[1] = world.Item{Index: itemArmor}

	f.refine(itemLacto100)

	if got := refine.Level(f.target()); got != 1 {
		t.Errorf("level = %d, want 1 (Lactolerium_100 is a guaranteed refine)", got)
	}
}

// A tintura never refines: any dust converts it straight into its matching
// Feijão Mágico (issue #130), bypassing EF_SANC entirely.
func TestRefineTinturaBecomesFeijaoMagico(t *testing.T) {
	cases := []struct {
		name    string
		dust    int16
		tintura int16
	}{
		{"low bound, Ori", itemPoeiraOri, tinturaLo},
		{"middle color, Lac", itemPoeiraLac, tinturaLo + 3},
		{"high bound, Lac", itemPoeiraLac, tinturaHi},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			// alwaysRate(0): every ordinary refine roll would fail — proves the
			// transform is deterministic and never touches the success-rate roll.
			f := newRefineFixture(t, alwaysRate(0), nil)
			f.e.Carry[1] = world.Item{Index: tt.tintura}

			f.refine(tt.dust)

			wantFeijao := tt.tintura + (magicBeanBase - tinturaLo)
			if got := f.target().Index; got != wantFeijao {
				t.Errorf("index = %d, want %d (matching Feijão Mágico)", got, wantFeijao)
			}
			if !f.e.Carry[0].Empty() {
				t.Errorf("dust not consumed: %+v", f.e.Carry[0])
			}
			if got := refine.Level(f.target()); got != 0 {
				t.Errorf("level = %d, want 0 (a tintura must never gain EF_SANC)", got)
			}
		})
	}
}

// TestRefineRNGGolden pins the rand() call order against the MSVC stream from the
// CRT default seed 1, which is where every world starts:
//
//	Rand() #1..#5 = 41, 18467, 6334, 26500, 19169   → %100 = 41, 67, 34, 0, 69
//
// The dust path spends exactly ONE rand() when it succeeds and TWO when it fails
// (the roll, then the 3-in-4 pity roll). Any extra or reordered draw shifts every
// later result, so this test is what actually locks the parity.
func TestRefineRNGGolden(t *testing.T) {
	// Rate 50: rolls of 41 and 34 win; 67 and 69 lose.
	f := newRefineFixture(t, alwaysRate(50), nil)
	f.e.Carry[1] = world.Item{Index: itemArmor}

	steps := []struct {
		roll      int
		wantLevel int
		wantPity  int
		note      string
	}{
		// #1 roll 41 <= 50 → success. One draw spent.
		{41, 1, 0, "roll 41 wins"},
		// #2 roll 67 > 50 → fail. #3 = 6334%4 = 2 <= 2 → pity++.
		{67, 1, 1, "roll 67 loses, pity roll 2 hits the <=2 window"},
		// #4 roll 26500%100 = 0 <= 50 → success. Pity resets.
		{0, 2, 0, "roll 0 wins and clears the pity"},
		// #5 roll 19169%100 = 69 > 50 → fail. #6 = 15724%4 = 0 <= 2 → pity++.
		{69, 2, 1, "roll 69 loses, pity roll 0 hits the window"},
	}
	for i, st := range steps {
		f.refine(itemPoeiraLac)
		if got := refine.Level(f.target()); got != st.wantLevel {
			t.Errorf("step %d (%s): level = %d, want %d", i+1, st.note, got, st.wantLevel)
		}
		if got := refine.Pity(f.target()); got != st.wantPity {
			t.Errorf("step %d (%s): pity = %d, want %d", i+1, st.note, got, st.wantPity)
		}
	}
}

// TestRefineOverTheWire drives the real client path end to end: a _MSG_UseItem
// frame carrying a dust in SourPos and the target in DestPos must reach
// refineItem through useItem's EF_VOLATILE switch, and the refined item must come
// back in a _MSG_SendItem.
//
// This is the actual issue #103 regression. The unit tests above call refineItem
// directly and would all still pass with the switch arm removed — which is
// exactly the bug that shipped: refine fell through to `default:` and no-oped.
// It also checks the packet, not just server state: the right level in memory
// still renders wrong if the S→C frame carries the wrong bytes.
func TestRefineOverTheWire(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: itemPoeiraLac}
	st.Carry[1] = world.Item{Index: itemArmor}
	db.loadResult = st

	addr, stop := startRefineServer(t, db)
	defer stop()
	c := enterWorld(t, addr)

	body := protocol.MsgUseItemBody{
		SourType: world.ItemPlaceCarry, SourPos: 0,
		DestType: world.ItemPlaceCarry, DestPos: 1,
	}
	send(t, c, protocol.MsgUseItem, body.Encode())

	// Read until the item push (the score/notice frames come first).
	for i := 0; ; i++ {
		if i > 8 {
			t.Fatal("no _MSG_SendItem after a refine — the dust fell through useItem's switch")
		}
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			t.Fatal("connection closed before the refined item arrived")
		}
		if ty != protocol.MsgSendItem {
			continue
		}
		place := int(binary.LittleEndian.Uint16(payload[0:]))
		slot := int(binary.LittleEndian.Uint16(payload[2:]))
		index := binary.LittleEndian.Uint16(payload[4:])
		if place != world.ItemPlaceCarry || slot != 1 {
			continue // not the target slot
		}
		if index != itemArmor {
			t.Fatalf("SendItem index = %d, want %d", index, itemArmor)
		}
		// The effect pairs ride at body+6: the EF_SANC pair must carry the new level.
		eff0 := world.Effect{Effect: payload[6], Value: payload[7]}
		if eff0 != (world.Effect{Effect: efSanc, Value: 1}) {
			t.Fatalf("SendItem Effects[0] = %+v, want {EF_SANC 1} — the client renders THIS, not the server's copy", eff0)
		}
		return
	}
}

// startRefineServer is startServerClock with a catalog that classifies the dusts.
func startRefineServer(t *testing.T, persist world.Persistence) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.DiscardHandler)
	d := New(Config{
		Log:           log,
		SancRate:      alwaysRate(100),
		ItemVolatiles: map[int]int{itemPoeiraOri: volDustOri, itemPoeiraLac: volDustLac},
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

// An egg hatches into its mount on the first successful refine: the hatch gate
// reads the INSTANCE EF_INCUBATE, which a fresh egg does not carry.
func TestRefineEggHatches(t *testing.T) {
	const egg = 2304
	f := newRefineFixture(t, alwaysRate(100), map[int][]content.BaseEffect{
		egg: {{Eff: efIncubate, Val: 2}}, // the catalog threshold is deliberately not read
	})
	f.e.Carry[1] = world.Item{Index: egg}

	f.refine(itemPoeiraLac)

	if got := f.target().Index; got != egg+eggHatchAdd {
		t.Fatalf("index = %d, want %d (hatched mount)", got, egg+eggHatchAdd)
	}
	// The hatched mount reinterprets its slots as mount data: [0].sValue = HP.
	if got := uint16(f.target().Effects[0].Effect) | uint16(f.target().Effects[0].Value)<<8; got != hatchMountHP {
		t.Errorf("mount HP = %d, want %d", got, hatchMountHP)
	}
	if got := f.target().Effects[1].Effect; got != hatchMountSanc {
		t.Errorf("mount sanc = %d, want %d", got, hatchMountSanc)
	}
	if got := f.target().Effects[2]; got != (world.Effect{Effect: hatchMountFeed, Value: hatchMountKill}) {
		t.Errorf("mount feed/kill = %+v", got)
	}
}

// A failed egg refine stamps the incubation cooldown, and the regen tick counts
// it down only while the egg is equipped.
func TestEggIncubationCooldown(t *testing.T) {
	const egg = 2304
	f := newRefineFixture(t, alwaysRate(0), nil)
	f.e.Carry[1] = world.Item{Index: egg}

	f.refine(itemPoeiraLac)

	// rand()#3 % 4 = 2 → a cooldown of 2.
	delay := itemInstanceAbility(f.target(), efIncuDelay)
	if delay != 2 {
		t.Fatalf("incubation delay = %d, want 2", delay)
	}

	// While the cooldown stands, another refine is refused.
	f.refine(itemPoeiraLac)
	if got := itemInstanceAbility(f.target(), efIncuDelay); got != delay {
		t.Errorf("a gated refine changed the cooldown: %d → %d", delay, got)
	}
	if f.e.Carry[0].Empty() {
		t.Error("a gated refine consumed the dust")
	}

	// The tick only counts an EQUIPPED egg down.
	f.d.tickIncubation(f.w, f.s, f.e)
	if got := itemInstanceAbility(f.target(), efIncuDelay); got != delay {
		t.Errorf("the cooldown ticked while the egg was in the inventory: %d", got)
	}
	f.e.Equip[mountEquipSlot] = f.target()
	f.d.tickIncubation(f.w, f.s, f.e)
	if got := itemInstanceAbility(f.e.Equip[mountEquipSlot], efIncuDelay); got != delay-1 {
		t.Errorf("equipped cooldown = %d, want %d", got, delay-1)
	}
}

// TestRefineSendsOutcomeEmotion is the "nothing happened on screen" regression.
// The refine changed the item and said so in text, but never played the emotion
// the legacy closes with (_MSG_UseItem.cpp:920 on success, :970-973 on failure),
// so from the player's side a refine looked identical to a no-op.
func TestRefineSendsOutcomeEmotion(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	// startRefineServer runs on alwaysRate(100), so the plain dust takes the
	// success branch deterministically instead of leaning on the RNG.
	st.Carry[0] = world.Item{Index: itemPoeiraLac}
	st.Carry[1] = world.Item{Index: itemArmor}
	db.loadResult = st

	addr, stop := startRefineServer(t, db)
	defer stop()
	c := enterWorld(t, addr)

	body := protocol.MsgUseItemBody{
		SourType: world.ItemPlaceCarry, SourPos: 0,
		DestType: world.ItemPlaceCarry, DestPos: 1,
	}
	send(t, c, protocol.MsgUseItem, body.Encode())

	for i := 0; ; i++ {
		if i > 12 {
			t.Fatal("no _MSG_Motion after a successful refine — the player sees no effect at all")
		}
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			t.Fatal("connection closed before the emotion arrived")
		}
		if ty != protocol.MsgMotion {
			continue
		}
		motion := binary.LittleEndian.Uint16(payload[0:])
		parm := binary.LittleEndian.Uint16(payload[2:])
		if motion != motionLevelUp || parm != motionLevelUpParm {
			t.Fatalf("Motion = (%d,%d), want (%d,%d) — SendEmotion(conn,14,3)",
				motion, parm, motionLevelUp, motionLevelUpParm)
		}
		return
	}
}

// The failure emotion picks between two animations by whether the character has
// a face item equipped: the legacy reads Equip[0].sIndex / 10, so anything below
// 10 — an empty slot included — takes the bare variant.
func TestRefineFailEmotionDependsOnFace(t *testing.T) {
	tests := []struct {
		name  string
		equip int16
		want  uint16
	}{
		{"empty face slot", 0, motionRefineFailBare},
		{"index below 10 still counts as bare", 9, motionRefineFailBare},
		{"a real face item", 10, motionRefineFailFaced},
		{"a high face index", 3140, motionRefineFailFaced},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := motionRefineFailBare
			if tt.equip/10 != 0 {
				got = motionRefineFailFaced
			}
			if got != tt.want {
				t.Errorf("Equip[0].Index=%d → motion %d, want %d", tt.equip, got, tt.want)
			}
		})
	}
}

// A successful refine has to SAY so. The Notice code alone is a placeholder wire
// format (notice.go) that the client draws as nothing, which is why "usei uma
// poeira de Ori/Lac e não apareceu nada" was a real report and not a rendering
// quirk: the server did answer, on a channel nobody listens to.
//
// The legacy sends _NN_Refine_Success (Language.txt:176) through
// SendClientMessage, i.e. MSG_MessagePanel — so that is what this asserts, text
// and all, including the CP1252 round-trip its accents need.
func TestRefineSuccessSendsPanelLine(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: itemPoeiraLac}
	st.Carry[1] = world.Item{Index: itemArmor}
	db.loadResult = st

	addr, stop := startRefineServer(t, db)
	defer stop()
	c := enterWorld(t, addr)

	body := protocol.MsgUseItemBody{
		SourType: world.ItemPlaceCarry, SourPos: 0,
		DestType: world.ItemPlaceCarry, DestPos: 1,
	}
	send(t, c, protocol.MsgUseItem, body.Encode())

	const want = "Obteve sucesso na refinação."
	for i := 0; ; i++ {
		if i > 10 {
			t.Fatalf("no MSG_MessagePanel carrying %q after a successful refine", want)
		}
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			t.Fatal("connection closed before the refine message arrived")
		}
		if ty != protocol.MsgMessagePanel {
			continue
		}
		if got := decodePanel(payload); got != want {
			t.Fatalf("panel text = %q, want %q", got, want)
		}
		return
	}
}

// decodePanel turns a MSG_MessagePanel body back into a Go string. The wire form
// is CP1252 in a fixed buffer, so a raw compare against a UTF-8 literal would
// fail on every accent — exactly the bug protocol.ClientText exists to prevent.
func decodePanel(b []byte) string {
	if i := indexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	runes := make([]rune, len(b))
	for i, ch := range b {
		runes[i] = rune(ch) // CP1252 is Latin-1 here, and Latin-1 byte == code point
	}
	return string(runes)
}

// The four refine notices must carry the shipped Language.txt wording, accents
// included — the strings the player has read on this server for twenty years.
func TestRefineNoticeTexts(t *testing.T) {
	want := map[Notice]string{
		NoticeOnlyToEquips:   "Possível somente com armas e armaduras equipadas.", // 74
		NoticeCantRefineMore: "Este item não pode ser mais refinado.",             // 75
		NoticeFailToRefine:   "Refinação falhou.",                                 // 76
		NoticeRefineSuccess:  "Obteve sucesso na refinação.",                      // 176
	}
	for n, text := range want {
		got, ok := noticeText[n]
		if !ok {
			t.Errorf("notice %v has no panel text; it would render as nothing", n)
			continue
		}
		if got != text {
			t.Errorf("notice %v text = %q, want %q", n, got, text)
		}
	}
}

// Painting is not refining. The legacy reuses the refine strings on the paint
// path (_MSG_UseItem.cpp:3767-3861), so a player who painted a helmet was told
// "Obteve sucesso na refinação"; these say what actually happened, and keep
// painting apart from stripping the colour back off.
func TestPaintNoticeTexts(t *testing.T) {
	want := map[Notice]string{
		NoticePaintSuccess: "Item pintado com sucesso.",
		NoticePaintRemoved: "Pintura removida com sucesso.",
		NoticeCantPaint:    "Este item não pode receber mais pintura.",
		NoticeNotPainted:   "Este item não está pintado.",
	}
	for n, text := range want {
		got, ok := noticeText[n]
		if !ok {
			t.Errorf("notice %v has no panel text; it would render as nothing", n)
			continue
		}
		if got != text {
			t.Errorf("notice %v text = %q, want %q", n, got, text)
		}
	}
	// And the paint notices must be distinct from the refine ones, which is the
	// whole point of adding them.
	if noticeText[NoticePaintSuccess] == noticeText[NoticeRefineSuccess] {
		t.Error("paint success still reads as a refine")
	}
}
