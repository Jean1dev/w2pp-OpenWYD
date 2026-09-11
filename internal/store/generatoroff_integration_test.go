//go:build integration

// Integration tests for the NPCGener block switches (0048_npc_generator_off).
// They require a real database. Run with:
//
//	W2PP_TEST_DSN=postgres://postgres:dev@localhost:5432/postgres go test -tags=integration ./internal/store/
package store

import (
	"context"
	"testing"
)

func TestGeneratorOffLigaEDesliga(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM npc_generator_off; UPDATE npc_generator_off_meta SET version = 0 WHERE id = TRUE`)
	s := New(pool)

	cfg, err := s.GeneratorsOff(ctx)
	if err != nil {
		t.Fatalf("GeneratorsOff: %v", err)
	}
	if len(cfg.Off) != 0 || cfg.Version != 0 {
		t.Fatalf("a tabela nasce vazia, li %+v", cfg)
	}

	if err := s.SetGeneratorOff(ctx, 6077, true, "gm"); err != nil {
		t.Fatalf("desligar: %v", err)
	}
	cfg, _ = s.GeneratorsOff(ctx)
	if len(cfg.Off) != 1 || cfg.Off[0].Index != 6077 || cfg.Off[0].By != "gm" || cfg.Version != 1 {
		t.Fatalf("depois de desligar o 6077, li %+v", cfg)
	}
	if v, _ := s.GeneratorOffVersion(ctx); v != cfg.Version {
		t.Errorf("GeneratorOffVersion = %d, o snapshot diz %d", v, cfg.Version)
	}

	// Switching off twice keeps one row; switching on removes it.
	if err := s.SetGeneratorOff(ctx, 6077, true, "outro"); err != nil {
		t.Fatalf("desligar de novo: %v", err)
	}
	if err := s.SetGeneratorOff(ctx, 6077, false, "gm"); err != nil {
		t.Fatalf("ligar: %v", err)
	}
	cfg, _ = s.GeneratorsOff(ctx)
	if len(cfg.Off) != 0 || cfg.Version != 3 {
		t.Fatalf("depois de ligar, li %+v (quero vazio, versão 3)", cfg)
	}

	if err := s.SetGeneratorOff(ctx, -1, true, "gm"); err == nil {
		t.Error("índice negativo foi aceito")
	}
}
