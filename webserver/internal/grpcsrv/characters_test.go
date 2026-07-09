package grpcsrv

import (
	"context"
	"errors"
	"testing"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

type fakeCharacters struct {
	chars []domain.Character
	err   error
	gotID int64
}

func (f *fakeCharacters) ListMyCharacters(_ context.Context, accountID int64) ([]domain.Character, error) {
	f.gotID = accountID
	return f.chars, f.err
}

func TestListMyCharactersMapping(t *testing.T) {
	f := &fakeCharacters{chars: []domain.Character{{
		Slot: 2, Name: "Hero", Class: 1, Level: 49, Exp: 123456, Coin: 987,
		MaxHp: 800, Hp: 700, MaxMp: 300, Mp: 250, Str: 50, Int: 10, Dex: 20, Con: 30,
	}}}
	resp, err := NewCharacters(f).ListMyCharacters(context.Background(), &webv1.ListMyCharactersRequest{AccountId: 99})
	if err != nil {
		t.Fatalf("ListMyCharacters: %v", err)
	}
	if f.gotID != 99 {
		t.Fatalf("accountID = %d, want 99", f.gotID)
	}
	if len(resp.GetCharacters()) != 1 {
		t.Fatalf("characters = %d, want 1", len(resp.GetCharacters()))
	}
	got := resp.GetCharacters()[0]
	if got.GetSlot() != 2 || got.GetName() != "Hero" || got.GetClass() != 1 || got.GetLevel() != 49 ||
		got.GetExp() != 123456 || got.GetCoin() != 987 || got.GetMaxHp() != 800 || got.GetHp() != 700 ||
		got.GetMaxMp() != 300 || got.GetMp() != 250 || got.GetStr() != 50 || got.GetInt() != 10 ||
		got.GetDex() != 20 || got.GetCon() != 30 {
		t.Fatalf("mapped character = %+v", got)
	}
}

func TestListMyCharactersEmpty(t *testing.T) {
	resp, err := NewCharacters(&fakeCharacters{}).ListMyCharacters(context.Background(), &webv1.ListMyCharactersRequest{AccountId: 1})
	if err != nil {
		t.Fatalf("ListMyCharacters: %v", err)
	}
	if len(resp.GetCharacters()) != 0 {
		t.Fatalf("characters = %+v, want empty", resp.GetCharacters())
	}
}

func TestListMyCharactersInfraError(t *testing.T) {
	_, err := NewCharacters(&fakeCharacters{err: errors.New("db down")}).
		ListMyCharacters(context.Background(), &webv1.ListMyCharactersRequest{AccountId: 1})
	if err == nil {
		t.Fatal("expected gRPC error")
	}
}
