package panel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/accounts"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/entrega"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/gamedata"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/jogo"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/plataforma"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/session"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/secret"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

const testPassword = "senha-de-teste"

// hashOnce keeps the argon2id cost (64 MiB, ~100ms) to a single call for the
// whole package instead of once per test.
var hashOnce = sync.OnceValue(func() string {
	h, err := secret.HashSecret(testPassword)
	if err != nil {
		panic(err)
	}
	return h
})

// fakeAccounts is an in-memory stand-in for the store, keyed by canonical name.
type fakeAccounts struct {
	mu           sync.Mutex
	rows         map[string]store.AccountAuth
	chars        map[int64][]domain.Character
	listCharsErr error
}

func newFakeAccounts(role string) *fakeAccounts {
	return &fakeAccounts{
		rows: map[string]store.AccountAuth{
			"chefe": {ID: 42, PassHash: hashOnce(), Role: role},
		},
		chars: map[int64][]domain.Character{},
	}
}

// add registers another account, so listing and search have something to sort.
func (f *fakeAccounts) add(name string, id int64, role string, blocked bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows[name] = store.AccountAuth{ID: id, PassHash: hashOnce(), Role: role, IsBlocked: blocked}
}

func (f *fakeAccounts) addChar(accountID int64, c domain.Character) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chars[accountID] = append(f.chars[accountID], c)
}

// listCharsErr forces the roster read to fail, which is the only way to test the
// difference between "no characters" and "could not read the characters".

func (f *fakeAccounts) SearchAccountsByNamePrefix(_ context.Context, prefix string, limit int) ([]domain.AccountSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	names := make([]string, 0, len(f.rows))
	for n := range f.rows {
		if strings.HasPrefix(n, prefix) {
			names = append(names, n)
		}
	}
	sort.Strings(names) // the real query is ORDER BY name
	out := make([]domain.AccountSummary, 0, len(names))
	for _, n := range names {
		if len(out) == limit {
			break
		}
		a := f.rows[n]
		out = append(out, domain.AccountSummary{
			ID: a.ID, Name: n, Role: a.Role, IsBlocked: a.IsBlocked,
		})
	}
	return out, nil
}

func (f *fakeAccounts) ListCharacters(_ context.Context, accountID int64) ([]domain.Character, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listCharsErr != nil {
		return nil, f.listCharsErr
	}
	return f.chars[accountID], nil
}

func (f *fakeAccounts) AccountByName(_ context.Context, name string) (store.AccountAuth, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.rows[name]
	if !ok {
		return store.AccountAuth{}, store.ErrNotFound
	}
	return a, nil
}

func (f *fakeAccounts) AccountRole(_ context.Context, id int64) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, a := range f.rows {
		if a.ID == id {
			return a.Role, nil
		}
	}
	return "", store.ErrNotFound
}

func (f *fakeAccounts) setRole(name, role string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := f.rows[name]
	a.Role = role
	f.rows[name] = a
}

func (f *fakeAccounts) setBlocked(name string, blocked bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := f.rows[name]
	a.IsBlocked = blocked
	f.rows[name] = a
}

// fakeAudit is an in-memory action log.
type fakeAudit struct {
	mu        sync.Mutex
	entries   []audit.Entry
	written   []audit.Record
	failWrite error
	failList  error
	limit     int
	// espelha makes Write feed ListActions, the way the real store does. Off by
	// default so tests that seed entries with add() keep seeing only those.
	espelha bool
}

func newFakeAudit() *fakeAudit { return &fakeAudit{limit: 100} }

// newFakeAuditEspelhado is the audit log a handler can read its own writes back
// from — needed by anything that restores a previous state from the trail.
func newFakeAuditEspelhado() *fakeAudit { return &fakeAudit{limit: 100, espelha: true} }

func (f *fakeAudit) Limit() int { return f.limit }

// List honours limit and offset like the real store, so a pagination test
// exercises the paging and not the fake's willingness to return everything.
func (f *fakeAudit) List(_ context.Context, targetID int64, limit, offset int) ([]audit.Entry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failList != nil {
		return nil, f.failList
	}
	out := make([]audit.Entry, 0, len(f.entries))
	for _, e := range f.entries {
		if targetID == 0 || e.TargetID == targetID {
			out = append(out, e)
		}
	}
	if offset >= len(out) {
		return nil, nil
	}
	out = out[offset:]
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeAudit) ListActions(_ context.Context, actions []string) ([]audit.Entry, error) {
	quer := make(map[string]bool, len(actions))
	for _, a := range actions {
		quer[a] = true
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]audit.Entry, 0, len(f.entries))
	for _, e := range f.entries {
		if quer[e.Action] {
			out = append(out, e)
		}
	}
	return out, nil
}

// Write records what a handler asked to log, and can be made to fail so the
// "changed but not audited" path is exercised rather than assumed.
func (f *fakeAudit) Write(_ context.Context, r audit.Record) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failWrite != nil {
		return f.failWrite
	}
	f.written = append(f.written, r)
	// Opt-in, because most tests assert on entries they placed with add() and
	// would start seeing their own writes echoed back. The handlers that read
	// their own trail — restoring a Mesa de XP branch from the log — need the
	// round trip to be real, including the JSON encoding the store does.
	if f.espelha {
		e := audit.Entry{
			ID: int64(len(f.entries) + 1), ActorID: r.ActorID, ActorRole: r.ActorRole,
			Action: r.Action, TargetID: r.TargetID,
			CreatedAt: time.Now(),
		}
		if r.Old != nil {
			b, err := json.MarshalIndent(r.Old, "", "  ")
			if err != nil {
				return err
			}
			e.Old = string(b)
		}
		if r.New != nil {
			b, err := json.MarshalIndent(r.New, "", "  ")
			if err != nil {
				return err
			}
			e.New = string(b)
		}
		f.entries = append(f.entries, e)
	}
	return nil
}

func (f *fakeAudit) recorded() []audit.Record {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]audit.Record(nil), f.written...)
}

// listadas is what a reader of the log would see, ids included — only populated
// on an espelha fake.
func (f *fakeAudit) listadas() []audit.Entry {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]audit.Entry(nil), f.entries...)
}

func (f *fakeAudit) add(e audit.Entry) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = append(f.entries, e)
}

// newTestPanel builds a handler over the given accounts, with logs discarded.
func newTestPanel(t *testing.T, acc Accounts) http.Handler {
	t.Helper()
	return newTestPanelWith(t, acc, newFakeAudit())
}

// fakeWriter records the writes asked of it and can be made to refuse.
type fakeWriter struct {
	mu           sync.Mutex
	roleCall     []string
	blkCall      []bool
	lastActor    int64 // who the handler said was acting
	lastTarget   int64 // and on whom
	vipDays      []int
	vipCleared   int
	prevRole     string
	prevBlk      bool
	prevVip      *time.Time
	pendentes    int
	ultimaEdicao time.Time
	details      accounts.Details
	senhaHash    []string
	criadas      []contaCriada // Criar calls
	criarErr     error         // forces Criar to return this
	motivos      []string
	diasBan      []int
	buscas       []string
	achados      []accounts.Achado
	buscaErr     error
	prevMotivo   string
	euBloqueado  bool // what Blocked() answers for the signed-in account
	blockedErr   error
	err          error
}

func (f *fakeWriter) Buscar(_ context.Context, prefixo string, _ int) ([]accounts.Achado, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.buscas = append(f.buscas, prefixo)
	if f.buscaErr != nil {
		return nil, f.buscaErr
	}
	out := []accounts.Achado{}
	for _, a := range f.achados {
		// Like the real one: the term hits the account name OR a character on it.
		if prefixo == "" || strings.HasPrefix(a.Name, prefixo) ||
			strings.HasPrefix(strings.ToLower(a.PorPersonagem), prefixo) {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeWriter) SetPassword(_ context.Context, _ int64, hash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.senhaHash = append(f.senhaHash, hash)
	return nil
}

// criada records one Criar call, so a test can assert on the name and the email
// without reaching for the hash.
type contaCriada struct {
	Nome  string
	Hash  string
	Email string
}

func (f *fakeWriter) Criar(_ context.Context, nome, hash, email string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.criarErr != nil {
		return 0, f.criarErr
	}
	f.criadas = append(f.criadas, contaCriada{Nome: nome, Hash: hash, Email: email})
	return int64(100 + len(f.criadas)), nil
}

func newFakeWriter() *fakeWriter { return &fakeWriter{prevRole: "player"} }

func (f *fakeWriter) SetRole(_ context.Context, actorID, targetID int64, role string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastActor, f.lastTarget = actorID, targetID
	if f.err != nil {
		return "", f.err
	}
	f.roleCall = append(f.roleCall, role)
	return f.prevRole, nil
}

func (f *fakeWriter) PendingSince(_ context.Context, _ time.Time) (int, time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.pendentes, f.ultimaEdicao, nil
}

func (f *fakeWriter) Get(_ context.Context, _ int64) (accounts.Details, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.details, nil
}

func (f *fakeWriter) AddVipDays(_ context.Context, actorID, targetID int64, days int) (*time.Time, *time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastActor, f.lastTarget = actorID, targetID
	if f.err != nil {
		return nil, nil, f.err
	}
	f.vipDays = append(f.vipDays, days)
	// Mirror the real extension rule so the handler's message can be tested:
	// count from the later of now and the current expiry.
	from := time.Now()
	if f.prevVip != nil && f.prevVip.After(from) {
		from = *f.prevVip
	}
	novo := from.AddDate(0, 0, days)
	return f.prevVip, &novo, nil
}

func (f *fakeWriter) ClearVip(_ context.Context, actorID, targetID int64) (*time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastActor, f.lastTarget = actorID, targetID
	if f.err != nil {
		return nil, f.err
	}
	f.vipCleared++
	return f.prevVip, nil
}

func (f *fakeWriter) SetBlocked(_ context.Context, actorID, targetID int64, blocked bool, motivo string, dias int) (accounts.Bloqueio, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastActor, f.lastTarget = actorID, targetID
	if f.err != nil {
		return accounts.Bloqueio{}, f.err
	}
	// The real store refuses a block with no reason; the fake has to as well, or
	// a handler that stopped sending one would still pass.
	if blocked && motivo == "" {
		return accounts.Bloqueio{}, accounts.ErrMotivo
	}
	f.blkCall = append(f.blkCall, blocked)
	f.motivos = append(f.motivos, motivo)
	f.diasBan = append(f.diasBan, dias)
	return accounts.Bloqueio{Blocked: f.prevBlk, Reason: f.prevMotivo}, nil
}

// Blocked answers the per-request check requireStaff now makes. It defaults to
// false so every existing test keeps its session.
func (f *fakeWriter) Blocked(_ context.Context, _ int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.euBloqueado, f.blockedErr
}

func newTestPanelWith(t *testing.T, acc Accounts, log AuditLog) http.Handler {
	t.Helper()
	return newTestPanelFull(t, acc, log, newFakeWriter())
}

func newTestPanelFull(t *testing.T, acc Accounts, log AuditLog, wr Writer) http.Handler {
	t.Helper()
	// The search moved from the store to the panel's own writer, so the two
	// fakes have to describe the same world: without this, a test that seeded
	// accounts would search an empty one.
	if fa, ok := acc.(*fakeAccounts); ok {
		if fw, ok := wr.(*fakeWriter); ok {
			fa.mu.Lock()
			for nome, row := range fa.rows {
				fw.achados = append(fw.achados, accounts.Achado{
					AccountSummary: domain.AccountSummary{
						ID: row.ID, Name: nome, Role: row.Role, IsBlocked: row.IsBlocked,
					},
				})
			}
			fa.mu.Unlock()
			sort.Slice(fw.achados, func(i, j int) bool { return fw.achados[i].Name < fw.achados[j].Name })
		}
	}
	h, err := New(Config{
		Accounts:   acc,
		Writer:     wr,
		Audit:      log,
		Sessions:   session.New(time.Hour),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

// postLogin submits the login form and returns the response recorder.
func postLogin(h http.Handler, user, pass string) *httptest.ResponseRecorder {
	form := url.Values{"usuario": {user}, "senha": {pass}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// sessionCookie pulls the session cookie out of a response, or "" if unset.
func sessionCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == cookieName && c.Value != "" {
			return c
		}
	}
	return nil
}

func TestHealthzNeedsNoSession(t *testing.T) {
	h := newTestPanel(t, newFakeAccounts(roleAdmin))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "ok" {
		t.Fatalf("body = %q, want ok", got)
	}
}

func TestHomeRedirectsWhenSignedOut(t *testing.T) {
	h := newTestPanel(t, newFakeAccounts(roleAdmin))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Fatalf("Location = %q, want /login", loc)
	}
}

func TestLoginSucceedsForStaff(t *testing.T) {
	for _, role := range []string{roleModerator, roleAdmin} {
		t.Run(role, func(t *testing.T) {
			h := newTestPanel(t, newFakeAccounts(role))
			rec := postLogin(h, "chefe", testPassword)
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("status = %d, want 303", rec.Code)
			}
			c := sessionCookie(rec)
			if c == nil {
				t.Fatal("no session cookie set")
			}
			if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode {
				t.Fatalf("cookie flags = HttpOnly:%v Secure:%v SameSite:%v; want all strict",
					c.HttpOnly, c.Secure, c.SameSite)
			}
		})
	}
}

func TestLoginIsRefusedAndLeaksNothing(t *testing.T) {
	cases := []struct {
		name  string
		user  string
		pass  string
		setup func(*fakeAccounts)
	}{
		{name: "senha errada", user: "chefe", pass: "outra-coisa"},
		{name: "conta inexistente", user: "ninguem", pass: testPassword},
		{name: "cargo de jogador", user: "chefe", pass: testPassword,
			setup: func(f *fakeAccounts) { f.setRole("chefe", "player") }},
		{name: "cargo desconhecido", user: "chefe", pass: testPassword,
			setup: func(f *fakeAccounts) { f.setRole("chefe", "superadmin") }},
		{name: "conta bloqueada", user: "chefe", pass: testPassword,
			setup: func(f *fakeAccounts) { f.setBlocked("chefe", true) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			acc := newFakeAccounts(roleAdmin)
			if tc.setup != nil {
				tc.setup(acc)
			}
			h := newTestPanel(t, acc)
			rec := postLogin(h, tc.user, tc.pass)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
			if c := sessionCookie(rec); c != nil {
				t.Fatal("a refused login set a session cookie")
			}
			// Every rejection says the same thing: the page must not hint at
			// which of the five reasons applied.
			if !strings.Contains(rec.Body.String(), loginFailed) {
				t.Fatalf("body does not carry the generic message: %q", rec.Body.String())
			}
		})
	}
}

func TestLoginNameIsCanonicalisedToLowercase(t *testing.T) {
	// The game lowercases the account name at login (handler/login.go), so the
	// panel must match or staff typing "Chefe" would be told their account does
	// not exist.
	h := newTestPanel(t, newFakeAccounts(roleAdmin))
	rec := postLogin(h, "  CHEFE  ", testPassword)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
}

func TestLoginIsRateLimited(t *testing.T) {
	h := newTestPanel(t, newFakeAccounts(roleAdmin))
	for i := 0; i < loginBurst; i++ {
		if rec := postLogin(h, "chefe", "errada"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want 401", i+1, rec.Code)
		}
	}
	if rec := postLogin(h, "chefe", "errada"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("attempt %d: status = %d, want 429", loginBurst+1, rec.Code)
	}
}

func TestRateLimitAlsoStopsSprayingManyNames(t *testing.T) {
	// A different account name each time must still be stopped: the source
	// address is charged too, so the per-account bucket cannot be dodged.
	h := newTestPanel(t, newFakeAccounts(roleAdmin))
	for i := 0; i < loginBurst; i++ {
		postLogin(h, "conta"+string(rune('a'+i)), "errada")
	}
	if rec := postLogin(h, "outronome", "errada"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
}

func TestDemotionEndsTheSessionOnTheNextRequest(t *testing.T) {
	acc := newFakeAccounts(roleAdmin)
	h := newTestPanel(t, acc)

	c := sessionCookie(postLogin(h, "chefe", testPassword))
	if c == nil {
		t.Fatal("login did not set a cookie")
	}

	get := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(c)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	if rec := get(); rec.Code != http.StatusOK {
		t.Fatalf("signed-in request: status = %d, want 200", rec.Code)
	}

	// Demote in the database, touching nothing about the live session.
	acc.setRole("chefe", "player")

	rec := get()
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("after demotion: status = %d, want 303", rec.Code)
	}
	// And the session is gone, not merely refused: restoring the role must not
	// silently bring the old session back to life.
	acc.setRole("chefe", roleAdmin)
	if rec := get(); rec.Code != http.StatusSeeOther {
		t.Fatalf("session survived demotion: status = %d, want 303", rec.Code)
	}
}

func TestLogoutEndsTheSession(t *testing.T) {
	h := newTestPanel(t, newFakeAccounts(roleAdmin))
	c := sessionCookie(postLogin(h, "chefe", testPassword))
	if c == nil {
		t.Fatal("login did not set a cookie")
	}

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("logout status = %d, want 303", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(c)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("session still valid after logout: status = %d", rec.Code)
	}
}

func TestSecurityHeadersAreSet(t *testing.T) {
	h := newTestPanel(t, newFakeAccounts(roleAdmin))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
	for _, hdr := range []string{"Content-Security-Policy", "X-Content-Type-Options", "Referrer-Policy"} {
		if rec.Header().Get(hdr) == "" {
			t.Errorf("%s not set", hdr)
		}
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("CSP does not forbid framing: %q", csp)
	}
}

func TestClientIPPrefersLeftmostForwarded(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 70.41.3.18")
	if got := clientIP(req); got != "203.0.113.7" {
		t.Fatalf("clientIP = %q, want the original client 203.0.113.7", got)
	}
}

// --- contas ---

// signedIn logs in and returns a getter that carries the session cookie.
func signedIn(t *testing.T, h http.Handler) func(path string) *httptest.ResponseRecorder {
	t.Helper()
	c := sessionCookie(postLogin(h, "chefe", testPassword))
	if c == nil {
		t.Fatal("login did not set a cookie")
	}
	return func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(c)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
}

func TestContasNeedsASession(t *testing.T) {
	h := newTestPanel(t, newFakeAccounts(roleAdmin))
	for _, path := range []string{"/contas", "/contas/chefe"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s: status = %d, want 303 to /login", path, rec.Code)
		}
	}
}

func TestContasListsAndFiltersByPrefix(t *testing.T) {
	acc := newFakeAccounts(roleAdmin)
	acc.add("ana", 2, "player", false)
	acc.add("andre", 3, "moderator", false)
	acc.add("bruno", 4, "player", true)
	h := newTestPanel(t, acc)
	get := signedIn(t, h)

	all := get("/contas").Body.String()
	for _, name := range []string{"ana", "andre", "bruno", "chefe"} {
		if !strings.Contains(all, name) {
			t.Errorf("listing without a query omitted %q", name)
		}
	}

	// The blocked account has to be visible as blocked; that is the point of the
	// column for someone triaging a support ticket.
	if !strings.Contains(all, "bloqueada") {
		t.Error("listing does not mark the blocked account")
	}

	filtered := get("/contas?q=an").Body.String()
	if !strings.Contains(filtered, "andre") || !strings.Contains(filtered, "ana") {
		t.Error("prefix search dropped a matching account")
	}
	if strings.Contains(filtered, "bruno") {
		t.Error("prefix search returned a non-matching account")
	}
}

func TestContasSearchIsCaseInsensitive(t *testing.T) {
	// Names are stored canonical, so typing the name as it looks in game must work.
	acc := newFakeAccounts(roleAdmin)
	acc.add("ana", 2, "player", false)
	get := signedIn(t, newTestPanel(t, acc))
	if !strings.Contains(get("/contas?q=ANA").Body.String(), "ana") {
		t.Error("uppercase query found nothing")
	}
}

func TestContasCapsTheListingAndSaysSo(t *testing.T) {
	acc := newFakeAccounts(roleAdmin)
	for i := 0; i < searchLimit+10; i++ {
		acc.add(fmt.Sprintf("conta%03d", i), int64(1000+i), "player", false)
	}
	body := signedIn(t, newTestPanel(t, acc))("/contas").Body.String()
	if !strings.Contains(body, "Refine a busca") {
		t.Error("over the cap, the page does not say the list is partial")
	}
	if strings.Count(body, "<tr>") > searchLimit+1 { // +1 for the header row
		t.Error("more rows rendered than the cap allows")
	}
}

func TestContaShowsAccountAndCharacters(t *testing.T) {
	acc := newFakeAccounts(roleAdmin)
	acc.addChar(42, domain.Character{Slot: 0, Name: "Hanteste", Level: 7, Coin: 900000, Hp: 105, MaxHp: 105})
	// A conta e a lista de personagens vivem em abas diferentes agora, então a
	// identidade da conta é conferida na aba padrão e o elenco na dele.
	get := signedIn(t, newTestPanel(t, acc))
	if !strings.Contains(get("/contas/chefe").Body.String(), "chefe") {
		t.Error("detail page missing the account name")
	}
	body := get("/contas/chefe?aba=personagens").Body.String()

	for _, want := range []string{"Hanteste", "900000", "105"} {
		if !strings.Contains(body, want) {
			t.Errorf("detail page missing %q", want)
		}
	}
}

func TestContaPathIsCaseInsensitive(t *testing.T) {
	get := signedIn(t, newTestPanel(t, newFakeAccounts(roleAdmin)))
	if rec := get("/contas/CHEFE"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestContaUnknownIs404(t *testing.T) {
	get := signedIn(t, newTestPanel(t, newFakeAccounts(roleAdmin)))
	if rec := get("/contas/ninguem"); rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestContaNeverRendersThePasswordHash(t *testing.T) {
	// store.AccountAuth carries pass_hash, so the detail handler projects into a
	// view type instead of handing the row to the template. If that ever gets
	// undone, the hash lands in staff browsers and in any page cache.
	acc := newFakeAccounts(roleAdmin)
	acc.addChar(42, domain.Character{Slot: 0, Name: "Hanteste", Level: 7})
	body := signedIn(t, newTestPanel(t, acc))("/contas/chefe").Body.String()

	if strings.Contains(body, "$argon2id$") || strings.Contains(body, hashOnce()) {
		t.Fatal("the password hash was rendered into the page")
	}
}

func TestContaWithNoCharacters(t *testing.T) {
	body := signedIn(t, newTestPanel(t, newFakeAccounts(roleAdmin)))("/contas/chefe?aba=personagens").Body.String()
	if !strings.Contains(body, "ainda não criou personagem") {
		t.Error("empty roster does not say so")
	}
}

// --- auditoria ---

func TestAuditoriaIsAdminOnly(t *testing.T) {
	// A moderator is signed in and their session is fine, so they get 403 with an
	// explanation — not a redirect to login, which would read as a bug.
	acc := newFakeAccounts(roleModerator)
	get := signedIn(t, newTestPanel(t, acc))
	rec := get("/auditoria")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("moderator status = %d, want 403", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "só para administradores") {
		t.Error("403 page does not explain why")
	}
}

func TestAuditoriaNavIsHiddenFromModerators(t *testing.T) {
	mod := signedIn(t, newTestPanel(t, newFakeAccounts(roleModerator)))("/")
	if strings.Contains(mod.Body.String(), `href="/auditoria"`) {
		t.Error("a moderator is offered a link they will be refused from")
	}
	adm := signedIn(t, newTestPanel(t, newFakeAccounts(roleAdmin)))("/")
	if !strings.Contains(adm.Body.String(), `href="/auditoria"`) {
		t.Error("an admin is not offered the audit link")
	}
}

func TestAuditoriaListsEntries(t *testing.T) {
	log := newFakeAudit()
	log.add(audit.Entry{
		ID: 1, ActorID: 42, ActorName: "chefe", ActorRole: roleAdmin,
		Action: audit.ActionSetRole, TargetID: 7, TargetName: "ana",
		Old: `{"role":"player"}`, New: `{"role":"moderator"}`,
		CreatedAt: time.Date(2026, 9, 4, 17, 30, 0, 0, time.UTC),
	})
	body := signedIn(t, newTestPanelWith(t, newFakeAccounts(roleAdmin), log))("/auditoria").Body.String()

	for _, want := range []string{"chefe", "Mudou o cargo", "SET_ROLE", "ana", "player", "moderator", "04/09 17:30"} {
		if !strings.Contains(body, want) {
			t.Errorf("audit page missing %q", want)
		}
	}
}

func TestAuditoriaFiltersByAccount(t *testing.T) {
	log := newFakeAudit()
	log.add(audit.Entry{ID: 1, ActorName: "chefe", ActorRole: roleAdmin,
		Action: audit.ActionSetRole, TargetID: 7, TargetName: "ana"})
	log.add(audit.Entry{ID: 2, ActorName: "chefe", ActorRole: roleAdmin,
		Action: audit.ActionSetBlocked, TargetID: 9, TargetName: "bruno"})
	get := signedIn(t, newTestPanelWith(t, newFakeAccounts(roleAdmin), log))

	all := get("/auditoria").Body.String()
	if !strings.Contains(all, "ana") || !strings.Contains(all, "bruno") {
		t.Fatal("unfiltered page is missing an entry")
	}

	one := get("/auditoria?conta=7").Body.String()
	if !strings.Contains(one, "ana") {
		t.Error("filtered page dropped the matching entry")
	}
	if strings.Contains(one, "bruno") {
		t.Error("filtered page kept a non-matching entry")
	}
}

func TestAuditoriaRejectsABadAccountFilter(t *testing.T) {
	get := signedIn(t, newTestPanel(t, newFakeAccounts(roleAdmin)))
	for _, q := range []string{"abc", "-1", "0"} {
		if rec := get("/auditoria?conta=" + q); rec.Code != http.StatusBadRequest {
			t.Errorf("conta=%s: status = %d, want 400", q, rec.Code)
		}
	}
}

func TestAuditoriaEmpty(t *testing.T) {
	body := signedIn(t, newTestPanel(t, newFakeAccounts(roleAdmin)))("/auditoria").Body.String()
	if !strings.Contains(body, "Nenhuma ação administrativa registrada") {
		t.Error("empty log does not say so")
	}
}

// --- escritas: cargo e bloqueio ---

// signedInPost logs in and returns a poster that carries the cookie and, unless
// told otherwise, the session's real CSRF token.
func signedInPost(t *testing.T, h http.Handler) (func(path string, form url.Values) *httptest.ResponseRecorder, string) {
	t.Helper()
	c := sessionCookie(postLogin(h, "chefe", testPassword))
	if c == nil {
		t.Fatal("login did not set a cookie")
	}
	// The token is rendered into every form; read it back the way a browser would.
	req := httptest.NewRequest(http.MethodGet, "/contas/ana", nil)
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	token := csrfFrom(rec.Body.String())

	return func(path string, form url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(c)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}, token
}

func csrfFrom(body string) string {
	const marker = `name="csrf" value="`
	i := strings.Index(body, marker)
	if i < 0 {
		return ""
	}
	rest := body[i+len(marker):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		return ""
	}
	return rest[:j]
}

// withTarget gives the panel an account other than the signed-in one to act on.
func withTarget(role string) *fakeAccounts {
	acc := newFakeAccounts(role)
	acc.add("ana", 7, "player", false)
	return acc
}

func TestSetCargoAppliesAndAudits(t *testing.T) {
	acc := withTarget(roleAdmin)
	log := newFakeAudit()
	wr := newFakeWriter()
	h := newTestPanelFull(t, acc, log, wr)
	post, token := signedInPost(t, h)

	rec := post("/contas/ana/cargo", url.Values{"csrf": {token}, "cargo": {"moderator"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(wr.roleCall) != 1 || wr.roleCall[0] != "moderator" {
		t.Fatalf("writer calls = %v, want one moderator", wr.roleCall)
	}
	// The actor comes from the session, never from the form: a request that could
	// name its own actor would let anyone attribute a change to someone else, and
	// the self-change and last-admin guards both key off this value.
	if wr.lastActor != 42 {
		t.Errorf("actor passed to the writer = %d, want the signed-in account (42)", wr.lastActor)
	}
	if wr.lastTarget != 7 {
		t.Errorf("target passed to the writer = %d, want the account in the path (7)", wr.lastTarget)
	}

	recs := log.recorded()
	if len(recs) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(recs))
	}
	if recs[0].Action != audit.ActionSetRole || recs[0].TargetID != 7 {
		t.Fatalf("audit entry = %+v, want SET_ROLE on 7", recs[0])
	}
	if recs[0].ActorRole != roleAdmin {
		t.Errorf("actor role = %q, want the role at the time (%s)", recs[0].ActorRole, roleAdmin)
	}
}

func TestWritesRequireTheCSRFToken(t *testing.T) {
	h := newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), newFakeWriter())
	post, token := signedInPost(t, h)

	cases := []struct {
		name string
		form url.Values
	}{
		{"sem token", url.Values{"cargo": {"moderator"}}},
		{"token errado", url.Values{"csrf": {"nao-e-o-token"}, "cargo": {"moderator"}}},
		{"token de outra sessao", url.Values{"csrf": {token + "x"}, "cargo": {"moderator"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, path := range []string{"/contas/ana/cargo", "/contas/ana/bloqueio"} {
				if rec := post(path, tc.form); rec.Code != http.StatusForbidden {
					t.Errorf("%s: status = %d, want 403", path, rec.Code)
				}
			}
		})
	}
}

func TestSetCargoIsAdminOnly(t *testing.T) {
	// A moderator may block, but not promote.
	h := newTestPanelFull(t, withTarget(roleModerator), newFakeAudit(), newFakeWriter())
	post, token := signedInPost(t, h)

	if rec := post("/contas/ana/cargo", url.Values{"csrf": {token}, "cargo": {"admin"}}); rec.Code != http.StatusForbidden {
		t.Errorf("moderator changing role: status = %d, want 403", rec.Code)
	}
	if rec := post("/contas/ana/bloqueio", url.Values{"csrf": {token}, "bloquear": {"1"}, "motivo": {"teste"}}); rec.Code != http.StatusSeeOther {
		t.Errorf("moderator blocking: status = %d, want 303", rec.Code)
	}
}

func TestRefusalsAreExplained(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"propria conta", accounts.ErrSelf, "próprio acesso"},
		{"ultimo admin", accounts.ErrLastAdmin, "último admin"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wr := newFakeWriter()
			wr.err = tc.err
			h := newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), wr)
			post, token := signedInPost(t, h)

			rec := post("/contas/ana/cargo", url.Values{"csrf": {token}, "cargo": {"player"}})
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("status = %d, want 303 back to the account page", rec.Code)
			}
			loc := rec.Header().Get("Location")
			decoded, _ := url.QueryUnescape(loc)
			if !strings.Contains(decoded, tc.want) {
				t.Fatalf("redirect %q does not explain the refusal (want %q)", decoded, tc.want)
			}
		})
	}
}

func TestAChangeThatCannotBeAuditedIsReportedAsAFailure(t *testing.T) {
	// The audit write failing must not pass silently. Staff have to know the log
	// has a hole rather than trust a green screen.
	log := newFakeAudit()
	log.failWrite = errors.New("banco fora do ar")
	h := newTestPanelFull(t, withTarget(roleAdmin), log, newFakeWriter())
	post, token := signedInPost(t, h)

	rec := post("/contas/ana/cargo", url.Values{"csrf": {token}, "cargo": {"moderator"}})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "auditoria falhou") {
		t.Errorf("the page does not say the audit failed: %q", rec.Body.String())
	}
}

func TestOwnAccountShowsNoControls(t *testing.T) {
	get := signedIn(t, newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), newFakeWriter()))

	mine := get("/contas/chefe").Body.String()
	if strings.Contains(mine, `action="/contas/chefe/cargo"`) {
		t.Error("the panel offers a control on your own account that always fails")
	}
	if !strings.Contains(mine, "sua própria conta") {
		t.Error("no explanation of why the controls are absent")
	}

	other := get("/contas/ana").Body.String()
	if !strings.Contains(other, `action="/contas/ana/cargo"`) {
		t.Error("the role form is missing on another account")
	}
}

func TestBlockSaysItOnlyAppliesAtLogin(t *testing.T) {
	h := newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), newFakeWriter())
	post, token := signedInPost(t, h)
	rec := post("/contas/ana/bloqueio", url.Values{"csrf": {token}, "bloquear": {"1"}, "motivo": {"teste"}})
	decoded, _ := url.QueryUnescape(rec.Header().Get("Location"))
	if !strings.Contains(decoded, "continua até sair") {
		t.Errorf("staff are not told a blocked player stays online: %q", decoded)
	}
}

// --- vip ---

func TestVipGrantIsAuditedAndReported(t *testing.T) {
	wr := newFakeWriter()
	log := newFakeAudit()
	h := newTestPanelFull(t, withTarget(roleModerator), log, wr)
	post, token := signedInPost(t, h)

	rec := post("/contas/ana/vip", url.Values{"csrf": {token}, "dias": {"30"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(wr.vipDays) != 1 || wr.vipDays[0] != 30 {
		t.Fatalf("writer calls = %v, want one grant of 30", wr.vipDays)
	}

	recs := log.recorded()
	if len(recs) != 1 || recs[0].Action != audit.ActionSetVip {
		t.Fatalf("audit entries = %+v, want one SET_VIP", recs)
	}
}

func TestVipGrantOnTopOfAnActiveOneSaysItExtended(t *testing.T) {
	// Staff who read "VIP até <date>" on an already-active account cannot tell
	// whether the remaining days survived. Saying "estendido de X para Y" is what
	// stops somebody granting the days a second time.
	wr := newFakeWriter()
	ate := time.Now().AddDate(0, 0, 10)
	wr.prevVip = &ate
	h := newTestPanelFull(t, withTarget(roleModerator), newFakeAudit(), wr)
	post, token := signedInPost(t, h)

	rec := post("/contas/ana/vip", url.Values{"csrf": {token}, "dias": {"30"}})
	loc, _ := url.QueryUnescape(rec.Header().Get("Location"))
	if !strings.Contains(loc, "estendido") {
		t.Fatalf("message %q does not say the grant extended the existing one", loc)
	}
}

func TestVipRemoval(t *testing.T) {
	wr := newFakeWriter()
	ate := time.Now().AddDate(0, 0, 5)
	wr.prevVip = &ate
	log := newFakeAudit()
	h := newTestPanelFull(t, withTarget(roleModerator), log, wr)
	post, token := signedInPost(t, h)

	rec := post("/contas/ana/vip", url.Values{"csrf": {token}, "remover": {"1"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if wr.vipCleared != 1 {
		t.Fatalf("ClearVip calls = %d, want 1", wr.vipCleared)
	}
	if len(log.recorded()) != 1 {
		t.Error("removing VIP was not audited")
	}
}

func TestVipRemovalOnAnAccountWithoutVipChangesNothing(t *testing.T) {
	wr := newFakeWriter() // prevVip stays nil
	log := newFakeAudit()
	h := newTestPanelFull(t, withTarget(roleModerator), log, wr)
	post, token := signedInPost(t, h)

	post("/contas/ana/vip", url.Values{"csrf": {token}, "remover": {"1"}})
	if len(log.recorded()) != 0 {
		t.Error("a no-op removal was written to the audit log")
	}
}

func TestVipRejectsABadDayCount(t *testing.T) {
	h := newTestPanelFull(t, withTarget(roleModerator), newFakeAudit(), newFakeWriter())
	post, token := signedInPost(t, h)
	for _, d := range []string{"", "abc", "1,5"} {
		if rec := post("/contas/ana/vip", url.Values{"csrf": {token}, "dias": {d}}); rec.Code != http.StatusBadRequest {
			t.Errorf("dias=%q: status = %d, want 400", d, rec.Code)
		}
	}
}

func TestVipOutOfRangeIsExplained(t *testing.T) {
	wr := newFakeWriter()
	wr.err = accounts.ErrVipDays
	h := newTestPanelFull(t, withTarget(roleModerator), newFakeAudit(), wr)
	post, token := signedInPost(t, h)

	rec := post("/contas/ana/vip", url.Values{"csrf": {token}, "dias": {"99999"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "3650") {
		t.Errorf("the message does not say the allowed range: %q", rec.Body.String())
	}
}

func TestVipNeedsTheCSRFToken(t *testing.T) {
	h := newTestPanelFull(t, withTarget(roleModerator), newFakeAudit(), newFakeWriter())
	post, _ := signedInPost(t, h)
	if rec := post("/contas/ana/vip", url.Values{"dias": {"30"}}); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestAccountPageShowsVipEmailAndBalance(t *testing.T) {
	wr := newFakeWriter()
	ate := time.Now().AddDate(0, 0, 12)
	wr.details = accounts.Details{Email: "chefe@exemplo.com", DonateBalance: 4200, VipUntil: &ate}
	body := signedIn(t, newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), wr))("/contas/ana").Body.String()

	for _, want := range []string{"chefe@exemplo.com", "4200", ate.Local().Format("02/01/2006")} {
		if !strings.Contains(body, want) {
			t.Errorf("account page missing %q", want)
		}
	}
}

// --- itens ---

// fakeGameData stands in for the webServer link.
type fakeGameData struct {
	mu              sync.Mutex
	itens           []gamedata.Item
	setCalls        [][2]int64 // index, price
	listErr         error
	setErr          error
	versao          string
	npcs            []gamedata.NPC
	npcsErr         error
	shopErr         error
	shopSaves       [][]gamedata.ShopItem
	saved           []gamedata.NPC
	visible         []bool
	visibleFor      []int64
	deleted         []int64
	npcWriteErr     error
	mobs            []gamedata.MobTemplate
	mobRows         map[string]mobRow
	mobSaved        []gamedata.MobStat
	mobCleared      []string
	mobErr          error
	mobWriteErr     error
	mobEquip        map[string][]gamedata.MobEquipItem // captured SaveMobEquip calls
	itemRows        map[int32]itemRow
	itemSaved       []gamedata.ItemStat
	itemCleared     []int32
	itemStatErr     error
	itemWriteErr    error
	drops           []gamedata.Drop
	dropsPedidos    [][2]string
	dropsErr        error
	curvas          []gamedata.MountGrowthCurve
	curvaSalva      map[int32][]int32
	curvaLimpa      []int32
	curvaErr        error
	curvaGravaEr    error
	absorbs         []gamedata.MountAbsorb
	absorbSalvo     map[int32][2]int32
	absorbLimpo     []int32
	absorbErr       error
	absorbGravaEr   error
	versaoMontarias int64
}

func newFakeGameData() *fakeGameData {
	return &fakeGameData{
		versao: "abc123",
		itens: []gamedata.Item{
			{Index: 1415, Name: "Sapatos_Pele_de_Animal(A)", DisplayName: "Sapatos Pele de Animal (A)",
				Grade: 1, Slots: []string{"boots"}},
			{Index: 2000, Name: "Espada_Longa", DisplayName: "Espada Longa",
				Grade: 2, Slots: []string{"weapon"}, Price: 5000, Overridden: true},
		},
		npcs: []gamedata.NPC{{
			ID: 5, Slug: "mercador-armia", DisplayName: "Mercador de Armia",
			TemplateName: "Mercador", Enabled: true, MapID: 0, X: 2100, Y: 2100,
			Origin: "content",
			Shop: []gamedata.ShopItem{
				{Slot: 0, ItemIndex: 1415, Quantity: 1, Eff: [3][2]int32{{7, 42}, {0, 0}, {0, 0}}},
			},
		}},
		mobs: []gamedata.MobTemplate{
			{Name: "Kentania", DisplayName: "Kentania"},
			{Name: "Mercador", DisplayName: "Mercador de Armia", Merchant: 8},
		},
		mobRows: map[string]mobRow{
			"Kentania": {
				exibido: "Kentania Velha", overridden: true,
				valores: map[string]int64{"level": 10, "exp": 5000, "resist1": 7},
				origens: []gamedata.MobOrigem{
					{Local: "Água Místico", Pontos: 2, Quantidade: 24, RespawnMin: 3, X: 1250, Y: 3608},
					// A follower: it marks the place but its monsters are counted
					// on its leader's block.
					{Local: "Água Arcano", Pontos: 1, Quantidade: 0, X: 1342, Y: 3517},
				},
			},
			// No origins on purpose: the template that spawns nowhere is the case
			// this whole feature exists to make visible.
			"Mercador": {valores: map[string]int64{"level": 1}},
		},
		drops: []gamedata.Drop{
			{ItemIndex: 2000, ItemName: "Espada Longa", Mobs: []gamedata.DropMob{
				// Divisor 36 is a common slot on a low-level mob: the case where the
				// raw table (900) is off by twenty-five times.
				{TemplateName: "Kentania", MobName: "Kentania", MobLevel: 5, Slot: 0, Divisor: 36},
			}},
			{ItemIndex: 1415, ItemName: "Poção", Mobs: []gamedata.DropMob{
				// Slot 11 is the hard override: guaranteed, whatever the table says.
				{TemplateName: "Kentania", MobName: "Kentania", MobLevel: 5, Slot: 11, Divisor: 1},
			}},
		},
		itemRows: map[int32]itemRow{
			// A weapon carrying one of each kind of number: a stat the editor is
			// for, a requirement, and an identity effect nobody edits but a save
			// would strip if the form did not carry it.
			2000: {
				exibido: "Espada Longa", overridden: true,
				valores: map[string]int64{"damage": 120, "wtype": 3, "req_level": 30, "resist1": 4},
			},
			1415: {valores: map[string]int64{"ac": 12}},
		},
	}
}

func (f *fakeGameData) CatalogVersion() string { return f.versao }

func (f *fakeGameData) Items(_ context.Context, _ int64, query string) ([]gamedata.Item, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	if query == "" {
		return f.itens, nil
	}
	out := []gamedata.Item{}
	for _, it := range f.itens {
		if strings.Contains(strings.ToLower(it.DisplayName), strings.ToLower(query)) {
			out = append(out, it)
		}
	}
	return out, nil
}

func (f *fakeGameData) NPCs(_ context.Context, _ int64, query string) ([]gamedata.NPC, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.npcsErr != nil {
		return nil, f.npcsErr
	}
	out := []gamedata.NPC{}
	for _, n := range f.npcs {
		if query == "" || strings.Contains(strings.ToLower(n.DisplayName), strings.ToLower(query)) {
			out = append(out, n)
		}
	}
	return out, nil
}

func (f *fakeGameData) NPC(_ context.Context, _ int64, id int64) (gamedata.NPC, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, n := range f.npcs {
		if n.ID == id {
			return n, nil
		}
	}
	return gamedata.NPC{}, gamedata.ErrNotFound
}

func (f *fakeGameData) SetShop(_ context.Context, _ int64, _ int64, items []gamedata.ShopItem) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.shopErr != nil {
		return f.shopErr
	}
	f.shopSaves = append(f.shopSaves, items)
	return nil
}

func (f *fakeGameData) SaveNPC(_ context.Context, _ int64, n gamedata.NPC) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.npcWriteErr != nil {
		return f.npcWriteErr
	}
	f.saved = append(f.saved, n)
	for i := range f.npcs {
		if f.npcs[i].Slug == n.Slug {
			f.npcs[i] = n
		}
	}
	return nil
}

func (f *fakeGameData) SetNPCVisible(_ context.Context, _ int64, npcID int64, enabled bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.npcWriteErr != nil {
		return f.npcWriteErr
	}
	// The id is recorded, not ignored: a handler that hides the wrong NPC would
	// otherwise pass, since the visibility flag alone looks identical.
	f.visibleFor = append(f.visibleFor, npcID)
	f.visible = append(f.visible, enabled)
	return nil
}

func (f *fakeGameData) DeleteNPC(_ context.Context, _ int64, npcID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.npcWriteErr != nil {
		return f.npcWriteErr
	}
	f.deleted = append(f.deleted, npcID)
	return nil
}

func (f *fakeGameData) SetPrice(_ context.Context, _ int64, index int32, price int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setErr != nil {
		return f.setErr
	}
	f.setCalls = append(f.setCalls, [2]int64{int64(index), price})
	return nil
}

// newTestPanelGame builds a panel with the webServer link present.
func newTestPanelGame(t *testing.T, log AuditLog, game GameData) http.Handler {
	t.Helper()
	h, err := New(Config{
		Accounts:   withTarget(roleAdmin),
		Writer:     newFakeWriter(),
		GameData:   game,
		Audit:      log,
		Sessions:   session.New(time.Hour),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

func TestItensPageIsHiddenWithoutAWebServer(t *testing.T) {
	// The panel has to run standalone; a missing webServer hides the pages
	// rather than serving one that errors on every load.
	get := signedIn(t, newTestPanel(t, withTarget(roleAdmin)))
	if rec := get("/itens"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when no webServer is configured", rec.Code)
	}
	if strings.Contains(get("/").Body.String(), `href="/itens"`) {
		t.Error("the nav offers a link to a page that does not exist")
	}
}

func TestItensListsAndFilters(t *testing.T) {
	game := newFakeGameData()
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), game))

	all := get("/itens").Body.String()
	if !strings.Contains(all, "Sapatos Pele de Animal") || !strings.Contains(all, "Espada Longa") {
		t.Error("catalog listing is missing an item")
	}
	if !strings.Contains(all, "exceção") {
		t.Error("an overridden price is not marked as such")
	}
	if !strings.Contains(all, game.versao) {
		t.Error("the catalog version is not shown, so a stale page is unrecognisable")
	}
	if strings.Contains(all, `href="/itens"`) == false {
		t.Error("the nav does not offer the items page when it exists")
	}

	filtrado := get("/itens?q=espada").Body.String()
	if strings.Contains(filtrado, "Sapatos") {
		t.Error("search returned a non-matching item")
	}
}

func TestSetPrecoSendsAndAudits(t *testing.T) {
	game := newFakeGameData()
	log := newFakeAudit()
	h := newTestPanelGame(t, log, game)
	post, token := signedInPost(t, h)

	rec := post("/itens/1415/preco", url.Values{"csrf": {token}, "preco": {"250"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(game.setCalls) != 1 || game.setCalls[0] != [2]int64{1415, 250} {
		t.Fatalf("SetPrice calls = %v, want one {1415, 250}", game.setCalls)
	}
	recs := log.recorded()
	if len(recs) != 1 || recs[0].Action != audit.ActionSetItemPrice {
		t.Fatalf("audit = %+v, want one SET_ITEM_PRICE", recs)
	}
}

func TestEmptyPriceClearsTheOverride(t *testing.T) {
	// Empty and zero are different: one removes the exception, the other makes
	// the item free. A number box cannot express the first, so blank means clear.
	game := newFakeGameData()
	h := newTestPanelGame(t, newFakeAudit(), game)
	post, token := signedInPost(t, h)

	post("/itens/2000/preco", url.Values{"csrf": {token}, "preco": {""}})
	if len(game.setCalls) != 1 || game.setCalls[0][1] != -1 {
		t.Fatalf("SetPrice calls = %v, want the clear sentinel (-1)", game.setCalls)
	}

	post("/itens/2000/preco", url.Values{"csrf": {token}, "preco": {"0"}})
	if len(game.setCalls) != 2 || game.setCalls[1][1] != 0 {
		t.Fatalf("SetPrice calls = %v, want an explicit zero as the second", game.setCalls)
	}
}

func TestSetPrecoRejectsBadInput(t *testing.T) {
	h := newTestPanelGame(t, newFakeAudit(), newFakeGameData())
	post, token := signedInPost(t, h)
	for _, p := range []string{"abc", "-5", "1.5"} {
		if rec := post("/itens/1415/preco", url.Values{"csrf": {token}, "preco": {p}}); rec.Code != http.StatusBadRequest {
			t.Errorf("preco=%q: status = %d, want 400", p, rec.Code)
		}
	}
	if rec := post("/itens/0/preco", url.Values{"csrf": {token}, "preco": {"10"}}); rec.Code != http.StatusBadRequest {
		t.Errorf("item 0: status = %d, want 400", rec.Code)
	}
}

func TestSetPrecoNeedsTheCSRFToken(t *testing.T) {
	h := newTestPanelGame(t, newFakeAudit(), newFakeGameData())
	post, _ := signedInPost(t, h)
	if rec := post("/itens/1415/preco", url.Values{"preco": {"250"}}); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestWebServerFailureIsNotAPanelFailure(t *testing.T) {
	// The webServer redeploys on its own schedule. A page that reads as broken
	// during that window sends staff looking in the wrong place.
	game := newFakeGameData()
	game.listErr = errors.New("connection refused")
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), game))

	rec := get("/itens")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 — the failure is upstream, not here", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "reiniciando") {
		t.Error("the message does not point at the right service")
	}
}

// --- estado do servidor e reinicio ---

// fakePlatform stands in for the hosting API.
type fakePlatform struct {
	mu            sync.Mutex
	dep           plataforma.Deployment
	latestErr     error
	restartEr     error
	restarts      []string
	stops         []string
	redeploys     []string
	redeployEr    error
	usouLatestAny int
}

func newFakePlatform() *fakePlatform {
	return &fakePlatform{dep: plataforma.Deployment{
		ID: "dep-1", Status: "SUCCESS", CreatedAt: time.Now().Add(-3 * time.Hour),
	}}
}

func (f *fakePlatform) Latest(context.Context) (plataforma.Deployment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dep, f.latestErr
}

// restartCount is read by fakeJogo to prove the drain ran BEFORE the restart.
func (f *fakePlatform) restartCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.restarts)
}

func (f *fakePlatform) LatestAny(context.Context) (plataforma.Deployment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.usouLatestAny++
	return f.dep, f.latestErr
}

func (f *fakePlatform) Stop(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stops = append(f.stops, id)
	return nil
}

func (f *fakePlatform) Redeploy(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.redeployEr != nil {
		return f.redeployEr
	}
	f.redeploys = append(f.redeploys, id)
	return nil
}

func (f *fakePlatform) stopCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.stops)
}

func (f *fakePlatform) Restart(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.restartEr != nil {
		return f.restartEr
	}
	f.restarts = append(f.restarts, id)
	return nil
}

func newTestPanelPlat(t *testing.T, log AuditLog, wr Writer, plat Platform) http.Handler {
	t.Helper()
	h, err := New(Config{
		Accounts:   withTarget(roleAdmin),
		Writer:     wr,
		Platform:   plat,
		Audit:      log,
		Sessions:   session.New(time.Hour),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

func TestHomeShowsUptimeAndNoPending(t *testing.T) {
	body := signedIn(t, newTestPanelPlat(t, newFakeAudit(), newFakeWriter(), newFakePlatform()))("/").Body.String()
	// Both screens draw this from one shared block now, so the wording is
	// "No ar" plus the age rather than one sentence.
	if !strings.Contains(body, "No ar") {
		t.Error("uptime not shown")
	}
	if !strings.Contains(body, "nenhuma edição pendente") {
		t.Error("the no-pending state is not stated")
	}
	// The reassurance that matters: staff must not think every edit needs this.
	// The home card no longer repeats the per-screen timing sentence — that
	// answer belongs to the screen doing the saving, and there were eight
	// wordings of it before — so what it says here is that nothing is waiting.
	if !strings.Contains(body, "Nada esperando reinício") {
		t.Error("the page does not say that nothing is waiting for a restart")
	}
	if !strings.Contains(body, "relê sozinho") {
		t.Error("the page does not say the rest of the content reloads by itself")
	}
}

func TestHomeWarnsAboutPendingEdits(t *testing.T) {
	wr := newFakeWriter()
	wr.pendentes = 3
	wr.ultimaEdicao = time.Now().Add(-20 * time.Minute)
	body := signedIn(t, newTestPanelPlat(t, newFakeAudit(), wr, newFakePlatform()))("/").Body.String()

	if !strings.Contains(body, "3 edição") {
		t.Error("the pending count is not shown")
	}
	if !strings.Contains(body, "não existe recarga ao vivo") {
		t.Error("the page does not explain why a restart is needed")
	}
}

func TestRestartIsAdminOnlyAndAudited(t *testing.T) {
	plat := newFakePlatform()
	log := newFakeAudit()
	h := newTestPanelPlat(t, log, newFakeWriter(), plat)
	post, token := signedInPost(t, h)

	rec := post("/servidor/reiniciar", url.Values{"csrf": {token}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(plat.restarts) != 1 || plat.restarts[0] != "dep-1" {
		t.Fatalf("restarts = %v, want one for dep-1", plat.restarts)
	}
	recs := log.recorded()
	if len(recs) != 1 || recs[0].Action != audit.ActionRestartGame {
		t.Fatalf("audit = %+v, want one RESTART_GAME", recs)
	}
}

func TestRestartRefusedForModerators(t *testing.T) {
	plat := newFakePlatform()
	h := newTestPanelPlat(t, newFakeAudit(), newFakeWriter(), plat)
	// Rebuild with a moderator session by using the moderator account set.
	hm, err := New(Config{
		Accounts: withTarget(roleModerator), Writer: newFakeWriter(), Platform: plat,
		Audit: newFakeAudit(), Sessions: session.New(time.Hour),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	post, token := signedInPost(t, hm.Routes())
	if rec := post("/servidor/reiniciar", url.Values{"csrf": {token}}); rec.Code != http.StatusForbidden {
		t.Fatalf("moderator restart: status = %d, want 403", rec.Code)
	}
	if len(plat.restarts) != 0 {
		t.Fatal("a refused request still restarted the server")
	}
	_ = h
}

func TestRestartIsNotAttemptedWhenItCannotBeAudited(t *testing.T) {
	// Refusing outright, rather than restarting and logging nothing: a restart
	// nobody can explain is the exact thing the log exists to prevent, and it is
	// the one action that also takes the panel's own reporting offline.
	plat := newFakePlatform()
	log := newFakeAudit()
	log.failWrite = errors.New("banco fora do ar")
	h := newTestPanelPlat(t, log, newFakeWriter(), plat)
	post, token := signedInPost(t, h)

	rec := post("/servidor/reiniciar", url.Values{"csrf": {token}})
	if len(plat.restarts) != 0 {
		t.Fatal("the server was restarted even though the action could not be recorded")
	}
	// The refusal now comes back as the panel's own page carrying the reason,
	// rather than a bare browser error that drops the operator out of the panel.
	if recado := rec.Header().Get("Location"); !strings.Contains(recado, "auditoria") {
		t.Errorf("the operator is not told why it refused: %q", recado)
	}
}

func TestRestartNeedsTheCSRFToken(t *testing.T) {
	plat := newFakePlatform()
	h := newTestPanelPlat(t, newFakeAudit(), newFakeWriter(), plat)
	post, _ := signedInPost(t, h)
	if rec := post("/servidor/reiniciar", url.Values{}); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if len(plat.restarts) != 0 {
		t.Fatal("a request without the token restarted the server")
	}
}

func TestHostingFailureDoesNotBreakTheHomePage(t *testing.T) {
	plat := newFakePlatform()
	plat.latestErr = errors.New("token expirado")
	rec := signedIn(t, newTestPanelPlat(t, newFakeAudit(), newFakeWriter(), plat))("/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — the home page must survive a hosting outage", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "continua funcionando normalmente") {
		t.Error("the page does not reassure that the panel itself is fine")
	}
}

func TestRestartCardIsHiddenWithoutTheHostingAPI(t *testing.T) {
	get := signedIn(t, newTestPanel(t, withTarget(roleAdmin)))
	body := get("/").Body.String()
	if strings.Contains(body, "Servidor de jogo") {
		t.Error("the card is shown with no hosting API configured")
	}
	if rec := get("/servidor/reiniciar"); rec.Code != http.StatusNotFound {
		t.Errorf("route exists without the hosting API: status = %d", rec.Code)
	}
}

// --- npcs e lojas ---

func TestNpcsListsAndFilters(t *testing.T) {
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), newFakeGameData()))
	body := get("/npcs").Body.String()
	if !strings.Contains(body, "Mercador de Armia") {
		t.Error("the NPC listing is empty")
	}
	if strings.Contains(get("/npcs?q=zzz").Body.String(), "Mercador de Armia") {
		t.Error("search returned a non-matching NPC")
	}
}

func TestNpcPageRendersEveryStockSlot(t *testing.T) {
	// Occupied slots only would leave no way to ADD stock — the empty rows are
	// the input, not decoration.
	body := signedIn(t, newTestPanelGame(t, newFakeAudit(), newFakeGameData()))("/npcs/5").Body.String()
	for _, name := range []string{`name="item0"`, `name="item26"`, `name="qtd26"`} {
		if !strings.Contains(body, name) {
			t.Errorf("missing form field %s", name)
		}
	}
	if strings.Contains(body, `name="item27"`) {
		t.Error("rendered a slot the service does not accept")
	}
	if !strings.Contains(body, `value="1415"`) {
		t.Error("the existing stock is not filled in")
	}
}

func TestNpcPageCarriesEffectsThrough(t *testing.T) {
	body := signedIn(t, newTestPanelGame(t, newFakeAudit(), newFakeGameData()))("/npcs/5").Body.String()
	if !strings.Contains(body, `name="eff0_0" value="7"`) || !strings.Contains(body, `name="effv0_0" value="42"`) {
		t.Fatal("the effect pair of an existing stock item is not carried in the form")
	}
}

func TestSetLojaSavesAndAudits(t *testing.T) {
	game := newFakeGameData()
	log := newFakeAudit()
	h := newTestPanelGame(t, log, game)
	post, token := signedInPost(t, h)

	form := url.Values{"csrf": {token}, "item0": {"1415"}, "qtd0": {"3"},
		"eff0_0": {"7"}, "effv0_0": {"42"}, "item1": {"2000"}, "qtd1": {"1"}}
	rec := post("/npcs/5/loja", form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(game.shopSaves) != 1 {
		t.Fatalf("shop saves = %d, want 1", len(game.shopSaves))
	}
	saved := game.shopSaves[0]
	if len(saved) != 2 {
		t.Fatalf("saved %d items, want 2", len(saved))
	}
	if saved[0].ItemIndex != 1415 || saved[0].Quantity != 3 {
		t.Errorf("first slot = %+v, want item 1415 x3", saved[0])
	}
	if saved[0].Eff[0] != [2]int32{7, 42} {
		t.Errorf("the effect pair was lost on save: %v", saved[0].Eff[0])
	}
	if len(log.recorded()) != 1 || log.recorded()[0].Action != audit.ActionSetNpcShop {
		t.Error("the shop change was not audited")
	}
}

func TestEmptySlotsAreSimplyNotSent(t *testing.T) {
	game := newFakeGameData()
	h := newTestPanelGame(t, newFakeAudit(), game)
	post, token := signedInPost(t, h)

	// Everything blank: the shop is emptied rather than left alone.
	post("/npcs/5/loja", url.Values{"csrf": {token}})
	if len(game.shopSaves) != 1 || len(game.shopSaves[0]) != 0 {
		t.Fatalf("saves = %v, want one empty list", game.shopSaves)
	}
}

func TestSetLojaRejectsABadItem(t *testing.T) {
	h := newTestPanelGame(t, newFakeAudit(), newFakeGameData())
	post, token := signedInPost(t, h)
	if rec := post("/npcs/5/loja", url.Values{"csrf": {token}, "item0": {"abc"}}); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestSetLojaNeedsTheCSRFToken(t *testing.T) {
	game := newFakeGameData()
	h := newTestPanelGame(t, newFakeAudit(), game)
	post, _ := signedInPost(t, h)
	if rec := post("/npcs/5/loja", url.Values{"item0": {"1415"}}); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if len(game.shopSaves) != 0 {
		t.Fatal("a request without the token changed the shop")
	}
}

func TestUnknownNpcIs404(t *testing.T) {
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), newFakeGameData()))
	if rec := get("/npcs/999"); rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestWebServerRefusalIsExplained(t *testing.T) {
	// The panel already checked the role, so a FORBIDDEN from the webServer means
	// the two disagree — worth saying, not hiding behind a generic error.
	game := newFakeGameData()
	game.npcsErr = gamedata.ErrForbidden
	rec := signedIn(t, newTestPanelGame(t, newFakeAudit(), game))("/npcs")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "não reconhece esta conta") {
		t.Errorf("the message does not explain the disagreement: %q", rec.Body.String())
	}
}

// --- escrita de NPC ---

func TestSetLugarPreservesFieldsTheFormDoesNotCarry(t *testing.T) {
	// The service replaces the whole row, keyed on slug. If the handler built an
	// NPC from the form instead of editing the one it read, route type and
	// merchant kind would silently become zero.
	game := newFakeGameData()
	game.npcs[0].RouteType = 3
	game.npcs[0].Merchant = 8
	h := newTestPanelGame(t, newFakeAudit(), game)
	post, token := signedInPost(t, h)

	rec := post("/npcs/5/lugar", url.Values{
		"csrf": {token}, "mapa": {"1"}, "x": {"500"}, "y": {"600"}, "nome": {"Novo Nome"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(game.saved) != 1 {
		t.Fatalf("saves = %d, want 1", len(game.saved))
	}
	got := game.saved[0]
	if got.MapID != 1 || got.X != 500 || got.Y != 600 || got.DisplayName != "Novo Nome" {
		t.Errorf("saved = %+v, want the form's values", got)
	}
	if got.RouteType != 3 || got.Merchant != 8 {
		t.Errorf("route type / merchant lost: got %d and %d, want 3 and 8", got.RouteType, got.Merchant)
	}
	if got.Slug != "mercador-armia" {
		t.Errorf("slug = %q, want the original — the service keys on it", got.Slug)
	}
}

func TestSetLugarRejectsBadCoordinates(t *testing.T) {
	h := newTestPanelGame(t, newFakeAudit(), newFakeGameData())
	post, token := signedInPost(t, h)
	for _, f := range []url.Values{
		{"csrf": {token}, "mapa": {"0"}, "x": {"-1"}, "y": {"10"}},
		{"csrf": {token}, "mapa": {"0"}, "x": {"abc"}, "y": {"10"}},
		{"csrf": {token}, "mapa": {""}, "x": {"1"}, "y": {"1"}},
	} {
		if rec := post("/npcs/5/lugar", f); rec.Code != http.StatusBadRequest {
			t.Errorf("%v: status = %d, want 400", f, rec.Code)
		}
	}
}

func TestVisibilityTogglesAndIsAudited(t *testing.T) {
	game := newFakeGameData() // starts Enabled
	log := newFakeAudit()
	h := newTestPanelGame(t, log, game)
	post, token := signedInPost(t, h)

	rec := post("/npcs/5/visibilidade", url.Values{"csrf": {token}, "visivel": {"0"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if len(game.visible) != 1 || game.visible[0] {
		t.Fatalf("visibility calls = %v, want one false", game.visible)
	}
	if len(game.visibleFor) != 1 || game.visibleFor[0] != 5 {
		t.Fatalf("hid NPC %v, want the one in the path (5)", game.visibleFor)
	}
	loc, _ := url.QueryUnescape(rec.Header().Get("Location"))
	if !strings.Contains(loc, "some do mapa") {
		t.Errorf("the message does not say what happens in game: %q", loc)
	}
	if len(log.recorded()) != 1 {
		t.Error("hiding an NPC was not audited")
	}
}

func TestDeleteIsAdminOnly(t *testing.T) {
	game := newFakeGameData()
	hm, err := New(Config{
		Accounts: withTarget(roleModerator), Writer: newFakeWriter(), GameData: game,
		Audit: newFakeAudit(), Sessions: session.New(time.Hour),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	post, token := signedInPost(t, hm.Routes())
	if rec := post("/npcs/5/apagar", url.Values{"csrf": {token}}); rec.Code != http.StatusForbidden {
		t.Fatalf("moderator delete: status = %d, want 403", rec.Code)
	}
	if len(game.deleted) != 0 {
		t.Fatal("a refused request still deleted the NPC")
	}
}

func TestDeleteIsNotAttemptedWhenItCannotBeAudited(t *testing.T) {
	// Once deleted there is no row left to describe, so the record has to exist
	// first or the action becomes unexplainable.
	game := newFakeGameData()
	log := newFakeAudit()
	log.failWrite = errors.New("banco fora do ar")
	h := newTestPanelGame(t, log, game)
	post, token := signedInPost(t, h)

	rec := post("/npcs/5/apagar", url.Values{"csrf": {token}})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if len(game.deleted) != 0 {
		t.Fatal("the NPC was deleted even though the action could not be recorded")
	}
}

func TestContentOwnedDeleteIsExplained(t *testing.T) {
	game := newFakeGameData()
	game.npcWriteErr = gamedata.ErrContentOwned
	h := newTestPanelGame(t, newFakeAudit(), game)
	post, token := signedInPost(t, h)

	rec := post("/npcs/5/apagar", url.Values{"csrf": {token}})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "deixe oculto") {
		t.Errorf("the message does not offer the way out: %q", rec.Body.String())
	}
}

func TestNpcWritesNeedTheCSRFToken(t *testing.T) {
	game := newFakeGameData()
	h := newTestPanelGame(t, newFakeAudit(), game)
	post, _ := signedInPost(t, h)
	for _, path := range []string{"/npcs/5/lugar", "/npcs/5/visibilidade", "/npcs/5/apagar"} {
		if rec := post(path, url.Values{"mapa": {"0"}, "x": {"1"}, "y": {"1"}}); rec.Code != http.StatusForbidden {
			t.Errorf("%s: status = %d, want 403", path, rec.Code)
		}
	}
	if len(game.saved)+len(game.visible)+len(game.deleted) != 0 {
		t.Fatal("a request without the token changed something")
	}
}

// --- monstros ---

// mobRow is what the fake knows about one template, as plain numbers.
//
// A fresh gamedata.MobStat is built on every read, the way the real client gets
// a fresh message from each RPC reply. Handing out one shared value would let a
// handler's edits reach the fake's own state even when the save never happened,
// and a test asserting on the save would then pass for the wrong reason.
type mobRow struct {
	exibido    string
	overridden bool
	valores    map[string]int64
	origens    []gamedata.MobOrigem
}

func (f *fakeGameData) MobTemplates(_ context.Context, _ int64, query string) ([]gamedata.MobTemplate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.mobErr != nil {
		return nil, f.mobErr
	}
	q := strings.ToLower(query)
	out := []gamedata.MobTemplate{}
	for _, m := range f.mobs {
		if q == "" || strings.Contains(strings.ToLower(m.DisplayName), q) ||
			strings.Contains(strings.ToLower(m.Name), q) {
			out = append(out, m)
		}
	}
	return out, nil
}

func (f *fakeGameData) MobStat(_ context.Context, _ int64, name string) (gamedata.MobStat, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.mobErr != nil {
		return gamedata.MobStat{}, f.mobErr
	}
	row, ok := f.mobRows[name]
	if !ok {
		return gamedata.MobStat{}, gamedata.ErrNotFound
	}
	s := gamedata.NewMobStat(name, row.overridden)
	s.SetDisplayName(row.exibido)
	s.SetOrigens(row.origens)
	for nome, v := range row.valores {
		if !s.Set(nome, v) {
			return gamedata.MobStat{}, fmt.Errorf("fake: campo %q nao existe no formulario", nome)
		}
	}
	return s, nil
}

func (f *fakeGameData) ItemLookup(_ context.Context) (map[int32]gamedata.Item, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make(map[int32]gamedata.Item, len(f.itens))
	for _, it := range f.itens {
		out[it.Index] = it
	}
	return out, nil
}

func (f *fakeGameData) SaveMobEquip(_ context.Context, _ int64, name string, itens []gamedata.MobEquipItem) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.mobWriteErr != nil {
		return f.mobWriteErr
	}
	if f.mobEquip == nil {
		f.mobEquip = map[string][]gamedata.MobEquipItem{}
	}
	f.mobEquip[name] = itens
	return nil
}

func (f *fakeGameData) SaveMobStat(_ context.Context, _ int64, m gamedata.MobStat) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.mobWriteErr != nil {
		return f.mobWriteErr
	}
	f.mobSaved = append(f.mobSaved, m)
	return nil
}

func (f *fakeGameData) ClearMobStat(_ context.Context, _ int64, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.mobWriteErr != nil {
		return f.mobWriteErr
	}
	f.mobCleared = append(f.mobCleared, name)
	return nil
}

// campoDe reads one field back out of a saved stat, by its form name.
func campoDe(t *testing.T, m gamedata.MobStat, nome string) int64 {
	t.Helper()
	for _, c := range m.Fields() {
		if c.Nome == nome {
			return c.Valor
		}
	}
	t.Fatalf("campo %q não existe no formulário", nome)
	return 0
}

func TestMonstrosListaEFiltra(t *testing.T) {
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), newFakeGameData()))

	body := get("/monstros").Body.String()
	for _, want := range []string{"Kentania", "Mercador de Armia"} {
		if !strings.Contains(body, want) {
			t.Errorf("a lista não traz %q", want)
		}
	}

	body = get("/monstros?q=kentania").Body.String()
	if !strings.Contains(body, "Kentania") {
		t.Error("a busca perdeu o monstro procurado")
	}
	if strings.Contains(body, "Mercador de Armia") {
		t.Error("a busca trouxe quem não bate com o termo")
	}
}

func TestMonstroMostraOsNumerosEOAvisoDeReinicio(t *testing.T) {
	// The warning is the whole reason this screen differs from itens and npcs:
	// there an edit lands within ~15s, here it waits for a boot. A moderator who
	// does not read that will report the panel as broken.
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), newFakeGameData()))
	body := get("/monstros/Kentania").Body.String()

	for _, want := range []string{
		"Nível",
		// Not type="number" on purpose: that input hands the server an empty
		// string for anything the browser dislikes, which turned a typo into a
		// save that confirmed and changed nothing.
		`name="level" type="text"`,
		`value="10"`,
		"Kentania Velha",
		// A frase é a do bloco compartilhado, não uma redação própria desta
		// página: eram oito jeitos de dizer três coisas.
		"Só vale depois de reiniciar o servidor.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("a página do monstro não traz %q", want)
		}
	}
}

func TestMonstroDesconhecidoE404(t *testing.T) {
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), newFakeGameData()))
	if rec := get("/monstros/NaoExiste"); rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestSetMonstroPreservaOsCamposQueOFormularioNaoCarrega(t *testing.T) {
	// Upsert replaces the whole override row. If the handler built a fresh
	// message from the request instead of editing the one it read, every field
	// the form leaves out — the equipment list above all, which has its own RPC
	// and no place on this screen — would be silently zeroed.
	game := newFakeGameData()
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelGame(t, log, game))

	rec := post("/monstros/Kentania", url.Values{"csrf": {token}, "level": {"42"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(game.mobSaved) != 1 {
		t.Fatalf("saves = %d, want 1", len(game.mobSaved))
	}
	got := game.mobSaved[0]
	if v := campoDe(t, got, "level"); v != 42 {
		t.Errorf("level = %d, want 42", v)
	}
	if v := campoDe(t, got, "exp"); v != 5000 {
		t.Errorf("exp = %d, want 5000 — o formulário não mandou, tinha que ficar como estava", v)
	}
	if v := campoDe(t, got, "resist1"); v != 7 {
		t.Errorf("resist1 = %d, want 7 — o formulário não mandou, tinha que ficar como estava", v)
	}
	if got.Name() != "Kentania" {
		t.Errorf("template = %q, want Kentania — o serviço grava por esse nome", got.Name())
	}
	if len(log.recorded()) != 1 {
		t.Error("a edição do monstro não foi auditada")
	}
	loc, _ := url.QueryUnescape(rec.Header().Get("Location"))
	if !strings.Contains(loc, "reiniciar") {
		t.Errorf("o aviso não diz que falta reiniciar: %q", loc)
	}
}

func TestSetMonstroNaoApagaONomeQuandoOCampoNaoVem(t *testing.T) {
	// Absent means "leave it"; present-but-empty means "clear it". Treating both
	// as empty would let a partial post wipe the in-game name, which is the
	// opposite of how every number on the form behaves.
	game := newFakeGameData()
	post, token := signedInPost(t, newTestPanelGame(t, newFakeAudit(), game))

	if rec := post("/monstros/Kentania", url.Values{"csrf": {token}, "level": {"42"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if got := game.mobSaved[0].DisplayName(); got != "Kentania Velha" {
		t.Errorf("nome exibido = %q, want Kentania Velha", got)
	}

	if rec := post("/monstros/Kentania", url.Values{
		"csrf": {token}, "level": {"42"}, "display_name": {""},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if got := game.mobSaved[1].DisplayName(); got != "" {
		t.Errorf("nome exibido = %q, want vazio — o campo veio em branco de propósito", got)
	}
}

func TestSetMonstroRecusaValorInvalido(t *testing.T) {
	game := newFakeGameData()
	post, token := signedInPost(t, newTestPanelGame(t, newFakeAudit(), game))

	rec := post("/monstros/Kentania", url.Values{"csrf": {token}, "level": {"abc"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if len(game.mobSaved) != 0 {
		t.Fatal("um valor ilegível ainda assim gravou")
	}
}

func TestSetMonstroPrecisaDoCSRF(t *testing.T) {
	game := newFakeGameData()
	post, _ := signedInPost(t, newTestPanelGame(t, newFakeAudit(), game))
	if rec := post("/monstros/Kentania", url.Values{"level": {"42"}}); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if len(game.mobSaved) != 0 {
		t.Fatal("uma requisição sem o token gravou o monstro")
	}
}

func TestLimparMonstroChamaOServicoEAudita(t *testing.T) {
	game := newFakeGameData()
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelGame(t, log, game))

	rec := post("/monstros/Kentania/limpar", url.Values{"csrf": {token}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(game.mobCleared) != 1 || game.mobCleared[0] != "Kentania" {
		t.Fatalf("limpou %v, want o monstro do caminho (Kentania)", game.mobCleared)
	}
	if len(log.recorded()) != 1 {
		t.Error("a limpeza não foi auditada")
	}
}

func TestMonstroAlteradoMasNaoAuditadoFalha(t *testing.T) {
	// The write already happened, so the response has to say so rather than read
	// as a plain failure the operator would retry.
	game := newFakeGameData()
	log := newFakeAudit()
	log.failWrite = errors.New("banco fora do ar")
	post, token := signedInPost(t, newTestPanelGame(t, log, game))

	rec := post("/monstros/Kentania", url.Values{"csrf": {token}, "level": {"42"}})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if len(game.mobSaved) != 1 {
		t.Fatal("a gravação não aconteceu — a mensagem de erro estaria mentindo")
	}
	if !strings.Contains(rec.Body.String(), "foi alterado") {
		t.Errorf("a resposta não avisa que a mudança já valeu: %q", rec.Body.String())
	}
}

func TestMonstrosSomeSemWebServer(t *testing.T) {
	get := signedIn(t, newTestPanel(t, withTarget(roleAdmin)))
	for _, p := range []string{"/monstros", "/monstros/Kentania"} {
		if rec := get(p); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404 sem webServer", p, rec.Code)
		}
	}
	if strings.Contains(get("/").Body.String(), `href="/monstros"`) {
		t.Error("o menu oferece um link para uma página que não existe")
	}
}

// --- atributos de item ---

// itemRow is what the fake knows about one item, as plain numbers. Built fresh
// on every read for the same reason mobRow is: a shared value would let a
// handler edit reach the fake state even when the save never happened.
type itemRow struct {
	exibido    string
	overridden bool
	valores    map[string]int64
}

func (f *fakeGameData) ItemStat(_ context.Context, _ int64, index int32) (gamedata.ItemStat, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.itemStatErr != nil {
		return gamedata.ItemStat{}, f.itemStatErr
	}
	row, ok := f.itemRows[index]
	if !ok {
		return gamedata.ItemStat{}, gamedata.ErrNotFound
	}
	s := gamedata.NewItemStat(index, row.overridden)
	for nome, v := range row.valores {
		if !s.Set(nome, v) {
			return gamedata.ItemStat{}, fmt.Errorf("fake: campo %q nao aceito", nome)
		}
	}
	return s, nil
}

func (f *fakeGameData) SaveItemStat(_ context.Context, _ int64, m gamedata.ItemStat) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.itemWriteErr != nil {
		return f.itemWriteErr
	}
	f.itemSaved = append(f.itemSaved, m)
	return nil
}

func (f *fakeGameData) ClearItemStat(_ context.Context, _ int64, index int32) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.itemWriteErr != nil {
		return f.itemWriteErr
	}
	f.itemCleared = append(f.itemCleared, index)
	return nil
}

// campoItemDe reads one field back out of a saved item stat, by its form name.
func campoItemDe(t *testing.T, m gamedata.ItemStat, nome string) int64 {
	t.Helper()
	for _, c := range m.Fields() {
		if c.Nome == nome {
			return c.Valor
		}
	}
	t.Fatalf("campo %q não existe no formulário", nome)
	return 0
}

func TestAtributosItemMostraOsNumerosEOAvisoDeReinicio(t *testing.T) {
	// The warning has to distinguish this screen from the price field one row
	// above it in the same list: price lands in ~15s, this waits for a boot.
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), newFakeGameData()))
	body := get("/itens/2000/atributos").Body.String()

	for _, want := range []string{
		"Dano",
		`name="damage" type="number"`,
		`value="120"`,
		"Resistência ao fogo",
		"Só vale depois de reiniciar o servidor.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("a página de atributos não traz %q", want)
		}
	}
	// The advanced group has to be on the page: a save writes the whole effect
	// list, so a field the form left out would be zeroed.
	if !strings.Contains(body, `name="wtype"`) {
		t.Error("o campo de tipo de arma não está no formulário — gravar zeraria ele")
	}
}

func TestAtributosItemComIndiceInvalidoE404(t *testing.T) {
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), newFakeGameData()))
	for _, p := range []string{"/itens/abc/atributos", "/itens/-1/atributos", "/itens/9999/atributos"} {
		if rec := get(p); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", p, rec.Code)
		}
	}
}

func TestSetAtributosItemPreservaOsCamposQueOFormularioNaoCarrega(t *testing.T) {
	// The webServer replaces the whole override on save. If the handler built a
	// fresh message from the request, everything the form left out would be
	// zeroed — a weapon would stop being a weapon.
	game := newFakeGameData()
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelGame(t, log, game))

	rec := post("/itens/2000/atributos", url.Values{"csrf": {token}, "damage": {"250"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(game.itemSaved) != 1 {
		t.Fatalf("saves = %d, want 1", len(game.itemSaved))
	}
	got := game.itemSaved[0]
	if v := campoItemDe(t, got, "damage"); v != 250 {
		t.Errorf("dano = %d, want 250", v)
	}
	if v := campoItemDe(t, got, "wtype"); v != 3 {
		t.Errorf("tipo de arma = %d, want 3 — o formulário não mandou, tinha que ficar", v)
	}
	if v := campoItemDe(t, got, "req_level"); v != 30 {
		t.Errorf("nível exigido = %d, want 30 — o formulário não mandou, tinha que ficar", v)
	}
	if got.Index() != 2000 {
		t.Errorf("índice = %d, want 2000 — o serviço grava por ele", got.Index())
	}
	if len(log.recorded()) != 1 {
		t.Error("a edição do item não foi auditada")
	}
	loc, _ := url.QueryUnescape(rec.Header().Get("Location"))
	if !strings.Contains(loc, "reiniciar") {
		t.Errorf("o aviso não diz que falta reiniciar: %q", loc)
	}
}

func TestSetAtributosItemRecusaValorForaDoQueCabe(t *testing.T) {
	// The column is 16 bits and the loader narrows to int16 anyway. Storing
	// 40000 would read back as a negative number, which is worse than refusing.
	game := newFakeGameData()
	post, token := signedInPost(t, newTestPanelGame(t, newFakeAudit(), game))

	rec := post("/itens/2000/atributos", url.Values{"csrf": {token}, "damage": {"40000"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "32767") {
		t.Errorf("a mensagem não diz qual é o limite: %q", rec.Body.String())
	}
	if len(game.itemSaved) != 0 {
		t.Fatal("um valor fora do limite ainda assim gravou")
	}
}

func TestSetAtributosItemRecusaValorIlegivel(t *testing.T) {
	game := newFakeGameData()
	post, token := signedInPost(t, newTestPanelGame(t, newFakeAudit(), game))
	if rec := post("/itens/2000/atributos", url.Values{"csrf": {token}, "damage": {"abc"}}); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if len(game.itemSaved) != 0 {
		t.Fatal("um valor ilegível ainda assim gravou")
	}
}

func TestSetAtributosItemPrecisaDoCSRF(t *testing.T) {
	game := newFakeGameData()
	post, _ := signedInPost(t, newTestPanelGame(t, newFakeAudit(), game))
	if rec := post("/itens/2000/atributos", url.Values{"damage": {"250"}}); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if len(game.itemSaved) != 0 {
		t.Fatal("uma requisição sem o token gravou o item")
	}
}

func TestLimparAtributosItemChamaOServicoEAudita(t *testing.T) {
	game := newFakeGameData()
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelGame(t, log, game))

	rec := post("/itens/2000/atributos/limpar", url.Values{"csrf": {token}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(game.itemCleared) != 1 || game.itemCleared[0] != 2000 {
		t.Fatalf("limpou %v, want o item do caminho (2000)", game.itemCleared)
	}
	if len(log.recorded()) != 1 {
		t.Error("a limpeza não foi auditada")
	}
}

func TestItemAlteradoMasNaoAuditadoFalha(t *testing.T) {
	game := newFakeGameData()
	log := newFakeAudit()
	log.failWrite = errors.New("banco fora do ar")
	post, token := signedInPost(t, newTestPanelGame(t, log, game))

	rec := post("/itens/2000/atributos", url.Values{"csrf": {token}, "damage": {"250"}})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if len(game.itemSaved) != 1 {
		t.Fatal("a gravação não aconteceu — a mensagem de erro estaria mentindo")
	}
	if !strings.Contains(rec.Body.String(), "foi alterado") {
		t.Errorf("a resposta não avisa que a mudança já valeu: %q", rec.Body.String())
	}
}

func TestListaDeItensLevaParaOsAtributos(t *testing.T) {
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), newFakeGameData()))
	body := get("/itens").Body.String()
	if !strings.Contains(body, `href="/itens/2000/atributos"`) {
		t.Error("a lista de itens não oferece o caminho para os atributos")
	}
}

func TestAtributosDeItemSomemSemWebServer(t *testing.T) {
	get := signedIn(t, newTestPanel(t, withTarget(roleAdmin)))
	if rec := get("/itens/2000/atributos"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 sem webServer", rec.Code)
	}
}

// --- curvas de montaria ---

func (f *fakeGameData) MountGrowthCurves(_ context.Context) ([]gamedata.MountGrowthCurve, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.curvaErr != nil {
		return nil, f.curvaErr
	}
	return f.curvas, nil
}

func (f *fakeGameData) SetMountGrowthCurve(_ context.Context, _ int64, _ string, mountIndex int32, rates []int32) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.curvaGravaEr != nil {
		return f.curvaGravaEr
	}
	if f.curvaSalva == nil {
		f.curvaSalva = map[int32][]int32{}
	}
	f.curvaSalva[mountIndex] = append([]int32(nil), rates...)
	return nil
}

func (f *fakeGameData) ClearMountGrowthCurve(_ context.Context, _ int64, mountIndex int32) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.curvaGravaEr != nil {
		return f.curvaGravaEr
	}
	f.curvaLimpa = append(f.curvaLimpa, mountIndex)
	return nil
}

func (f *fakeGameData) MountAbsorbs(_ context.Context) ([]gamedata.MountAbsorb, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.absorbErr != nil {
		return nil, f.absorbErr
	}
	return f.absorbs, nil
}

func (f *fakeGameData) SetMountAbsorb(_ context.Context, _ int64, _ string, mountIndex, pvp, pve int32) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.absorbGravaEr != nil {
		return f.absorbGravaEr
	}
	if f.absorbSalvo == nil {
		f.absorbSalvo = map[int32][2]int32{}
	}
	f.absorbSalvo[mountIndex] = [2]int32{pvp, pve}
	return nil
}

func (f *fakeGameData) ClearMountAbsorb(_ context.Context, _ int64, mountIndex int32) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.absorbGravaEr != nil {
		return f.absorbGravaEr
	}
	f.absorbLimpo = append(f.absorbLimpo, mountIndex)
	return nil
}

func (f *fakeGameData) MountConfigVersion(_ context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.versaoMontarias, nil
}

// --- drops ---

func (f *fakeGameData) Drops(_ context.Context, _ int64, item, mob string) ([]gamedata.Drop, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.dropsErr != nil {
		return nil, f.dropsErr
	}
	f.dropsPedidos = append(f.dropsPedidos, [2]string{item, mob})
	return f.drops, nil
}

func TestDropsPedeUmFiltroAntesDeListar(t *testing.T) {
	// Unfiltered, the report is the whole catalog crossed with every mob
	// template. Asking for a term is cheaper than rendering and truncating it.
	game := newFakeGameData()
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), game))

	body := get("/drops").Body.String()
	if !strings.Contains(body, "Busque por um item") {
		t.Errorf("a página sem filtro não pede uma busca: %q", body)
	}
	if len(game.dropsPedidos) != 0 {
		t.Error("a página sem filtro ainda assim consultou o webServer")
	}
}

func TestDropsMostraChanceEMortes(t *testing.T) {
	game := newFakeGameData()
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), game))
	body := get("/drops?item=espada").Body.String()

	for _, want := range []string{
		"Espada Longa",
		"Kentania",
		"2.78%", // divisor 36
		">36<",  // mortes até cair
	} {
		if !strings.Contains(body, want) {
			t.Errorf("a página de drops não traz %q", want)
		}
	}
	if len(game.dropsPedidos) != 1 || game.dropsPedidos[0][0] != "espada" {
		t.Errorf("consultou %v, want o termo do formulário", game.dropsPedidos)
	}
}

func TestDropsMarcaOSlotGarantidoEmVezDePorcentagem(t *testing.T) {
	// Slot 11 always drops. Printing it as a percentage is the exact mistake the
	// raw rate table invites, and the one this screen exists to avoid.
	game := newFakeGameData()
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), game))
	body := get("/drops?item=poção").Body.String()

	if !strings.Contains(body, "sempre cai") {
		t.Error("o slot garantido não foi marcado")
	}
	if strings.Contains(body, "0.111%") {
		t.Error("o slot garantido foi impresso como uma raridade")
	}
}

func TestDropsAvisaQueONivelDoMonstroConta(t *testing.T) {
	// Without this the numbers look like a property of the slot, and a moderator
	// comparing two mobs would not understand why they differ.
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), newFakeGameData()))
	body := get("/drops").Body.String()
	if !strings.Contains(body, "nível do monstro") {
		t.Error("a página não explica que a chance depende do nível do monstro")
	}
	if !strings.Contains(body, "slot 11 sempre cai") {
		t.Error("a página não avisa do slot garantido")
	}
}

func TestDropsBuscaTambemPorMonstro(t *testing.T) {
	game := newFakeGameData()
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), game))
	if rec := get("/drops?mob=kentania"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(game.dropsPedidos) != 1 || game.dropsPedidos[0][1] != "kentania" {
		t.Errorf("consultou %v, want o nome do monstro", game.dropsPedidos)
	}
}

func TestDropsSomemSemWebServer(t *testing.T) {
	get := signedIn(t, newTestPanel(t, withTarget(roleAdmin)))
	if rec := get("/drops"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 sem webServer", rec.Code)
	}
	if strings.Contains(get("/").Body.String(), `href="/drops"`) {
		t.Error("o menu oferece um link para uma página que não existe")
	}
}

func TestDropsNoMenuComWebServer(t *testing.T) {
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), newFakeGameData()))
	if !strings.Contains(get("/").Body.String(), `href="/drops"`) {
		t.Error("o menu não oferece a página de drops")
	}
}

// --- entrega de item ---

// fakeEntregas stands in for the item mailbox.
type fakeEntregas struct {
	mu         sync.Mutex
	fila       []entrega.Pendente
	enfileirou []entrega.Item
	paraConta  []int64
	cancelou   []int64
	erroFila   error
	erroEnf    error
	erroCancel error
}

func (f *fakeEntregas) Enfileirar(_ context.Context, _, contaID int64, it entrega.Item) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.erroEnf != nil {
		return 0, f.erroEnf
	}
	// The account id is recorded, not ignored: a handler that granted to the
	// wrong account would otherwise pass, since the item alone looks identical.
	f.paraConta = append(f.paraConta, contaID)
	f.enfileirou = append(f.enfileirou, it)
	return int64(len(f.enfileirou)), nil
}

func (f *fakeEntregas) Pendentes(_ context.Context, _ int64) ([]entrega.Pendente, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fila, f.erroFila
}

func (f *fakeEntregas) Cancelar(_ context.Context, _, entregaID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.erroCancel != nil {
		return f.erroCancel
	}
	f.cancelou = append(f.cancelou, entregaID)
	return nil
}

// newTestPanelEntrega builds a panel with the mailbox and the webServer present.
func newTestPanelEntrega(t *testing.T, log AuditLog, game GameData, ent Deliveries) http.Handler {
	t.Helper()
	h, err := New(Config{
		Accounts:   withTarget(roleAdmin),
		Writer:     newFakeWriter(),
		GameData:   game,
		Entregas:   ent,
		Audit:      log,
		Sessions:   session.New(time.Hour),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

func TestEntregaSemJogoLigadoAvisaQueChegaNoLogin(t *testing.T) {
	// Without the game link the mailbox is the only path, and it drains at login.
	// A moderator who does not read that grants an item, watches nothing happen
	// for the player standing in front of them, and reports the panel as broken.
	// With the link configured the item arrives at once — see the deliver-now
	// tests in servidor_test.go.
	ent := &fakeEntregas{}
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelEntrega(t, log, newFakeGameData(), ent))

	rec := post("/contas/ana/entregar", url.Values{
		"csrf": {token}, "item": {"1415"}, "dias": {"30"},
		"eff0": {"7"}, "effv0": {"42"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(ent.enfileirou) != 1 {
		t.Fatalf("entregas = %d, want 1", len(ent.enfileirou))
	}
	got := ent.enfileirou[0]
	if got.Index != 1415 || got.Dias != 30 || got.Eff[0] != [2]uint8{7, 42} {
		t.Errorf("entregou %+v, want item 1415 por 30 dias com efeito 7/42", got)
	}
	if len(ent.paraConta) != 1 || ent.paraConta[0] != 7 {
		t.Errorf("entregou para a conta %v, want a do caminho", ent.paraConta)
	}
	loc, _ := url.QueryUnescape(rec.Header().Get("Location"))
	if !strings.Contains(loc, "próximo login") {
		t.Errorf("o aviso não diz que só chega no próximo login: %q", loc)
	}
	if len(log.recorded()) != 1 {
		t.Error("a entrega não foi auditada")
	}
}

func TestEntregaRecusaItemForaDoCatalogo(t *testing.T) {
	// The mailbox would accept any number and the game would then try to
	// materialize an item that is not in ItemList.csv.
	ent := &fakeEntregas{}
	post, token := signedInPost(t, newTestPanelEntrega(t, newFakeAudit(), newFakeGameData(), ent))

	rec := post("/contas/ana/entregar", url.Values{"csrf": {token}, "item": {"999999"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "catálogo") {
		t.Errorf("a mensagem não explica o problema: %q", rec.Body.String())
	}
	if len(ent.enfileirou) != 0 {
		t.Fatal("um item inexistente ainda assim entrou na fila")
	}
}

func TestEntregaRecusaCamposInvalidos(t *testing.T) {
	ent := &fakeEntregas{}
	post, token := signedInPost(t, newTestPanelEntrega(t, newFakeAudit(), newFakeGameData(), ent))

	for _, f := range []url.Values{
		{"csrf": {token}},                                     // sem item
		{"csrf": {token}, "item": {"0"}},                      // índice zero
		{"csrf": {token}, "item": {"abc"}},                    // ilegível
		{"csrf": {token}, "item": {"1415"}, "dias": {"-1"}},   // prazo negativo
		{"csrf": {token}, "item": {"1415"}, "eff0": {"300"}},  // efeito acima de 255
		{"csrf": {token}, "item": {"1415"}, "effv0": {"300"}}, // valor acima de 255
	} {
		if rec := post("/contas/ana/entregar", f); rec.Code != http.StatusBadRequest {
			t.Errorf("%v: status = %d, want 400", f, rec.Code)
		}
	}
	if len(ent.enfileirou) != 0 {
		t.Fatal("um formulário recusado ainda assim entregou")
	}
}

func TestEntregaPrecisaDoCSRF(t *testing.T) {
	ent := &fakeEntregas{}
	post, _ := signedInPost(t, newTestPanelEntrega(t, newFakeAudit(), newFakeGameData(), ent))
	if rec := post("/contas/ana/entregar", url.Values{"item": {"1415"}}); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if len(ent.enfileirou) != 0 {
		t.Fatal("uma requisição sem o token entregou")
	}
}

func TestFilaApareceNaPaginaDaConta(t *testing.T) {
	ent := &fakeEntregas{fila: []entrega.Pendente{
		{ID: 3, ItemIndex: 1415, Eff: [3][2]uint8{{7, 42}}, CriadoEm: time.Now(), Origem: "painel:1"},
	}}
	get := signedIn(t, newTestPanelEntrega(t, newFakeAudit(), newFakeGameData(), ent))
	body := get("/contas/ana?aba=itens").Body.String()

	for _, want := range []string{"Entregar item", "Na fila", "1415", "7/42", "permanente"} {
		if !strings.Contains(body, want) {
			t.Errorf("a página da conta não traz %q", want)
		}
	}
	if !strings.Contains(body, `action="/contas/ana/entregas/3/cancelar"`) {
		t.Error("a fila não oferece o botão de cancelar")
	}
}

func TestCancelarEntregaChamaOServicoEAudita(t *testing.T) {
	ent := &fakeEntregas{}
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelEntrega(t, log, newFakeGameData(), ent))

	rec := post("/contas/ana/entregas/3/cancelar", url.Values{"csrf": {token}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(ent.cancelou) != 1 || ent.cancelou[0] != 3 {
		t.Fatalf("cancelou %v, want a entrega do caminho (3)", ent.cancelou)
	}
	if len(log.recorded()) != 1 {
		t.Error("o cancelamento não foi auditado")
	}
}

func TestCancelarItemJaEntregueExplicaOPorque(t *testing.T) {
	// A button that silently does nothing is worse than one that says why.
	ent := &fakeEntregas{erroCancel: entrega.ErrJaEntregue}
	post, token := signedInPost(t, newTestPanelEntrega(t, newFakeAudit(), newFakeGameData(), ent))

	rec := post("/contas/ana/entregas/3/cancelar", url.Values{"csrf": {token}})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "já recebeu") {
		t.Errorf("a mensagem não explica: %q", rec.Body.String())
	}
}

func TestEntregaSomeSemAFilaConfigurada(t *testing.T) {
	// The panel has to run standalone; without the mailbox the controls are
	// hidden rather than shown and then failing.
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), newFakeGameData()))
	if strings.Contains(get("/contas/ana").Body.String(), "Entregar item") {
		t.Error("a página oferece entregar item sem a fila configurada")
	}
	post, token := signedInPost(t, newTestPanelGame(t, newFakeAudit(), newFakeGameData()))
	if rec := post("/contas/ana/entregar", url.Values{"csrf": {token}, "item": {"1415"}}); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 sem a fila", rec.Code)
	}
}

// --- troca de senha ---

func TestSenhaVaziaGeraEmVezDeApagar(t *testing.T) {
	// This is the trap the whole feature is shaped around. secret.HashSecret("")
	// returns an EMPTY hash meaning "no secret set", and VerifySecret then matches
	// it against an empty password — so a blank form must never reach the hash.
	// A blank field generates instead, which makes the state unreachable rather
	// than merely guarded.
	wr := newFakeWriter()
	h := newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), wr)
	post, token := signedInPost(t, h)

	rec := post("/contas/ana/senha", url.Values{"csrf": {token}, "senha": {""}})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(wr.senhaHash) != 1 {
		t.Fatalf("escritas = %d, want 1", len(wr.senhaHash))
	}
	if wr.senhaHash[0] == "" {
		t.Fatal("gravou hash vazio — a conta entraria com senha em branco")
	}
	if !strings.Contains(wr.senhaHash[0], "$argon2id$") {
		t.Errorf("hash = %q, want um argon2id", wr.senhaHash[0])
	}
}

func TestSenhaNovaApareceUmaVezENaoNoRedirect(t *testing.T) {
	// The password must not travel in a query string: that lands in browser
	// history and in any proxy log. Every other write here redirects; this one
	// renders.
	wr := newFakeWriter()
	h := newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), wr)
	post, token := signedInPost(t, h)

	rec := post("/contas/ana/senha", url.Values{"csrf": {token}, "senha": {"Trocada9"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — não pode redirecionar com a senha na URL", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Errorf("respondeu com Location %q — a senha iria para o histórico", loc)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Trocada9") {
		t.Error("a tela não mostra a senha nova, e não há outro lugar de onde tirá-la")
	}
	if !strings.Contains(body, "não volta") {
		t.Error("a tela não avisa que não dá para ver de novo")
	}
	if !strings.Contains(body, "reiniciar") {
		t.Error("a tela não avisa do bloqueio de login que só some com reinício")
	}
}

func TestSenhaRecusaOQueOJogoNaoCarrega(t *testing.T) {
	// Every rule here comes from the client, not from taste: the login packet
	// carries a fixed [12]byte and trims trailing spaces.
	wr := newFakeWriter()
	h := newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), wr)
	post, token := signedInPost(t, h)

	for _, c := range []struct{ senha, esperado string }{
		{"esta_senha_e_longa_demais", "12 caracteres"},
		{"ab", "4 caracteres"},
		{"com espaco", "espaço"},
		{"senhã123", "acento"},
	} {
		rec := post("/contas/ana/senha", url.Values{"csrf": {token}, "senha": {c.senha}})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%q: status = %d, want 400", c.senha, rec.Code)
			continue
		}
		if !strings.Contains(rec.Body.String(), c.esperado) {
			t.Errorf("%q: mensagem = %q, want algo com %q", c.senha, rec.Body.String(), c.esperado)
		}
	}
	if len(wr.senhaHash) != 0 {
		t.Fatal("uma senha recusada ainda assim foi gravada")
	}
}

func TestModeradorNaoTrocaSenhaDeOutroDaEquipe(t *testing.T) {
	// Without this a moderator resets the admin's password and signs in as them —
	// "moderator" becomes "admin" in one click, and the panel has no other rank
	// check to fall back on.
	// withTarget also seeds "ana", which is where signedInPost reads the CSRF
	// token from — without her the token comes back empty and this test would
	// pass on a CSRF refusal instead of the rank guard, which is also a 403.
	acc := withTarget(roleModerator)
	acc.add("diretor", 9, roleAdmin, false)
	wr := newFakeWriter()
	h := newTestPanelFull(t, acc, newFakeAudit(), wr)
	post, token := signedInPost(t, h)

	rec := post("/contas/diretor/senha", url.Values{"csrf": {token}, "senha": {"Tentando1"}})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "admin") {
		t.Errorf("recusou por outro motivo que não a hierarquia: %q", rec.Body.String())
	}
	if len(wr.senhaHash) != 0 {
		t.Fatal("um moderador trocou a senha de um admin")
	}
}

func TestAdminTrocaSenhaDaEquipe(t *testing.T) {
	acc := withTarget(roleAdmin)
	acc.add("colega", 9, roleModerator, false)
	wr := newFakeWriter()
	h := newTestPanelFull(t, acc, newFakeAudit(), wr)
	post, token := signedInPost(t, h)

	rec := post("/contas/colega/senha", url.Values{"csrf": {token}, "senha": {"Nova12345678"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(wr.senhaHash) != 1 {
		t.Fatal("um admin não conseguiu trocar a senha de um moderador")
	}
}

func TestSenhaPrecisaDoCSRF(t *testing.T) {
	wr := newFakeWriter()
	h := newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), wr)
	post, _ := signedInPost(t, h)
	if rec := post("/contas/ana/senha", url.Values{"senha": {"Trocada9"}}); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if len(wr.senhaHash) != 0 {
		t.Fatal("uma requisição sem o token trocou a senha")
	}
}

func TestAuditoriaDeSenhaNaoGuardaASenhaNemOHash(t *testing.T) {
	// The audit log is readable by every admin. A hash in it is a hash to attack
	// offline, and the plaintext would be worse.
	log := newFakeAudit()
	wr := newFakeWriter()
	h := newTestPanelFull(t, withTarget(roleAdmin), log, wr)
	post, token := signedInPost(t, h)

	if rec := post("/contas/ana/senha", url.Values{"csrf": {token}, "senha": {"Segredo99"}}); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	regs := log.recorded()
	if len(regs) != 1 {
		t.Fatalf("registros = %d, want 1", len(regs))
	}
	texto := fmt.Sprintf("%+v", regs[0])
	if strings.Contains(texto, "Segredo99") {
		t.Error("a auditoria guardou a senha em texto")
	}
	if strings.Contains(texto, "argon2id") {
		t.Error("a auditoria guardou o hash")
	}
	if regs[0].Action != audit.ActionSetPassword {
		t.Errorf("ação = %q, want %q", regs[0].Action, audit.ActionSetPassword)
	}
}

func TestGerarSenhaSempreObedeceAsRegras(t *testing.T) {
	// The generator is the default path, so a single bad draw would be a password
	// that verifies in the panel and never works in game.
	vistas := map[string]bool{}
	for i := 0; i < 200; i++ {
		s, err := accounts.GerarSenha()
		if err != nil {
			t.Fatalf("GerarSenha: %v", err)
		}
		if err := accounts.ValidarSenha(s); err != nil {
			t.Fatalf("gerou %q, que a própria regra recusa: %v", s, err)
		}
		vistas[s] = true
	}
	if len(vistas) < 190 {
		t.Errorf("200 senhas geradas deram só %d valores distintos", len(vistas))
	}
}

func TestSetPasswordRecusaHashVazio(t *testing.T) {
	// Belt to the handler's suspenders: even called directly, the store must not
	// write the hash that means "any empty password logs in".
	if err := (&accounts.Store{}).SetPassword(context.Background(), 1, ""); !errors.Is(err, accounts.ErrSenhaVazia) {
		t.Fatalf("SetPassword com hash vazio = %v, want ErrSenhaVazia", err)
	}
}

// --- bloqueio com motivo ---

func TestBloqueioExigeUmMotivo(t *testing.T) {
	// A ban with no reason is the state migration 0024 exists to remove: the
	// player writes in asking why and nobody can answer.
	wr := newFakeWriter()
	h := newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), wr)
	post, token := signedInPost(t, h)

	rec := post("/contas/ana/bloqueio", url.Values{"csrf": {token}, "bloquear": {"1"}, "motivo": {"   "}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "motivo") {
		t.Errorf("a mensagem não pede o motivo: %q", rec.Body.String())
	}
	if len(wr.blkCall) != 0 {
		t.Fatal("bloqueou sem motivo")
	}
}

func TestBloqueioGuardaOMotivoEAudita(t *testing.T) {
	wr := newFakeWriter()
	log := newFakeAudit()
	h := newTestPanelFull(t, withTarget(roleAdmin), log, wr)
	post, token := signedInPost(t, h)

	rec := post("/contas/ana/bloqueio", url.Values{
		"csrf": {token}, "bloquear": {"1"}, "motivo": {"Uso de programa de terceiros"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(wr.motivos) != 1 || wr.motivos[0] != "Uso de programa de terceiros" {
		t.Fatalf("motivos = %v, want o do formulário", wr.motivos)
	}
	regs := log.recorded()
	if len(regs) != 1 {
		t.Fatalf("registros = %d, want 1", len(regs))
	}
	if !strings.Contains(fmt.Sprintf("%+v", regs[0]), "programa de terceiros") {
		t.Error("a auditoria não guardou o motivo — é o único lugar que sobra depois de desbloquear")
	}
}

func TestEditarOMotivoDeUmBanEmVigorGravaEAudita(t *testing.T) {
	// The old handler short-circuited on the flag alone, so correcting the reason
	// of a ban already in force wrote nothing, reported "nothing changed", and
	// left no trace of the attempt.
	wr := newFakeWriter()
	wr.prevBlk = true
	wr.prevMotivo = "motivo velho"
	log := newFakeAudit()
	h := newTestPanelFull(t, withTarget(roleAdmin), log, wr)
	post, token := signedInPost(t, h)

	rec := post("/contas/ana/bloqueio", url.Values{
		"csrf": {token}, "bloquear": {"1"}, "motivo": {"motivo corrigido"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	loc, _ := url.QueryUnescape(rec.Header().Get("Location"))
	if strings.Contains(loc, "Nada mudou") {
		t.Fatal("editar o motivo de um ban em vigor foi tratado como se nada tivesse mudado")
	}
	if len(wr.motivos) != 1 || wr.motivos[0] != "motivo corrigido" {
		t.Errorf("motivos = %v, want o corrigido", wr.motivos)
	}
	if len(log.recorded()) != 1 {
		t.Error("a correção do motivo não foi auditada")
	}
}

func TestRebloquearComOMesmoMotivoNaoFazNada(t *testing.T) {
	wr := newFakeWriter()
	wr.prevBlk = true
	wr.prevMotivo = "mesmo motivo"
	log := newFakeAudit()
	h := newTestPanelFull(t, withTarget(roleAdmin), log, wr)
	post, token := signedInPost(t, h)

	rec := post("/contas/ana/bloqueio", url.Values{
		"csrf": {token}, "bloquear": {"1"}, "motivo": {"mesmo motivo"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	loc, _ := url.QueryUnescape(rec.Header().Get("Location"))
	if !strings.Contains(loc, "Nada mudou") {
		t.Errorf("aviso = %q, want dizer que nada mudou", loc)
	}
	if len(log.recorded()) != 0 {
		t.Error("auditou uma requisição que não mudou nada")
	}
}

func TestDesbloquearNaoPedeMotivo(t *testing.T) {
	wr := newFakeWriter()
	wr.prevBlk = true
	wr.prevMotivo = "qualquer coisa"
	h := newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), wr)
	post, token := signedInPost(t, h)

	rec := post("/contas/ana/bloqueio", url.Values{"csrf": {token}, "bloquear": {"0"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(wr.blkCall) != 1 || wr.blkCall[0] {
		t.Fatalf("chamadas = %v, want um desbloqueio", wr.blkCall)
	}
}

func TestPaginaDaContaMostraOMotivo(t *testing.T) {
	acc := withTarget(roleAdmin)
	acc.add("ana", 7, "player", true)
	wr := newFakeWriter()
	wr.details = accounts.Details{Bloqueio: accounts.Bloqueio{
		Blocked: true, Reason: "Anúncio de venda de conta",
	}}
	get := signedIn(t, newTestPanelFull(t, acc, newFakeAudit(), wr))
	body := get("/contas/ana").Body.String()

	if !strings.Contains(body, "Anúncio de venda de conta") {
		t.Error("a página não mostra o motivo do bloqueio")
	}
	if !strings.Contains(body, "não aparece para o jogador") {
		t.Error("a página não avisa que o motivo não é visível ao jogador")
	}
}

func TestBanidoPerdeASessaoDoPainel(t *testing.T) {
	// Until this check existed, blocking a moderator stopped them logging IN and
	// left them signed in — still moderating, with the ban in place.
	wr := newFakeWriter()
	h := newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), wr)
	get := signedIn(t, h)

	if rec := get("/contas"); rec.Code != http.StatusOK {
		t.Fatalf("antes do bloqueio: status = %d, want 200", rec.Code)
	}

	wr.mu.Lock()
	wr.euBloqueado = true
	wr.mu.Unlock()

	rec := get("/contas")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("depois do bloqueio: status = %d, want 303 para o login", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "/login") {
		t.Errorf("mandou para %q, want o login", loc)
	}
}

func TestFalhaAoLerOBloqueioEncerraASessao(t *testing.T) {
	// Failing open would mean a database blip hands a banned moderator their
	// panel back. Failing closed costs one re-login.
	wr := newFakeWriter()
	wr.blockedErr = errors.New("banco fora do ar")
	h := newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), wr)
	get := signedIn(t, h)

	rec := get("/contas")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 para o login", rec.Code)
	}
}

// --- trocas ---

type fakeTrocas struct {
	mu          sync.Mutex
	trocas      []domain.TradeRecord
	pedidos     []store.TradeQuery
	err         error
	chao        []domain.GroundEvent
	pedidosChao []store.GroundQuery
	errChao     error
}

func (f *fakeTrocas) ListGround(_ context.Context, q store.GroundQuery) ([]domain.GroundEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errChao != nil {
		return nil, f.errChao
	}
	f.pedidosChao = append(f.pedidosChao, q)
	return f.chao, nil
}

func (f *fakeTrocas) ListTrades(_ context.Context, q store.TradeQuery) ([]domain.TradeRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	f.pedidos = append(f.pedidos, q)
	return f.trocas, nil
}

func newTestPanelTrocas(t *testing.T, tl TradeLog) http.Handler {
	t.Helper()
	h, err := New(Config{
		Accounts:   withTarget(roleAdmin),
		Writer:     newFakeWriter(),
		Trocas:     tl,
		Audit:      newFakeAudit(),
		Sessions:   session.New(time.Hour),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

func umaTroca() domain.TradeRecord {
	return domain.TradeRecord{
		ID: 1, At: time.Now(),
		CharA: "Vendedor", CharB: "Comprador",
		GoldA: 0, GoldB: 5000,
		ItemsA: []domain.TradeItem{{Index: 1415, Eff: [3][2]uint8{{7, 42}}}},
		ItemsB: nil,
	}
}

func TestTrocasMostraOsDoisLados(t *testing.T) {
	tl := &fakeTrocas{trocas: []domain.TradeRecord{umaTroca()}}
	get := signedIn(t, newTestPanelTrocas(t, tl))
	body := get("/trocas").Body.String()

	for _, want := range []string{"Vendedor", "Comprador", "item 1415", "7/42", "5000 de ouro"} {
		if !strings.Contains(body, want) {
			t.Errorf("a página não traz %q", want)
		}
	}
	// The side that gave nothing has to say so, or the reader assumes the page
	// failed to load half of it.
	if !strings.Contains(body, "nada") {
		t.Error("o lado que não entregou nada não foi marcado")
	}
}

// umaPassagemDeMao is the case the ground log exists for: A drops, B takes it
// from the same floor slot moments later.
func umaPassagemDeMao(base time.Time) []domain.GroundEvent {
	item := domain.TradeItem{Index: 1415, Eff: [3][2]uint8{{7, 42}}}
	return []domain.GroundEvent{ // newest first, the order the store returns
		{ID: 2, At: base.Add(14 * time.Second), Acao: domain.GroundPegou,
			Character: "Pegador", Item: item, X: 2100, Y: 2100, GroundID: 9},
		{ID: 1, At: base, Acao: domain.GroundLargou,
			Character: "Doador", Item: item, X: 2100, Y: 2100, GroundID: 9},
	}
}

func TestTrocasMostraOChaoEmparelhado(t *testing.T) {
	// The floor used to be the route with no record: getItem gives a ground item
	// to anyone within three tiles, with no owner check. The page has to show
	// both halves and say plainly that the item changed owner.
	tl := &fakeTrocas{chao: umaPassagemDeMao(time.Now().Add(-time.Minute))}
	get := signedIn(t, newTestPanelTrocas(t, tl))
	body := get("/trocas").Body.String()

	for _, want := range []string{"Pelo chão", "Doador", "Pegador", "passou de mão", "14s depois", "2100, 2100"} {
		if !strings.Contains(body, want) {
			t.Errorf("a página não traz %q", want)
		}
	}
	// Uma linha a mais do que cabe na página: é assim que a tela sabe se existe
	// próxima, sem um COUNT(*) que já nasce desatualizado.
	if tl.pedidosChao[0].Limit != paginaTam+1 {
		t.Errorf("limite do chão = %d, want %d", tl.pedidosChao[0].Limit, paginaTam+1)
	}
}

func TestTrocasFiltraSoAsPassagensDeMao(t *testing.T) {
	base := time.Now().Add(-time.Minute)
	rows := umaPassagemDeMao(base)
	// Somebody dropping and taking their own item back is noise, not a hand-off.
	rows = append(rows, domain.GroundEvent{
		ID: 4, At: base.Add(-time.Minute), Acao: domain.GroundPegou,
		Character: "Sozinho", Item: domain.TradeItem{Index: 30}, GroundID: 3,
	}, domain.GroundEvent{
		ID: 3, At: base.Add(-2 * time.Minute), Acao: domain.GroundLargou,
		Character: "Sozinho", Item: domain.TradeItem{Index: 30}, GroundID: 3,
	})
	get := signedIn(t, newTestPanelTrocas(t, &fakeTrocas{chao: rows}))
	body := get("/trocas?maos=1").Body.String()

	if !strings.Contains(body, "Pegador") {
		t.Error("o filtro escondeu a passagem de mão")
	}
	if strings.Contains(body, "Sozinho") {
		t.Error("o filtro deixou passar quem pegou o próprio item de volta")
	}
}

// A failed read of one list must not take the other down with it: the moderator
// still gets the trade window, and is told the floor is missing rather than
// reading an empty floor as proof that nothing happened.
func TestTrocasFalhaDoChaoNaoDerrubaAPagina(t *testing.T) {
	tl := &fakeTrocas{trocas: []domain.TradeRecord{umaTroca()}, errChao: errors.New("sem banco")}
	get := signedIn(t, newTestPanelTrocas(t, tl))
	rec := get("/trocas")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Vendedor") {
		t.Error("a lista que funcionou sumiu junto com a que falhou")
	}
	if !strings.Contains(body, "o registro do chão") {
		t.Error("a página não avisa que não conseguiu ler o chão")
	}
}

// The floor slot is reused, so pairing on the slot alone would invent a link
// between two items that never touched each other.
func TestEmparelhaChaoIgnoraCasaReusada(t *testing.T) {
	base := time.Now()
	rows := []domain.GroundEvent{
		{At: base, Acao: domain.GroundPegou, Character: "Depois", GroundID: 9},
		{At: base.Add(-2 * chaoParMax), Acao: domain.GroundLargou, Character: "Antes", GroundID: 9},
	}
	got := emparelhaChao(rows)
	if got[0].De != "" || got[0].TrocouMao {
		t.Errorf("emparelhou através de %v: %+v", 2*chaoParMax, got[0])
	}
}

func TestTrocasPassaOPersonagemDaBusca(t *testing.T) {
	tl := &fakeTrocas{}
	get := signedIn(t, newTestPanelTrocas(t, tl))
	if rec := get("/trocas?personagem=Vendedor"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(tl.pedidos) != 1 || tl.pedidos[0].Char != "Vendedor" {
		t.Errorf("consultou %+v, want o nome do formulário", tl.pedidos)
	}
	if tl.pedidos[0].Limit != paginaTam+1 {
		t.Errorf("limite = %d, want %d", tl.pedidos[0].Limit, paginaTam+1)
	}
}

func TestTrocasSomemSemAConfiguracao(t *testing.T) {
	get := signedIn(t, newTestPanel(t, withTarget(roleAdmin)))
	if rec := get("/trocas"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 sem o log de trocas", rec.Code)
	}
	if strings.Contains(get("/").Body.String(), `href="/trocas"`) {
		t.Error("o menu oferece um link para uma página que não existe")
	}
}

func TestTrocasNoMenuQuandoConfigurado(t *testing.T) {
	get := signedIn(t, newTestPanelTrocas(t, &fakeTrocas{}))
	if !strings.Contains(get("/").Body.String(), `href="/trocas"`) {
		t.Error("o menu não oferece a página de trocas")
	}
}

// --- servidor ao vivo ---

type fakeJogo struct {
	mu                   sync.Mutex
	estado               jogo.Estado
	derrubadas           []string
	avisos               []string
	sessoes              int32
	estadoErr            error
	kickErr              error
	avisoErr             error
	drenagens            []string
	drenarErr            error
	desatolados          []string
	desatolarErr         error
	entregasAgora        []string
	entregarErr          error
	entregues            int32
	perdidos             int32
	overlays             jogo.Overlays
	overlaysErr          error
	avisados             int32
	derrubados           int32
	drenouAntesDoRestart bool
	drenouAntesDoStop    bool
	plat                 *fakePlatform // set to observe the ORDER of drain and restart
}

func (f *fakeJogo) Estado(context.Context) (jogo.Estado, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.estado, f.estadoErr
}

func (f *fakeJogo) Derrubar(_ context.Context, conta string) (int32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.kickErr != nil {
		return 0, f.kickErr
	}
	f.derrubadas = append(f.derrubadas, conta)
	return f.sessoes, nil
}

func (f *fakeJogo) Desatolar(_ context.Context, conta string) (jogo.Desatolo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.desatolarErr != nil {
		return jogo.Desatolo{}, f.desatolarErr
	}
	f.desatolados = append(f.desatolados, conta)
	// Answers from whoever the fake world says is in play, so a test cannot
	// assert an unstuck on somebody who was never there.
	for _, p := range f.estado.Players {
		if p.Conta == conta && p.Jogando {
			return jogo.Desatolo{
				Achou: true, Personagem: p.Personagem,
				DeX: p.X, DeY: p.Y, ParaX: 2090, ParaY: 2097, Cidade: "Armia",
			}, nil
		}
	}
	return jogo.Desatolo{}, nil
}

func (f *fakeJogo) EntregarAgora(_ context.Context, conta string) (jogo.Entrega, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.entregarErr != nil {
		return jogo.Entrega{}, f.entregarErr
	}
	f.entregasAgora = append(f.entregasAgora, conta)
	for _, p := range f.estado.Players {
		if p.Conta == conta {
			return jogo.Entrega{
				Conectado: true, Entregues: f.entregues, Perdidos: f.perdidos,
				Personagem: p.Personagem,
			}, nil
		}
	}
	return jogo.Entrega{}, nil
}

func (f *fakeJogo) Ajustes(context.Context) (jogo.Overlays, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.overlays, f.overlaysErr
}

func (f *fakeJogo) Avisar(_ context.Context, msg string) (int32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.avisoErr != nil {
		return 0, f.avisoErr
	}
	f.avisos = append(f.avisos, msg)
	return 3, nil
}

func (f *fakeJogo) Drenar(_ context.Context, aviso string) (jogo.Drenagem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.drenarErr != nil {
		return jogo.Drenagem{}, f.drenarErr
	}
	// Recorded so a test can prove the drain came FIRST. Restarting before
	// emptying would still pass every count assertion and lose the data.
	if f.plat == nil || f.plat.restartCount() == 0 {
		f.drenouAntesDoRestart = true
	}
	if f.plat == nil || f.plat.stopCount() == 0 {
		f.drenouAntesDoStop = true
	}
	f.drenagens = append(f.drenagens, aviso)
	return jogo.Drenagem{Avisados: f.avisados, Derrubados: f.derrubados}, nil
}

func newTestPanelJogo(t *testing.T, log AuditLog, j Live) http.Handler {
	t.Helper()
	h, err := New(Config{
		Accounts:   withTarget(roleAdmin),
		Writer:     newFakeWriter(),
		Jogo:       j,
		Audit:      log,
		Sessions:   session.New(time.Hour),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

func estadoDeTeste() jogo.Estado {
	return jogo.Estado{
		Jogando: 1, Conectados: 2,
		Players: []jogo.Player{
			{Conta: "ana", Personagem: "Heroina", Nivel: 200, X: 2100, Y: 2100, Jogando: true},
			{Conta: "bruno", Jogando: false},
		},
	}
}

func TestServidorSeparaJogandoDeConectado(t *testing.T) {
	// Two different numbers have both been called "online" in this codebase.
	// Showing one of them alone is how a moderator concludes a player is not
	// there when they are sitting on the character screen.
	get := signedIn(t, newTestPanelJogo(t, newFakeAudit(), &fakeJogo{estado: estadoDeTeste()}))
	body := get("/servidor").Body.String()

	for _, want := range []string{"Jogando", "Conectados", "Heroina", "escolhendo", "2100, 2100"} {
		if !strings.Contains(body, want) {
			t.Errorf("a página não traz %q", want)
		}
	}
	if !strings.Contains(body, "inatividade desligado") {
		t.Error("a página não avisa que uma conexão caída continua contando")
	}
}

func TestDerrubarChamaOJogoEAudita(t *testing.T) {
	j := &fakeJogo{estado: estadoDeTeste(), sessoes: 1}
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelJogo(t, log, j))

	rec := post("/servidor/derrubar", url.Values{"csrf": {token}, "conta": {"ana"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(j.derrubadas) != 1 || j.derrubadas[0] != "ana" {
		t.Fatalf("derrubou %v, want [ana]", j.derrubadas)
	}
	if len(log.recorded()) != 1 {
		t.Error("derrubar não foi auditado")
	}
}

func TestDerrubarQuemNaoEstavaOnlineNaoEErro(t *testing.T) {
	// Zero sessions is the answer, not a failure. Reporting it as an error would
	// make a moderator retry something that already worked.
	j := &fakeJogo{sessoes: 0}
	post, token := signedInPost(t, newTestPanelJogo(t, newFakeAudit(), j))

	rec := post("/servidor/derrubar", url.Values{"csrf": {token}, "conta": {"ana"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	loc, _ := url.QueryUnescape(rec.Header().Get("Location"))
	if !strings.Contains(loc, "não estava conectada") {
		t.Errorf("aviso = %q, want dizer que não estava conectada", loc)
	}
}

func TestAvisoVaiEFicaNaAuditoriaComOTexto(t *testing.T) {
	// A notice reaches everyone at once, so "who said that" is the first
	// question when one lands badly. The text has to be in the log.
	j := &fakeJogo{}
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelJogo(t, log, j))

	rec := post("/servidor/aviso", url.Values{"csrf": {token}, "mensagem": {"Manutencao em 10 minutos"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(j.avisos) != 1 || j.avisos[0] != "Manutencao em 10 minutos" {
		t.Fatalf("avisos = %v, want o texto do formulário", j.avisos)
	}
	regs := log.recorded()
	if len(regs) != 1 {
		t.Fatalf("registros = %d, want 1", len(regs))
	}
	if !strings.Contains(fmt.Sprintf("%+v", regs[0]), "Manutencao em 10 minutos") {
		t.Error("a auditoria não guardou o texto do aviso")
	}
}

func TestAvisoVazioERecusado(t *testing.T) {
	j := &fakeJogo{}
	post, token := signedInPost(t, newTestPanelJogo(t, newFakeAudit(), j))
	if rec := post("/servidor/aviso", url.Values{"csrf": {token}, "mensagem": {"   "}}); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if len(j.avisos) != 0 {
		t.Fatal("enviou um aviso vazio")
	}
}

func TestTokenTrocadoEExplicado(t *testing.T) {
	// The three failures lead to different actions: a refused token is somebody's
	// configuration to fix, an unreachable server is something to wait out.
	// Collapsing them into "erro" sends the operator looking in the wrong place.
	j := &fakeJogo{estadoErr: fmt.Errorf("ler: %w", jogo.ErrRecusado)}
	get := signedIn(t, newTestPanelJogo(t, newFakeAudit(), j))
	body := get("/servidor").Body.String()
	if !strings.Contains(body, "segredos diferentes") {
		t.Errorf("a página não explica o token trocado: %q", body)
	}

	fora := &fakeJogo{estadoErr: fmt.Errorf("ler: %w", jogo.ErrForaDoAr)}
	body = signedIn(t, newTestPanelJogo(t, newFakeAudit(), fora))("/servidor").Body.String()
	if !strings.Contains(body, "reiniciando") {
		t.Errorf("a página não explica o servidor fora do ar: %q", body)
	}
}

func TestServidorSomeSemALigacao(t *testing.T) {
	get := signedIn(t, newTestPanel(t, withTarget(roleAdmin)))
	if rec := get("/servidor"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 sem a ligação com o jogo", rec.Code)
	}
	if strings.Contains(get("/").Body.String(), `href="/servidor"`) {
		t.Error("o menu oferece um link para uma página que não existe")
	}
}

func TestBloquearDerrubaQuandoOJogoEstaLigado(t *testing.T) {
	// This is what closes the loop: before the game link, a ban stopped the
	// player logging IN and left them playing.
	j := &fakeJogo{sessoes: 1}
	wr := newFakeWriter()
	h, err := New(Config{
		Accounts: withTarget(roleAdmin), Writer: wr, Jogo: j,
		Audit: newFakeAudit(), Sessions: session.New(time.Hour),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	post, token := signedInPost(t, h.Routes())

	rec := post("/contas/ana/bloqueio", url.Values{
		"csrf": {token}, "bloquear": {"1"}, "motivo": {"uso de programa"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(j.derrubadas) != 1 || j.derrubadas[0] != "ana" {
		t.Fatalf("bloqueou sem derrubar: %v", j.derrubadas)
	}
	loc, _ := url.QueryUnescape(rec.Header().Get("Location"))
	if !strings.Contains(loc, "derrubada") {
		t.Errorf("aviso = %q, want dizer que derrubou", loc)
	}
}

func TestBloquearAindaFuncionaQuandoNaoDaParaDerrubar(t *testing.T) {
	// The ban is the important half. If the kick fails, say so rather than
	// pretending the ban failed too.
	j := &fakeJogo{kickErr: fmt.Errorf("x: %w", jogo.ErrForaDoAr)}
	wr := newFakeWriter()
	h, err := New(Config{
		Accounts: withTarget(roleAdmin), Writer: wr, Jogo: j,
		Audit: newFakeAudit(), Sessions: session.New(time.Hour),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	post, token := signedInPost(t, h.Routes())

	rec := post("/contas/ana/bloqueio", url.Values{
		"csrf": {token}, "bloquear": {"1"}, "motivo": {"uso de programa"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if len(wr.blkCall) != 1 || !wr.blkCall[0] {
		t.Fatal("o bloqueio não foi gravado porque o kick falhou")
	}
	loc, _ := url.QueryUnescape(rec.Header().Get("Location"))
	if !strings.Contains(loc, "não consegui derrubar") {
		t.Errorf("aviso = %q, want dizer que bloqueou mas não derrubou", loc)
	}
}

// --- prazo do banimento ---

func TestBanTemporarioGuardaOPrazo(t *testing.T) {
	wr := newFakeWriter()
	h := newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), wr)
	post, token := signedInPost(t, h)

	rec := post("/contas/ana/bloqueio", url.Values{
		"csrf": {token}, "bloquear": {"1"}, "motivo": {"briga no chat"}, "dias": {"7"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(wr.diasBan) != 1 || wr.diasBan[0] != 7 {
		t.Fatalf("dias = %v, want [7]", wr.diasBan)
	}
	loc, _ := url.QueryUnescape(rec.Header().Get("Location"))
	if !strings.Contains(loc, "7 dia") {
		t.Errorf("aviso = %q, want dizer o prazo", loc)
	}
}

func TestPrazoVazioEBanimentoSemFim(t *testing.T) {
	// Nobody types "forever" as a number, so an empty field has to mean
	// permanent rather than "zero days" or a validation error.
	wr := newFakeWriter()
	h := newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), wr)
	post, token := signedInPost(t, h)

	rec := post("/contas/ana/bloqueio", url.Values{
		"csrf": {token}, "bloquear": {"1"}, "motivo": {"uso de programa"}, "dias": {""},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(wr.diasBan) != 1 || wr.diasBan[0] != 0 {
		t.Fatalf("dias = %v, want [0] (sem prazo)", wr.diasBan)
	}
	loc, _ := url.QueryUnescape(rec.Header().Get("Location"))
	if strings.Contains(loc, "expira") {
		t.Errorf("aviso = %q, não devia falar em expirar", loc)
	}
}

func TestPrazoInvalidoERecusado(t *testing.T) {
	wr := newFakeWriter()
	h := newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), wr)
	post, token := signedInPost(t, h)

	for _, dias := range []string{"-1", "abc", "3.5"} {
		rec := post("/contas/ana/bloqueio", url.Values{
			"csrf": {token}, "bloquear": {"1"}, "motivo": {"x"}, "dias": {dias},
		})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("dias=%q: status = %d, want 400", dias, rec.Code)
		}
	}
	if len(wr.diasBan) != 0 {
		t.Fatal("um prazo inválido ainda assim bloqueou")
	}
}

func TestPaginaMostraAteQuandoOBanVale(t *testing.T) {
	ate := time.Now().Add(48 * time.Hour)
	acc := withTarget(roleAdmin)
	acc.add("ana", 7, "player", true)
	wr := newFakeWriter()
	wr.details = accounts.Details{Bloqueio: accounts.Bloqueio{
		Blocked: true, Reason: "briga no chat", Until: &ate,
	}}
	get := signedIn(t, newTestPanelFull(t, acc, newFakeAudit(), wr))
	body := get("/contas/ana").Body.String()

	if !strings.Contains(body, "até "+ate.Local().Format("02/01/2006")) {
		t.Error("a página não mostra até quando o banimento vale")
	}
}

func TestPaginaDizQuandoOPrazoJaVenceu(t *testing.T) {
	// An expired ban still has is_blocked set in the row; what changed is that
	// every reader now ignores it. The page has to say that, or a moderator
	// looking at a "banned" account cannot understand why the player is in.
	venceu := time.Now().Add(-time.Hour)
	acc := withTarget(roleAdmin)
	acc.add("ana", 7, "player", true)
	wr := newFakeWriter()
	wr.details = accounts.Details{Bloqueio: accounts.Bloqueio{
		Blocked: true, Reason: "briga no chat", Until: &venceu,
	}}
	get := signedIn(t, newTestPanelFull(t, acc, newFakeAudit(), wr))
	body := get("/contas/ana").Body.String()

	if !strings.Contains(body, "prazo venceu") {
		t.Error("a página não avisa que o prazo já passou")
	}
	if !strings.Contains(body, "já entra") {
		t.Error("a página não diz que a conta voltou a funcionar")
	}
}

func TestVigenteSegueAMesmaRegraDoLogin(t *testing.T) {
	// The panel must not disagree with store.BlockedNowSQL about who is banned.
	// nil is permanent — explicitly, because a past sentinel would read as lifted.
	futuro := time.Now().Add(time.Hour)
	passado := time.Now().Add(-time.Hour)
	for _, c := range []struct {
		nome string
		b    accounts.Bloqueio
		quer bool
	}{
		{"sem bloqueio", accounts.Bloqueio{}, false},
		{"permanente", accounts.Bloqueio{Blocked: true}, true},
		{"prazo no futuro", accounts.Bloqueio{Blocked: true, Until: &futuro}, true},
		{"prazo vencido", accounts.Bloqueio{Blocked: true, Until: &passado}, false},
		{"prazo sem bloqueio", accounts.Bloqueio{Until: &futuro}, false},
	} {
		if got := c.b.Vigente(); got != c.quer {
			t.Errorf("%s: Vigente = %v, want %v", c.nome, got, c.quer)
		}
	}
}

// --- o cartao de reinicio na aba Servidor ---

// newTestPanelJogoPlat builds a panel with both the live link and the hosting
// API, which is what the Servidor tab needs to show everything it offers.
func newTestPanelJogoPlat(t *testing.T, j Live, p Platform) http.Handler {
	t.Helper()
	// Wire the two fakes together so the drain can see whether a restart has
	// already happened — the order is what makes this safe.
	if fj, ok := j.(*fakeJogo); ok {
		if fp, ok := p.(*fakePlatform); ok {
			fj.plat = fp
		}
	}
	h, err := New(Config{
		Accounts:   withTarget(roleAdmin),
		Writer:     newFakeWriter(),
		Jogo:       j,
		Platform:   p,
		Audit:      newFakeAudit(),
		Sessions:   session.New(time.Hour),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

func TestAbaServidorTrazOBotaoDeReiniciar(t *testing.T) {
	// The only restart button used to live on Início. This tab is where anyone
	// looks for a server control, and finding kick and broadcast but no restart
	// reads as the feature being missing — which is exactly what happened.
	get := signedIn(t, newTestPanelJogoPlat(t, &fakeJogo{estado: estadoDeTeste()}, newFakePlatform()))
	body := get("/servidor").Body.String()

	if !strings.Contains(body, `action="/servidor/reiniciar"`) {
		t.Error("a aba Servidor não oferece o reinício")
	}
	if !strings.Contains(body, "No ar") {
		t.Error("a aba Servidor não mostra que o servidor está de pé")
	}
	if !strings.Contains(body, "não é apagar") {
		t.Error("a aba não distingue desligar de apagar, que é a confusão que a Railway cria")
	}
}

func TestReinicioVoltaParaAAbaDeOndeSaiu(t *testing.T) {
	post, token := signedInPost(t, newTestPanelJogoPlat(t, &fakeJogo{}, newFakePlatform()))

	rec := post("/servidor/reiniciar", url.Values{"csrf": {token}, "voltar": {"/servidor"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/servidor?") {
		t.Errorf("voltou para %q, want a aba Servidor", loc)
	}
}

func TestReinicioDaPaginaInicialContinuaNaInicial(t *testing.T) {
	post, token := signedInPost(t, newTestPanelJogoPlat(t, &fakeJogo{}, newFakePlatform()))

	rec := post("/servidor/reiniciar", url.Values{"csrf": {token}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/?") {
		t.Errorf("voltou para %q, want a página inicial", loc)
	}
}

func TestVoltarSoAceitaOsCaminhosConhecidos(t *testing.T) {
	// The field comes from a form, so it is caller-controlled. An open redirect
	// is not worth the convenience of a general "back where I was".
	post, token := signedInPost(t, newTestPanelJogoPlat(t, &fakeJogo{}, newFakePlatform()))

	for _, destino := range []string{"https://exemplo.invalido", "//exemplo.invalido", "/contas"} {
		rec := post("/servidor/reiniciar", url.Values{"csrf": {token}, "voltar": {destino}})
		if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/?") {
			t.Errorf("voltar=%q levou para %q, want a página inicial", destino, loc)
		}
	}
}

// --- reinicio seguro ---

func TestReinicioSeguroEsvaziaAntesDeReiniciar(t *testing.T) {
	// The order is the feature. Restarting first and saving during shutdown puts
	// the saves inside the platform's kill window; emptying first takes the clock
	// out of it entirely.
	j := &fakeJogo{derrubados: 3, avisados: 3}
	plat := newFakePlatform()
	post, token := signedInPost(t, newTestPanelJogoPlat(t, j, plat))

	rec := post("/servidor/reiniciar-seguro", url.Values{"csrf": {token}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(j.drenagens) != 1 {
		t.Fatalf("drenagens = %d, want 1", len(j.drenagens))
	}
	if len(plat.restarts) != 1 {
		t.Fatalf("reinícios = %d, want 1", len(plat.restarts))
	}
	if !j.drenouAntesDoRestart {
		t.Error("reiniciou antes de esvaziar — a ordem é o que torna isto seguro")
	}
	loc, _ := url.QueryUnescape(rec.Header().Get("Location"))
	if !strings.Contains(loc, "3 sessão") {
		t.Errorf("aviso = %q, want dizer quantas sessões foram salvas", loc)
	}
}

func TestReinicioSeguroNaoReiniciaSeAsGravacoesNaoConfirmam(t *testing.T) {
	// This is the case the whole feature exists for. The sessions are already
	// gone when the drain reports failure, so restarting would drop exactly what
	// it was waiting to write.
	j := &fakeJogo{drenarErr: fmt.Errorf("x: %w", jogo.ErrForaDoAr)}
	plat := newFakePlatform()
	post, token := signedInPost(t, newTestPanelJogoPlat(t, j, plat))

	rec := post("/servidor/reiniciar-seguro", url.Values{"csrf": {token}})
	// O que prova a recusa é a hospedagem não ter recebido nada. A falha agora
	// volta para a página do botão com o recado, em vez de responder uma tela de
	// erro do navegador que tira a pessoa do painel.
	if len(plat.restarts) != 0 {
		t.Fatal("reiniciou mesmo sem as gravações confirmarem")
	}
	recado := rec.Header().Get("Location")
	if !strings.Contains(recado, "reiniciei") {
		t.Errorf("o recado não deixa claro que não reiniciou: %q", recado)
	}
}

func TestReinicioSeguroMandaOAvisoAntes(t *testing.T) {
	j := &fakeJogo{}
	post, token := signedInPost(t, newTestPanelJogoPlat(t, j, newFakePlatform()))

	rec := post("/servidor/reiniciar-seguro", url.Values{
		"csrf": {token}, "aviso": {"Manutencao rapida, ja voltamos"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(j.drenagens) != 1 || j.drenagens[0] != "Manutencao rapida, ja voltamos" {
		t.Errorf("aviso enviado = %v, want o do formulário", j.drenagens)
	}
}

func TestReinicioSeguroUsaUmAvisoPadraoQuandoNaoEscrevem(t *testing.T) {
	// An empty field must not mean an empty notice: players would be dropped
	// with no explanation, which is the thing a planned restart is for.
	j := &fakeJogo{}
	post, token := signedInPost(t, newTestPanelJogoPlat(t, j, newFakePlatform()))

	if rec := post("/servidor/reiniciar-seguro", url.Values{"csrf": {token}, "aviso": {"  "}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if len(j.drenagens) != 1 || j.drenagens[0] == "" {
		t.Errorf("aviso = %v, want o padrão em vez de vazio", j.drenagens)
	}
}

func TestReinicioSeguroNaoReiniciaSemAuditoria(t *testing.T) {
	// The players are already out and saved. An empty server that is still up is
	// recoverable; an unaudited restart is not.
	j := &fakeJogo{}
	plat := newFakePlatform()
	log := newFakeAudit()
	log.failWrite = errors.New("banco fora do ar")
	h, err := New(Config{
		Accounts: withTarget(roleAdmin), Writer: newFakeWriter(), Jogo: j, Platform: plat,
		Audit: log, Sessions: session.New(time.Hour),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	post, token := signedInPost(t, h.Routes())

	rec := post("/servidor/reiniciar-seguro", url.Values{"csrf": {token}})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if len(plat.restarts) != 0 {
		t.Fatal("reiniciou sem registrar na auditoria")
	}
	if !strings.Contains(rec.Body.String(), "de pé e vazio") {
		t.Errorf("a mensagem não diz em que estado ficou: %q", rec.Body.String())
	}
}

func TestReinicioSeguroEAdminOnly(t *testing.T) {
	j := &fakeJogo{}
	plat := newFakePlatform()
	h, err := New(Config{
		Accounts: withTarget(roleModerator), Writer: newFakeWriter(), Jogo: j, Platform: plat,
		Audit: newFakeAudit(), Sessions: session.New(time.Hour),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	post, token := signedInPost(t, h.Routes())

	if rec := post("/servidor/reiniciar-seguro", url.Values{"csrf": {token}}); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if len(j.drenagens) != 0 || len(plat.restarts) != 0 {
		t.Fatal("um moderador esvaziou ou reiniciou o servidor")
	}
}

func TestAbaServidorOferecaOsDoisReinicios(t *testing.T) {
	get := signedIn(t, newTestPanelJogoPlat(t, &fakeJogo{estado: estadoDeTeste()}, newFakePlatform()))
	body := get("/servidor").Body.String()

	if !strings.Contains(body, `action="/servidor/reiniciar-seguro"`) {
		t.Error("a aba não oferece o reinício seguro")
	}
	if !strings.Contains(body, `action="/servidor/reiniciar"`) {
		t.Error("a aba perdeu o reinício direto")
	}
	if !strings.Contains(body, "sem relógio") {
		t.Error("a página não explica por que um é mais seguro que o outro")
	}
	if !strings.Contains(body, `action="/servidor/desligar"`) {
		t.Error("a página não oferece desligar")
	}
}

// --- ligar e desligar ---

func TestDesligarEsvaziaAntesDeTirarDoAr(t *testing.T) {
	// Stopping is the exact moment unsaved progress disappears, so the drain has
	// to come first — same order as the safe restart, same reason.
	j := &fakeJogo{derrubados: 2, avisados: 2}
	plat := newFakePlatform()
	post, token := signedInPost(t, newTestPanelJogoPlat(t, j, plat))

	rec := post("/servidor/desligar", url.Values{"csrf": {token}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(j.drenagens) != 1 {
		t.Fatalf("drenagens = %d, want 1", len(j.drenagens))
	}
	if len(plat.stops) != 1 {
		t.Fatalf("desligamentos = %d, want 1", len(plat.stops))
	}
	if !j.drenouAntesDoStop {
		t.Error("desligou antes de esvaziar — a ordem é o que salva o progresso")
	}
	loc, _ := url.QueryUnescape(rec.Header().Get("Location"))
	if !strings.Contains(loc, "Ligar") {
		t.Errorf("aviso = %q, want dizer como trazer de volta", loc)
	}
}

func TestDesligarNaoDesligaSeAsGravacoesNaoConfirmam(t *testing.T) {
	j := &fakeJogo{drenarErr: fmt.Errorf("x: %w", jogo.ErrForaDoAr)}
	plat := newFakePlatform()
	post, token := signedInPost(t, newTestPanelJogoPlat(t, j, plat))

	rec := post("/servidor/desligar", url.Values{"csrf": {token}})
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	if len(plat.stops) != 0 {
		t.Fatal("desligou mesmo sem as gravações confirmarem")
	}
	if !strings.Contains(rec.Body.String(), "NÃO desliguei") {
		t.Errorf("a mensagem não deixa claro: %q", rec.Body.String())
	}
}

func TestLigarUsaODeploymentMaisRecenteSejaQualForOEstado(t *testing.T) {
	// Stopping may leave a status the "successful only" filter hides. If the
	// start button looked there, the one control that fixes an off server would
	// be the one that could not find anything to press.
	plat := newFakePlatform()
	plat.dep.Status = "REMOVED"
	post, token := signedInPost(t, newTestPanelJogoPlat(t, &fakeJogo{}, plat))

	rec := post("/servidor/ligar", url.Values{"csrf": {token}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(plat.redeploys) != 1 || plat.redeploys[0] != "dep-1" {
		t.Fatalf("redeploys = %v, want [dep-1]", plat.redeploys)
	}
	if plat.usouLatestAny == 0 {
		t.Error("procurou o deployment com o filtro de sucesso, que esconderia um parado")
	}
}

func TestPaginaMostraDesligadoEOfereceLigar(t *testing.T) {
	plat := newFakePlatform()
	plat.dep.Status = "REMOVED"
	get := signedIn(t, newTestPanelJogoPlat(t, &fakeJogo{}, plat))
	body := get("/servidor").Body.String()

	if !strings.Contains(body, "Desligado") {
		t.Error("a página não diz que o servidor está desligado")
	}
	if !strings.Contains(body, "estado parada") {
		t.Error("o farol não marca o servidor como parado — a cor é metade do recado")
	}
	if !strings.Contains(body, `action="/servidor/ligar"`) {
		t.Error("a página não oferece ligar")
	}
	if strings.Contains(body, `action="/servidor/desligar"`) {
		t.Error("a página oferece desligar um servidor que já está desligado")
	}
	if strings.Contains(body, `action="/servidor/reiniciar"`) {
		t.Error("a página oferece reiniciar um servidor que está desligado")
	}
}

func TestPaginaComServidorNoArNaoOfereceLigar(t *testing.T) {
	get := signedIn(t, newTestPanelJogoPlat(t, &fakeJogo{estado: estadoDeTeste()}, newFakePlatform()))
	body := get("/servidor").Body.String()

	if strings.Contains(body, `action="/servidor/ligar"`) {
		t.Error("oferece ligar um servidor que já está no ar")
	}
	if !strings.Contains(body, `action="/servidor/desligar"`) {
		t.Error("não oferece desligar um servidor no ar")
	}
	if !strings.Contains(body, "não é apagar") {
		t.Error("a página não explica que desligar preserva as variáveis e o volume")
	}
}

func TestLigarEDesligarSaoAdminOnly(t *testing.T) {
	j := &fakeJogo{}
	plat := newFakePlatform()
	h, err := New(Config{
		Accounts: withTarget(roleModerator), Writer: newFakeWriter(), Jogo: j, Platform: plat,
		Audit: newFakeAudit(), Sessions: session.New(time.Hour),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	post, token := signedInPost(t, h.Routes())

	for _, rota := range []string{"/servidor/desligar", "/servidor/ligar"} {
		if rec := post(rota, url.Values{"csrf": {token}}); rec.Code != http.StatusForbidden {
			t.Errorf("%s como moderador: status = %d, want 403", rota, rec.Code)
		}
	}
	if len(plat.stops) != 0 || len(plat.redeploys) != 0 {
		t.Fatal("um moderador ligou ou desligou o servidor")
	}
}

func TestLigarNaoLigaSemAuditoria(t *testing.T) {
	plat := newFakePlatform()
	log := newFakeAudit()
	log.failWrite = errors.New("banco fora do ar")
	h, err := New(Config{
		Accounts: withTarget(roleAdmin), Writer: newFakeWriter(), Jogo: &fakeJogo{}, Platform: plat,
		Audit: log, Sessions: session.New(time.Hour),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	post, token := signedInPost(t, h.Routes())

	rec := post("/servidor/ligar", url.Values{"csrf": {token}})
	if len(plat.redeploys) != 0 {
		t.Fatal("ligou sem registrar na auditoria")
	}
	if recado := rec.Header().Get("Location"); !strings.Contains(recado, "auditoria") {
		t.Errorf("o recado não diz por que recusou: %q", recado)
	}
}

// --- busca por personagem e prazos prontos ---

func TestBuscaPorNomeDePersonagemMostraPorQueAchou(t *testing.T) {
	// Moderation starts from a report, and a report names a CHARACTER. Returning
	// the account without saying which character matched gives the moderator a
	// name they do not recognise — account names rarely resemble the characters
	// on them.
	acc := withTarget(roleAdmin)
	wr := newFakeWriter()
	h := newTestPanelFull(t, acc, newFakeAudit(), wr)
	// Seeded after the panel is built, so it replaces what the helper derived.
	wr.mu.Lock()
	wr.achados = []accounts.Achado{{
		AccountSummary: domain.AccountSummary{ID: 7, Name: "lokitoo", Role: "player"},
		PorPersonagem:  "Vandalyzz",
	}}
	wr.mu.Unlock()

	body := signedIn(t, h)("/contas?q=vandal").Body.String()
	if !strings.Contains(body, "lokitoo") {
		t.Error("a busca não trouxe a conta do personagem")
	}
	if !strings.Contains(body, "Vandalyzz") {
		t.Error("a página não diz por qual personagem a conta apareceu")
	}
}

func TestBuscaPorNomeDeContaNaoInventaPersonagem(t *testing.T) {
	acc := withTarget(roleAdmin)
	wr := newFakeWriter()
	h := newTestPanelFull(t, acc, newFakeAudit(), wr)

	body := signedIn(t, h)("/contas?q=ana").Body.String()
	if !strings.Contains(body, "nome da conta") {
		t.Error("uma conta achada pelo próprio nome não foi marcada como tal")
	}
}

func TestPrazoProntoBloqueiaPeloBotao(t *testing.T) {
	// The preset buttons carry their own value. No JavaScript: the panel serves
	// none and its CSP forbids it, so a control that needed script would look
	// like a button and do nothing.
	wr := newFakeWriter()
	h := newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), wr)
	post, token := signedInPost(t, h)

	rec := post("/contas/ana/bloqueio", url.Values{
		"csrf": {token}, "bloquear": {"1"}, "motivo": {"briga"}, "preset": {"7"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(wr.diasBan) != 1 || wr.diasBan[0] != 7 {
		t.Fatalf("dias = %v, want [7] vindo do botão", wr.diasBan)
	}
}

func TestPrazoProntoGanhaDoCampoLivre(t *testing.T) {
	// Both are in the same form, so both are submitted. The button is the one
	// the moderator just pressed; the field is what was left lying there.
	wr := newFakeWriter()
	h := newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), wr)
	post, token := signedInPost(t, h)

	rec := post("/contas/ana/bloqueio", url.Values{
		"csrf": {token}, "bloquear": {"1"}, "motivo": {"briga"},
		"preset": {"3"}, "dias": {"365"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if len(wr.diasBan) != 1 || wr.diasBan[0] != 3 {
		t.Fatalf("dias = %v, want [3] — o botão apertado ganha do campo", wr.diasBan)
	}
}

func TestSemPrazoNenhumContinuaSendoPermanente(t *testing.T) {
	wr := newFakeWriter()
	h := newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), wr)
	post, token := signedInPost(t, h)

	rec := post("/contas/ana/bloqueio", url.Values{
		"csrf": {token}, "bloquear": {"1"}, "motivo": {"briga"}, "preset": {""}, "dias": {""},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if len(wr.diasBan) != 1 || wr.diasBan[0] != 0 {
		t.Fatalf("dias = %v, want [0]", wr.diasBan)
	}
}

func TestOsPrazosProntosEstaoNaTela(t *testing.T) {
	acc := withTarget(roleAdmin)
	get := signedIn(t, newTestPanelFull(t, acc, newFakeAudit(), newFakeWriter()))
	body := get("/contas/ana").Body.String()

	for _, dias := range []string{"1", "3", "7", "30"} {
		if !strings.Contains(body, `name="preset" value="`+dias+`"`) {
			t.Errorf("falta o prazo pronto de %s dia(s)", dias)
		}
	}
	// And the page still carries no script: every control here has to work
	// without it.
	if strings.Contains(strings.ToLower(body), "<script") {
		t.Error("a página passou a depender de JavaScript, que a política de segurança bloqueia")
	}
}

// --- censo de itens ---

type fakeCenso struct {
	mu      sync.Mutex
	cmp     domain.CensusCompare
	pedidos []store.CensusQuery
	err     error

	dups       []domain.ItemDup
	dupLimites []int
	dupErr     error
	marcados   int64
	semMarca   int64
}

func (f *fakeCenso) ListDupes(_ context.Context, limite int) ([]domain.ItemDup, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.dupErr != nil {
		return nil, f.dupErr
	}
	f.dupLimites = append(f.dupLimites, limite)
	return f.dups, nil
}

func (f *fakeCenso) CountMarked(context.Context) (int64, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.marcados, f.semMarca, nil
}

func (f *fakeCenso) CensusGrowth(_ context.Context, q store.CensusQuery) (domain.CensusCompare, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return domain.CensusCompare{}, f.err
	}
	f.pedidos = append(f.pedidos, q)
	return f.cmp, nil
}

func newTestPanelCenso(t *testing.T, c Censo) http.Handler {
	t.Helper()
	h, err := New(Config{
		Accounts:   withTarget(roleAdmin),
		Writer:     newFakeWriter(),
		Censo:      c,
		Audit:      newFakeAudit(),
		Sessions:   session.New(time.Hour),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

// duasFotos is a week apart, with one refined item up by two and one plain item
// down by five.
func duasFotos() domain.CensusCompare {
	hoje := time.Now().Truncate(24 * time.Hour)
	return domain.CensusCompare{
		De:  domain.CensusRun{Day: hoje.AddDate(0, 0, -7), CountedAt: hoje.AddDate(0, 0, -7), Units: 903, Kinds: 40},
		Ate: domain.CensusRun{Day: hoje, CountedAt: hoje, Units: 900, Kinds: 41},
		Linha: []domain.ItemCensus{
			{Index: 1415, Sanc: 11, Units: 6, Was: 4, Delta: 2, Equipped: 4, Carried: 1, Stored: 1},
			{Index: 30, Sanc: 0, Units: 95, Was: 100, Delta: -5, Carried: 95},
		},
	}
}

func TestCensoMostraOQueMudou(t *testing.T) {
	get := signedIn(t, newTestPanelCenso(t, &fakeCenso{cmp: duasFotos()}))
	body := get("/censo").Body.String()

	for _, want := range []string{"Censo de itens", "1415", "+11", "+2", "-5", "900"} {
		if !strings.Contains(body, want) {
			t.Errorf("a página não traz %q", want)
		}
	}
	// The hour of the photo is not decoration: items only reach the database at
	// logout, so the count is the world as of that moment, not of now.
	if !strings.Contains(body, "não chega no banco") && !strings.Contains(body, "sai do jogo") {
		t.Error("a página não diz que a contagem é do momento da foto")
	}
}

func TestCensoPassaAJanelaEOsFiltros(t *testing.T) {
	fc := &fakeCenso{cmp: duasFotos()}
	get := signedIn(t, newTestPanelCenso(t, fc))
	if rec := get("/censo?dias=30&ordem=sumiu&refino=1"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	q := fc.pedidos[0]
	if q.Dias != 30 || q.Subiu || !q.SoRefinado {
		t.Errorf("consulta = %+v, want 30 dias, ordem por queda, só refinado", q)
	}
}

// A window nobody offered is a window nobody tested. Anything else falls back to
// the default rather than reaching the database as a free-form number.
func TestCensoRecusaJanelaInventada(t *testing.T) {
	fc := &fakeCenso{cmp: duasFotos()}
	get := signedIn(t, newTestPanelCenso(t, fc))
	get("/censo?dias=999")
	if fc.pedidos[0].Dias != 7 {
		t.Errorf("dias = %d, want 7 (o padrão)", fc.pedidos[0].Dias)
	}
}

func TestCensoAvisaQuandoSoTemUmaFoto(t *testing.T) {
	hoje := time.Now().Truncate(24 * time.Hour)
	so := domain.CensusRun{Day: hoje, CountedAt: hoje, Units: 10, Kinds: 2}
	get := signedIn(t, newTestPanelCenso(t, &fakeCenso{
		cmp: domain.CensusCompare{De: so, Ate: so},
	}))
	body := get("/censo").Body.String()
	if !strings.Contains(body, "Só existe uma foto") {
		t.Error("a página compara uma foto com ela mesma sem avisar")
	}
}

// Com menos história do que a janela pedida, a comparação cai para a foto mais
// antiga. Sem dizer isso, a tela reporta o crescimento de dois dias como se
// fosse o de trinta.
func TestCensoAvisaQuandoAJanelaFoiEncurtada(t *testing.T) {
	hoje := time.Now().Truncate(24 * time.Hour)
	cmp := duasFotos()
	cmp.De.Day = hoje.AddDate(0, 0, -2)
	get := signedIn(t, newTestPanelCenso(t, &fakeCenso{cmp: cmp}))
	body := get("/censo?dias=30").Body.String()
	if !strings.Contains(body, "Não existe foto de 30 dias atrás") {
		t.Error("a página não avisa que a janela real é menor do que a pedida")
	}
}

func TestCensoSemFotoNaoPareceErro(t *testing.T) {
	get := signedIn(t, newTestPanelCenso(t, &fakeCenso{}))
	rec := get("/censo")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Nenhuma foto ainda") {
		t.Error("banco sem foto não explica o que vai acontecer")
	}
}

func TestCensoFalhaDeLeituraNaoDerrubaAPagina(t *testing.T) {
	get := signedIn(t, newTestPanelCenso(t, &fakeCenso{err: errors.New("sem banco")}))
	rec := get("/censo")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "o censo") {
		t.Error("a página não avisa que a leitura falhou")
	}
}

func TestCensoSomeSemAConfiguracao(t *testing.T) {
	get := signedIn(t, newTestPanel(t, withTarget(roleAdmin)))
	if rec := get("/censo"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 sem o censo", rec.Code)
	}
	if strings.Contains(get("/").Body.String(), `href="/censo"`) {
		t.Error("o menu oferece um link para uma página que não existe")
	}
}

func TestCensoNoMenuQuandoConfigurado(t *testing.T) {
	get := signedIn(t, newTestPanelCenso(t, &fakeCenso{}))
	if !strings.Contains(get("/").Body.String(), `href="/censo"`) {
		t.Error("o menu não oferece a página do censo")
	}
}

// umaCopia is one serial found on two characters — the whole point of the
// serial: proof, not suspicion.
func umaCopia() []domain.ItemDup {
	it := domain.ItemDup{Serial: 4242, Index: 1415, Onde: "char_carry",
		Character: "Doador", Account: "conta_a"}
	outra := it
	outra.Onde, outra.Character, outra.Account = "char_equip", "Pegador", "conta_b"
	return []domain.ItemDup{it, outra}
}

func TestCensoMostraACopiaComOsDoisDonos(t *testing.T) {
	get := signedIn(t, newTestPanelCenso(t, &fakeCenso{
		cmp: duasFotos(), dups: umaCopia(), marcados: 900,
	}))
	body := get("/censo").Body.String()

	for _, want := range []string{"Cópias encontradas", "4242", "Doador", "Pegador",
		"conta_a", "conta_b", "2 cópias", "mochila", "equipado"} {
		if !strings.Contains(body, want) {
			t.Errorf("a página não traz %q", want)
		}
	}
}

// "Nenhuma cópia" e "nada marcado ainda" parecem a mesma tela vazia e querem
// dizer o oposto. Logo depois da migração tudo é zero, e ler isso como
// inocência é o erro que a contagem existe para impedir.
func TestCensoSeparaSemCopiaDeSemMarca(t *testing.T) {
	semMarca := signedIn(t, newTestPanelCenso(t, &fakeCenso{
		cmp: duasFotos(), marcados: 0, semMarca: 5000,
	}))("/censo").Body.String()
	if !strings.Contains(semMarca, "Nada marcado ainda") {
		t.Error("banco sem marca nenhuma não avisa que a conta ainda não começou")
	}
	if strings.Contains(semMarca, "Nenhuma cópia") {
		t.Error("banco sem marca disse que não há cópias, o que não sabe")
	}
	if !strings.Contains(semMarca, "5000") {
		t.Error("não diz quantos itens ainda esperam marca")
	}

	semCopia := signedIn(t, newTestPanelCenso(t, &fakeCenso{
		cmp: duasFotos(), marcados: 900,
	}))("/censo").Body.String()
	if !strings.Contains(semCopia, "Nenhuma cópia") {
		t.Error("com itens marcados e sem repetição, a página não diz que está limpo")
	}
}

// Uma cópia feita antes da marcação não aparece, e a tela tem de dizer isso: um
// moderador que leia a lista vazia como "nunca houve dupe" tira a conclusão
// errada no primeiro dia.
func TestCensoAvisaQueACopiaAntigaNaoAparece(t *testing.T) {
	body := signedIn(t, newTestPanelCenso(t, &fakeCenso{cmp: duasFotos()}))("/censo").Body.String()
	if !strings.Contains(body, "ANTES desta marcação") {
		t.Error("a página não avisa que cópia anterior à marcação é invisível")
	}
}

func TestCensoAgrupaTresCopiasDoMesmoNumero(t *testing.T) {
	rows := umaCopia()
	terceira := rows[0]
	terceira.Onde, terceira.Character, terceira.Account = "account_cargo", "", "conta_c"
	rows = append(rows, terceira)

	get := signedIn(t, newTestPanelCenso(t, &fakeCenso{cmp: duasFotos(), dups: rows, marcados: 9}))
	body := get("/censo").Body.String()
	if !strings.Contains(body, "3 cópias") {
		t.Error("três linhas do mesmo serial não viraram um bloco só")
	}
	// Item de baú é da conta e não tem personagem; ainda assim precisa de dono.
	if !strings.Contains(body, "(baú da conta)") || !strings.Contains(body, "conta_c") {
		t.Error("a cópia guardada no baú apareceu sem dono")
	}
}

func TestCensoFalhaDasCopiasNaoDerrubaOCenso(t *testing.T) {
	get := signedIn(t, newTestPanelCenso(t, &fakeCenso{
		cmp: duasFotos(), dupErr: errors.New("sem banco"),
	}))
	rec := get("/censo")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "as cópias") {
		t.Error("a página não avisa que não conseguiu ler as cópias")
	}
	if !strings.Contains(body, "1415") {
		t.Error("o censo sumiu junto com a lista que falhou")
	}
}

func TestAgrupaCopiasJuntaPorSerial(t *testing.T) {
	rows := []domain.ItemDup{
		{Serial: 9, Index: 1, Onde: "char_carry", Character: "A"},
		{Serial: 9, Index: 1, Onde: "char_equip", Character: "B"},
		{Serial: 8, Index: 2, Onde: "account_cargo"},
	}
	got := agrupaCopias(rows, map[int32]string{1: "Espada"})
	if len(got) != 2 {
		t.Fatalf("grupos = %d, want 2", len(got))
	}
	if got[0].Serial != 9 || len(got[0].Copias) != 2 || got[0].Nome != "Espada" {
		t.Errorf("primeiro grupo = %+v", got[0])
	}
	if got[1].Serial != 8 || len(got[1].Copias) != 1 {
		t.Errorf("segundo grupo = %+v", got[1])
	}
	if got[1].Copias[0].Onde != "baú" {
		t.Errorf("owner_kind não foi traduzido: %q", got[1].Copias[0].Onde)
	}
}

// --- conversa (0034_chat_log) ---

type fakeChat struct {
	mu       sync.Mutex
	linhas   []domain.ChatLinha
	pedidos  []store.ChatQuery
	err      error
	varre    domain.ChatVarredura
	varreErr error
}

func (f *fakeChat) ListChat(_ context.Context, q store.ChatQuery) ([]domain.ChatLinha, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	f.pedidos = append(f.pedidos, q)
	return f.linhas, nil
}

func (f *fakeChat) ChatSweep(context.Context) (domain.ChatVarredura, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.varre, f.varreErr
}

func newTestPanelChat(t *testing.T, c Chat, aud *fakeAudit) http.Handler {
	t.Helper()
	h, err := New(Config{
		Accounts:   withTarget(roleAdmin),
		Writer:     newFakeWriter(),
		Chat:       c,
		Audit:      aud,
		Sessions:   session.New(time.Hour),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

func umaConversa() *fakeChat {
	agora := time.Now()
	return &fakeChat{
		varre: domain.ChatVarredura{VarridoEm: agora.Add(-2 * time.Hour), Dias: 30, Apagadas: 41},
		linhas: []domain.ChatLinha{
			{At: agora.Add(-time.Minute), Tipo: domain.ChatSussurro,
				Character: "Acusado", Alvo: "Vitima", Texto: "me passa o item", X: 2100, Y: 2100},
			{At: agora.Add(-2 * time.Minute), Tipo: domain.ChatPublico,
				Character: "Vitima", Texto: "alguem ai", X: 2100, Y: 2100},
		},
	}
}

// A regra que justifica a tela existir do jeito que existe: conversa privada só
// se lê deixando rastro de quem leu.
func TestConversaGravaNaAuditoriaQuemLeu(t *testing.T) {
	aud := newFakeAudit()
	get := signedIn(t, newTestPanelChat(t, umaConversa(), aud))

	if rec := get("/chat?personagem=Vitima"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	aud.mu.Lock()
	escritas := append([]audit.Record(nil), aud.written...)
	aud.mu.Unlock()

	if len(escritas) != 1 {
		t.Fatalf("registros na auditoria = %d, want 1", len(escritas))
	}
	if escritas[0].Action != "chat.ler" {
		t.Errorf("ação = %q, want chat.ler", escritas[0].Action)
	}
	novos, _ := escritas[0].New.(map[string]any)
	if novos["personagem"] != "Vitima" {
		t.Errorf("a auditoria não guarda o que foi procurado: %+v", novos)
	}
}

// Inclusive a busca que não acha nada: "procurei e não tinha" é exatamente o
// que alguém diria depois, e sem registro não dá para saber que procurou.
func TestConversaGravaNaAuditoriaAteQuandoNaoAchaNada(t *testing.T) {
	aud := newFakeAudit()
	get := signedIn(t, newTestPanelChat(t, &fakeChat{}, aud))
	get("/chat?personagem=Ninguem")

	aud.mu.Lock()
	n := len(aud.written)
	aud.mu.Unlock()
	if n != 1 {
		t.Errorf("registros = %d, want 1 mesmo sem resultado", n)
	}
}

// Abrir a tela não é ler conversa. Registrar isso encheria a auditoria de ruído
// e enterraria as entradas que significam alguma coisa.
func TestConversaAbrirSemBuscarNaoGravaNada(t *testing.T) {
	aud := newFakeAudit()
	get := signedIn(t, newTestPanelChat(t, umaConversa(), aud))
	body := get("/chat").Body.String()

	aud.mu.Lock()
	n := len(aud.written)
	aud.mu.Unlock()
	if n != 0 {
		t.Errorf("registros = %d, want 0 ao só abrir a tela", n)
	}
	if !strings.Contains(body, "Digite um nome") {
		t.Error("a tela não explica que abre vazia de propósito")
	}
	if strings.Contains(body, "me passa o item") {
		t.Error("mostrou conversa sem ninguém ter buscado")
	}
}

// Se a auditoria não aceita a escrita, a leitura NÃO acontece. É o único
// resultado que esta tela não pode produzir: ler mensagem privada sem rastro.
func TestConversaSemAuditoriaRecusaALeitura(t *testing.T) {
	aud := newFakeAudit()
	aud.failWrite = errors.New("sem banco")
	fc := umaConversa()
	get := signedIn(t, newTestPanelChat(t, fc, aud))

	rec := get("/chat?personagem=Vitima")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	fc.mu.Lock()
	consultas := len(fc.pedidos)
	fc.mu.Unlock()
	if consultas != 0 {
		t.Error("leu a conversa mesmo sem conseguir registrar quem leu")
	}
}

func TestConversaMostraOsDoisCanaisEOPrazoQueEstaValendo(t *testing.T) {
	get := signedIn(t, newTestPanelChat(t, umaConversa(), newFakeAudit()))
	body := get("/chat?personagem=Vitima").Body.String()

	for _, want := range []string{"me passa o item", "Acusado", "para Vitima",
		"sussurro", "público", "30 dias", "vai para a auditoria"} {
		if !strings.Contains(body, want) {
			t.Errorf("a página não traz %q", want)
		}
	}
}

// Vazio aqui quer dizer duas coisas opostas — não foi dito, ou já foi apagado —
// e quem lê precisa saber disso antes de concluir inocência.
func TestConversaVaziaExplicaOPrazo(t *testing.T) {
	fc := &fakeChat{varre: domain.ChatVarredura{Dias: 30}}
	get := signedIn(t, newTestPanelChat(t, fc, newFakeAudit()))
	body := get("/chat?personagem=Ninguem").Body.String()
	if !strings.Contains(body, "passou do prazo") {
		t.Error("não avisa que vazio pode ser conversa apagada")
	}
}

func TestConversaPassaOsFiltrosParaAConsulta(t *testing.T) {
	fc := umaConversa()
	get := signedIn(t, newTestPanelChat(t, fc, newFakeAudit()))
	get("/chat?personagem=Vitima&texto=item&tipo=sussurro&dias=1")

	q := fc.pedidos[0]
	if q.Char != "Vitima" || q.Texto != "item" || q.Tipo != domain.ChatSussurro {
		t.Errorf("consulta = %+v", q)
	}
	if q.Desde.IsZero() {
		t.Error("o período de 1 dia não virou um corte de data")
	}
}

// Canal inventado na URL vira "os dois" em vez de chegar cru no banco.
func TestConversaIgnoraCanalInventado(t *testing.T) {
	fc := umaConversa()
	get := signedIn(t, newTestPanelChat(t, fc, newFakeAudit()))
	get("/chat?personagem=Vitima&tipo=grito")
	if fc.pedidos[0].Tipo != "" {
		t.Errorf("tipo = %q, want vazio", fc.pedidos[0].Tipo)
	}
}

func TestConversaSomeSemAConfiguracao(t *testing.T) {
	get := signedIn(t, newTestPanel(t, withTarget(roleAdmin)))
	if rec := get("/chat"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 sem o registro de conversa", rec.Code)
	}
	if strings.Contains(get("/").Body.String(), `href="/chat"`) {
		t.Error("o menu oferece um link para uma página que não existe")
	}
}
