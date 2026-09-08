//go:build integration

// Integration tests for the respawn pacing (0038_spawn_rate). They require a
// real database and are excluded from the default build. Run with:
//
//	W2PP_TEST_DSN=postgres://postgres:dev@localhost:5432/postgres go test -tags=integration ./internal/store/
package store

import (
	"context"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

func limparSpawnRate(t *testing.T, ctx context.Context) *Store {
	t.Helper()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM spawn_rate; UPDATE spawn_rate_meta SET version = 0 WHERE id = TRUE`)
	return New(pool)
}

func TestSpawnRateCRUD(t *testing.T) {
	ctx := context.Background()
	s := limparSpawnRate(t, ctx)

	cfg, err := s.SpawnRates(ctx)
	if err != nil {
		t.Fatalf("SpawnRates: %v", err)
	}
	if len(cfg.Areas) != 0 || cfg.Version != 0 {
		t.Fatalf("a tabela nasce vazia, li %+v", cfg)
	}

	// Uma área intocada lê como 100%, e `tinha` diz que não havia linha. As duas
	// coisas juntas são o que a auditoria precisa para não escrever "estava em
	// 0%" quando na verdade estava no arquivo de conteúdo.
	antes, tinha, err := s.SetSpawnRate(ctx, domain.SpawnRate{Area: 0, Percent: 200}, 0)
	if err != nil {
		t.Fatalf("SetSpawnRate: %v", err)
	}
	if tinha {
		t.Error("disse que já havia linha numa tabela vazia")
	}
	if antes.Percent != 100 {
		t.Errorf("o valor anterior de uma área intocada leu %d%%, quero 100%%", antes.Percent)
	}

	cfg, _ = s.SpawnRates(ctx)
	if cfg.Version == 0 {
		t.Error("a versão não subiu numa escrita — o tmServer nunca releria")
	}
	if len(cfg.Areas) != 1 || cfg.Areas[0].Percent != 200 {
		t.Fatalf("li de volta %+v", cfg.Areas)
	}

	// Regravar é atualizar, não criar uma segunda linha.
	antes, tinha, err = s.SetSpawnRate(ctx, domain.SpawnRate{Area: 0, Percent: 150}, 0)
	if err != nil {
		t.Fatalf("SetSpawnRate: %v", err)
	}
	if !tinha || antes.Percent != 200 {
		t.Errorf("o valor anterior leu %+v (tinha=%v), quero 200%%", antes, tinha)
	}
	cfg, _ = s.SpawnRates(ctx)
	if len(cfg.Areas) != 1 {
		t.Fatalf("%d linhas depois de regravar, quero 1", len(cfg.Areas))
	}

	antes, tinha, err = s.DeleteSpawnRate(ctx, 0, 0)
	if err != nil {
		t.Fatalf("DeleteSpawnRate: %v", err)
	}
	if !tinha || antes.Percent != 150 {
		t.Errorf("apagou %+v (tinha=%v), quero 150%%", antes, tinha)
	}
	cfg, _ = s.SpawnRates(ctx)
	if len(cfg.Areas) != 0 {
		t.Fatalf("sobrou %+v depois de limpar", cfg.Areas)
	}
}

// TestSpawnRateRecusaValorAbsurdo: o período do timer de minuto é um módulo, e
// um ritmo que o zerasse dividiria por zero dentro do laço do jogo. O CHECK é a
// última linha de defesa, depois do painel.
func TestSpawnRateRecusaValorAbsurdo(t *testing.T) {
	ctx := context.Background()
	s := limparSpawnRate(t, ctx)
	for _, pct := range []int32{0, -1, 9, 1001} {
		if _, _, err := s.SetSpawnRate(ctx, domain.SpawnRate{Area: 0, Percent: pct}, 0); err == nil {
			t.Errorf("o banco aceitou %d%%", pct)
		}
	}
}

// TestSpawnRateVersionSobeACadaEscrita: tmServer only re-reads when the version
// moves, so a write that did not bump it would be a dial that turns and does
// nothing in game. Clearing counts too — a second "voltar ao conteúdo" that
// silently did nothing would leave somebody wondering whether the first worked.
func TestSpawnRateVersionSobeACadaEscrita(t *testing.T) {
	ctx := context.Background()
	s := limparSpawnRate(t, ctx)

	antes, _ := s.SpawnRateVersion(ctx)
	if _, _, err := s.SetSpawnRate(ctx, domain.SpawnRate{Area: 0, Percent: 200}, 0); err != nil {
		t.Fatalf("SetSpawnRate: %v", err)
	}
	if _, _, err := s.DeleteSpawnRate(ctx, 0, 0); err != nil {
		t.Fatalf("DeleteSpawnRate: %v", err)
	}
	if _, _, err := s.DeleteSpawnRate(ctx, 0, 0); err != nil {
		t.Fatalf("DeleteSpawnRate numa área sem linha: %v", err)
	}
	depois, _ := s.SpawnRateVersion(ctx)
	if depois != antes+3 {
		t.Fatalf("versão %d → %d depois de três escritas", antes, depois)
	}
}
