package ranking

import (
	"context"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

type fakeStore struct {
	limit  int
	offset int
}

func (f *fakeStore) ListExpRanking(_ context.Context, limit, offset int) ([]domain.RankingEntry, int, error) {
	f.limit = limit
	f.offset = offset
	return []domain.RankingEntry{{Name: "Hero"}, {Name: "Mage"}}, 2, nil
}

func (f *fakeStore) ListDuelRanking(_ context.Context, limit, offset int) ([]domain.DuelRankingEntry, int, error) {
	f.limit = limit
	f.offset = offset
	return []domain.DuelRankingEntry{{Name: "Hero", Wins: 5}, {Name: "Mage", Wins: 2}}, 2, nil
}

func TestListExpNormalizesPaginationAndRanks(t *testing.T) {
	cases := []struct {
		name       string
		limit      int
		offset     int
		wantLimit  int
		wantOffset int
		wantRank0  int32
	}{
		{name: "default limit", limit: 0, offset: 0, wantLimit: 50, wantOffset: 0, wantRank0: 1},
		{name: "clamp max", limit: 500, offset: 25, wantLimit: 100, wantOffset: 25, wantRank0: 26},
		{name: "negative offset", limit: 10, offset: -7, wantLimit: 10, wantOffset: 0, wantRank0: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := &fakeStore{}
			entries, total, err := New(fs).ListExp(context.Background(), tc.limit, tc.offset)
			if err != nil {
				t.Fatalf("ListExp: %v", err)
			}
			if fs.limit != tc.wantLimit || fs.offset != tc.wantOffset {
				t.Fatalf("store got limit=%d offset=%d; want limit=%d offset=%d",
					fs.limit, fs.offset, tc.wantLimit, tc.wantOffset)
			}
			if total != 2 || len(entries) != 2 {
				t.Fatalf("got total=%d entries=%d; want total=2 entries=2", total, len(entries))
			}
			if entries[0].Rank != tc.wantRank0 || entries[1].Rank != tc.wantRank0+1 {
				t.Fatalf("ranks = %d/%d, want %d/%d", entries[0].Rank, entries[1].Rank, tc.wantRank0, tc.wantRank0+1)
			}
		})
	}
}
