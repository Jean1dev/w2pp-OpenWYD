package migrations_test

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/droprule"
	"github.com/jeanluca/w2pp-openwyd/internal/migrations"
)

// The drop_rule CHECKs and internal/droprule must agree: a stricter database
// kills a rule the panel accepted as "erro ao gravar", a looser one lets the
// game fetch a row it then refuses, while the panel shows it as in force.
func TestCheckDaMesaDeDropsBateComOCodigo(t *testing.T) {
	b, err := migrations.FS.ReadFile("0049_drop_rule.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(b)
	for _, c := range []struct {
		expr  string
		faixa [2]int
	}{
		{`item`, [2]int{droprule.MinItem, droprule.MaxItem}},
		{`chance`, [2]int{0, droprule.MaxChance}},
		{`length\(mob\)`, [2]int{1, droprule.MaxMobName}},
	} {
		re := regexp.MustCompile(`CHECK\s*\(\s*` + c.expr + `\s+BETWEEN\s+(\d+)\s+AND\s+(\d+)\s*\)`)
		m := re.FindStringSubmatch(sql)
		if m == nil {
			t.Errorf("não achei o CHECK de %s", c.expr)
			continue
		}
		lo, _ := strconv.Atoi(m[1])
		hi, _ := strconv.Atoi(m[2])
		if lo != c.faixa[0] || hi != c.faixa[1] {
			t.Errorf("CHECK de %s é %d..%d, mas internal/droprule diz %d..%d", c.expr, lo, hi, c.faixa[0], c.faixa[1])
		}
	}
	// "Todos os monstros" só tira: a regra com chance é recusada pelo banco também.
	if !regexp.MustCompile(`CHECK\s*\(\s*mob\s*<>\s*'\*'\s+OR\s+chance\s*=\s*0\s*\)`).MatchString(sql) {
		t.Error("falta o CHECK que limita '*' a 0%")
	}
	if droprule.AllMobs != "*" {
		t.Errorf("droprule.AllMobs = %q, e o CHECK usa '*'", droprule.AllMobs)
	}
}
