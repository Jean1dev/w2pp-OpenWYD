package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// QuestRewardStore is the read surface tmServer needs (satisfied by
// *store.Store). Moderator writes go through the admin panel.
type QuestRewardStore interface {
	QuestRewards(ctx context.Context) (domain.QuestRewardConfig, error)
}

// QuestRewardServer implements dbv1.QuestRewardServiceServer.
type QuestRewardServer struct {
	dbv1.UnimplementedQuestRewardServiceServer
	store QuestRewardStore
}

// NewQuestReward builds the service over the given store.
func NewQuestReward(s QuestRewardStore) *QuestRewardServer { return &QuestRewardServer{store: s} }

// GetQuestRewards returns every edited tier and the version it belongs to.
func (s *QuestRewardServer) GetQuestRewards(ctx context.Context, _ *dbv1.GetQuestRewardsRequest) (*dbv1.GetQuestRewardsResponse, error) {
	cfg, err := s.store.QuestRewards(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "quest rewards: %v", err)
	}
	resp := &dbv1.GetQuestRewardsResponse{
		Version: cfg.Version, Tiers: make([]*dbv1.QuestReward, 0, len(cfg.Tiers)),
	}
	for _, q := range cfg.Tiers {
		resp.Tiers = append(resp.Tiers, &dbv1.QuestReward{
			Tier: q.Tier, MortalExp: q.MortalExp, ArchExp: q.ArchExp, Coin: q.Coin,
			MortalMin: q.MortalMin, MortalMax: q.MortalMax,
			ArchMin: q.ArchMin, ArchMax: q.ArchMax,
		})
	}
	return resp, nil
}
