//go:build integration

package store

import (
	"context"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// TestRenascimentoDosChefesNoBanco: a linha que já existe ganha as 24 h da 0056
// ao migrar, o valor gravado volta como foi, e o CHECK recusa zero e mais de uma
// semana — a última defesa depois do painel e do serviço web.
func TestRenascimentoDosChefesNoBanco(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	resetTestSchema(ctx, pool)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	st := New(pool)
	inicial, err := st.WorldEventConfig(ctx)
	if err != nil || inicial.BossRespawnHours != domain.DefaultBossRespawnHours {
		t.Fatalf("config inicial = %+v/%v, want chefes em %d h", inicial, err, domain.DefaultBossRespawnHours)
	}

	cfg := domain.DefaultWorldEventConfig()
	cfg.BossRespawnHours = 12
	if err := st.UpsertWorldEventConfig(ctx, cfg, 0); err != nil {
		t.Fatalf("UpsertWorldEventConfig: %v", err)
	}
	got, err := st.WorldEventConfig(ctx)
	if err != nil || got.BossRespawnHours != 12 {
		t.Fatalf("voltou %+v/%v, want chefes em 12 h", got, err)
	}

	for _, horas := range []int32{0, 169} {
		ruim := domain.DefaultWorldEventConfig()
		ruim.BossRespawnHours = horas
		if err := st.UpsertWorldEventConfig(ctx, ruim, 0); err == nil {
			t.Errorf("o banco aceitou os chefes em %d h", horas)
		}
	}
}
