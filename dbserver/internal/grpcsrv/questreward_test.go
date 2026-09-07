package grpcsrv

import (
	"context"
	"testing"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

type fakeQuestStore struct{ cfg domain.QuestRewardConfig }

func (f *fakeQuestStore) QuestRewards(context.Context) (domain.QuestRewardConfig, error) {
	return f.cfg, nil
}

func TestQuestRewardServerCarriesEveryField(t *testing.T) {
	s := NewQuestReward(&fakeQuestStore{cfg: domain.QuestRewardConfig{
		Version: 4,
		Tiers: []domain.QuestReward{{
			Tier: 2, MortalExp: 500000, ArchExp: 250000, Coin: 1000000,
			MortalMin: 190, MortalMax: 400, ArchMin: 190, ArchMax: 399,
		}},
	}})
	resp, err := s.GetQuestRewards(context.Background(), &dbv1.GetQuestRewardsRequest{})
	if err != nil {
		t.Fatalf("GetQuestRewards: %v", err)
	}
	if resp.GetVersion() != 4 || len(resp.GetTiers()) != 1 {
		t.Fatalf("resp = %+v", resp)
	}
	// Every field matters: dropping one on the wire would silently pay the
	// content file's value for it while the panel showed the edited one.
	got := resp.GetTiers()[0]
	if got.GetTier() != 2 || got.GetMortalExp() != 500000 || got.GetArchExp() != 250000 ||
		got.GetCoin() != 1000000 || got.GetMortalMin() != 190 || got.GetMortalMax() != 400 ||
		got.GetArchMin() != 190 || got.GetArchMax() != 399 {
		t.Errorf("tier = %+v", got)
	}
}

// TestQuestRewardEmptyKeepsTheContentFile: a fresh server has no rows, and
// tmServer reads an empty list as "leave QuestsRate.txt alone".
func TestQuestRewardEmptyKeepsTheContentFile(t *testing.T) {
	s := NewQuestReward(&fakeQuestStore{})
	resp, err := s.GetQuestRewards(context.Background(), &dbv1.GetQuestRewardsRequest{})
	if err != nil {
		t.Fatalf("GetQuestRewards: %v", err)
	}
	if len(resp.GetTiers()) != 0 || resp.GetVersion() != 0 {
		t.Fatalf("resp = %+v, want empty at version 0", resp)
	}
}
