package siteapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Kills answers the fake board and records what the handler asked for, so the
// test can check the limits without a database.
func (f *fakeBanco) Kills(_ context.Context, limite, deslocamento int) ([]KillRanking, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.killsLimite, f.killsDeslocamento = limite, deslocamento
	return f.kills, f.killsTotal, nil
}

func comKills(c *cenario) {
	c.banco.mu.Lock()
	defer c.banco.mu.Unlock()
	c.banco.kills = []KillRanking{
		{Nome: "Matador", Classe: 0, Evolucao: 2, Reino: 1, Nivel: 355, Kills: 120},
		{Nome: "Segundo", Classe: 3, Evolucao: 1, Reino: 2, Nivel: 200, Kills: 7},
	}
	c.banco.killsTotal = 2
}

func TestRankingDeKillsResponde(t *testing.T) {
	c := novoCenario(t)
	comKills(c)
	rec := c.pede("GET", "/site/v1/ranking/kills", "")
	confereStatus(t, rec, http.StatusOK, "")

	var r struct {
		Total  int         `json:"total"`
		Linhas []linhaKill `json:"linhas"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("corpo não é JSON: %v: %s", err, rec.Body.String())
	}
	if r.Total != 2 || len(r.Linhas) != 2 {
		t.Fatalf("total = %d, linhas = %d", r.Total, len(r.Linhas))
	}
	primeiro := r.Linhas[0]
	if primeiro.Nome != "Matador" || primeiro.Kills != 120 || primeiro.Nivel != 355 || primeiro.Evolucao != 2 || primeiro.Reino != 1 {
		t.Errorf("primeira linha = %+v", primeiro)
	}
	// Nada de conta: o ranking é público e não leva id de ninguém.
	for _, proibido := range []string{"conta", "account", "id\":", "email"} {
		if strings.Contains(rec.Body.String(), proibido) {
			t.Errorf("o ranking vazou %q: %s", proibido, rec.Body.String())
		}
	}
}

func TestRankingDeKillsSeguraOsLimites(t *testing.T) {
	c := novoCenario(t)
	comKills(c)
	casos := []struct {
		query          string
		limite, desloc int
	}{
		{"", limiteRankingPadrao, 0},                 // padrão
		{"?limite=10&deslocamento=20", 10, 20},       // o que foi pedido
		{"?limite=9999", limiteRankingMax, 0},        // teto
		{"?limite=0", 1, 0},                          // piso
		{"?limite=abc", limiteRankingPadrao, 0},      // lixo cai no padrão
		{"?deslocamento=-5", limiteRankingPadrao, 0}, // piso do deslocamento
	}
	for _, k := range casos {
		rec := c.pede("GET", "/site/v1/ranking/kills"+k.query, "")
		confereStatus(t, rec, http.StatusOK, "")
		c.banco.mu.Lock()
		lim, des := c.banco.killsLimite, c.banco.killsDeslocamento
		c.banco.mu.Unlock()
		if lim != k.limite || des != k.desloc {
			t.Errorf("%q: pediu limite=%d deslocamento=%d, esperado %d e %d", k.query, lim, des, k.limite, k.desloc)
		}
	}
}

func TestRankingDeKillsPrecisaDaChaveESoAceitaGET(t *testing.T) {
	c := novoCenario(t)
	comKills(c)
	if rec := c.pedeCom("GET", "/site/v1/ranking/kills", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("sem chave: status = %d, want 401", rec.Code)
	}
	if rec := c.pedeCom("GET", "/site/v1/ranking/kills", "", "Bearer outra"); rec.Code != http.StatusUnauthorized {
		t.Errorf("chave errada: status = %d, want 401", rec.Code)
	}
	// Fora da lista fechada: escrever no ranking não existe.
	for _, m := range []string{"POST", "PUT", "DELETE"} {
		if rec := c.pede(m, "/site/v1/ranking/kills", ""); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", m, rec.Code)
		}
	}
	// Vizinhas que não existem.
	for _, p := range []string{"/site/v1/ranking", "/site/v1/ranking/", "/site/v1/ranking/duelos", "/site/v1/ranking/kills/1"} {
		if rec := c.pede("GET", p, ""); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", p, rec.Code)
		}
	}
}
