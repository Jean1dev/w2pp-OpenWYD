//go:build integration

package store

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// Migration 0027: the equipment-free base round-trips through SaveCharacter /
// LoadCharacter; a row saved before it loads with HasBase false (the tmServer
// derives once); and a save that does not carry the base (a tmServer older than
// 0027, e.g. after a rollback) clears it, so a stale base is never trusted.
func TestBaseScoreColumns(t *testing.T) {
	s, ctx := freshStore(t)
	accID, err := s.SaveAccount(ctx, domain.Account{Name: "basescore", PassHash: "$argon2id$hash",
		Characters: []domain.Character{{Slot: 0, Name: "Archer", Class: 3, Level: 399,
			Str: 112, Int: 112, Dex: 2612, Con: 960, Hp: 2000, MaxHp: 2000, Mp: 500, MaxMp: 500, ClassMaster: 2}}})
	if err != nil {
		t.Fatalf("SaveAccount: %v", err)
	}
	ch, err := s.LoadCharacter(ctx, accID, 0)
	if err != nil {
		t.Fatalf("LoadCharacter: %v", err)
	}
	if ch.HasBase {
		t.Fatalf("a row without base columns loaded HasBase true: %+v", ch)
	}

	ch.HasBase = true
	ch.BaseStr, ch.BaseInt, ch.BaseDex, ch.BaseCon, ch.BaseMaxHp, ch.BaseMaxMp = 12, 12, 2512, 860, 1800, 450
	ch.Str = 212
	if err := s.SaveCharacter(ctx, accID, ch); err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}
	got, err := s.LoadCharacter(ctx, accID, 0)
	if err != nil {
		t.Fatalf("LoadCharacter: %v", err)
	}
	if !got.HasBase || got.BaseStr != 12 || got.BaseInt != 12 || got.BaseDex != 2512 || got.BaseCon != 860 ||
		got.BaseMaxHp != 1800 || got.BaseMaxMp != 450 || got.Str != 212 {
		t.Fatalf("base round trip: %+v", got)
	}

	// A save without the base (an older writer) invalidates the stored one: the
	// CurrentScore it wrote may carry allocations the old base does not have.
	got.HasBase = false
	got.Str = 213
	if err := s.SaveCharacter(ctx, accID, got); err != nil {
		t.Fatalf("SaveCharacter without base: %v", err)
	}
	again, err := s.LoadCharacter(ctx, accID, 0)
	if err != nil {
		t.Fatalf("LoadCharacter: %v", err)
	}
	if again.HasBase || again.Str != 213 {
		t.Fatalf("a save without the base kept a stale base: %+v", again)
	}
}
