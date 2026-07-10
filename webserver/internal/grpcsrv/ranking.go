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
