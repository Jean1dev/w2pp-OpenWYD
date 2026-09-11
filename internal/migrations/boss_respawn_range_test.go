package migrations_test

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/migrations"
)

// TestRenascimentoDosChefesBateComOCodigo: o CHECK e o DEFAULT da 0056 têm de
// bater com internal/domain. Um CHECK mais estreito faz o painel morrer no
// Postgres ao salvar; um DEFAULT diferente faz um servidor com a linha e outro
// sem ela devolverem os chefes em prazos que ninguém escolheu.
func TestRenascimentoDosChefesBateComOCodigo(t *testing.T) {
	b, err := migrations.FS.ReadFile("0056_boss_respawn.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(b)

	m := regexp.MustCompile(`CHECK\s*\(\s*boss_respawn_hours\s+BETWEEN\s+(\d+)\s+AND\s+(\d+)\s*\)`).FindStringSubmatch(sql)
	if m == nil {
		t.Fatal("não achei o CHECK de boss_respawn_hours na 0056")
	}
	if lo, _ := strconv.Atoi(m[1]); lo != domain.MinBossRespawnHours {
		t.Errorf("CHECK de boss_respawn_hours começa em %d, mas domain.MinBossRespawnHours diz %d", lo, domain.MinBossRespawnHours)
	}
	if hi, _ := strconv.Atoi(m[2]); hi != domain.MaxBossRespawnHours {
		t.Errorf("CHECK de boss_respawn_hours vai até %d, mas domain.MaxBossRespawnHours diz %d", hi, domain.MaxBossRespawnHours)
	}

	d := regexp.MustCompile(`ADD COLUMN\s+boss_respawn_hours\s+SMALLINT\s+NOT NULL\s+DEFAULT\s+(\d+)`).FindStringSubmatch(sql)
	if d == nil {
		t.Fatal("boss_respawn_hours precisa ser SMALLINT NOT NULL DEFAULT <horas>")
	}
	if h, _ := strconv.Atoi(d[1]); h != domain.DefaultBossRespawnHours {
		t.Errorf("DEFAULT de boss_respawn_hours é %d, mas domain.DefaultBossRespawnHours diz %d", h, domain.DefaultBossRespawnHours)
	}
}
