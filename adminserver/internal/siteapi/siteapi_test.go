package siteapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	gamev1 "github.com/jeanluca/w2pp-openwyd/api/game/v1"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/accounts"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/donate"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/entrega"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/jogo"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/secret"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

const chaveTeste = "chave-do-site-para-teste"

// Two accounts, so every test can ask "did anything of the other one leak?".
const (
	idAlfa, nomeAlfa = int64(1), "alfa"
	idBeta, nomeBeta = int64(2), "beta"
)

// fakeBanco answers every read and write the API makes, per account, the way
// the real stores do: by the id it is given and nothing else.
type fakeBanco struct {
	mu        sync.Mutex
	nomes     map[int64]string
	detalhes  map[int64]accounts.Details
	hashes    map[int64]string
	bloqueada map[int64]bool
	eventos   map[int64][]donate.Evento
	pendentes map[int64][]entrega.Pendente
	perdidos  map[int64][]entrega.Pendente
	senhas    int // SetPassword calls
	// ranking de kills: o que devolver e o que o handler pediu
	kills             []KillRanking
	killsTotal        int
	killsLimite       int
	killsDeslocamento int
	// eventos e taxas: a configuração que o painel mostra, e os erros para
	// provar que a rota não inventa resposta quando o banco falha
	eventosJogo domain.WorldEventConfig
	eventosErr  error
	drop        domain.DropBonusConfig
	dropErr     error
}

func (f *fakeBanco) WorldEventConfig(_ context.Context) (domain.WorldEventConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.eventosErr != nil {
		return domain.WorldEventConfig{}, f.eventosErr
	}
	return f.eventosJogo, nil
}

func (f *fakeBanco) DropBonus(_ context.Context) (domain.DropBonusConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.dropErr != nil {
		return domain.DropBonusConfig{}, f.dropErr
	}
	return f.drop, nil
}

func (f *fakeBanco) Nome(_ context.Context, id int64) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, ok := f.nomes[id]
	if !ok {
		return "", ErrContaNaoExiste
	}
	return n, nil
}

func (f *fakeBanco) Get(_ context.Context, id int64) (accounts.Details, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d, ok := f.detalhes[id]
	if !ok {
		return accounts.Details{}, accounts.ErrNotFound
	}
	return d, nil
}

func (f *fakeBanco) SetPassword(_ context.Context, id int64, hash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hashes[id] = hash
	f.senhas++
	return nil
}

func (f *fakeBanco) AccountAuthByID(_ context.Context, id int64) (store.AccountAuth, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.nomes[id]; !ok {
		return store.AccountAuth{}, store.ErrNotFound
	}
	return store.AccountAuth{ID: id, PassHash: f.hashes[id], IsBlocked: f.bloqueada[id]}, nil
}

func (f *fakeBanco) Historico(_ context.Context, id int64, limite int) ([]donate.Evento, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if limite != limiteHistorico {
		return nil, errors.New("the site asks for 50")
	}
	return f.eventos[id], nil
}

func (f *fakeBanco) Pendentes(_ context.Context, id int64) ([]entrega.Pendente, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.pendentes[id], nil
}

func (f *fakeBanco) Perdidos(_ context.Context, id int64, _ int) ([]entrega.Pendente, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.perdidos[id], nil
}

func (f *fakeBanco) hash(id int64) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hashes[id]
}

type fakeAudit struct {
	mu     sync.Mutex
	linhas []audit.Record
	err    error
}

func (f *fakeAudit) Write(_ context.Context, r audit.Record) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.linhas = append(f.linhas, r)
	return nil
}

func (f *fakeAudit) todas() []audit.Record {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]audit.Record(nil), f.linhas...)
}

type fakeSessoes struct{ encerradas []int64 }

func (f *fakeSessoes) DeleteByAccount(id int64) int {
	f.encerradas = append(f.encerradas, id)
	return 0
}

// jogoFalso is a GameControlService on the other end of a real gRPC connection,
// so the calls go through jogo.Client exactly as they do in production.
type jogoFalso struct {
	gamev1.UnimplementedGameControlServiceServer

	mu        sync.Mutex
	listas    int
	kicks     []string
	unstucks  []*gamev1.UnstuckRequest
	delivers  []string
	noMundo   map[string]string // account name -> character in the world
	perdeItem bool
	tokens    []string
}

func (j *jogoFalso) token(ctx context.Context) {
	md, _ := metadata.FromIncomingContext(ctx)
	j.tokens = append(j.tokens, strings.Join(md.Get(gamev1.TokenHeader), ","))
}

func (j *jogoFalso) ListOnline(ctx context.Context, _ *gamev1.ListOnlineRequest) (*gamev1.ListOnlineResponse, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.token(ctx)
	j.listas++
	return &gamev1.ListOnlineResponse{
		InPlay: 1, Connected: 2,
		Players: []*gamev1.OnlinePlayer{
			{AccountName: nomeAlfa, CharacterName: "HeroiAlfa", Ip: "203.0.113.7", MapX: 2100, MapY: 2100, InPlay: true},
			{AccountName: nomeBeta, Ip: "198.51.100.9"},
		},
	}, nil
}

func (j *jogoFalso) Kick(ctx context.Context, r *gamev1.KickRequest) (*gamev1.KickResponse, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.token(ctx)
	j.kicks = append(j.kicks, r.GetAccountName())
	if _, ok := j.noMundo[r.GetAccountName()]; ok {
		return &gamev1.KickResponse{Sessions: 1}, nil
	}
	return &gamev1.KickResponse{}, nil
}

func (j *jogoFalso) Unstuck(ctx context.Context, r *gamev1.UnstuckRequest) (*gamev1.UnstuckResponse, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.token(ctx)
	j.unstucks = append(j.unstucks, r)
	p, ok := j.noMundo[r.GetAccountName()]
	if !ok {
		return &gamev1.UnstuckResponse{}, nil
	}
	return &gamev1.UnstuckResponse{Found: true, CharacterName: p, FromX: 3000, FromY: 3000, ToX: 2100, ToY: 2100, City: "Armia"}, nil
}

func (j *jogoFalso) DeliverNow(ctx context.Context, r *gamev1.DeliverNowRequest) (*gamev1.DeliverNowResponse, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.token(ctx)
	j.delivers = append(j.delivers, r.GetAccountName())
	p, ok := j.noMundo[r.GetAccountName()]
	if !ok {
		return &gamev1.DeliverNowResponse{}, nil
	}
	if j.perdeItem {
		return &gamev1.DeliverNowResponse{Found: true, Delivered: 1, Lost: 1, CharacterName: p}, nil
	}
	return &gamev1.DeliverNowResponse{Found: true, Delivered: 2, CharacterName: p}, nil
}

// cenario is one API under test with everything it talks to.
type cenario struct {
	h       http.Handler
	banco   *fakeBanco
	audit   *fakeAudit
	sessoes *fakeSessoes
	jogo    *jogoFalso
	srv     *grpc.Server
	agora   time.Time
}

func (c *cenario) avanca(d time.Duration) { c.agora = c.agora.Add(d) }

func novoCenario(t *testing.T) *cenario {
	t.Helper()
	hashAlfa, err := secret.HashSecret("senhaalfa")
	if err != nil {
		t.Fatal(err)
	}
	vip := time.Now().Add(20 * 24 * time.Hour)
	vencido := time.Now().Add(-48 * time.Hour)
	saldo := int64(40)
	c := &cenario{
		banco: &fakeBanco{
			nomes:     map[int64]string{idAlfa: nomeAlfa, idBeta: nomeBeta},
			detalhes:  map[int64]accounts.Details{idAlfa: {VipUntil: &vip, Email: "alfa@exemplo", DonateBalance: 40}, idBeta: {VipUntil: &vencido}},
			hashes:    map[int64]string{idAlfa: hashAlfa, idBeta: "hash-da-beta-que-nunca-e-lido"},
			bloqueada: map[int64]bool{},
			eventos: map[int64][]donate.Evento{
				idAlfa: {
					{Tipo: donate.TipoCompra, Quando: time.Now(), Creditos: -10, Titulo: "Comprou Poeira da Alfa", ItemTitulo: "Poeira da Alfa", Detalhe: "Loja de donate", Saldo: &saldo, Entregue: "pending"},
					{Tipo: donate.TipoAjuste, Quando: time.Now(), Creditos: 5, Titulo: "Ajuste manual por moderadorx", Detalhe: "Motivo: nota interna"},
					{Tipo: donate.Tipo("futuro"), Quando: time.Now(), Titulo: "tipo que esta versao nao conhece"},
				},
				idBeta: {{Tipo: donate.TipoRecarga, Quando: time.Now(), Creditos: 30, Titulo: "Recarga confirmada", Detalhe: "PIX · R$ 10,00 · ref. beta123"}},
			},
			pendentes: map[int64][]entrega.Pendente{
				idAlfa: {{ID: 11, ItemIndex: 412, Origem: "donate_shop:3", CriadoEm: time.Now()}, {ID: 12, ItemIndex: 2222, Eff: [3][2]uint8{{43, 7}}, ExpiresAt: time.Now().Add(time.Hour).Unix(), Origem: "painel:77", CriadoEm: time.Now()}},
				idBeta: {{ID: 21, ItemIndex: 3314, Origem: "donate_shop:9", CriadoEm: time.Now()}},
			},
			perdidos: map[int64][]entrega.Pendente{idAlfa: {{ID: 13, ItemIndex: 1481, Origem: "donate_shop:4", CriadoEm: time.Now()}}},
		},
		audit:   &fakeAudit{},
		sessoes: &fakeSessoes{},
		jogo:    &jogoFalso{noMundo: map[string]string{nomeAlfa: "HeroiAlfa"}},
		agora:   time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC),
	}

	lis := bufconn.Listen(1 << 20)
	c.srv = grpc.NewServer()
	gamev1.RegisterGameControlServiceServer(c.srv, c.jogo)
	go func() { _ = c.srv.Serve(lis) }()
	t.Cleanup(c.srv.Stop)
	conn, err := grpc.NewClient("passthrough:///jogo",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	api, err := New(Config{
		Chave: chaveTeste, Contas: c.banco, Credenciais: c.banco, Leitura: c.banco,
		Eventos: c.banco, Taxas: c.banco,
		Carteira: c.banco, Entregas: c.banco, Jogo: jogo.New(conn, "token-do-jogo"),
		Audit: c.audit, Sessoes: c.sessoes,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Agora:  func() time.Time { return c.agora },
	})
	if err != nil {
		t.Fatal(err)
	}
	c.h = api.Routes()
	return c
}

// pede makes one call with the site's key.
func (c *cenario) pede(metodo, caminho, corpo string) *httptest.ResponseRecorder {
	return c.pedeCom(metodo, caminho, corpo, "Bearer "+chaveTeste)
}

func (c *cenario) pedeCom(metodo, caminho, corpo, autorizacao string) *httptest.ResponseRecorder {
	var body io.Reader
	if corpo != "" {
		body = strings.NewReader(corpo)
	}
	req := httptest.NewRequest(metodo, caminho, body)
	if autorizacao != "" {
		req.Header.Set("Authorization", autorizacao)
	}
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	return rec
}

func jsonDe(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body is not JSON: %v: %s", err, rec.Body.String())
	}
	return m
}

func confereStatus(t *testing.T, rec *httptest.ResponseRecorder, want int, erro string) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, want, rec.Body.String())
	}
	if erro != "" {
		if got := jsonDe(t, rec)["erro"]; got != erro {
			t.Fatalf("erro = %v, want %q", got, erro)
		}
	}
}

func TestNewRefusesAnEmptyKey(t *testing.T) {
	if _, err := New(Config{}); !errors.Is(err, ErrSemChave) {
		t.Fatalf("err = %v, want ErrSemChave", err)
	}
}

func TestTheKeyIsRequiredOnEveryRoute(t *testing.T) {
	c := novoCenario(t)
	rotas := [][2]string{
		{"GET", "/site/v1/jogo"},
		{"GET", "/site/v1/contas/1/estado"},
		{"POST", "/site/v1/contas/1/entregar-agora"},
		{"POST", "/site/v1/contas/1/senha"},
		{"GET", "/nao/existe"},
	}
	for _, r := range rotas {
		for nome, aut := range map[string]string{
			"sem chave":         "",
			"chave errada":      "Bearer outra-chave",
			"chave com sobra":   "Bearer " + chaveTeste + "x",
			"bearer vazio":      "Bearer ",
			"sem bearer":        chaveTeste,
			"esquema diferente": "Basic " + chaveTeste,
		} {
			rec := c.pedeCom(r[0], r[1], "", aut)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s %s (%s): status = %d, want 401", r[0], r[1], nome, rec.Code)
			}
		}
	}
	if n := len(c.audit.todas()); n != 0 {
		t.Errorf("%d audit rows written by refused calls", n)
	}
	c.jogo.mu.Lock()
	defer c.jogo.mu.Unlock()
	if len(c.jogo.delivers)+len(c.jogo.kicks)+c.jogo.listas != 0 {
		t.Error("a refused call reached the game")
	}
}

func TestStaffRoutesDoNotExistOnThisListener(t *testing.T) {
	c := novoCenario(t)
	for _, r := range [][2]string{
		{"GET", "/"}, {"GET", "/login"}, {"POST", "/login"}, {"GET", "/healthz"},
		{"GET", "/contas"}, {"GET", "/contas/alfa"}, {"POST", "/contas/alfa/senha"},
		{"GET", "/auditoria"}, {"GET", "/servidor"}, {"POST", "/servidor/derrubar"},
		{"GET", "/site/v1/contas"}, {"GET", "/site/v1/contas/1"},
		{"POST", "/site/v1/contas/1/estado"}, {"GET", "/site/v1/contas/1/senha"},
	} {
		rec := c.pede(r[0], r[1], "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s: status = %d, want 404", r[0], r[1], rec.Code)
		}
	}
}

func TestUnknownAccountIs404(t *testing.T) {
	c := novoCenario(t)
	for _, p := range []string{"/site/v1/contas/99/estado", "/site/v1/contas/0/estado", "/site/v1/contas/-1/estado", "/site/v1/contas/alfa/estado"} {
		confereStatus(t, c.pede("GET", p, ""), http.StatusNotFound, "conta")
	}
}

func TestOnlineCountCarriesCountsOnlyAndIsCached(t *testing.T) {
	c := novoCenario(t)
	rec := c.pede("GET", "/site/v1/jogo", "")
	confereStatus(t, rec, http.StatusOK, "")
	m := jsonDe(t, rec)
	if m["online"] != true || m["jogando"] != float64(1) || m["conectados"] != float64(2) {
		t.Fatalf("body = %v", m)
	}
	if len(m) != 3 {
		t.Errorf("fields = %v, want only online/jogando/conectados", m)
	}
	for _, vazou := range []string{nomeAlfa, nomeBeta, "HeroiAlfa", "203.0.113", "198.51.100", "2100"} {
		if strings.Contains(rec.Body.String(), vazou) {
			t.Errorf("body leaks %q: %s", vazou, rec.Body.String())
		}
	}

	c.avanca(29 * time.Second)
	c.pede("GET", "/site/v1/jogo", "")
	c.avanca(2 * time.Second)
	c.pede("GET", "/site/v1/jogo", "")
	c.jogo.mu.Lock()
	defer c.jogo.mu.Unlock()
	if c.jogo.listas != 2 {
		t.Errorf("ListOnline called %d times over 31 s, want 2 (30 s cache)", c.jogo.listas)
	}
	if c.jogo.tokens[0] != "token-do-jogo" {
		t.Errorf("game token = %q", c.jogo.tokens[0])
	}
}

func TestGameDownIsOfflineNotAnError(t *testing.T) {
	c := novoCenario(t)
	c.srv.Stop()
	rec := c.pede("GET", "/site/v1/jogo", "")
	confereStatus(t, rec, http.StatusOK, "")
	if m := jsonDe(t, rec); m["online"] != false {
		t.Fatalf("body = %v, want online false", m)
	}
	confereStatus(t, c.pede("POST", "/site/v1/contas/1/entregar-agora", ""), http.StatusServiceUnavailable, "jogo_fora_do_ar")
}

func TestStateOfEachAccountIsItsOwn(t *testing.T) {
	c := novoCenario(t)
	a := jsonDe(t, c.pede("GET", "/site/v1/contas/1/estado", ""))
	if a["bloqueada"] != false || a["vip_ativo"] != true || a["vip_ate"] == nil || a["bloqueio_ate"] != nil {
		t.Errorf("alfa = %v", a)
	}
	if _, ok := a["email"]; ok {
		t.Error("estado carries the email")
	}
	b := jsonDe(t, c.pede("GET", "/site/v1/contas/2/estado", ""))
	if b["vip_ativo"] != false || b["vip_ate"] == nil {
		t.Errorf("beta = %v, want an expired VIP date", b)
	}
}

func TestWalletHistoryIsIsolatedAndHidesStaff(t *testing.T) {
	c := novoCenario(t)
	rec := c.pede("GET", "/site/v1/contas/1/historico", "")
	confereStatus(t, rec, http.StatusOK, "")
	corpo := rec.Body.String()
	for _, vazou := range []string{"moderadorx", "nota interna", "beta123", "tipo que esta versao"} {
		if strings.Contains(corpo, vazou) {
			t.Errorf("alfa's history leaks %q: %s", vazou, corpo)
		}
	}
	var h struct {
		Eventos []eventoSite `json:"eventos"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &h); err != nil {
		t.Fatal(err)
	}
	if len(h.Eventos) != 2 || h.Eventos[0].Titulo != "Brinde resgatado: Poeira da Alfa" || h.Eventos[1].Titulo != "Ajuste da equipe" || h.Eventos[1].Detalhe != "" {
		t.Errorf("eventos = %+v", h.Eventos)
	}
	if b := c.pede("GET", "/site/v1/contas/2/historico", "").Body.String(); strings.Contains(b, "Alfa") || !strings.Contains(b, "beta123") {
		t.Errorf("beta's history = %s", b)
	}
}

// TestWalletTitlesSpeakDonationNotPurchase freezes the four sentences the door
// rewrites. The wallet history is the one player screen whose words are written
// on the panel side, so a change over there - or a merge that drops this
// rewrite - has to fail here instead of putting "Comprou" on a site that does
// not sell.
func TestWalletTitlesSpeakDonationNotPurchase(t *testing.T) {
	casos := []struct {
		nome    string
		e       donate.Evento
		titulo  string
		detalhe string
	}{
		{"recarga paga", donate.Evento{Tipo: donate.TipoRecarga, Titulo: "Recarga confirmada", Detalhe: "PIX · R$ 10,00 · ref. x"}, "Doação confirmada", "PIX · R$ 10,00 · ref. x"},
		{"recarga que nunca confirmou", donate.Evento{Tipo: donate.TipoPendente, Titulo: "Recarga não confirmada", Detalhe: "PIX · R$ 10,00 · ref. y"}, "Doação não confirmada", "PIX · R$ 10,00 · ref. y"},
		{"brinde com nome", donate.Evento{Tipo: donate.TipoCompra, Titulo: "Comprou Poeira da Alfa", ItemTitulo: "Poeira da Alfa", Detalhe: "Loja de donate"}, "Brinde resgatado: Poeira da Alfa", ""},
		{"brinde cadastrado sem titulo", donate.Evento{Tipo: donate.TipoCompra, Titulo: "Comprou oferta #7", ItemID: 7, Detalhe: "Loja de donate"}, "Brinde resgatado", ""},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			s, ok := eventoDoSite(caso.e)
			if !ok {
				t.Fatalf("%s: recusado", caso.nome)
			}
			if s.Titulo != caso.titulo || s.Detalhe != caso.detalhe {
				t.Errorf("titulo=%q detalhe=%q, want %q e %q", s.Titulo, s.Detalhe, caso.titulo, caso.detalhe)
			}
			for _, palavra := range []string{"Recarga", "Comprou", "Loja", "oferta"} {
				if strings.Contains(s.Titulo+" "+s.Detalhe, palavra) {
					t.Errorf("%s ainda diz %q: titulo=%q detalhe=%q", caso.nome, palavra, s.Titulo, s.Detalhe)
				}
			}
		})
	}
}

func TestDeliveriesAreIsolatedAndHideStaffIDs(t *testing.T) {
	c := novoCenario(t)
	var e struct {
		Pendentes []itemSite `json:"pendentes"`
		Perdidos  []itemSite `json:"perdidos"`
	}
	rec := c.pede("GET", "/site/v1/contas/1/entregas", "")
	confereStatus(t, rec, http.StatusOK, "")
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if len(e.Pendentes) != 2 || len(e.Perdidos) != 1 || e.Perdidos[0].Item != 1481 {
		t.Fatalf("alfa = %+v", e)
	}
	if e.Pendentes[0].Origem != "loja" || e.Pendentes[1].Origem != "equipe" || e.Pendentes[1].ExpiraEm == nil {
		t.Errorf("pendentes = %+v", e.Pendentes)
	}
	if e.Pendentes[1].Efeitos[0] != [2]int{43, 7} {
		t.Errorf("efeitos = %v", e.Pendentes[1].Efeitos)
	}
	if strings.Contains(rec.Body.String(), "77") || strings.Contains(rec.Body.String(), "3314") {
		t.Errorf("alfa's deliveries leak a staff id or beta's item: %s", rec.Body.String())
	}
	b := c.pede("GET", "/site/v1/contas/2/entregas", "").Body.String()
	if !strings.Contains(b, "3314") || strings.Contains(b, "412") || strings.Contains(b, "1481") {
		t.Errorf("beta's deliveries = %s", b)
	}
}

func TestDeliverNowGoesThroughTheGameUnderTheRightName(t *testing.T) {
	c := novoCenario(t)
	rec := c.pede("POST", "/site/v1/contas/1/entregar-agora", "")
	confereStatus(t, rec, http.StatusOK, "")
	m := jsonDe(t, rec)
	if m["conectado"] != true || m["entregues"] != float64(2) || m["perdidos"] != float64(0) || m["personagem"] != "HeroiAlfa" {
		t.Fatalf("body = %v", m)
	}
	c.jogo.mu.Lock()
	if len(c.jogo.delivers) != 1 || c.jogo.delivers[0] != nomeAlfa {
		t.Errorf("DeliverNow calls = %v, want [alfa]", c.jogo.delivers)
	}
	c.jogo.mu.Unlock()

	// Beta is not in the world: an answer, not an action, so nothing is audited.
	m = jsonDe(t, c.pede("POST", "/site/v1/contas/2/entregar-agora", ""))
	if m["conectado"] != false {
		t.Errorf("beta = %v", m)
	}
	linhas := c.audit.todas()
	if len(linhas) != 1 {
		t.Fatalf("audit rows = %d, want 1 (beta was offline)", len(linhas))
	}
	l := linhas[0]
	if l.ActorRole != AtorSite || l.ActorID != idAlfa || l.TargetID != idAlfa || l.Action != AcaoEntregarAgora {
		t.Errorf("audit = %+v", l)
	}
}

func TestDeliverNowSaysWhatWasLost(t *testing.T) {
	c := novoCenario(t)
	c.jogo.perdeItem = true
	m := jsonDe(t, c.pede("POST", "/site/v1/contas/1/entregar-agora", ""))
	if m["entregues"] != float64(1) || m["perdidos"] != float64(1) {
		t.Fatalf("body = %v", m)
	}
}

func TestDeliverNowOncePerMinute(t *testing.T) {
	c := novoCenario(t)
	confereStatus(t, c.pede("POST", "/site/v1/contas/1/entregar-agora", ""), http.StatusOK, "")
	c.avanca(30 * time.Second)
	rec := c.pede("POST", "/site/v1/contas/1/entregar-agora", "")
	confereStatus(t, rec, http.StatusTooManyRequests, "limite")
	if s := jsonDe(t, rec)["espera_s"]; s != float64(30) {
		t.Errorf("espera_s = %v, want 30", s)
	}
	if rec.Header().Get("Retry-After") != "30" {
		t.Errorf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
	// The other account has its own minute.
	confereStatus(t, c.pede("POST", "/site/v1/contas/2/entregar-agora", ""), http.StatusOK, "")
	c.avanca(31 * time.Second)
	confereStatus(t, c.pede("POST", "/site/v1/contas/1/entregar-agora", ""), http.StatusOK, "")
}

func TestUnstuckGoesToTheNearestCityOnly(t *testing.T) {
	c := novoCenario(t)
	for _, corpo := range []string{`{"x":100,"y":200}`, `{}`, "x"} {
		confereStatus(t, c.pede("POST", "/site/v1/contas/1/desatolar", corpo), http.StatusBadRequest, "corpo")
	}
	c.jogo.mu.Lock()
	if len(c.jogo.unstucks) != 0 {
		t.Fatal("a refused body still reached the game")
	}
	c.jogo.mu.Unlock()

	rec := c.pede("POST", "/site/v1/contas/1/desatolar", "")
	confereStatus(t, rec, http.StatusOK, "")
	if m := jsonDe(t, rec); m["achou"] != true || m["cidade"] != "Armia" || m["personagem"] != "HeroiAlfa" || len(m) != 3 {
		t.Fatalf("body = %v, want achou/personagem/cidade and no coordinates", m)
	}
	c.jogo.mu.Lock()
	u := c.jogo.unstucks[0]
	c.jogo.mu.Unlock()
	if u.GetAccountName() != nomeAlfa || u.GetToX() != 0 || u.GetToY() != 0 {
		t.Errorf("Unstuck = %v, want alfa to 0,0", u)
	}
	l := c.audit.todas()
	if len(l) != 1 || l[0].Action != audit.ActionUnstuck || l[0].ActorRole != AtorSite || l[0].TargetID != idAlfa {
		t.Errorf("audit = %+v", l)
	}
}

func TestUnstuckOncePerHourOnlyWhenSomethingMoved(t *testing.T) {
	c := novoCenario(t)
	// Beta is offline: nothing moves, the hour does not start, only the minute.
	confereStatus(t, c.pede("POST", "/site/v1/contas/2/desatolar", ""), http.StatusOK, "")
	confereStatus(t, c.pede("POST", "/site/v1/contas/2/desatolar", ""), http.StatusTooManyRequests, "limite")
	c.avanca(61 * time.Second)
	confereStatus(t, c.pede("POST", "/site/v1/contas/2/desatolar", ""), http.StatusOK, "")

	confereStatus(t, c.pede("POST", "/site/v1/contas/1/desatolar", ""), http.StatusOK, "")
	c.avanca(10 * time.Minute)
	rec := c.pede("POST", "/site/v1/contas/1/desatolar", "")
	confereStatus(t, rec, http.StatusTooManyRequests, "limite")
	if s := jsonDe(t, rec)["espera_s"]; s != float64(50*60) {
		t.Errorf("espera_s = %v, want 3000", s)
	}
	c.avanca(50 * time.Minute)
	confereStatus(t, c.pede("POST", "/site/v1/contas/1/desatolar", ""), http.StatusOK, "")
}

func TestPasswordChange(t *testing.T) {
	c := novoCenario(t)
	antes := c.banco.hash(idAlfa)
	casos := []struct {
		nome, corpo string
		status      int
		erro, regra string
	}{
		{"atual errada", `{"atual":"errada","nova":"nova123"}`, http.StatusForbidden, "senha_atual", ""},
		{"atual vazia", `{"atual":"","nova":"nova123"}`, http.StatusForbidden, "senha_atual", ""},
		{"curta", `{"atual":"senhaalfa","nova":"abc"}`, http.StatusBadRequest, "senha_nova", "curta"},
		{"12 cabe no ValidarSenha, não no site", `{"atual":"senhaalfa","nova":"abcdefghijkl"}`, http.StatusBadRequest, "senha_nova", "longa"},
		{"espaço", `{"atual":"senhaalfa","nova":"abc def"}`, http.StatusBadRequest, "senha_nova", "espaco"},
		{"acento", `{"atual":"senhaalfa","nova":"açaí1234"}`, http.StatusBadRequest, "senha_nova", "caractere"},
	}
	for _, k := range casos {
		c.avanca(time.Hour) // each case on a fresh window; the window has its own test
		rec := c.pede("POST", "/site/v1/contas/1/senha", k.corpo)
		confereStatus(t, rec, k.status, k.erro)
		if k.regra != "" && jsonDe(t, rec)["regra"] != k.regra {
			t.Errorf("%s: regra = %v, want %s", k.nome, jsonDe(t, rec)["regra"], k.regra)
		}
		if c.banco.hash(idAlfa) != antes {
			t.Fatalf("%s: the password changed", k.nome)
		}
	}
	c.avanca(time.Hour)
	for _, corpo := range []string{`{"atual":"senhaalfa"}`, `{"atual":"senhaalfa","nova":"x","extra":1}`, `nada`} {
		confereStatus(t, c.pede("POST", "/site/v1/contas/1/senha", corpo), http.StatusBadRequest, "corpo")
	}
	confereStatus(t, c.pede("POST", "/site/v1/contas/1/senha", `{"atual":"senhaalfa","nova":"senhaalfa"}`), http.StatusBadRequest, "senha_igual")

	rec := c.pede("POST", "/site/v1/contas/1/senha", `{"atual":"senhaalfa","nova":"Nova!123"}`)
	confereStatus(t, rec, http.StatusOK, "")
	if m := jsonDe(t, rec); m["trocada"] != true || m["sessoes_derrubadas"] != float64(1) {
		t.Fatalf("body = %v", m)
	}
	novo := c.banco.hash(idAlfa)
	if ok, _ := secret.VerifySecret("Nova!123", novo); !ok {
		t.Error("the new password does not verify")
	}
	if ok, _ := secret.VerifySecret("senhaalfa", novo); ok {
		t.Error("the old password still verifies")
	}
	if c.banco.hash(idBeta) != "hash-da-beta-que-nunca-e-lido" {
		t.Error("beta's password was touched")
	}
	c.jogo.mu.Lock()
	if len(c.jogo.kicks) != 1 || c.jogo.kicks[0] != nomeAlfa {
		t.Errorf("kicks = %v, want [alfa]", c.jogo.kicks)
	}
	c.jogo.mu.Unlock()
	if len(c.sessoes.encerradas) != 1 || c.sessoes.encerradas[0] != idAlfa {
		t.Errorf("panel sessions ended for %v", c.sessoes.encerradas)
	}
	l := c.audit.todas()
	if len(l) != 1 || l[0].Action != audit.ActionSetPassword || l[0].ActorRole != AtorSite || l[0].TargetID != idAlfa {
		t.Fatalf("audit = %+v", l)
	}
	registro, _ := json.Marshal(l[0].New)
	if strings.Contains(string(registro), "Nova!123") || strings.Contains(string(registro), "argon2") {
		t.Errorf("the audit row carries the password or its hash: %s", registro)
	}
}

func TestPasswordAttemptsFivePerHour(t *testing.T) {
	c := novoCenario(t)
	for i := 0; i < 5; i++ {
		confereStatus(t, c.pede("POST", "/site/v1/contas/1/senha", `{"atual":"errada","nova":"nova123"}`), http.StatusForbidden, "senha_atual")
	}
	rec := c.pede("POST", "/site/v1/contas/1/senha", `{"atual":"senhaalfa","nova":"nova123"}`)
	confereStatus(t, rec, http.StatusTooManyRequests, "limite")
	if c.banco.senhas != 0 {
		t.Fatal("the right password went through after the limit")
	}
}

func TestPasswordChangedEvenIfTheGameIsDown(t *testing.T) {
	c := novoCenario(t)
	c.srv.Stop()
	rec := c.pede("POST", "/site/v1/contas/1/senha", `{"atual":"senhaalfa","nova":"nova123"}`)
	confereStatus(t, rec, http.StatusOK, "")
	if m := jsonDe(t, rec); m["sessoes_derrubadas"] != nil {
		t.Errorf("sessoes_derrubadas = %v, want null (the game was not told)", m["sessoes_derrubadas"])
	}
}

func TestBlockedAccountCannotWrite(t *testing.T) {
	c := novoCenario(t)
	ate := time.Now().Add(72 * time.Hour)
	c.banco.bloqueada[idAlfa] = true
	c.banco.detalhes[idAlfa] = accounts.Details{Bloqueio: accounts.Bloqueio{Blocked: true, Reason: "motivo interno", Until: &ate}}

	rec := c.pede("GET", "/site/v1/contas/1/estado", "")
	m := jsonDe(t, rec)
	if m["bloqueada"] != true || m["bloqueio_ate"] == nil || strings.Contains(rec.Body.String(), "motivo interno") {
		t.Errorf("estado = %s", rec.Body.String())
	}
	confereStatus(t, c.pede("POST", "/site/v1/contas/1/entregar-agora", ""), http.StatusForbidden, "bloqueada")
	confereStatus(t, c.pede("POST", "/site/v1/contas/1/desatolar", ""), http.StatusForbidden, "bloqueada")
	confereStatus(t, c.pede("POST", "/site/v1/contas/1/senha", `{"atual":"senhaalfa","nova":"nova123"}`), http.StatusForbidden, "bloqueada")
	c.jogo.mu.Lock()
	defer c.jogo.mu.Unlock()
	if len(c.jogo.delivers)+len(c.jogo.unstucks)+len(c.jogo.kicks) != 0 || c.banco.senhas != 0 {
		t.Error("a blocked account's write went through")
	}
}

func TestAChangeThatCannotBeAuditedIsReportedAsAFailure(t *testing.T) {
	c := novoCenario(t)
	c.audit.err = errors.New("banco caiu")
	confereStatus(t, c.pede("POST", "/site/v1/contas/1/desatolar", ""), http.StatusInternalServerError, "interno")
}

func TestGameRefusingTheTokenIsNotReportedAsSuccess(t *testing.T) {
	c := novoCenario(t)
	c.srv.Stop()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer(grpc.UnaryInterceptor(func(context.Context, any, *grpc.UnaryServerInfo, grpc.UnaryHandler) (any, error) {
		return nil, status.Error(codes.Unauthenticated, "token")
	}))
	gamev1.RegisterGameControlServiceServer(srv, &jogoFalso{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient("passthrough:///recusa",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	api, err := New(Config{
		Chave: chaveTeste, Contas: c.banco, Credenciais: c.banco, Leitura: c.banco, Carteira: c.banco,
		Entregas: c.banco, Jogo: jogo.New(conn, "errado"), Audit: c.audit,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	c.h = api.Routes()
	confereStatus(t, c.pede("POST", "/site/v1/contas/1/entregar-agora", ""), http.StatusServiceUnavailable, "jogo_fora_do_ar")
	if len(c.audit.todas()) != 0 {
		t.Error("a refused call was audited as done")
	}
}

func TestLimitsForgetOldKeys(t *testing.T) {
	l := novosLimites()
	t0 := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	for i := 0; i < maxChaves; i++ {
		l.marca(chaveDe("x", alvo{ID: int64(i + 1)}), janelaEntregar, t0)
	}
	l.marca("depois", janelaEntregar, t0.Add(2*time.Hour))
	if len(l.marcas) != 1 {
		t.Errorf("keys after the sweep = %d, want 1", len(l.marcas))
	}
}
