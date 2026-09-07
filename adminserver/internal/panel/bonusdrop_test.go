package panel

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/session"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

type fakeBonusDrop struct {
	mu      sync.Mutex
	faixas  map[int32]domain.DropBonusBand
	ligado  bool
	versao  int64
	lerErr  error
	gravErr error
}

func newFakeBonusDrop() *fakeBonusDrop {
	return &fakeBonusDrop{faixas: map[int32]domain.DropBonusBand{}, ligado: true}
}

func (f *fakeBonusDrop) DropBonus(context.Context) (domain.DropBonusConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lerErr != nil {
		return domain.DropBonusConfig{}, f.lerErr
	}
	cfg := domain.DropBonusConfig{Version: f.versao, Ligado: f.ligado}
	for _, b := range f.faixas {
		cfg.Faixas = append(cfg.Faixas, b)
	}
	return cfg, nil
}

func (f *fakeBonusDrop) SetDropBonus(_ context.Context, b domain.DropBonusBand, _ int64) (domain.DropBonusBand, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.gravErr != nil {
		return domain.DropBonusBand{}, false, f.gravErr
	}
	antes, tinha := f.faixas[b.Distancia]
	f.faixas[b.Distancia] = b
	f.versao++
	return antes, tinha, nil
}

func (f *fakeBonusDrop) DeleteDropBonus(_ context.Context, d int32, _ int64) (domain.DropBonusBand, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	antes, tinha := f.faixas[d]
	delete(f.faixas, d)
	f.versao++
	return antes, tinha, nil
}

func (f *fakeBonusDrop) SetDropBonusLigado(_ context.Context, ligado bool) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	antes := f.ligado
	f.ligado = ligado
	f.versao++
	return antes, nil
}

func painelBonusDrop(t *testing.T, bd *fakeBonusDrop, cargo string) (http.Handler, *fakeAudit) {
	t.Helper()
	log := newFakeAudit()
	h, err := New(Config{
		Accounts: withTarget(cargo), Writer: newFakeWriter(), Audit: log, BonusDrop: bd,
		Sessions: session.New(time.Hour),
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes(), log
}

// formPadrao is the legacy band 0 as the form carries it, so a test only has to
// name the field it wants wrong.
func formPadrao(token string) url.Values {
	p := domain.DropBonusDefaults[0]
	v := url.Values{"csrf": {token}, "distancia": {"0"}}
	for i, n := range p.Limite {
		v.Set("limite"+strconv.Itoa(i+1), strconv.Itoa(int(n)))
	}
	for i, n := range p.Degrau {
		v.Set("degrau"+strconv.Itoa(i+1), strconv.Itoa(int(n)))
	}
	for i, n := range p.Refino {
		v.Set("refino"+strconv.Itoa(i+1), strconv.Itoa(int(n)))
	}
	return v
}

// TestBonusDropMostraAsChances is the reason the page exists: eight boxes of
// numbers mean nothing without the odds they produce beside them.
func TestBonusDropMostraAsChances(t *testing.T) {
	h, _ := painelBonusDrop(t, newFakeBonusDrop(), roleAdmin)
	body := signedIn(t, h)("/rates/bonus-drop").Body.String()

	// Band 0's legacy refine ladder is 6/22/75/90, so the five slices are
	// 6, 16, 53, 15 and 10 percent.
	//
	// "Refino &#43;2" and not "Refino +2": html/template escapes a bare plus in
	// a text node. The browser still shows "+2"; only the source differs.
	for _, quer := range []string{"Refino &#43;2", "6%", "16%", "53%", "15%", "10%"} {
		if !strings.Contains(body, quer) {
			t.Errorf("a página não mostra %q", quer)
		}
	}
	// Band 0's magnitude ladder ends on step 0 — the 45% that leaves a dropped
	// item with nothing, which is the number somebody tuning this came to see.
	if !strings.Contains(body, "degrau 0") || !strings.Contains(body, "45%") {
		t.Error("a página não mostra a fatia que deixa o item sem nada")
	}
}

func TestBonusDropGravaEAuditaUmaFaixa(t *testing.T) {
	bd := newFakeBonusDrop()
	h, log := painelBonusDrop(t, bd, roleAdmin)

	post, token := signedInPost(t, h)
	f := formPadrao(token)
	f.Set("refino1", "20") // +2 em 20% em vez de 6%
	if rec := post("/rates/bonus-drop", f); rec.Code != http.StatusSeeOther {
		t.Fatalf("gravar respondeu %d, corpo = %s", rec.Code, rec.Body.String())
	}
	if got := bd.faixas[0].Refino[0]; got != 20 {
		t.Errorf("gravou refino1 = %d, queria 20", got)
	}
	if len(log.written) != 1 || log.written[0].Action != "SET_DROP_BONUS" {
		t.Errorf("auditoria = %+v, queria um SET_DROP_BONUS", log.written)
	}
}

// TestBonusDropRecusaEscadaForaDeOrdem guards the failure that leaves no trace:
// a threshold that does not advance deletes a rung, and the page still renders.
func TestBonusDropRecusaEscadaForaDeOrdem(t *testing.T) {
	casos := []struct {
		nome, campo, valor string
	}{
		{"magnitude fora de ordem", "limite3", "1"},
		{"refino fora de ordem", "refino3", "1"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			bd := newFakeBonusDrop()
			h, _ := painelBonusDrop(t, bd, roleAdmin)
			post, token := signedInPost(t, h)
			f := formPadrao(token)
			f.Set(c.campo, c.valor)
			if rec := post("/rates/bonus-drop", f); rec.Code != http.StatusBadRequest {
				t.Fatalf("respondeu %d, queria 400", rec.Code)
			}
			if len(bd.faixas) != 0 {
				t.Error("gravou uma escada fora de ordem")
			}
		})
	}
}

func TestBonusDropRecusaValorForaDaFaixa(t *testing.T) {
	casos := []struct{ nome, campo, valor string }{
		{"limite acima de cem", "limite4", "101"},
		{"degrau acima do teto", "degrau1", "21"},
		{"numero negativo", "refino1", "-1"},
		{"nao e numero", "limite1", "muito"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			bd := newFakeBonusDrop()
			h, _ := painelBonusDrop(t, bd, roleAdmin)
			post, token := signedInPost(t, h)
			f := formPadrao(token)
			f.Set(c.campo, c.valor)
			if rec := post("/rates/bonus-drop", f); rec.Code != http.StatusBadRequest {
				t.Fatalf("respondeu %d, queria 400", rec.Code)
			}
			if len(bd.faixas) != 0 {
				t.Error("gravou um valor fora da faixa")
			}
		})
	}
}

// TestBonusDropInterruptor covers the one-click undo, including that the page
// says out loud what turning it off does.
func TestBonusDropInterruptor(t *testing.T) {
	bd := newFakeBonusDrop()
	h, log := painelBonusDrop(t, bd, roleAdmin)

	post, token := signedInPost(t, h)
	if rec := post("/rates/bonus-drop/ligar", url.Values{"csrf": {token}, "ligado": {"0"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("desligar respondeu %d, corpo = %s", rec.Code, rec.Body.String())
	}
	if bd.ligado {
		t.Error("o sorteio continuou ligado")
	}
	if len(log.written) != 1 || log.written[0].Action != "SET_DROP_BONUS_LIGADO" {
		t.Errorf("auditoria = %+v, queria um SET_DROP_BONUS_LIGADO", log.written)
	}

	body := signedIn(t, h)("/rates/bonus-drop").Body.String()
	if !strings.Contains(body, "desligado") || !strings.Contains(body, "Ligar o sorteio") {
		t.Error("a página não mostra que o sorteio está desligado")
	}
}

// TestBonusDropSoAdmin: these numbers decide the economy of the whole server,
// which is a different weight from banning one account.
func TestBonusDropSoAdmin(t *testing.T) {
	h, _ := painelBonusDrop(t, newFakeBonusDrop(), roleModerator)
	if got := signedIn(t, h)("/rates/bonus-drop").Code; got != http.StatusForbidden {
		t.Errorf("moderador viu a página com %d, queria 403", got)
	}
}

// TestBonusDropLimparVoltaAoLegado also checks the page stops calling the band
// edited once its row is gone.
func TestBonusDropLimparVoltaAoLegado(t *testing.T) {
	bd := newFakeBonusDrop()
	bd.faixas[0] = domain.DropBonusBand{Distancia: 0, Refino: [4]int32{50, 60, 70, 80}}
	h, log := painelBonusDrop(t, bd, roleAdmin)

	post, token := signedInPost(t, h)
	if rec := post("/rates/bonus-drop/limpar", url.Values{"csrf": {token}, "distancia": {"0"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("limpar respondeu %d, corpo = %s", rec.Code, rec.Body.String())
	}
	if len(bd.faixas) != 0 {
		t.Error("a exceção continuou gravada")
	}
	if len(log.written) != 1 || log.written[0].Action != "CLEAR_DROP_BONUS" {
		t.Errorf("auditoria = %+v, queria um CLEAR_DROP_BONUS", log.written)
	}
	if strings.Contains(signedIn(t, h)("/rates/bonus-drop").Body.String(), "editada") {
		t.Error("a página ainda chama a faixa de editada depois de limpar")
	}
}
