package migrations_test

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/migrations"
)

// TestHorarioDaGuerraDeTorresBateComOCodigo: the 0051 CHECK and DEFAULTs must
// agree with internal/domain. A stricter CHECK makes the panel's save die in
// Postgres; a DEFAULT that drifts from domain.DefaultTowerWar* makes a server
// with a row and a server without one run the war at different hours, with
// nobody having chosen either.
func TestHorarioDaGuerraDeTorresBateComOCodigo(t *testing.T) {
	b, err := migrations.FS.ReadFile("0051_tower_war_schedule.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(b)

	m := regexp.MustCompile(`CHECK\s*\(\s*tower_war_hour\s+BETWEEN\s+(\d+)\s+AND\s+(\d+)\s*\)`).FindStringSubmatch(sql)
	if m == nil {
		t.Fatal("não achei o CHECK de tower_war_hour na 0051")
	}
	if lo, _ := strconv.Atoi(m[1]); lo != 0 {
		t.Errorf("CHECK de tower_war_hour começa em %d, want 0", lo)
	}
	if hi, _ := strconv.Atoi(m[2]); hi != domain.MaxTowerWarHour {
		t.Errorf("CHECK de tower_war_hour vai até %d, mas domain.MaxTowerWarHour diz %d", hi, domain.MaxTowerWarHour)
	}

	hora := regexp.MustCompile(`ADD COLUMN\s+tower_war_hour\s+SMALLINT\s+NOT NULL\s+DEFAULT\s+(\d+)`).FindStringSubmatch(sql)
	if hora == nil {
		t.Fatal("tower_war_hour precisa ser SMALLINT NOT NULL DEFAULT <hora>")
	}
	if h, _ := strconv.Atoi(hora[1]); h != domain.DefaultTowerWarHour {
		t.Errorf("DEFAULT de tower_war_hour é %d, mas domain.DefaultTowerWarHour diz %d", h, domain.DefaultTowerWarHour)
	}

	liga := regexp.MustCompile(`ADD COLUMN\s+tower_war_enabled\s+BOOLEAN\s+NOT NULL\s+DEFAULT\s+(TRUE|FALSE)`).FindStringSubmatch(sql)
	if liga == nil {
		t.Fatal("tower_war_enabled precisa ser BOOLEAN NOT NULL DEFAULT TRUE|FALSE")
	}
	if (liga[1] == "TRUE") != domain.DefaultTowerWarEnabled {
		t.Errorf("DEFAULT de tower_war_enabled é %s, mas domain.DefaultTowerWarEnabled diz %v", liga[1], domain.DefaultTowerWarEnabled)
	}
}
