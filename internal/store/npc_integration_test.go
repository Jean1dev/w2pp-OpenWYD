//go:build integration

// Integration tests for the NPC-editing store (npc-editing-plan.md). They require
// a real database and are excluded from the default build. Run with:
//
//	W2PP_TEST_DSN=postgres://postgres:dev@localhost:5432/postgres go test -tags=integration ./internal/store/
package store

import (
	"context"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// TestNPCConfigCRUD exercises the full moderator write path: upsert a definition,
// set its shop, toggle visibility, override a price, and delete — asserting the
// config version bumps on every write and reads reflect the writes.
func TestNPCConfigCRUD(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// Clean slate for the NPC tables (leave account schema alone).
	_, _ = pool.Exec(ctx, `DELETE FROM npc_audit; DELETE FROM npc_shop_item; DELETE FROM npc_definition; DELETE FROM item_price; UPDATE npc_config_meta SET version = 0 WHERE id = TRUE`)

	s := New(pool)

	v0, err := s.NPCConfigVersion(ctx)
	if err != nil {
		t.Fatalf("version: %v", err)
	}

	// A moderator account to satisfy the updated_by FK / audit account_id.
	var modID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO account (name, pass_hash, role) VALUES ('mod_npc_test','x','moderator') RETURNING id`).
		Scan(&modID); err != nil {
		t.Fatalf("seed moderator: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM account WHERE id = $1`, modID) })

	id, err := s.UpsertNPCDefinition(ctx, domain.NPCDefinition{
		Slug: "shop-int-1", TemplateName: "Keeper", DisplayName: "Keeper",
		Enabled: true, PosX: 100, PosY: 200, Merchant: 1,
	}, modID)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	if err := s.SetNPCShop(ctx, id, []domain.NPCShopItem{
		{Slot: 0, ItemIndex: 1100, Quantity: 120}, {Slot: 5, ItemIndex: 1101, Eff1: 1, EffV1: 9},
	}, modID); err != nil {
		t.Fatalf("set shop: %v", err)
	}
	if err := s.SetNPCVisibility(ctx, id, false, modID); err != nil {
		t.Fatalf("set visibility: %v", err)
	}
	if err := s.SetItemPrice(ctx, 1100, 777, modID); err != nil {
		t.Fatalf("set item price: %v", err)
	}

	// Read back the full snapshot.
	defs, err := s.ListNPCDefinitions(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("definitions = %d, want 1", len(defs))
	}
	got := defs[0]
	if got.Slug != "shop-int-1" || got.Enabled || got.PosX != 100 || got.Merchant != 1 {
		t.Errorf("definition mismatch: %+v", got)
	}
	if len(got.Shop) != 2 || got.Shop[0].Quantity != 120 || got.Shop[1].ItemIndex != 1101 || got.Shop[1].EffV1 != 9 {
		t.Errorf("shop mismatch: %+v", got.Shop)
	}

	prices, err := s.ItemPriceOverrides(ctx)
	if err != nil {
		t.Fatalf("prices: %v", err)
	}
	if len(prices) != 1 || prices[0].ItemIndex != 1100 || prices[0].Price != 777 {
		t.Errorf("price override mismatch: %+v", prices)
	}

	// Version must have advanced by the four writes.
	vN, err := s.NPCConfigVersion(ctx)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if vN != v0+4 {
		t.Errorf("version = %d, want %d (4 writes)", vN, v0+4)
	}

	// Delete cascades the shop rows.
	if err := s.DeleteNPCDefinition(ctx, id, modID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	defs, _ = s.ListNPCDefinitions(ctx)
	if len(defs) != 0 {
		t.Errorf("definitions after delete = %d, want 0", len(defs))
	}
}

// TestSeedNPCDefinitionsIdempotent checks the bulk import inserts once and skips
// existing slugs on a re-run.
func TestSeedNPCDefinitionsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM npc_shop_item; DELETE FROM npc_definition WHERE slug LIKE 'seed-int-%'`)

	s := New(pool)
	defs := []domain.NPCDefinition{
		{Slug: "seed-int-1", TemplateName: "A", Enabled: true, Merchant: 1,
			Shop: []domain.NPCShopItem{{Slot: 0, ItemIndex: 10, Quantity: 2}}},
		{Slug: "seed-int-2", TemplateName: "B", Enabled: true, Merchant: 1},
	}
	n, err := s.SeedNPCDefinitions(ctx, defs)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if n != 2 {
		t.Fatalf("first seed inserted %d, want 2", n)
	}
	n, err = s.SeedNPCDefinitions(ctx, defs)
	if err != nil {
		t.Fatalf("seed (2nd): %v", err)
	}
	if n != 0 {
		t.Errorf("second seed inserted %d, want 0 (idempotent)", n)
	}
}
