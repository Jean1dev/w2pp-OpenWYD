package handler

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/npccfg"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// merchantTemplate builds a minimal 816-byte STRUCT_MOB with CurrentScore.Merchant
// set (the shop flag the tmServer honours, offset 104) and non-zero HP so
// SpawnMobAt keeps it alive.
func merchantTemplate(name string) []byte {
	tmpl := make([]byte, 816)
	copy(tmpl[0:16], name)
	tmpl[92+12] = 1                                  // CurrentScore.Merchant = normal shop
	binary.LittleEndian.PutUint32(tmpl[92+16:], 100) // MaxHp
	binary.LittleEndian.PutUint32(tmpl[92+24:], 100) // Hp
	return tmpl
}

// staticSource is a fixed npccfg.Source for driving applyNPCConfig directly.
type staticSource struct{ snap npccfg.Snapshot }

func (s staticSource) Version(context.Context) (int64, error)            { return s.snap.Version, nil }
func (s staticSource) Snapshot(context.Context) (npccfg.Snapshot, error) { return s.snap, nil }

func newNPCDispatcher(base map[int]int32) (*Dispatcher, *world.World) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, ItemPrices: base, NpcConfig: staticSource{}})
	w := world.New(world.Config{GridDim: 32}, log, world.NopPersistence{}, d.Handle)
	return d, w
}

// TestApplyNPCConfigSpawnsMerchant checks a boot apply materializes an enabled
// merchant definition: the entity exists at its position, is a merchant, carries
// the moderator shop stock, and the global price override lands.
func TestApplyNPCConfigSpawnsMerchant(t *testing.T) {
	d, w := newNPCDispatcher(map[int]int32{1100: 999})
	snap := npccfg.Snapshot{
		Version: 1,
		Defs: []npccfg.Definition{{
			Slug: "shop-1", Template: merchantTemplate("Keeper"), DisplayName: "Ferreiro", Enabled: true,
			X: 8, Y: 8, Merchant: 1,
			// Slots 0/2 are tab 1 (identity); slot 9 is tab 2 → Carry[27]; slot 18
			// is tab 3 → Carry[54]. Proves the ShopSlot mapping is applied.
			Shop: []npccfg.ShopItem{
				{Slot: 0, Index: 1100, Quantity: 120}, {Slot: 2, Index: 1101},
				{Slot: 9, Index: 1102}, {Slot: 18, Index: 1103},
			},
		}},
		PriceOverrides: map[int]int32{1100: 500},
	}
	d.applyNPCConfig(w, snap, false)

	id, ok := d.managedNPCs["shop-1"]
	if !ok {
		t.Fatal("managed NPC not registered")
	}
	e := w.Entity(id)
	if e == nil {
		t.Fatal("managed NPC entity missing")
	}
	if e.Merchant == 0 {
		t.Errorf("entity Merchant = 0, want merchant")
	}
	if e.X != 8 || e.Y != 8 {
		t.Errorf("entity at (%d,%d), want (8,8)", e.X, e.Y)
	}
	if e.Name != "Ferreiro" {
		t.Errorf("entity Name = %q, want Ferreiro", e.Name)
	}
	if e.Carry[0].Index != 1100 || e.Carry[2].Index != 1101 {
		t.Errorf("shop stock = [0]=%d [2]=%d, want 1100/1101", e.Carry[0].Index, e.Carry[2].Index)
	}
	if e.Carry[0].Effects[0].Effect != 61 || e.Carry[0].Effects[0].Value != 120 {
		t.Errorf("stack effect = %+v, want EF_AMOUNT 120", e.Carry[0].Effects[0])
	}
	// Tabs 2/3 map through ShopSlot: slot 9 → Carry[27], slot 18 → Carry[54].
	if e.Carry[27].Index != 1102 {
		t.Errorf("tab-2 slot 9 landed at Carry[27]=%d, want 1102", e.Carry[27].Index)
	}
	if e.Carry[54].Index != 1103 {
		t.Errorf("tab-3 slot 18 landed at Carry[54]=%d, want 1103", e.Carry[54].Index)
	}
	if e.Carry[9].Index != 0 || e.Carry[18].Index != 0 {
		t.Errorf("gap slots written: Carry[9]=%d Carry[18]=%d, want empty", e.Carry[9].Index, e.Carry[18].Index)
	}
	if e.Carry[1].Index != 0 {
		t.Errorf("slot 1 = %d, want empty", e.Carry[1].Index)
	}
	if d.itemPrices[1100] != 500 {
		t.Errorf("price[1100] = %d, want overridden 500", d.itemPrices[1100])
	}
	if d.npcVersion != 1 {
		t.Errorf("npcVersion = %d, want 1", d.npcVersion)
	}
}

func TestApplyNPCConfigDisplayNameFallbackAndSharedTemplate(t *testing.T) {
	d, w := newNPCDispatcher(nil)
	tmpl := merchantTemplate("Default")
	d.applyNPCConfig(w, npccfg.Snapshot{
		Version: 1,
		Defs: []npccfg.Definition{
			{Slug: "named-a", Template: tmpl, DisplayName: "Armeiro", Enabled: true, X: 7, Y: 7, Merchant: 1},
			{Slug: "named-b", Template: tmpl, DisplayName: "Alquimista", Enabled: true, X: 8, Y: 8, Merchant: 1},
			{Slug: "fallback", Template: tmpl, DisplayName: " ", Enabled: true, X: 9, Y: 9, Merchant: 1},
		},
	}, false)

	for slug, want := range map[string]string{
		"named-a":  "Armeiro",
		"named-b":  "Alquimista",
		"fallback": "Default",
	} {
		id, ok := d.managedNPCs[slug]
		if !ok {
			t.Fatalf("%s not spawned", slug)
		}
		if got := w.Entity(id).Name; got != want {
			t.Errorf("%s name = %q, want %q", slug, got, want)
		}
	}
	if got := string(tmpl[:7]); got != "Default" {
		t.Errorf("shared template mutated to %q, want Default", got)
	}
}

// TestApplyNPCConfigReconcile checks a second snapshot that disables the NPC and
// clears the price override despawns the entity and restores the base price.
func TestApplyNPCConfigReconcile(t *testing.T) {
	d, w := newNPCDispatcher(map[int]int32{1100: 999})
	tmpl := merchantTemplate("Keeper")
	d.applyNPCConfig(w, npccfg.Snapshot{
		Version: 1,
		Defs: []npccfg.Definition{{
			Slug: "shop-1", Template: tmpl, Enabled: true, X: 8, Y: 8, Merchant: 1,
			Shop: []npccfg.ShopItem{{Slot: 0, Index: 1100}},
		}},
		PriceOverrides: map[int]int32{1100: 500},
	}, false)
	id := d.managedNPCs["shop-1"]

	// Reload: NPC disabled, override gone.
	d.applyNPCConfig(w, npccfg.Snapshot{
		Version: 2,
		Defs: []npccfg.Definition{{
			Slug: "shop-1", Template: tmpl, Enabled: false, X: 8, Y: 8, Merchant: 1,
		}},
	}, false)

	if _, ok := d.managedNPCs["shop-1"]; ok {
		t.Error("disabled NPC still managed")
	}
	if w.Entity(id) != nil {
		t.Error("disabled NPC entity not despawned")
	}
	if d.itemPrices[1100] != 999 {
		t.Errorf("price[1100] = %d, want base 999 after override cleared", d.itemPrices[1100])
	}
	if d.npcVersion != 2 {
		t.Errorf("npcVersion = %d, want 2", d.npcVersion)
	}
}

// TestApplyNPCConfigSkipsInvalid checks definitions with no template or no
// position are skipped rather than spawned.
func TestApplyNPCConfigSkipsInvalid(t *testing.T) {
	d, w := newNPCDispatcher(nil)
	d.applyNPCConfig(w, npccfg.Snapshot{
		Version: 1,
		Defs: []npccfg.Definition{
			{Slug: "no-template", Enabled: true, X: 5, Y: 5, Merchant: 1},
			{Slug: "no-pos", Template: merchantTemplate("X"), Enabled: true, X: 0, Y: 0, Merchant: 1},
		},
	}, false)
	if len(d.managedNPCs) != 0 {
		t.Errorf("managed = %d, want 0 (both invalid)", len(d.managedNPCs))
	}
	_ = w
}
