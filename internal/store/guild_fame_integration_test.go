//go:build integration

// Integration tests for guild fame persistence. They require a real database
// and are excluded from the default build. Run with:
//
//	W2PP_TEST_DSN=postgres://postgres:dev@localhost:5432/postgres go test -tags=integration ./internal/store/
package store

import (
	"context"
	"errors"
	"testing"
)

// guildaDeTeste is an id far from the allocator's low range, so the test never
// collides with a guild another test created.
const guildaDeTeste = 65001

func TestUpdateGuildFameSobreviveAoReinicio(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	limpar := func() { _, _ = pool.Exec(ctx, `DELETE FROM guild WHERE id = $1`, guildaDeTeste) }
	limpar()
	t.Cleanup(limpar)
	if _, err := pool.Exec(ctx,
		`INSERT INTO guild (id, name, fame) VALUES ($1, 'fama_teste', 10)`, guildaDeTeste); err != nil {
		t.Fatalf("seed guild: %v", err)
	}

	st := New(pool)
	if err := st.UpdateGuildFame(ctx, guildaDeTeste, 110); err != nil {
		t.Fatalf("UpdateGuildFame: %v", err)
	}
	// "Reiniciar" é ler de novo do banco pelo mesmo caminho do boot do tmServer.
	guildas, err := st.ListGuilds(ctx)
	if err != nil {
		t.Fatalf("ListGuilds: %v", err)
	}
	achou := false
	for _, g := range guildas {
		if g.ID == guildaDeTeste {
			achou = true
			if g.Fame != 110 {
				t.Errorf("fama depois de gravar = %d, want 110", g.Fame)
			}
		}
	}
	if !achou {
		t.Fatal("a guilda de teste sumiu da lista")
	}

	// É um valor absoluto: gravar de novo o mesmo número não soma nada.
	if err := st.UpdateGuildFame(ctx, guildaDeTeste, 110); err != nil {
		t.Fatalf("UpdateGuildFame repetido: %v", err)
	}
	var fama int32
	if err := pool.QueryRow(ctx, `SELECT fame FROM guild WHERE id = $1`, guildaDeTeste).Scan(&fama); err != nil {
		t.Fatal(err)
	}
	if fama != 110 {
		t.Errorf("fama depois de repetir = %d, want 110 (não é soma)", fama)
	}
}

func TestUpdateGuildFameDeGuildaQueNaoExiste(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM guild WHERE id = $1`, guildaDeTeste)
	if err := New(pool).UpdateGuildFame(ctx, guildaDeTeste, 5); !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateGuildFame numa guilda que não existe = %v, want ErrNotFound", err)
	}
}
