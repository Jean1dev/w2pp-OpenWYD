// Package account holds the web platform's account logic: sign-up and the
// credential check the BFF uses to mint a session cookie. It writes only the
// `account` row (cold storage) — never live character state — so it is safe to
// run outside tmServer's single-owner game loop (web-platform-plan.md).
package account

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/secret"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

// Store is the persistence surface the service needs (satisfied by *store.Store).
// Kept as an interface so the service is unit-testable without a live database.
type Store interface {
	AccountByName(ctx context.Context, name string) (store.AccountAuth, error)
	SaveAccount(ctx context.Context, acc domain.Account) (int64, error)
}

// Service creates and authenticates web accounts.
type Service struct {
	store Store
}

// New builds the account service over the given store.
func New(s Store) *Service { return &Service{store: s} }

// CreateResult is the business outcome of a sign-up attempt.
type CreateResult int

const (
	// CreateOK means the account was created; the returned id is valid.
	CreateOK CreateResult = iota
	// CreateNameTaken means the canonical name already exists.
	CreateNameTaken
	// CreateInvalid means name/password/email failed validation.
	CreateInvalid
)

// Account-name rules: 4–12 ASCII alphanumerics (the legacy login field is short
// and case-insensitive). Passwords are at least 4 chars. These are intentionally
// conservative; tighten as product needs dictate.
const (
	minNameLen = 4
	maxNameLen = 12
	minPassLen = 4
)

// Create registers a new account. The name is canonicalized to lowercase; the
// password is stored only as an argon2id hash (never plaintext). Email is
// optional but, when present, must parse as an address.
//
// Business outcomes (invalid input, name taken) are returned as a CreateResult,
// not an error — error is reserved for infrastructure failures. A concurrent
// sign-up that wins the unique-name race is reported as CreateNameTaken via the
// Postgres unique violation, so the DB constraint is the final arbiter.
func (s *Service) Create(ctx context.Context, name, password, email string) (CreateResult, int64, error) {
	canonical := strings.ToLower(strings.TrimSpace(name))
	if !validName(canonical) || len(password) < minPassLen || !validEmail(email) {
		return CreateInvalid, 0, nil
	}

	switch _, err := s.store.AccountByName(ctx, canonical); {
	case err == nil:
		return CreateNameTaken, 0, nil
	case !errors.Is(err, store.ErrNotFound):
		return CreateInvalid, 0, fmt.Errorf("account: lookup %q: %w", canonical, err)
	}

	passHash, err := secret.HashSecret(password)
	if err != nil {
		return CreateInvalid, 0, fmt.Errorf("account: hash password: %w", err)
	}

	id, err := s.store.SaveAccount(ctx, domain.Account{
		Name:     canonical,
		PassHash: passHash,
		Email:    email,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return CreateNameTaken, 0, nil
		}
		return CreateInvalid, 0, fmt.Errorf("account: save %q: %w", canonical, err)
	}
	return CreateOK, id, nil
}

// Verify reports whether name+password match a stored account. A missing account
// or wrong password both return ok=false (no account enumeration via timing is
// attempted here beyond the constant-time hash compare). blocked reflects
// account.is_blocked so the caller can decide how to surface a banned login.
func (s *Service) Verify(ctx context.Context, name, password string) (ok bool, accountID int64, blocked bool, err error) {
	canonical := strings.ToLower(strings.TrimSpace(name))
	auth, err := s.store.AccountByName(ctx, canonical)
	if errors.Is(err, store.ErrNotFound) {
		return false, 0, false, nil
	}
	if err != nil {
		return false, 0, false, fmt.Errorf("account: lookup %q: %w", canonical, err)
	}
	match, err := secret.VerifySecret(password, auth.PassHash)
	if err != nil {
		return false, 0, false, fmt.Errorf("account: verify password: %w", err)
	}
	if !match {
		return false, 0, false, nil
	}
	return true, auth.ID, auth.IsBlocked, nil
}

// validName enforces the 4–12 ASCII-alphanumeric login rule on an already
// lowercased name.
func validName(name string) bool {
	if len(name) < minNameLen || len(name) > maxNameLen {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// validEmail accepts an empty address (email is optional) or any RFC 5322
// address that net/mail can parse.
func validEmail(email string) bool {
	if email == "" {
		return true
	}
	_, err := mail.ParseAddress(email)
	return err == nil
}

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505), used to turn a lost name race into CreateNameTaken.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
