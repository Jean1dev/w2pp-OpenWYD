//go:build integration

// Integration tests for the combat rule (0044_combat_rule). They require a real
// database and are excluded from the default build. Run with:
//
//	W2PP_TEST_DSN=postgres://postgres:dev@localhost:5432/postgres go test -tags=integration ./internal/store/
package store

import (
	"context"
	"errors"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/combatrule"
)

func limparCombatRule(t *testing.T, ctx context.Context) *Store {
	t.Helper()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM combat_rule; UPDATE combat_rule_meta SET version = 0 WHERE id = TRUE`)
	return New(pool)
}

func TestCombatRuleCRUD(t *testing.T) {
	ctx := context.Background()
	s := limparCombatRule(t, ctx)

	// A tabela nasce vazia, e vazia é a regra decidida — não o valor zero, que
	// nem é uma regra válida (base de resistência 0).
	cfg, err := s.CombatRule(ctx)
	if err != nil {
		t.Fatalf("CombatRule: %v", err)
	}
	if cfg.Configured || cfg.Version != 0 || cfg.Rules != combatrule.Default() {
		t.Fatalf("a tabela nasce vazia e no padrão, li %+v", cfg)
	}

	// A primeira gravação não tinha linha, e `antes` precisa dizer isso para a
	// auditoria não escrever "estava em 0%" quando estava no padrão.
	antes, err := s.SetCombatRule(ctx, combatrule.Kersef(), 0)
	if err != nil {
		t.Fatalf("SetCombatRule: %v", err)
	}
	if antes.Configured || antes.Rules != combatrule.Default() {
		t.Errorf("o anterior de uma tabela vazia leu %+v", antes)
	}

	cfg, _ = s.CombatRule(ctx)
	if cfg.Version == 0 {
		t.Error("a versão não subiu numa escrita — o tmServer nunca releria")
	}
	if !cfg.Configured || cfg.Rules != combatrule.Kersef() {
		t.Fatalf("li de volta %+v", cfg)
	}

	// Regravar é atualizar a linha única, e o anterior é o que estava gravado.
	// Os pares novos diferentes entre si, do legado e do padrão, para uma coluna
	// trocada com a outra não passar despercebida.
	meio := combatrule.Rules{
		WeaponIntMagicPct: 40, SpellDamageMulti: false, MobResistBase: 120,
		PvPSkillPct: 60, PvPMeleePct: 80,
		SpellIntAccuracyPct: 30, MaxMissStreak: 4, WeaponDamageGrants: 2, DoubleCriticalMaxPct: 40,
	}
	antes, err = s.SetCombatRule(ctx, meio, 0)
	if err != nil {
		t.Fatalf("SetCombatRule: %v", err)
	}
	if !antes.Configured || antes.Rules != combatrule.Kersef() {
		t.Errorf("o anterior leu %+v, quero o Kersef gravado", antes)
	}
	if cfg, _ = s.CombatRule(ctx); cfg.Rules != meio {
		t.Fatalf("li de volta %+v, quero %+v", cfg.Rules, meio)
	}

	antes, err = s.ClearCombatRule(ctx, 0)
	if err != nil {
		t.Fatalf("ClearCombatRule: %v", err)
	}
	if !antes.Configured || antes.Rules != meio {
		t.Errorf("apagou %+v, quero %+v", antes, meio)
	}
	cfg, _ = s.CombatRule(ctx)
	if cfg.Configured || cfg.Rules != combatrule.Default() {
		t.Fatalf("depois de limpar li %+v, quero o padrão", cfg)
	}
}

// TestCombatRuleRecusaValorForaDaFaixa: o store confere antes do banco, e o
// CHECK é a última linha de defesa depois dele.
func TestCombatRuleRecusaValorForaDaFaixa(t *testing.T) {
	ctx := context.Background()
	s := limparCombatRule(t, ctx)
	// Cada caso é o padrão com UM botão fora da faixa, para ser recusado pelo
	// motivo que o nome diz e não por outro campo zerado.
	com := func(mudar func(*combatrule.Rules)) combatrule.Rules {
		r := combatrule.Default()
		mudar(&r)
		return r
	}
	for _, r := range []combatrule.Rules{
		com(func(r *combatrule.Rules) { r.WeaponIntMagicPct = 101 }),
		com(func(r *combatrule.Rules) { r.WeaponIntMagicPct = -1 }),
		com(func(r *combatrule.Rules) { r.MobResistBase = 49 }),
		com(func(r *combatrule.Rules) { r.MobResistBase = 151 }),
		com(func(r *combatrule.Rules) { r.PvPSkillPct = 0 }),
		com(func(r *combatrule.Rules) { r.PvPSkillPct = 201 }),
		com(func(r *combatrule.Rules) { r.PvPMeleePct = 0 }),
		com(func(r *combatrule.Rules) { r.PvPMeleePct = 201 }),
		com(func(r *combatrule.Rules) { r.SpellIntAccuracyPct = -1 }),
		com(func(r *combatrule.Rules) { r.SpellIntAccuracyPct = 101 }),
		com(func(r *combatrule.Rules) { r.MaxMissStreak = -1 }),
		com(func(r *combatrule.Rules) { r.MaxMissStreak = 11 }),
		com(func(r *combatrule.Rules) { r.WeaponDamageGrants = 0 }),
		com(func(r *combatrule.Rules) { r.WeaponDamageGrants = 4 }),
		com(func(r *combatrule.Rules) { r.DoubleCriticalMaxPct = -1 }),
		com(func(r *combatrule.Rules) { r.DoubleCriticalMaxPct = 101 }),
	} {
		if _, err := s.SetCombatRule(ctx, r, 0); !errors.Is(err, ErrInvalidCombatRule) {
			t.Errorf("SetCombatRule(%+v) = %v, quero ErrInvalidCombatRule", r, err)
		}
	}
	if v, _ := s.CombatRuleVersion(ctx); v != 0 {
		t.Errorf("uma escrita recusada subiu a versão para %d", v)
	}

	// E o CHECK, se alguém escrever direto no banco sem passar pelo store.
	for nome, sql := range map[string]string{
		"base de resistência 10": `INSERT INTO combat_rule (id, weapon_int_magic_pct, spell_damage_multi, mob_resist_base)
			VALUES (TRUE, 0, FALSE, 10)`,
		"skill em jogador 0%": `INSERT INTO combat_rule (id, weapon_int_magic_pct, spell_damage_multi, mob_resist_base, pvp_skill_pct)
			VALUES (TRUE, 0, FALSE, 100, 0)`,
		"golpe físico em jogador 201%": `INSERT INTO combat_rule (id, weapon_int_magic_pct, spell_damage_multi, mob_resist_base, pvp_melee_pct)
			VALUES (TRUE, 0, FALSE, 100, 201)`,
		"precisão da magia 101%": `INSERT INTO combat_rule (id, weapon_int_magic_pct, spell_damage_multi, mob_resist_base, spell_int_accuracy_pct)
			VALUES (TRUE, 0, FALSE, 100, 101)`,
		"11 erros seguidos": `INSERT INTO combat_rule (id, weapon_int_magic_pct, spell_damage_multi, mob_resist_base, max_miss_streak)
			VALUES (TRUE, 0, FALSE, 100, 11)`,
	} {
		if _, err := s.pool.Exec(ctx, sql); err == nil {
			t.Errorf("o banco aceitou %s", nome)
			_, _ = s.pool.Exec(ctx, `DELETE FROM combat_rule`) // o próximo caso precisa da tabela vazia
		}
	}
}

// TestLinhaAnteriorAoPvPContinuaValida é a linha gravada antes da 0045 e da
// 0046: ela não tem os campos de PvP nem os de precisão. O DEFAULT 100 do PvP é
// o que a mantém uma regra válida — voltando com zero, o tmServer descartaria a
// regra inteira. Os de precisão nascem no padrão DECIDIDO, não no legado: uma
// linha gravada como Kersef antes da 0046 passa a ter a precisão nova.
func TestLinhaAnteriorAoPvPContinuaValida(t *testing.T) {
	ctx := context.Background()
	s := limparCombatRule(t, ctx)
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO combat_rule (id, weapon_int_magic_pct, spell_damage_multi, mob_resist_base)
		VALUES (TRUE, 100, TRUE, 150)`); err != nil {
		t.Fatalf("gravar a linha sem os campos de PvP: %v", err)
	}
	cfg, err := s.CombatRule(ctx)
	if err != nil {
		t.Fatalf("CombatRule: %v", err)
	}
	if !cfg.Rules.Valid() {
		t.Fatalf("a linha anterior à 0045 leu %+v, que o jogo recusaria", cfg.Rules)
	}
	quer := combatrule.Kersef()
	quer.SpellIntAccuracyPct = combatrule.Default().SpellIntAccuracyPct
	quer.MaxMissStreak = combatrule.Default().MaxMissStreak
	quer.WeaponDamageGrants = combatrule.Default().WeaponDamageGrants
	quer.DoubleCriticalMaxPct = combatrule.Default().DoubleCriticalMaxPct
	if cfg.Rules != quer {
		t.Errorf("leu %+v, quero %+v: o Kersef com o legado no PvP e a precisão no padrão decidido",
			cfg.Rules, quer)
	}
}

// TestCombatRuleVersionSobeACadaEscrita: tmServer only re-reads when the version
// moves, so a write that did not bump it would be a knob that turns and does
// nothing in game. Clearing counts too, even with nothing to clear.
func TestCombatRuleVersionSobeACadaEscrita(t *testing.T) {
	ctx := context.Background()
	s := limparCombatRule(t, ctx)

	antes, _ := s.CombatRuleVersion(ctx)
	if _, err := s.SetCombatRule(ctx, combatrule.Kersef(), 0); err != nil {
		t.Fatalf("SetCombatRule: %v", err)
	}
	if _, err := s.ClearCombatRule(ctx, 0); err != nil {
		t.Fatalf("ClearCombatRule: %v", err)
	}
	if _, err := s.ClearCombatRule(ctx, 0); err != nil {
		t.Fatalf("ClearCombatRule sem linha: %v", err)
	}
	depois, _ := s.CombatRuleVersion(ctx)
	if depois != antes+3 {
		t.Fatalf("versão %d → %d depois de três escritas", antes, depois)
	}
}
