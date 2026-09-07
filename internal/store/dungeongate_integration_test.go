//go:build integration

// Integration tests for the dungeon doors (0035_dungeon_gate). They require a
// real database and are excluded from the default build. Run with:
//
//	W2PP_TEST_DSN=postgres://postgres:dev@localhost:5432/postgres go test -tags=integration ./internal/store/
package store

import (
	"context"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

func TestDungeonGateCRUD(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM dungeon_gate; UPDATE dungeon_gate_meta SET version = 0 WHERE id = TRUE`)

	s := New(pool)

	cfg, err := s.DungeonGates(ctx)
	if err != nil {
		t.Fatalf("DungeonGates: %v", err)
	}
	if len(cfg.Gates) != 0 || cfg.Version != 0 {
		t.Fatalf("a tabela nasce vazia, li %+v", cfg)
	}

	// The previous value of an untouched door reads as open and announced, which
	// is what the audit entry has to show — not the zero of the type.
	antes, err := s.SetDungeonGate(ctx, domain.DungeonGate{Gate: 1, Open: false, Announce: true}, 0)
	if err != nil {
		t.Fatalf("SetDungeonGate: %v", err)
	}
	if !antes.Open || !antes.Announce {
		t.Errorf("o estado anterior de uma porta intocada leu %+v, quero aberta e avisando", antes)
	}

	cfg, _ = s.DungeonGates(ctx)
	if cfg.Version == 0 {
		t.Error("a versão não subiu numa escrita — o tmServer nunca releria")
	}
	if len(cfg.Gates) != 1 || cfg.Gates[0].Open || !cfg.Gates[0].Announce {
		t.Fatalf("li de volta %+v", cfg.Gates)
	}

	// The two flags are independent: a door can be open and quiet.
	if _, err := s.SetDungeonGate(ctx, domain.DungeonGate{Gate: 4, Open: true, Announce: false}, 0); err != nil {
		t.Fatalf("SetDungeonGate: %v", err)
	}
	cfg, _ = s.DungeonGates(ctx)
	for _, g := range cfg.Gates {
		if g.Gate == 4 && (!g.Open || g.Announce) {
			t.Errorf("a porta 4 devia estar aberta e muda, está %+v", g)
		}
	}

	// Reopening is an update, not a second row.
	if _, err := s.SetDungeonGate(ctx, domain.DungeonGate{Gate: 1, Open: true, Announce: true}, 0); err != nil {
		t.Fatalf("SetDungeonGate: %v", err)
	}
	cfg, _ = s.DungeonGates(ctx)
	if len(cfg.Gates) != 2 {
		t.Fatalf("%d linhas depois de reabrir a porta 1, quero 2", len(cfg.Gates))
	}
}

// TestDungeonGateVersionSobeACadaEscrita: tmServer only re-reads when the
// version moves, so a write that did not bump it would be a door that never
// closes in game.
func TestDungeonGateVersionSobeACadaEscrita(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM dungeon_gate; UPDATE dungeon_gate_meta SET version = 0 WHERE id = TRUE`)
	s := New(pool)

	antes, _ := s.DungeonGateVersion(ctx)
	for i := range 3 {
		if _, err := s.SetDungeonGate(ctx, domain.DungeonGate{Gate: int32(i), Open: false}, 0); err != nil {
			t.Fatalf("SetDungeonGate: %v", err)
		}
	}
	depois, _ := s.DungeonGateVersion(ctx)
	if depois != antes+3 {
		t.Fatalf("versão %d → %d depois de três escritas", antes, depois)
	}
}
