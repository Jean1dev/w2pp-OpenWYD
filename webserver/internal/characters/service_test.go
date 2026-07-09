package characters

import (
	"context"
	"errors"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

type fakeStore struct {
	chars []domain.Character
	err   error
	gotID int64
}

func (f *fakeStore) ListCharacters(_ context.Context, accountID int64) ([]domain.Character, error) {
	f.gotID = accountID
	return f.chars, f.err
}

func TestListMyCharacters(t *testing.T) {
	fs := &fakeStore{chars: []domain.Character{
		{Slot: 0, Name: "Hero", Level: 10},
		{Slot: 2, Name: "Mage", Level: 20},
	}}
	got, err := New(fs).ListMyCharacters(context.Background(), 42)
	if err != nil {
		t.Fatalf("ListMyCharacters: %v", err)
	}
	if fs.gotID != 42 {
		t.Fatalf("accountID = %d, want 42", fs.gotID)
	}
	if len(got) != 2 || got[0].Slot != 0 || got[1].Slot != 2 {
		t.Fatalf("characters = %+v", got)
	}
}

func TestListMyCharactersError(t *testing.T) {
	want := errors.New("db down")
	_, err := New(&fakeStore{err: want}).ListMyCharacters(context.Background(), 7)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want wrapping %v", err, want)
	}
}
