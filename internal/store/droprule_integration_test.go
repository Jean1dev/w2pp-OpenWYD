//go:build integration

// Integration tests for the Mesa de Drops (0049_drop_rule). They require a real
// database and are excluded from the default build. Run with:
//
//	W2PP_TEST_DSN=postgres://postgres:dev@localhost:5432/postgres go test -tags=integration ./internal/store/
package store

import (
	"context"
	"errors"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/droprule"
)

func limparDropRule(t *testing.T, ctx context.Context) *Store {
	t.Helper()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM drop_rule; UPDATE drop_rule_meta SET version = 0 WHERE id = TRUE`)
	return New(pool)
}

func TestDropRuleCRUD(t *testing.T) {
	ctx := context.Background()
	s := limparDropRule(t, ctx)

	cfg, err := s.DropRules(ctx)
	if err != nil {
		t.Fatalf("DropRules: %v", err)
	}
	if cfg.Version != 0 || len(cfg.Rules) != 0 {
		t.Fatalf("a mesa nasce vazia, li %+v", cfg)
	}

	ovo := droprule.Rule{Mob: "Dark_Shadow_", Item: 2316, Chance: 800}
	antes, existia, err := s.SetDropRule(ctx, ovo, 0)
	if err != nil {
		t.Fatalf("SetDropRule: %v", err)
	}
	if existia {
		t.Errorf("a primeira gravação disse que já existia: %+v", antes)
	}
	tirar := droprule.Rule{Mob: droprule.AllMobs, Item: 2405, Chance: 0}
	if _, _, err := s.SetDropRule(ctx, tirar, 0); err != nil {
		t.Fatalf("SetDropRule todos: %v", err)
	}

	cfg, _ = s.DropRules(ctx)
	if cfg.Version != 2 || len(cfg.Rules) != 2 {
		t.Fatalf("depois de duas gravações li %+v, want versão 2 e duas regras", cfg)
	}

	// Regravar o mesmo par atualiza, e o anterior é o que estava gravado.
	antes, existia, err = s.SetDropRule(ctx, droprule.Rule{Mob: "Dark_Shadow_", Item: 2316, Chance: 500}, 0)
	if err != nil || !existia || antes.Chance != 800 {
		t.Errorf("regravar: antes %+v existia %v err %v, want 8%% existente", antes, existia, err)
	}

	// Fora da faixa é recusado antes do banco.
	if _, _, err := s.SetDropRule(ctx, droprule.Rule{Mob: droprule.AllMobs, Item: 2405, Chance: 1}, 0); !errors.Is(err, ErrInvalidDropRule) {
		t.Errorf("'todos' com chance: err %v, want ErrInvalidDropRule", err)
	}

	antes, existia, err = s.DeleteDropRule(ctx, "Dark_Shadow_", 2316, 0)
	if err != nil || !existia || antes.Chance != 500 {
		t.Errorf("apagar: antes %+v existia %v err %v", antes, existia, err)
	}
	cfg, _ = s.DropRules(ctx)
	if len(cfg.Rules) != 1 || cfg.Rules[0] != tirar {
		t.Errorf("depois de apagar li %+v, want só a regra de todos", cfg.Rules)
	}
	v, _ := s.DropRuleVersion(ctx)
	if v != cfg.Version || v != 4 {
		t.Errorf("DropRuleVersion = %d, DropRules.Version = %d, want 4 nas duas", v, cfg.Version)
	}
}
