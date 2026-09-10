package migrations_test

import (
	"regexp"
	"strconv"
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
	b, err := migrations.FS.ReadFile("0044_combat_rule.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for coluna, faixa := range map[string][2]int{
		"weapon_int_magic_pct": {combatrule.MinWeaponIntMagicPct, combatrule.MaxWeaponIntMagicPct},
		"mob_resist_base":      {combatrule.MinMobResistBase, combatrule.MaxMobResistBase},
	} {
		re := regexp.MustCompile(`CHECK\s*\(\s*` + coluna + `\s+BETWEEN\s+(\d+)\s+AND\s+(\d+)\s*\)`)
		m := re.FindStringSubmatch(string(b))
		if m == nil {
			t.Errorf("não achei o CHECK de %s na 0044", coluna)
			continue
		}
		lo, _ := strconv.Atoi(m[1])
		hi, _ := strconv.Atoi(m[2])
		if lo != faixa[0] || hi != faixa[1] {
			t.Errorf("CHECK de %s é %d..%d, mas internal/combatrule diz %d..%d",
				coluna, lo, hi, faixa[0], faixa[1])
		}
	}
}
