package migrations_test

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/migrations"
	"github.com/jeanluca/w2pp-openwyd/internal/mountbonus"
)

// TestMontariasBComoN checks migration 0050 against the screen it was copied
// from. The panel shows attack and magic at mount level 120, and the migration
// writes the coefficients behind them; this is the arithmetic in both
// directions, plus the two things the B mounts had to keep — their own
// immunity and no evasion.
func TestMontariasBComoN(t *testing.T) {
	b, err := migrations.FS.ReadFile("0050_montarias_b_como_n.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	// What /rates/montarias showed for the N horses (dano, magia at level 120).
	tela := map[int16][2]int{
		2371: {392, 67},  // ← Cavalo s/Sela N
		2372: {462, 87},  // ← Cavalo Fantasma N
		2373: {532, 94},  // ← Cavalo Leve N
		2374: {630, 103}, // ← Cavalo Equipado N
		2375: {812, 128}, // ← Andaluz N
	}
	linha := regexp.MustCompile(`\((237[1-5]), (\d+), (\d+), (\d+), (\d+), 'migração 0050`)
	achou := 0
	for _, m := range linha.FindAllStringSubmatch(string(b), -1) {
		idx64, _ := strconv.ParseInt(m[1], 10, 16)
		idx := int16(idx64)
		n := func(i int) int16 { v, _ := strconv.Atoi(m[i]); return int16(v) }
		got := mountbonus.Bonus{Attack: n(2), Magic: n(3), Evasion: n(4), Resist: n(5)}
		achou++

		dano, magia := got.AtLevel(120)
		if want := tela[idx]; dano != want[0] || magia != want[1] {
			t.Errorf("%d: no nível 120 dá %d/%d, a tela das N mostrava %d/%d", idx, dano, magia, want[0], want[1])
		}
		padrao, _ := mountbonus.Default(idx)
		if got.Resist != padrao.Resist {
			t.Errorf("%d: imunidade %d, o padrão da B é %d — não era para mexer", idx, got.Resist, padrao.Resist)
		}
		if got.Evasion != 0 || padrao.Evasion != 0 {
			t.Errorf("%d: evasão %d (padrão %d); as B não têm, a equipe põe à mão", idx, got.Evasion, padrao.Evasion)
		}
		if !got.Valid() {
			t.Errorf("%d: %+v fora das faixas do mount_bonus", idx, got)
		}
	}
	if achou != 5 {
		t.Fatalf("achei %d linhas de atributos na 0050, want as 5 montarias B", achou)
	}
	// Six growth bands for each of the five.
	faixas := regexp.MustCompile(`\((237[1-5]), ([0-5]), (\d+)\)`).FindAllStringSubmatch(string(b), -1)
	if len(faixas) != 30 {
		t.Errorf("achei %d faixas de crescimento, want 30 (5 montarias × 6 faixas)", len(faixas))
	}
}
