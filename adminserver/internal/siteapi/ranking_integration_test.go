//go:build integration

// Ranking de kills contra um PostgreSQL de verdade:
//
//	W2PP_TEST_DSN=postgres://... go test -tags=integration ./adminserver/internal/siteapi/
package siteapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// personagem cria um personagem para a conta, com kills e nível.
//
// Sem ON CONFLICT: a migração 0011 trocou o UNIQUE(name) por um índice único em
// (name, class_master), então não existe constraint de "name" para casar. O teste
// apaga estes personagens antes, o que já garante a inserção limpa. Cada conta
// recebe slots diferentes, porque o slot é por conta.
func personagem(t *testing.T, pool *pgxpool.Pool, contaID int64, slot int, nome string, nivel, kills int) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO character (account_id, slot, name, class, clan, level, class_master, tot_kill)
		VALUES ($1, $2, $3, 1, 2, $4, 2, $5)`,
		contaID, slot, nome, nivel, kills)
	if err != nil {
		t.Fatalf("personagem %q: %v", nome, err)
	}
}

func nomesDoRanking(t *testing.T, h http.Handler) []string {
	t.Helper()
	rec := chama(h, "GET", "/site/v1/ranking/kills?limite=200", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var r struct {
		Total  int `json:"total"`
		Linhas []struct {
			Nome  string `json:"nome"`
			Kills int32  `json:"kills"`
		} `json:"linhas"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(r.Linhas))
	for _, l := range r.Linhas {
		out = append(out, l.Nome)
	}
	if r.Total != len(r.Linhas) {
		t.Errorf("total = %d mas vieram %d linhas (com limite 200 deviam bater)", r.Total, len(r.Linhas))
	}
	return out
}

// §: o ranking mostra só quem deve aparecer, na ordem certa.
func TestRankingDeKillsFiltraEOrdena(t *testing.T) {
	pool := testPool(t)
	h := realAPI(t, pool)
	ctx := context.Background()

	// Limpa personagens de execuções anteriores deste teste.
	for _, n := range []string{"kr_top", "kr_meio", "kr_zero", "kr_nivelalto", "kr_bloqueado", "kr_staff"} {
		if _, err := pool.Exec(ctx, `DELETE FROM character WHERE name = $1`, n); err != nil {
			t.Fatal(err)
		}
	}

	normal := seed(t, pool, "kr_conta_normal", "senha1")
	outra := seed(t, pool, "kr_conta_outra", "senha2")
	bloqueada := seed(t, pool, "kr_conta_bloqueada", "senha3")
	staff := seed(t, pool, "kr_conta_staff", "senha4")
	if _, err := pool.Exec(ctx, `UPDATE account SET is_blocked = true, blocked_until = NULL WHERE id = $1`, bloqueada); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE account SET role = 'moderator' WHERE id = $1`, staff); err != nil {
		t.Fatal(err)
	}

	personagem(t, pool, normal, 0, "kr_top", 300, 50)          // entra, primeiro
	personagem(t, pool, outra, 0, "kr_meio", 200, 10)          // entra, segundo
	personagem(t, pool, normal, 1, "kr_zero", 300, 0)          // fora: ninguém matou ninguém
	personagem(t, pool, outra, 1, "kr_nivelalto", 1000, 99)    // fora: o mesmo corte do ranking de XP
	personagem(t, pool, bloqueada, 0, "kr_bloqueado", 300, 80) // fora: conta bloqueada
	personagem(t, pool, staff, 0, "kr_staff", 300, 90)         // fora: conta da equipe

	nomes := nomesDoRanking(t, h)

	pos := map[string]int{}
	for i, n := range nomes {
		pos[n] = i
	}
	for _, fora := range []string{"kr_zero", "kr_nivelalto", "kr_bloqueado", "kr_staff"} {
		if _, apareceu := pos[fora]; apareceu {
			t.Errorf("%s apareceu no ranking e não devia", fora)
		}
	}
	iTop, ok1 := pos["kr_top"]
	iMeio, ok2 := pos["kr_meio"]
	if !ok1 || !ok2 {
		t.Fatalf("faltou quem devia entrar: %v", nomes)
	}
	if iTop > iMeio {
		t.Errorf("ordem errada: quem tem mais kills tem que vir antes (%v)", nomes)
	}

	// O bloqueio com prazo VENCIDO não esconde ninguém: é a mesma regra do login.
	if _, err := pool.Exec(ctx, `UPDATE account SET blocked_until = now() - interval '1 day' WHERE id = $1`, bloqueada); err != nil {
		t.Fatal(err)
	}
	depois := nomesDoRanking(t, h)
	achou := false
	for _, n := range depois {
		if n == "kr_bloqueado" {
			achou = true
		}
	}
	if !achou {
		t.Errorf("com o bloqueio vencido o personagem devia voltar ao ranking: %v", depois)
	}
}
