//go:build integration

// Integration tests for the duel ranking store (issue #118). Require a real
// database; excluded from the default build.
//
//	W2PP_TEST_DSN=postgres://postgres:dev@localhost:5432/postgres go test -tags=integration ./internal/store/
package store

import (
	"errors"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

func TestRecordDuelResult(t *testing.T) {
	s, ctx := freshStore(t)

	if _, err := s.SaveAccount(ctx, domain.Account{
		Name: "duel_acct", PassHash: "x",
		Characters: []domain.Character{
			{Slot: 0, Name: "Winner", Class: 1},
			{Slot: 1, Name: "Loser", Class: 2},
		},
	}); err != nil {
		t.Fatalf("SaveAccount: %v", err)
	}

	// First result: upsert-creates both rows.
	if err := s.RecordDuelResult(ctx, "Winner", "Loser"); err != nil {
		t.Fatalf("RecordDuelResult: %v", err)
	}
	// Second result, reversed: exercises the increment (not overwrite) path,
	// and that the same character can accumulate both wins and losses.
	if err := s.RecordDuelResult(ctx, "Loser", "Winner"); err != nil {
		t.Fatalf("RecordDuelResult (2): %v", err)
	}

	entries, total, err := s.ListDuelRanking(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListDuelRanking: %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	byName := map[string]domain.DuelRankingEntry{}
	for _, e := range entries {
		byName[e.Name] = e
	}
	if w := byName["Winner"]; w.Wins != 1 || w.Losses != 1 {
		t.Errorf("Winner stats = %+v, want wins=1 losses=1", w)
	}
	if l := byName["Loser"]; l.Wins != 1 || l.Losses != 1 {
		t.Errorf("Loser stats = %+v, want wins=1 losses=1", l)
	}
}

func TestRecordDuelResultUnknownCharacter(t *testing.T) {
	s, ctx := freshStore(t)

	if _, err := s.SaveAccount(ctx, domain.Account{
		Name: "duel_acct2", PassHash: "x",
		Characters: []domain.Character{{Slot: 0, Name: "Known", Class: 1}},
	}); err != nil {
		t.Fatalf("SaveAccount: %v", err)
	}

	err := s.RecordDuelResult(ctx, "Known", "Ghost")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("RecordDuelResult with unknown loser: err = %v, want ErrNotFound", err)
	}
}
