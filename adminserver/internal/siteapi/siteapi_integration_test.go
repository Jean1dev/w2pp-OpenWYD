//go:build integration

// Integration tests against a real PostgreSQL:
//
//	W2PP_TEST_DSN=postgres://postgres@localhost:5432/postgres go test -tags=integration ./adminserver/internal/siteapi/
package siteapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/accounts"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/donate"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/entrega"
	"github.com/jeanluca/w2pp-openwyd/internal/secret"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("W2PP_TEST_DSN")
	if dsn == "" {
		t.Skip("W2PP_TEST_DSN not set")
	}
	ctx := context.Background()
	pool, err := store.Pool(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	if err := store.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seed creates (or resets) a player account with a real password hash.
func seed(t *testing.T, pool *pgxpool.Pool, name, senha string) int64 {
	t.Helper()
	hash, err := secret.HashSecret(senha)
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	err = pool.QueryRow(context.Background(), `
		INSERT INTO account (name, pass_hash, role) VALUES ($1, $2, 'player')
		ON CONFLICT (name) DO UPDATE SET pass_hash = EXCLUDED.pass_hash, is_blocked = false, blocked_until = NULL
		RETURNING id`, name, hash).Scan(&id)
	if err != nil {
		t.Fatalf("seed %q: %v", name, err)
	}
	if _, err := pool.Exec(context.Background(), `DELETE FROM delivery_queue WHERE account_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	return id
}

func realAPI(t *testing.T, pool *pgxpool.Pool) http.Handler {
	t.Helper()
	api, err := New(Config{
		Chave: chaveTeste, Contas: accounts.New(pool), Credenciais: store.New(pool),
		Leitura: NovoLeitor(pool), Carteira: donate.New(pool), Entregas: entrega.New(pool),
		Audit: audit.New(pool), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return api.Routes()
}

func chama(h http.Handler, metodo, caminho, corpo string) *httptest.ResponseRecorder {
	var body io.Reader
	if corpo != "" {
		body = strings.NewReader(corpo)
	}
	req := httptest.NewRequest(metodo, caminho, body)
	req.Header.Set("Authorization", "Bearer "+chaveTeste)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// entra is what the web-api's VerifyCredentials does: the account by name, then
// the password against its hash.
func entra(t *testing.T, pool *pgxpool.Pool, nome, senha string) bool {
	t.Helper()
	auth, err := store.New(pool).AccountByName(context.Background(), nome)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := secret.VerifySecret(senha, auth.PassHash)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

func TestNameAndLostDeliveriesAgainstTheRealSchema(t *testing.T) {
	pool := testPool(t)
	l := NovoLeitor(pool)
	ctx := context.Background()
	alfa := seed(t, pool, "siteapi_alfa", "senhaalfa")
	beta := seed(t, pool, "siteapi_beta", "senhabeta")

	if n, err := l.Nome(ctx, alfa); err != nil || n != "siteapi_alfa" {
		t.Fatalf("Nome = %q, %v", n, err)
	}
	if _, err := l.Nome(ctx, 1<<60); err != ErrContaNaoExiste {
		t.Fatalf("Nome of a missing id: err = %v", err)
	}

	for _, r := range []struct {
		conta  int64
		status string
		item   int
	}{{alfa, "lost", 1481}, {alfa, "pending", 412}, {alfa, "delivered", 3314}, {beta, "lost", 2222}} {
		_, err := pool.Exec(ctx, `
			INSERT INTO delivery_queue (account_id, kind, payload, status, source)
			VALUES ($1, 'item', $2, $3, 'donate_shop:1')`,
			r.conta, `{"item_index":`+jsonNum(r.item)+`,"eff1":43,"effv1":7,"eff2":0,"effv2":0,"eff3":0,"effv3":0,"expires_at":0}`, r.status)
		if err != nil {
			t.Fatal(err)
		}
	}
	perd, err := l.Perdidos(ctx, alfa, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(perd) != 1 || perd[0].ItemIndex != 1481 || perd[0].Eff[0] != [2]uint8{43, 7} {
		t.Fatalf("Perdidos(alfa) = %+v, want only item 1481", perd)
	}
}

func jsonNum(n int) string { b, _ := json.Marshal(n); return string(b) }

// The endpoints over the real stores, two accounts side by side.
func TestTwoAccountsSeeOnlyTheirOwn(t *testing.T) {
	pool := testPool(t)
	h := realAPI(t, pool)
	ctx := context.Background()
	alfa := seed(t, pool, "siteapi_iso_a", "senhaalfa")
	beta := seed(t, pool, "siteapi_iso_b", "senhabeta")
	for conta, item := range map[int64]string{alfa: "1481", beta: "2222"} {
		for _, status := range []string{"pending", "lost"} {
			if _, err := pool.Exec(ctx, `
				INSERT INTO delivery_queue (account_id, kind, payload, status, source)
				VALUES ($1, 'item', $2, $3, 'painel:1')`,
				conta, `{"item_index":`+item+`,"eff1":0,"effv1":0,"eff2":0,"effv2":0,"eff3":0,"effv3":0,"expires_at":0}`, status); err != nil {
				t.Fatal(err)
			}
		}
	}
	a := chama(h, "GET", "/site/v1/contas/"+jsonNum(int(alfa))+"/entregas", "")
	b := chama(h, "GET", "/site/v1/contas/"+jsonNum(int(beta))+"/entregas", "")
	if a.Code != 200 || b.Code != 200 {
		t.Fatalf("status %d / %d: %s %s", a.Code, b.Code, a.Body.String(), b.Body.String())
	}
	if !strings.Contains(a.Body.String(), "1481") || strings.Contains(a.Body.String(), "2222") {
		t.Errorf("alfa sees %s", a.Body.String())
	}
	if !strings.Contains(b.Body.String(), "2222") || strings.Contains(b.Body.String(), "1481") {
		t.Errorf("beta sees %s", b.Body.String())
	}
	if strings.Contains(a.Body.String(), "painel:") {
		t.Errorf("the staff source leaked: %s", a.Body.String())
	}
	for _, p := range []string{"estado", "historico"} {
		if rec := chama(h, "GET", "/site/v1/contas/"+jsonNum(int(alfa))+"/"+p, ""); rec.Code != 200 {
			t.Errorf("%s: status %d: %s", p, rec.Code, rec.Body.String())
		}
	}
}

// §7: wrong current password changes nothing; a new one outside the rule changes
// nothing; after the change the old one no longer signs in and the new one does;
// and the audit log has a row with the site as the actor.
func TestPasswordChangeEndToEnd(t *testing.T) {
	pool := testPool(t)
	h := realAPI(t, pool)
	id := seed(t, pool, "siteapi_senha", "antiga1")
	caminho := "/site/v1/contas/" + jsonNum(int(id)) + "/senha"

	if rec := chama(h, "POST", caminho, `{"atual":"errada","nova":"nova1234"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("wrong current: %d %s", rec.Code, rec.Body.String())
	}
	if rec := chama(h, "POST", caminho, `{"atual":"antiga1","nova":"ab"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("short new: %d %s", rec.Code, rec.Body.String())
	}
	if !entra(t, pool, "siteapi_senha", "antiga1") {
		t.Fatal("a refused change still touched the password")
	}

	rec := chama(h, "POST", caminho, `{"atual":"antiga1","nova":"nova1234"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("change: %d %s", rec.Code, rec.Body.String())
	}
	if entra(t, pool, "siteapi_senha", "antiga1") {
		t.Error("the old password still signs in")
	}
	if !entra(t, pool, "siteapi_senha", "nova1234") {
		t.Error("the new password does not sign in")
	}

	var papel, acao string
	var alvo int64
	err := pool.QueryRow(context.Background(), `
		SELECT actor_role, action, target_account_id FROM admin_audit_log
		 WHERE actor_account_id = $1 ORDER BY id DESC LIMIT 1`, id).Scan(&papel, &acao, &alvo)
	if err != nil {
		t.Fatalf("audit row: %v", err)
	}
	if papel != AtorSite || acao != audit.ActionSetPassword || alvo != id {
		t.Errorf("audit = %s %s %d", papel, acao, alvo)
	}
}
