package handler

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

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

// --- boot load retry ---
//
// Production incident 2026-08-13: tmServer and dbServer deploy together, and the
// dbServer reconciles the content catalog BEFORE it listens, so the tmServer
// routinely wins the race. Boot must wait it out instead of exiting and burning
// the platform's restart budget — while still failing fast on config that will
// never become valid.

// bootStep is one scripted answer from fakeBootSource.
type bootStep struct {
	snap npccfg.Snapshot
	err  error
}

// fakeBootSource replays scripted Snapshot answers, one per call, repeating the
// last step once the script runs out. Unlike staticSource it counts calls, which
// is how the tests tell a retry from a fail-fast.
type fakeBootSource struct {
	steps []bootStep
	calls int
}

func (f *fakeBootSource) Version(context.Context) (int64, error) { return 0, nil }

func (f *fakeBootSource) Snapshot(ctx context.Context) (npccfg.Snapshot, error) {
	f.calls++
	// The real dbclient fails this way on a cancelled/expired boot context.
	if err := ctx.Err(); err != nil {
		return npccfg.Snapshot{}, err
	}
	step := f.steps[min(f.calls-1, len(f.steps)-1)]
	return step.snap, step.err
}

// shrinkBootRetry collapses the retry budget so the tests finish in
// milliseconds. Package vars, restored on cleanup; no t.Parallel here.
func shrinkBootRetry(t *testing.T) {
	t.Helper()
	budget, lo, hi := npcBootRetryBudget, npcBootRetryInitial, npcBootRetryMax
	npcBootRetryBudget, npcBootRetryInitial, npcBootRetryMax = 150*time.Millisecond, time.Millisecond, 2*time.Millisecond
	t.Cleanup(func() {
		npcBootRetryBudget, npcBootRetryInitial, npcBootRetryMax = budget, lo, hi
	})
}

// newBootDispatcher wires a Dispatcher to src over a world holding managed
// DB-managed generator slots — the shape ApplyNPCConfigBoot validates against.
func newBootDispatcher(src npccfg.Source, managed int) (*Dispatcher, *world.World) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, NpcConfig: src})
	w := world.New(world.Config{GridDim: 32}, log, world.NopPersistence{}, d.Handle)
	gens := make([]*world.Generator, managed)
	for i := range gens {
		gens[i] = &world.Generator{DBManaged: true}
	}
	w.RegisterGenerators(gens)
	return d, w
}

// bootSnapshot covers the first n DB-managed generator slots. The definitions are
// left disabled: they still count toward the completeness check, which keeps
// these tests on the boot-load path rather than the spawn path already covered
// by TestApplyNPCConfigSpawnsMerchant.
func bootSnapshot(version int64, n int) npccfg.Snapshot {
	snap := npccfg.Snapshot{Version: version}
	for i := 0; i < n; i++ {
		snap.Defs = append(snap.Defs, npccfg.Definition{
			Slug:           fmt.Sprintf("merchant-%d", i),
			Origin:         "content",
			GeneratorIndex: i,
		})
	}
	return snap
}

func TestApplyNPCConfigBootRetries(t *testing.T) {
	const managed = 3
	unavailable := errors.New("connection refused")

	tests := []struct {
		name  string
		steps []bootStep
		// wantCalls is exact; 0 means "only require that it retried".
		wantCalls int
		wantErr   bool
	}{
		{
			// The nine transport failures of the production crash-loop.
			name: "transport failure clears once dbServer listens",
			steps: []bootStep{
				{err: unavailable},
				{err: unavailable},
				{snap: bootSnapshot(7, managed)},
			},
			wantCalls: 3,
		},
		{
			// The other production signature: dbServer answered mid-seed, so the
			// catalog was short rather than the connection refused.
			name: "short catalog clears once the seed commits",
			steps: []bootStep{
				{snap: bootSnapshot(7, managed-1)},
				{snap: bootSnapshot(7, managed)},
			},
			wantCalls: 2,
		},
		{
			name:      "succeeds on the first attempt",
			steps:     []bootStep{{snap: bootSnapshot(7, managed)}},
			wantCalls: 1,
		},
		{
			// Duplicate generator index is corrupt data — waiting cannot fix it,
			// so boot must not spend the budget before reporting it.
			name: "duplicate generator index fails without retrying",
			steps: []bootStep{{snap: npccfg.Snapshot{Defs: []npccfg.Definition{
				{Slug: "a", Origin: "content", GeneratorIndex: 0},
				{Slug: "b", Origin: "content", GeneratorIndex: 0},
			}}}},
			wantCalls: 1,
			wantErr:   true,
		},
		{
			name:      "generator outside the DB-managed set fails without retrying",
			steps:     []bootStep{{snap: bootSnapshot(7, managed+1)}},
			wantCalls: 1,
			wantErr:   true,
		},
		{
			// Fail-closed is preserved: a dbServer that never comes back still
			// stops the boot rather than starting on an empty catalog.
			name:    "permanent failure still fails closed",
			steps:   []bootStep{{err: unavailable}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shrinkBootRetry(t)
			src := &fakeBootSource{steps: tt.steps}
			d, w := newBootDispatcher(src, managed)

			err := d.ApplyNPCConfigBoot(context.Background(), w)

			if tt.wantErr && err == nil {
				t.Fatal("ApplyNPCConfigBoot succeeded, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ApplyNPCConfigBoot: %v", err)
			}
			switch {
			case tt.wantCalls > 0 && src.calls != tt.wantCalls:
				t.Errorf("Snapshot called %d times, want %d", src.calls, tt.wantCalls)
			case tt.wantCalls == 0 && src.calls < 2:
				t.Errorf("Snapshot called %d times, want more than one attempt", src.calls)
			}
			if !tt.wantErr && d.npcVersion != 7 {
				t.Errorf("npcVersion = %d, want the applied snapshot version 7", d.npcVersion)
			}
		})
	}
}

// TestApplyNPCConfigBootHonoursContext proves SIGTERM during boot aborts the wait
// instead of holding the process for the whole retry budget. The budget is left
// at full size on purpose: cancellation, not the budget, must end the wait.
func TestApplyNPCConfigBootHonoursContext(t *testing.T) {
	src := &fakeBootSource{steps: []bootStep{{err: errors.New("connection refused")}}}
	d, w := newBootDispatcher(src, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := d.ApplyNPCConfigBoot(ctx, w)
	if err == nil {
		t.Fatal("ApplyNPCConfigBoot succeeded on a cancelled context, want error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want it to wrap context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("returned after %s, want it to abort promptly", elapsed)
	}
}
