package account

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/playcode"
	"github.com/jeanluca/w2pp-openwyd/internal/secret"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

// fakeStore is an in-memory Store for unit tests (no PostgreSQL).
type fakeStore struct {
	byName  map[string]store.AccountAuth
	saveErr error // returned by SaveAccount instead of inserting
	saved   []domain.Account
	nextID  int64

	issue      store.PlayCodeIssue // IssuePlayCode result
	issueErr   error
	issuedFor  int64  // last IssuePlayCode account id
	issuedHash []byte // last stored hash
	issuedExp  time.Time
}

func (f *fakeStore) IssuePlayCode(_ context.Context, accountID int64, codeHash []byte, expiresAt time.Time) (store.PlayCodeIssue, error) {
	f.issuedFor, f.issuedHash, f.issuedExp = accountID, codeHash, expiresAt
	return f.issue, f.issueErr
}

func (f *fakeStore) AccountByName(_ context.Context, name string) (store.AccountAuth, error) {
	a, ok := f.byName[name]
	if !ok {
		return store.AccountAuth{}, store.ErrNotFound
	}
	return a, nil
}

func (f *fakeStore) SaveAccount(_ context.Context, acc domain.Account) (int64, error) {
	if f.saveErr != nil {
		return 0, f.saveErr
	}
	f.saved = append(f.saved, acc)
	f.nextID++
	if f.byName == nil {
		f.byName = map[string]store.AccountAuth{}
	}
	f.byName[acc.Name] = store.AccountAuth{ID: f.nextID, PassHash: acc.PassHash}
	return f.nextID, nil
}

func TestCreate(t *testing.T) {
	cases := []struct {
		name              string
		login, pass, mail string
		existing          string // pre-seeded canonical name, if any
		saveErr           error
		want              CreateResult
		wantErr           bool
	}{
		{name: "ok", login: "Alice", pass: "s3cret", mail: "a@b.com", want: CreateOK},
		{name: "ok no email", login: "bob1", pass: "pass", want: CreateOK},
		{name: "name taken (lookup)", login: "carol", pass: "pass", existing: "carol", want: CreateNameTaken},
		{name: "short name", login: "ab", pass: "pass", want: CreateInvalid},
		{name: "long name", login: "thisnameistoolong", pass: "pass", want: CreateInvalid},
		{name: "non-alnum name", login: "bad-name", pass: "pass", want: CreateInvalid},
		{name: "short password", login: "dave", pass: "no", want: CreateInvalid},
		{name: "bad email", login: "erin", pass: "pass", mail: "not-an-email", want: CreateInvalid},
		{name: "unique race -> taken", login: "frank", pass: "pass", saveErr: &pgconn.PgError{Code: "23505"}, want: CreateNameTaken},
		{name: "infra error", login: "grace", pass: "pass", saveErr: errors.New("boom"), want: CreateInvalid, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := &fakeStore{saveErr: tc.saveErr}
			if tc.existing != "" {
				fs.byName = map[string]store.AccountAuth{tc.existing: {ID: 99}}
			}
			s := New(fs)

			res, id, err := s.Create(context.Background(), tc.login, tc.pass, tc.mail)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if res != tc.want {
				t.Fatalf("result = %v, want %v", res, tc.want)
			}
			if tc.want == CreateOK {
				if id == 0 {
					t.Fatal("expected non-zero account id on OK")
				}
				if len(fs.saved) != 1 {
					t.Fatalf("expected 1 saved account, got %d", len(fs.saved))
				}
				got := fs.saved[0]
				if got.Name != "alice" && got.Name != "bob1" {
					t.Errorf("name not canonicalized: %q", got.Name)
				}
				if got.PassHash == "" || got.PassHash == tc.pass {
					t.Errorf("password not hashed: %q", got.PassHash)
				}
			}
		})
	}
}

func TestVerify(t *testing.T) {
	pw := "correct horse"
	hash, err := secret.HashSecret(pw)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	fs := &fakeStore{byName: map[string]store.AccountAuth{
		"alice":  {ID: 1, PassHash: hash, Role: "moderator"},
		"banned": {ID: 2, PassHash: hash, IsBlocked: true},
	}}
	s := New(fs)

	cases := []struct {
		name, login, pass string
		wantOK, wantBlk   bool
		wantID            int64
		wantRole          string
	}{
		{"ok moderator", "Alice", pw, true, false, 1, "moderator"},
		{"wrong password", "alice", "nope", false, false, 0, ""},
		{"no account", "ghost", pw, false, false, 0, ""},
		{"blocked but valid", "banned", pw, true, true, 2, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, id, blocked, role, err := s.Verify(context.Background(), tc.login, tc.pass)
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if ok != tc.wantOK || blocked != tc.wantBlk || id != tc.wantID || role != tc.wantRole {
				t.Fatalf("got ok=%v blocked=%v id=%d role=%q; want ok=%v blocked=%v id=%d role=%q",
					ok, blocked, id, role, tc.wantOK, tc.wantBlk, tc.wantID, tc.wantRole)
			}
		})
	}
}

func TestIssuePlayCode(t *testing.T) {
	now := time.Unix(1790000000, 0)
	fs := &fakeStore{issue: store.PlayCodeIssue{Name: "alice"}}
	s := New(fs)
	s.now = func() time.Time { return now }

	res, name, code, err := s.IssuePlayCode(context.Background(), 7)
	if err != nil || res != PlayCodeOK || name != "alice" || !playcode.Valid(code) {
		t.Fatalf("IssuePlayCode = %v %q %q %v", res, name, code, err)
	}
	// Only the hash is stored, with the two-minute expiry.
	if fs.issuedFor != 7 || string(fs.issuedHash) != string(playcode.Hash(code)) || !fs.issuedExp.Equal(now.Add(playcode.TTL)) {
		t.Fatalf("stored id=%d exp=%v hash matches=%v", fs.issuedFor, fs.issuedExp, string(fs.issuedHash) == string(playcode.Hash(code)))
	}

	cases := []struct {
		name  string
		store *fakeStore
		want  PlayCodeResult
		err   bool
	}{
		{"no account", &fakeStore{issueErr: store.ErrNotFound}, PlayCodeNoAccount, false},
		{"blocked", &fakeStore{issue: store.PlayCodeIssue{Name: "x", Blocked: true}}, PlayCodeBlocked, false},
		{"db down", &fakeStore{issueErr: errors.New("db down")}, PlayCodeNoAccount, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, _, code, err := New(tc.store).IssuePlayCode(context.Background(), 7)
			if res != tc.want || (err != nil) != tc.err || code != "" {
				t.Fatalf("got %v code=%q err=%v; want %v err=%v", res, code, err, tc.want, tc.err)
			}
		})
	}
}
