//go:build integration

// Integration tests for the quest-trophy payouts (0036_quest_reward). They
// require a real database and are excluded from the default build. Run with:
//
//	W2PP_TEST_DSN=postgres://postgres:dev@localhost:5432/postgres go test -tags=integration ./internal/store/
package store

import (
	"context"
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

func questBase(tier int32) domain.QuestReward {
	return domain.QuestReward{
		Tier: tier, MortalExp: 500_000, ArchExp: 250_000, Coin: 1_000_000,
		MortalMin: 39, MortalMax: 400, ArchMin: 39, ArchMax: 400,
	}
}

func TestQuestRewardCRUD(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM quest_reward; UPDATE quest_reward_meta SET version = 0 WHERE id = TRUE`)
	s := New(pool)

	cfg, err := s.QuestRewards(ctx)
	if err != nil {
		t.Fatalf("QuestRewards: %v", err)
	}
	if len(cfg.Tiers) != 0 || cfg.Version != 0 {
		t.Fatalf("a tabela nasce vazia, li %+v", cfg)
	}

	// "Não havia linha" e "pagava zero" são coisas diferentes, e a auditoria
	// depende de saber qual foi.
	_, tinha, err := s.SetQuestReward(ctx, questBase(0), 0)
	if err != nil {
		t.Fatalf("SetQuestReward: %v", err)
	}
	if tinha {
		t.Error("disse que já havia linha numa tabela vazia")
	}

	antes, tinha, err := s.SetQuestReward(ctx, questBase(0), 0)
	if err != nil {
		t.Fatalf("SetQuestReward: %v", err)
	}
	if !tinha || antes.MortalExp != 500_000 {
		t.Errorf("a segunda gravação viu %+v (tinha=%v)", antes, tinha)
	}

	cfg, _ = s.QuestRewards(ctx)
	if cfg.Version == 0 {
		t.Error("a versão não subiu numa escrita")
	}
	if len(cfg.Tiers) != 1 || cfg.Tiers[0].ArchMax != 400 {
		t.Fatalf("li de volta %+v", cfg.Tiers)
	}

	antes, tinha, err = s.DeleteQuestReward(ctx, 0, 0)
	if err != nil {
		t.Fatalf("DeleteQuestReward: %v", err)
	}
	if !tinha || antes.Coin != 1_000_000 {
		t.Errorf("o delete devolveu %+v (tinha=%v)", antes, tinha)
	}
	cfg, _ = s.QuestRewards(ctx)
	if len(cfg.Tiers) != 0 {
		t.Fatalf("sobrou %+v depois do delete", cfg.Tiers)
	}
	// Apagar o que não existe não é erro, e diz que não havia nada.
	if _, tinha, err := s.DeleteQuestReward(ctx, 0, 0); err != nil || tinha {
		t.Errorf("delete de linha ausente: tinha=%v err=%v", tinha, err)
	}
}

// TestQuestRewardRecusaFaixaInvertida: uma faixa invertida recusa TODO MUNDO em
// silêncio, então o banco a barra mesmo que a tela deixe passar.
func TestQuestRewardRecusaFaixaInvertida(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM quest_reward`)
	s := New(pool)

	ruim := questBase(1)
	ruim.MortalMax = ruim.MortalMin
	if _, _, err := s.SetQuestReward(ctx, ruim, 0); err == nil {
		t.Fatal("o banco aceitou uma faixa vazia para Mortal")
	} else if !strings.Contains(err.Error(), "quest_reward_faixa_mortal") {
		t.Logf("recusado por %v", err)
	}

	ruim = questBase(1)
	ruim.ArchMax = ruim.ArchMin - 1
	if _, _, err := s.SetQuestReward(ctx, ruim, 0); err == nil {
		t.Fatal("o banco aceitou uma faixa invertida para Arch")
	}
}
