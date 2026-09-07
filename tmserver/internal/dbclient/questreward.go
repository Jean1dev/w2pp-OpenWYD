package dbclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// QuestRewardSource reads the panel-managed quest-trophy payouts from dbServer.
//
// Fetched once at boot and not polled, like the Mesa de XP: it is a balance
// number, and two players using the same trophy minutes apart must not get
// different amounts.
type QuestRewardSource struct {
	api dbv1.QuestRewardServiceClient
}

// NewQuestRewardSource wraps a gRPC connection.
func NewQuestRewardSource(conn grpc.ClientConnInterface) *QuestRewardSource {
	return &QuestRewardSource{api: dbv1.NewQuestRewardServiceClient(conn)}
}

// Fetch returns every edited tier. An empty result leaves the content file's
// values untouched, which is how the server ran before this existed.
func (c *QuestRewardSource) Fetch(ctx context.Context) (domain.QuestRewardConfig, error) {
	resp, err := c.api.GetQuestRewards(ctx, &dbv1.GetQuestRewardsRequest{})
	if err != nil {
		return domain.QuestRewardConfig{}, fmt.Errorf("dbclient: get quest rewards: %w", err)
	}
	cfg := domain.QuestRewardConfig{Version: resp.GetVersion()}
	for _, q := range resp.GetTiers() {
		cfg.Tiers = append(cfg.Tiers, domain.QuestReward{
			Tier: q.GetTier(), MortalExp: q.GetMortalExp(), ArchExp: q.GetArchExp(),
			Coin:      q.GetCoin(),
			MortalMin: q.GetMortalMin(), MortalMax: q.GetMortalMax(),
			ArchMin: q.GetArchMin(), ArchMax: q.GetArchMax(),
		})
	}
	return cfg, nil
}
