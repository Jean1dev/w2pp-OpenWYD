package account

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/secret"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

// fakeStore is an in-memory Store for unit tests (no PostgreSQL).
type fakeStore struct {
	byName  map[string]store.AccountAuth
	saveErr error // returned by SaveAccount instead of inserting
	saved   []domain.Account
	nextID  int64
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
		"alice":  {ID: 1, PassHash: hash},
		"banned": {ID: 2, PassHash: hash, IsBlocked: true},
	}}
	s := New(fs)

	cases := []struct {
		name, login, pass string
		wantOK, wantBlk   bool
		wantID            int64
	}{
		{"ok", "Alice", pw, true, false, 1},
		{"wrong password", "alice", "nope", false, false, 0},
		{"no account", "ghost", pw, false, false, 0},
		{"blocked but valid", "banned", pw, true, true, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, id, blocked, err := s.Verify(context.Background(), tc.login, tc.pass)
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if ok != tc.wantOK || blocked != tc.wantBlk || id != tc.wantID {
				t.Fatalf("got ok=%v blocked=%v id=%d; want ok=%v blocked=%v id=%d",
					ok, blocked, id, tc.wantOK, tc.wantBlk, tc.wantID)
			}
		})
	}
}
