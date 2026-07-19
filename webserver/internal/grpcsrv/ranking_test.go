package grpcsrv

import (
	"context"
	"errors"
	"testing"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

type fakeRanking struct {
	limit  int
	offset int
	err    error
}

func (f *fakeRanking) ListExp(_ context.Context, limit, offset int) ([]domain.RankingEntry, int, error) {
	f.limit = limit
	f.offset = offset
	if f.err != nil {
		return nil, 0, f.err
	}
	return []domain.RankingEntry{{
		Rank: 3, Name: "Hero", Class: 1, Clan: 7, GuildID: 99, Level: 400, Exp: 12345, ClassMaster: 2,
	}}, 10, nil
}

func (f *fakeRanking) ListDuel(_ context.Context, limit, offset int) ([]domain.DuelRankingEntry, int, error) {
	f.limit = limit
	f.offset = offset
	if f.err != nil {
		return nil, 0, f.err
	}
	return []domain.DuelRankingEntry{{
		Rank: 1, Name: "Hero", Class: 1, Clan: 7, GuildID: 99, Wins: 5, Losses: 1,
	}}, 4, nil
}

func TestListExpRankingMapping(t *testing.T) {
	fake := &fakeRanking{}
	s := NewRanking(fake)

	resp, err := s.ListExpRanking(context.Background(), &webv1.ListExpRankingRequest{Limit: 25, Offset: 2})
	if err != nil {
		t.Fatalf("ListExpRanking: %v", err)
	}
	if fake.limit != 25 || fake.offset != 2 {
		t.Fatalf("forwarded limit=%d offset=%d; want 25/2", fake.limit, fake.offset)
	}
	if resp.GetTotalCount() != 10 || len(resp.GetEntries()) != 1 {
		t.Fatalf("response total=%d entries=%d; want 10/1", resp.GetTotalCount(), len(resp.GetEntries()))
	}
	got := resp.GetEntries()[0]
	if got.GetRank() != 3 || got.GetName() != "Hero" || got.GetClass() != 1 ||
		got.GetClan() != 7 || got.GetGuildId() != 99 || got.GetLevel() != 400 ||
		got.GetExp() != 12345 || got.GetClassMaster() != 2 {
		t.Fatalf("entry mapping mismatch: %+v", got)
	}
}

func TestListExpRankingInfraError(t *testing.T) {
	s := NewRanking(&fakeRanking{err: errors.New("db down")})
	if _, err := s.ListExpRanking(context.Background(), &webv1.ListExpRankingRequest{}); err == nil {
		t.Fatal("expected gRPC error on infra failure")
	}
}
