//go:build integration

package store

import (
	"reflect"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

func TestArchCrystalProgressionPersistence(t *testing.T) {
	s, ctx := freshStore(t)
	accountID, err := s.SaveAccount(ctx, domain.Account{Name: "archquest", PassHash: "test-hash", Characters: []domain.Character{{
		Slot: 0, Name: "Arch", ClassMaster: 1, Level: 399, MortalLevel: 399, Exp: 2_000_000_000,
		MaxHp: 1000, Hp: 1000, MaxMp: 500, Mp: 500,
		Carry: []domain.Item{{Slot: 0, Index: 4106}, {Slot: 1, Index: 4107}, {Slot: 2, Index: 4108}, {Slot: 3, Index: 4109}, {Slot: 4, Index: 1742}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := s.LoadCharacter(ctx, accountID, 0)
	if err != nil || ch.ArchCrystalStage != 0 {
		t.Fatalf("initial stage: %d err=%v", ch.ArchCrystalStage, err)
	}
	for stage := uint8(1); stage <= 4; stage++ {
		ch.ArchCrystalStage = stage
		ch.Exp -= 100_000_000
		ch.Carry = ch.Carry[1:]
		switch stage {
		case 1:
			ch.MaxMp += 80
		case 3:
			ch.MaxHp += 80
		case 4:
			ch.MaxHp += 60
			ch.MaxMp += 60
		}
		if err := s.SaveCharacter(ctx, accountID, ch); err != nil {
			t.Fatal(err)
		}
		loaded, err := s.LoadCharacter(ctx, accountID, 0)
		if err != nil {
			t.Fatal(err)
		}
		if loaded.ArchCrystalStage != stage || loaded.MaxHp != ch.MaxHp || loaded.MaxMp != ch.MaxMp || loaded.Exp != ch.Exp || !reflect.DeepEqual(loaded.Carry, ch.Carry) {
			t.Fatalf("stage %d not persisted atomically: %+v", stage, loaded)
		}
		ch = loaded
	}
	ch.ArchLv355, ch.ArchLv370, ch.Fame = 1, 1, 0
	ch.Equip = []domain.Item{{Slot: 15, Index: 3191}}
	if err := s.SaveCharacter(ctx, accountID, ch); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.LoadCharacter(ctx, accountID, 0)
	if err != nil || loaded.ArchLv355 != 1 || loaded.ArchLv370 != 1 || loaded.ArchCrystalStage != 4 || loaded.Level != 399 || len(loaded.Equip) != 1 || loaded.Equip[0].Index != 3191 {
		t.Fatalf("late unlock/relog failed: %+v err=%v", loaded, err)
	}
	// A failed character transaction must not consume the Ideal Stone or
	// replace the cape while its crystal stage remains uncommitted.
	invalid := loaded
	invalid.ArchCrystalStage = 5
	invalid.Carry, invalid.Equip = nil, nil
	if err := s.SaveCharacter(ctx, accountID, invalid); err == nil {
		t.Fatal("invalid crystal stage accepted")
	}
	after, err := s.LoadCharacter(ctx, accountID, 0)
	if err != nil || !reflect.DeepEqual(after, loaded) {
		t.Fatalf("failed save changed character: %+v err=%v", after, err)
	}
}
