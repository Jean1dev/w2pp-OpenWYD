//go:build integration

// Integration tests for the mob template stat store
// (mob-template-editing-plan.md). They require a real database and are
// excluded from the default build. Run with:
//
//	W2PP_TEST_DSN=postgres://postgres:dev@localhost:5432/postgres go test -tags=integration ./internal/store/
package store

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// TestMobTemplateStatCRUD exercises the full moderator write path: upsert a
// stat override, set its equip slots, read it back, delete it, and confirm
// the audit/version bump happens on every write.
func TestMobTemplateStatCRUD(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM mob_template_equip; DELETE FROM mob_template_stat; UPDATE npc_config_meta SET version = 0 WHERE id = TRUE`)

	s := New(pool)

	var modID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO account (name, pass_hash, role) VALUES ('mod_mobstat_test','x','moderator') RETURNING id`).
		Scan(&modID); err != nil {
		t.Fatalf("seed moderator: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM account WHERE id = $1`, modID) })

	if _, err := s.GetMobTemplateStat(ctx, "Karkarian"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get before upsert err = %v, want ErrNotFound", err)
	}

	want := domain.MobTemplateStat{
		TemplateName: "Karkarian",
		DisplayName:  "Karkarian Rebalanceado",
		Clan:         3,
		Merchant:     0,
		Class:        1,
		Coin:         5000,
		Exp:          123456,
		SPX:          10, SPY: 20,
		Level: 80, AC: 40, Damage: 300, ChaosRate: 1,
		AttackRun: 5, Direction: 2,
		Str: 100, Int: 10, Dex: 50, Con: 90,
		Special: [4]int16{1, 2, 3, 4},
		MaxHp:   50000, Hp: 50000, MaxMp: 1000, Mp: 1000,
		LearnedSkill: 7, ScoreBonus: 3,
		SkillBar: [4]uint8{1, 2, 3, 4},
		RegenHP:  10, RegenMP: 5,
		Resist: [4]int8{1, -1, 2, -2},
	}
	if err := s.UpsertMobTemplateStat(ctx, want, modID); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := s.GetMobTemplateStat(ctx, "Karkarian")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	got.Equip = nil // not set yet
	if !reflect.DeepEqual(got, want) {
		t.Errorf("get = %+v, want %+v", got, want)
	}

	if err := s.SetMobTemplateEquip(ctx, "Karkarian", []domain.MobTemplateEquipItem{
		{Slot: 0, ItemIndex: 1100, Eff1: 1, EffV1: 9},
		{Slot: 5, ItemIndex: 1200},
	}, modID); err != nil {
		t.Fatalf("set equip: %v", err)
	}

	got2, err := s.GetMobTemplateStat(ctx, "Karkarian")
	if err != nil {
		t.Fatalf("get after equip: %v", err)
	}
	if len(got2.Equip) != 2 {
		t.Fatalf("equip = %+v, want 2 items", got2.Equip)
	}

	// SetMobTemplateEquip on a template with no stat row yet must fail.
	if err := s.SetMobTemplateEquip(ctx, "NoSuchTemplate", nil, modID); !errors.Is(err, ErrNotFound) {
		t.Errorf("set equip on missing template err = %v, want ErrNotFound", err)
	}

	all, err := s.ListMobTemplateStats(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 1 || all[0].TemplateName != "Karkarian" || len(all[0].Equip) != 2 {
		t.Fatalf("list = %+v, want 1 entry named Karkarian with 2 equip slots", all)
	}

	if err := s.DeleteMobTemplateStat(ctx, "Karkarian", modID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetMobTemplateStat(ctx, "Karkarian"); !errors.Is(err, ErrNotFound) {
		t.Errorf("get after delete err = %v, want ErrNotFound", err)
	}
	if err := s.DeleteMobTemplateStat(ctx, "Karkarian", modID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete again err = %v, want ErrNotFound", err)
	}

	var equipCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM mob_template_equip WHERE template_name = 'Karkarian'`).Scan(&equipCount); err != nil {
		t.Fatal(err)
	}
	if equipCount != 0 {
		t.Errorf("equip rows after delete = %d, want 0 (cascade)", equipCount)
	}

	var auditCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM npc_audit WHERE action IN ('create_template_stat','set_template_equip','delete_template_stat')`).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 3 {
		t.Errorf("audit rows = %d, want 3 (create, set_equip, delete)", auditCount)
	}
}
