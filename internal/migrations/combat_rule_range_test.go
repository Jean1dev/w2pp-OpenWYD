package migrations_test

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/combatrule"
	"github.com/jeanluca/w2pp-openwyd/internal/migrations"
)

// The combat_rule CHECKs and internal/combatrule must agree on every range.
//
// The panel form, the store's Valid() and the database all refuse a value out
// of range. If the SQL is stricter, a number the panel accepts dies in Postgres
// as "erro ao gravar"; if it is looser, the database stops being the last word —
// and tmServer would then fetch a row it refuses whole, keeping the old rule
// while the panel shows the new one.
func TestCheckDaRegraDeCombateBateComOCodigo(t *testing.T) {
	casos := []struct {
		arquivo string
		coluna  string
		faixa   [2]int
	}{
		{"0044_combat_rule.up.sql", "weapon_int_magic_pct", [2]int{combatrule.MinWeaponIntMagicPct, combatrule.MaxWeaponIntMagicPct}},
		{"0044_combat_rule.up.sql", "mob_resist_base", [2]int{combatrule.MinMobResistBase, combatrule.MaxMobResistBase}},
		{"0045_combat_rule_pvp.up.sql", "pvp_skill_pct", [2]int{combatrule.MinPvPPct, combatrule.MaxPvPPct}},
		{"0045_combat_rule_pvp.up.sql", "pvp_melee_pct", [2]int{combatrule.MinPvPPct, combatrule.MaxPvPPct}},
		{"0046_combat_rule_precisao.up.sql", "spell_int_accuracy_pct", [2]int{combatrule.MinSpellIntAccuracy, combatrule.MaxSpellIntAccuracy}},
		{"0046_combat_rule_precisao.up.sql", "max_miss_streak", [2]int{combatrule.MinMissStreak, combatrule.MaxMissStreak}},
	}
	for _, c := range casos {
		b, err := migrations.FS.ReadFile(c.arquivo)
		if err != nil {
			t.Fatal(err)
		}
		re := regexp.MustCompile(`CHECK\s*\(\s*` + c.coluna + `\s+BETWEEN\s+(\d+)\s+AND\s+(\d+)\s*\)`)
		m := re.FindStringSubmatch(string(b))
		if m == nil {
			t.Errorf("não achei o CHECK de %s na %s", c.coluna, c.arquivo)
			continue
		}
		lo, _ := strconv.Atoi(m[1])
		hi, _ := strconv.Atoi(m[2])
		if lo != c.faixa[0] || hi != c.faixa[1] {
			t.Errorf("CHECK de %s é %d..%d, mas internal/combatrule diz %d..%d",
				c.coluna, lo, hi, c.faixa[0], c.faixa[1])
		}
	}
}

// TestColunasDePvPNascemNoLegado: a 0045 acrescenta colunas a uma tabela que já
// pode ter linha gravada. Sem DEFAULT 100 essa linha voltaria com zero — fora da
// faixa —, e o tmServer descartaria a regra inteira calado.
func TestColunasDePvPNascemNoLegado(t *testing.T) {
	b, err := migrations.FS.ReadFile("0045_combat_rule_pvp.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(b)
	for _, coluna := range []string{"pvp_skill_pct", "pvp_melee_pct"} {
		re := regexp.MustCompile(`ADD COLUMN\s+` + coluna + `\s+[^,;]*`)
		def := re.FindString(sql)
		if def == "" {
			t.Errorf("a 0045 não acrescenta %s", coluna)
			continue
		}
		if !strings.Contains(def, "NOT NULL") || !strings.Contains(def, "DEFAULT 100") {
			t.Errorf("%s precisa de NOT NULL DEFAULT 100 (o legado), achei: %s", coluna, def)
		}
	}
}

// TestColunasDePrecisaoNascemNoPadraoDecidido: ao contrário da 0045, os DEFAULTs
// da 0046 são o padrão DECIDIDO (combatrule.Default), não o legado — uma linha
// já gravada passa a ter a precisão nova. Se o padrão do código mudar e o da
// migração não, um servidor sem linha e um com linha antiga rodariam precisões
// diferentes sem ninguém ter escolhido isso.
func TestColunasDePrecisaoNascemNoPadraoDecidido(t *testing.T) {
	b, err := migrations.FS.ReadFile("0046_combat_rule_precisao.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(b)
	padrao := combatrule.Default()
	for coluna, quer := range map[string]int32{
		"spell_int_accuracy_pct": padrao.SpellIntAccuracyPct,
		"max_miss_streak":        padrao.MaxMissStreak,
	} {
		re := regexp.MustCompile(`ADD COLUMN\s+` + coluna + `\s+[^,;]*`)
		def := re.FindString(sql)
		if def == "" {
			t.Errorf("a 0046 não acrescenta %s", coluna)
			continue
		}
		m := regexp.MustCompile(`DEFAULT\s+(\d+)`).FindStringSubmatch(def)
		if !strings.Contains(def, "NOT NULL") || m == nil {
			t.Errorf("%s precisa de NOT NULL DEFAULT, achei: %s", coluna, def)
			continue
		}
		if n, _ := strconv.Atoi(m[1]); int32(n) != quer {
			t.Errorf("DEFAULT de %s é %d, mas combatrule.Default() diz %d", coluna, n, quer)
		}
	}
}
