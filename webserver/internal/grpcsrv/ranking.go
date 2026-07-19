package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// Ranking is the public ranking surface the server depends on.
type Ranking interface {
	ListExp(ctx context.Context, limit, offset int) ([]domain.RankingEntry, int, error)
	ListDuel(ctx context.Context, limit, offset int) ([]domain.DuelRankingEntry, int, error)
}

// RankingServer implements webv1.RankingWebServiceServer.
type RankingServer struct {
	webv1.UnimplementedRankingWebServiceServer
	ranking Ranking
}

// NewRanking builds the RankingWebService over the given ranking logic.
func NewRanking(r Ranking) *RankingServer { return &RankingServer{ranking: r} }

// ListExpRanking returns the public Top EXP character ranking.
func (s *RankingServer) ListExpRanking(ctx context.Context, req *webv1.ListExpRankingRequest) (*webv1.ListExpRankingResponse, error) {
	entries, total, err := s.ranking.ListExp(ctx, int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list exp ranking: %v", err)
	}
	out := make([]*webv1.RankingEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, rankingEntryToProto(e))
	}
	return &webv1.ListExpRankingResponse{Entries: out, TotalCount: int32(total)}, nil
}

func rankingEntryToProto(e domain.RankingEntry) *webv1.RankingEntry {
	return &webv1.RankingEntry{
		Rank:        e.Rank,
		Name:        e.Name,
		Class:       int32(e.Class),
		Clan:        int32(e.Clan),
		GuildId:     uint32(e.GuildID),
		Level:       e.Level,
		Exp:         e.Exp,
		ClassMaster: int32(e.ClassMaster),
	}
}

// ListDuelRanking returns the public 1v1 duel win/loss leaderboard (issue #118).
func (s *RankingServer) ListDuelRanking(ctx context.Context, req *webv1.ListDuelRankingRequest) (*webv1.ListDuelRankingResponse, error) {
	entries, total, err := s.ranking.ListDuel(ctx, int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list duel ranking: %v", err)
	}
	out := make([]*webv1.DuelRankingEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, duelRankingEntryToProto(e))
	}
	return &webv1.ListDuelRankingResponse{Entries: out, TotalCount: int32(total)}, nil
}

func duelRankingEntryToProto(e domain.DuelRankingEntry) *webv1.DuelRankingEntry {
	return &webv1.DuelRankingEntry{
		Rank:    e.Rank,
		Name:    e.Name,
		Class:   int32(e.Class),
		Clan:    int32(e.Clan),
		GuildId: uint32(e.GuildID),
		Wins:    e.Wins,
		Losses:  e.Losses,
	}
}
