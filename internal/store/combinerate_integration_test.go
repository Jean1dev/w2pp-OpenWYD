//go:build integration

// Integration tests for the Mesa das Máquinas (0040_combine_rate and
// 0041_combine_band_compositor). They require a real database and are excluded
// from the default build. Run with:
//
//	W2PP_TEST_DSN=postgres://postgres:dev@localhost:5432/postgres go test -tags=integration ./internal/store/
package store

import (
	"context"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

func limparCombineRate(t *testing.T, ctx context.Context) *Store {
	t.Helper()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM combine_band; DELETE FROM combine_rate; UPDATE combine_rate_meta SET version = 0 WHERE id = TRUE`)
	return New(pool)
}

// TestCompositorTemCurvaPropria is what 0041 exists for: the compositor's bands
// (kinds 3 and 4) are accepted by the database, read back with their kind, and
// replacing them leaves the +10's curve alone.
func TestCompositorTemCurvaPropria(t *testing.T) {
	ctx := context.Background()
	s := limparCombineRate(t, ctx)

	mais10 := []domain.CombineBand{{SlotKind: domain.CombineSlotWeapon, ReqLvlMin: 0, ReqLvlMax: 99, Label: "Armas D", MultPct: 90}}
	if _, err := s.SetCombineBands(ctx, domain.CombineSlotWeapon, mais10, 0); err != nil {
		t.Fatalf("SetCombineBands +10: %v", err)
	}
	comp := []domain.CombineBand{{SlotKind: domain.CombineSlotCompositorWeapon, ReqLvlMin: 0, ReqLvlMax: 99, Label: "Armas D", MultPct: 50}}
	if _, err := s.SetCombineBands(ctx, domain.CombineSlotCompositorWeapon, comp, 0); err != nil {
		t.Fatalf("SetCombineBands compositor: a restrição de 0041 recusou o tipo 3? %v", err)
	}
	if _, err := s.SetCombineBands(ctx, domain.CombineSlotCompositorArmour, nil, 0); err != nil {
		t.Fatalf("SetCombineBands compositor armadura: %v", err)
	}

	cfg, err := s.CombineRates(ctx)
	if err != nil {
		t.Fatalf("CombineRates: %v", err)
	}
	porTipo := map[domain.CombineSlotKind]int32{}
	for _, b := range cfg.Bands {
		porTipo[b.SlotKind] = b.MultPct
	}
	if porTipo[domain.CombineSlotWeapon] != 90 {
		t.Errorf("a curva da +10 mudou: %+v", cfg.Bands)
	}
	if porTipo[domain.CombineSlotCompositorWeapon] != 50 {
		t.Errorf("a curva do compositor não voltou com o tipo 3: %+v", cfg.Bands)
	}
}
