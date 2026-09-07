//go:build integration

// Integration tests for the drop-bonus ladders (0037_drop_bonus). They require a
// real database and are excluded from the default build. Run with:
//
//	W2PP_TEST_DSN=postgres://postgres:dev@localhost:5432/postgres go test -tags=integration ./internal/store/
package store

import (
	"context"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

func bonusBase(d int32) domain.DropBonusBand {
	return domain.DropBonusBand{
		Distancia: d,
		Limite:    [4]int32{5, 15, 40, 70},
		Degrau:    [5]int32{5, 4, 3, 2, 1},
		Refino:    [4]int32{10, 30, 80, 95},
	}
}

func limparDropBonus(ctx context.Context, t *testing.T, s *Store) {
	t.Helper()
	if _, err := s.pool.Exec(ctx,
		`DELETE FROM drop_bonus; UPDATE drop_bonus_meta SET version = 0, ligado = TRUE WHERE id = TRUE`); err != nil {
		t.Fatalf("limpar drop_bonus: %v", err)
	}
}

func TestDropBonusCRUD(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	s := New(pool)
	limparDropBonus(ctx, t, s)

	cfg, err := s.DropBonus(ctx)
	if err != nil {
		t.Fatalf("DropBonus: %v", err)
	}
	if len(cfg.Faixas) != 0 || cfg.Version != 0 {
		t.Fatalf("a tabela nasce vazia, li %+v", cfg)
	}
	if !cfg.Ligado {
		t.Error("o sorteio nasce desligado; devia nascer ligado")
	}

	// "Não havia linha" e "estava zerada" são coisas diferentes, e a auditoria
	// depende de saber qual foi.
	_, tinha, err := s.SetDropBonus(ctx, bonusBase(0), 0)
	if err != nil {
		t.Fatalf("SetDropBonus: %v", err)
	}
	if tinha {
		t.Error("disse que já havia linha numa tabela vazia")
	}

	antes, tinha, err := s.SetDropBonus(ctx, bonusBase(0), 0)
	if err != nil {
		t.Fatalf("SetDropBonus: %v", err)
	}
	if !tinha || antes.Refino[0] != 10 {
		t.Errorf("a linha anterior voltou como %+v (tinha=%v)", antes, tinha)
	}

	cfg, err = s.DropBonus(ctx)
	if err != nil {
		t.Fatalf("DropBonus: %v", err)
	}
	if len(cfg.Faixas) != 1 {
		t.Fatalf("li %d faixas, queria 1", len(cfg.Faixas))
	}
	// Os três vetores viajam inteiros: um deles chegando curto é o defeito que
	// faria o sorteio rolar zeros em silêncio.
	got := cfg.Faixas[0]
	if got != bonusBase(0) {
		t.Errorf("voltou %+v, gravei %+v", got, bonusBase(0))
	}
	if cfg.Version == 0 {
		t.Error("a versão não subiu depois de duas gravações")
	}

	antes, tinha, err = s.DeleteDropBonus(ctx, 0, 0)
	if err != nil {
		t.Fatalf("DeleteDropBonus: %v", err)
	}
	if !tinha || antes.Refino[0] != 10 {
		t.Errorf("o apagado voltou como %+v (tinha=%v)", antes, tinha)
	}
	cfg, _ = s.DropBonus(ctx)
	if len(cfg.Faixas) != 0 {
		t.Errorf("sobrou faixa depois de apagar: %+v", cfg.Faixas)
	}
}

func TestDropBonusInterruptor(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	s := New(pool)
	limparDropBonus(ctx, t, s)

	v0, err := s.DropBonusVersion(ctx)
	if err != nil {
		t.Fatalf("DropBonusVersion: %v", err)
	}
	antes, err := s.SetDropBonusLigado(ctx, false)
	if err != nil {
		t.Fatalf("SetDropBonusLigado: %v", err)
	}
	if !antes {
		t.Error("o estado anterior devia ser ligado")
	}
	cfg, _ := s.DropBonus(ctx)
	if cfg.Ligado {
		t.Error("continuou ligado depois de desligar")
	}
	// A versão precisa subir também aqui: o tmServer decide se relê comparando
	// ela, e um interruptor que não mexe na versão passa despercebido.
	if cfg.Version <= v0 {
		t.Errorf("a versão não subiu com o interruptor: %d, era %d", cfg.Version, v0)
	}
}

// TestDropBonusRecusaEscadaForaDeOrdem is the constraint's own test. A threshold
// that does not advance deletes a rung of the ladder in silence, and the only
// symptom is an outcome that stops happening — so the database refuses it even
// if a future caller forgets to.
func TestDropBonusRecusaEscadaForaDeOrdem(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	s := New(pool)
	limparDropBonus(ctx, t, s)

	casos := []struct {
		nome  string
		monta func(domain.DropBonusBand) domain.DropBonusBand
	}{
		{"magnitude fora de ordem", func(b domain.DropBonusBand) domain.DropBonusBand {
			b.Limite = [4]int32{40, 30, 20, 10}
			return b
		}},
		{"refino fora de ordem", func(b domain.DropBonusBand) domain.DropBonusBand {
			b.Refino = [4]int32{90, 80, 70, 60}
			return b
		}},
		{"limite acima de cem", func(b domain.DropBonusBand) domain.DropBonusBand {
			b.Limite = [4]int32{5, 15, 40, 101}
			return b
		}},
		{"faixa que nao existe", func(b domain.DropBonusBand) domain.DropBonusBand {
			b.Distancia = 4
			return b
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if _, _, err := s.SetDropBonus(ctx, c.monta(bonusBase(1)), 0); err == nil {
				t.Error("o banco aceitou")
			}
		})
	}
}

// TestDropBonusPadraoPassaNoBanco keeps the shipped defaults and the table's
// constraints from drifting apart: a default the database would refuse could
// never be written back after somebody edited it.
func TestDropBonusPadraoPassaNoBanco(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	s := New(pool)
	limparDropBonus(ctx, t, s)

	for _, b := range domain.DropBonusDefaults {
		if _, _, err := s.SetDropBonus(ctx, b, 0); err != nil {
			t.Errorf("o padrão da faixa %d não passa no banco: %v", b.Distancia, err)
		}
	}
}
