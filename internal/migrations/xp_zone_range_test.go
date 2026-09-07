package migrations_test

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/internal/migrations"
)

// The xp_rule CHECK has to admit every zone the code can produce.
//
// This is the test that was missing when the Deserto zones were added: the Go
// side gained zones 7..11, every unit test and the whole build stayed green,
// and the panel only found out in production, where Postgres rejected the
// insert and the screen could say no more than "erro ao gravar a zona Deserto
// Pilar". Nothing here needs a database — it reads the shipped SQL and compares
// it to level.Zones(), which is the disagreement that caused the outage.
func TestCheckDeZonaCobreTodasAsZonas(t *testing.T) {
	entradas, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	// The effective bound is the one the LAST migration to touch it sets, so
	// the files are read in name order and the final match wins.
	re := regexp.MustCompile(`CHECK\s*\(\s*zone\s+BETWEEN\s+0\s+AND\s+(\d+)\s*\)`)
	teto := -1
	var origem string
	for _, e := range entradas {
		nome := e.Name()
		if len(nome) < 7 || nome[len(nome)-7:] != ".up.sql" {
			continue
		}
		b, err := migrations.FS.ReadFile(nome)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			n, err := strconv.Atoi(m[1])
			if err != nil {
				t.Fatalf("%s: teto ilegível %q", nome, m[1])
			}
			teto, origem = n, nome
		}
	}
	if teto < 0 {
		t.Fatal("não achei o CHECK de zone em migração nenhuma — o teste parou de vigiar o que devia")
	}

	maiorZona := len(level.Zones()) - 1
	if teto < maiorZona {
		t.Errorf("o CHECK em %s aceita zone até %d, mas o código já tem a zona %d (%s). "+
			"Gravar essa zona falha no banco e o painel só consegue dizer «erro ao gravar».",
			origem, teto, maiorZona, level.Zone(maiorZona).Name())
	}
}
