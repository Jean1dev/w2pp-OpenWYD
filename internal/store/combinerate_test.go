package store

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// O painel e as migrações moram em serviços diferentes: só o dbServer e o
// webServer rodam store.Migrate. Entre subir a tabela e o próximo boot do
// dbServer, o painel conversa com um banco que ainda não a tem — e essa janela
// virou uma tela de erro em produção. A resposta honesta ali é "nada editado",
// que é literalmente verdade e deixa cada máquina no CompRate.txt.
func TestTabelaAusenteReconhece42P01(t *testing.T) {
	casos := []struct {
		nome string
		err  error
		want bool
	}{
		{"relation does not exist", &pgconn.PgError{Code: "42P01"}, true},
		{"embrulhado", fmt.Errorf("store: combine rates: %w", &pgconn.PgError{Code: "42P01"}), true},
		{"outro erro de pg", &pgconn.PgError{Code: "23505"}, false},
		{"erro comum", errors.New("timeout"), false},
		{"nil", nil, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := TabelaAusente(c.err); got != c.want {
				t.Errorf("tabelaAusente = %v, esperado %v", got, c.want)
			}
		})
	}
}
