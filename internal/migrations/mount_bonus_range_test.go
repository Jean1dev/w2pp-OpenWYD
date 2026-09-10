package migrations_test

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/migrations"
	"github.com/jeanluca/w2pp-openwyd/internal/mountbonus"
)

// The mount_bonus CHECKs and internal/mountbonus must agree on every ceiling.
//
// Three places validate these numbers — the panel form, the web service and the
// database — and the XP zone outage came from exactly this shape: code widened,
// CHECK not, and production refusing a save that every test had passed. If the
// SQL is stricter, a value the panel accepts dies in Postgres as "erro ao
// gravar"; if it is looser, the database stops being the last word.
func TestCheckDosAtributosDeMontariaBateComOCodigo(t *testing.T) {
	b, err := migrations.FS.ReadFile("0043_mount_bonus.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for coluna, teto := range map[string]int{
		"attack":  mountbonus.MaxAttack,
		"magic":   mountbonus.MaxMagic,
		"evasion": mountbonus.MaxEvasion,
		"resist":  mountbonus.MaxResist,
	} {
		re := regexp.MustCompile(`CHECK\s*\(\s*` + coluna + `\s+BETWEEN\s+0\s+AND\s+(\d+)\s*\)`)
		m := re.FindStringSubmatch(string(b))
		if m == nil {
			t.Errorf("não achei o CHECK de %s na 0043", coluna)
			continue
		}
		if n, _ := strconv.Atoi(m[1]); n != teto {
			t.Errorf("CHECK de %s vai até %d, mas internal/mountbonus diz %d", coluna, n, teto)
		}
	}
	re := regexp.MustCompile(`CHECK\s*\(\s*mount_index\s+BETWEEN\s+(\d+)\s+AND\s+(\d+)\s*\)`)
	m := re.FindStringSubmatch(string(b))
	if m == nil || m[1] != strconv.Itoa(mountbonus.AdultLo) || m[2] != strconv.Itoa(mountbonus.AdultHi) {
		t.Errorf("CHECK de mount_index = %v, want %d..%d", m, mountbonus.AdultLo, mountbonus.AdultHi)
	}
}
