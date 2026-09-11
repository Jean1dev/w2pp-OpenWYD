package migrations_test

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/migrations"
)

// O saque do campo de treino (0057) é o que o Marco pediu em 11/09/2026, e só
// para os três bichos que nascem só lá. Os números ficam presos aqui porque a
// Mesa de Drops os aceita calada: um 5000 no lugar de 500 daria Repletion em
// metade das mortes, e nada no jogo diria que foi engano.
func TestSaqueDoCampoDeTreino(t *testing.T) {
	b, err := migrations.FS.ReadFile("0057_campo_de_treino_drops.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`\('([^']+)',\s*(\d+),\s*(\d+)\)`)
	type chave struct {
		mob  string
		item int
	}
	regras := make(map[chave]int)
	for _, m := range re.FindAllStringSubmatch(string(b), -1) {
		item, _ := strconv.Atoi(m[2])
		chance, _ := strconv.Atoi(m[3])
		regras[chave{m[1], item}] = chance
	}

	const repletionA, ori, lac, bolsaDaSorte = 4016, 412, 413, 4104
	quer := map[chave]int{}
	for _, mob := range []string{"Porco", "Aguia", "Serpente"} {
		quer[chave{mob, repletionA}] = 500 // 5%
		quer[chave{mob, ori}] = 50         // 0,5%
		quer[chave{mob, lac}] = 20         // 0,2%
	}
	quer[chave{"Porco", bolsaDaSorte}] = 0
	quer[chave{"Aguia", bolsaDaSorte}] = 0

	for k, want := range quer {
		got, ok := regras[k]
		if !ok {
			t.Errorf("falta a regra %s/%d", k.mob, k.item)
			continue
		}
		if got != want {
			t.Errorf("%s/%d = %d, want %d", k.mob, k.item, got, want)
		}
	}
	for k := range regras {
		if _, ok := quer[k]; !ok {
			t.Errorf("regra a mais: %s/%d — Krill, Gremlin, Rei_Gremlin e Chefe_Krill "+
				"também nascem fora do campo, sem limite de nível", k.mob, k.item)
		}
	}
}
