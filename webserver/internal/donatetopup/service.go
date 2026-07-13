// Package donatetopup holds the web platform's payment-method-agnostic donate
// top-up logic: the payer profile (name + CPF) and the top-up order lifecycle
// (create PENDING -> confirm PAID -> poll). Unlike DonateAdminService these RPCs
// are NOT moderator-gated — the BFF passes the logged account_id from its
// httpOnly session (like DonateShopService). The credit of a paid order happens
// inside ConfirmTopupOrder, transactionally and idempotently on
// external_reference, so a replayed gateway webhook credits exactly once. It
// never touches live game state — donate_balance is a partial column update the
// tmServer's single-owner loop is unaffected by.
package donatetopup

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

// Store is the persistence surface the service needs (satisfied by *store.Store).
type Store interface {
	SavePayerProfile(ctx context.Context, accountID int64, name, cpf string) error
	GetPayerProfile(ctx context.Context, accountID int64) (name, cpf string, found bool, err error)
	CreateTopupOrder(ctx context.Context, o domain.TopupOrder) (int64, error)
	ConfirmTopupOrder(ctx context.Context, externalRef string) (store.TopupConfirmOutcome, int32, error)
	GetTopupOrder(ctx context.Context, externalRef string, accountID int64) (status int16, credits int32, newBalance int32, err error)
}

// Result is the business outcome of a profile/create operation, mirroring
// donateshop.Result and carried in the response's AdminResult. Only infra
// failures are returned as errors.
type Result int

const (
	// OK means the operation succeeded.
	OK Result = iota
	// Invalid means the request failed validation.
	Invalid
	// NotFound means the target account does not exist.
	NotFound
)

// ConfirmOutcome is the result of ConfirmTopupOrder, mirroring webv1.TopupResult.
type ConfirmOutcome int

const (
	// Confirmed means the order was PENDING and was credited now.
	Confirmed ConfirmOutcome = iota
	// AlreadyConfirmed means the order was already PAID — not re-credited.
	AlreadyConfirmed
	// ConfirmNotFound means no order has that external_reference.
	ConfirmNotFound
)

// Service implements the donate top-up operations.
type Service struct {
	store Store
}

// New builds the service over the given store.
func New(s Store) *Service { return &Service{store: s} }

// --- payer profile ---

// SavePayerProfile upserts the payer's name + CPF. The CPF is normalized to 11
// digits (non-digits stripped); a name or CPF that fails validation, or a
// non-positive accountID, yields Invalid. A missing account yields NotFound.
func (s *Service) SavePayerProfile(ctx context.Context, accountID int64, name, cpf string) (Result, error) {
	if accountID <= 0 {
		return Invalid, nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Invalid, nil
	}
	digits, ok := normalizeCPF(cpf)
	if !ok {
		return Invalid, nil
	}
	err := s.store.SavePayerProfile(ctx, accountID, name, digits)
	if errors.Is(err, store.ErrNotFound) {
		return NotFound, nil
	}
	if err != nil {
		return Invalid, fmt.Errorf("donatetopup: save profile a=%d: %w", accountID, err)
	}
	return OK, nil
}

// GetPayerProfile returns the payer's stored name + CPF. found is false (no
// error) when no profile exists yet.
func (s *Service) GetPayerProfile(ctx context.Context, accountID int64) (found bool, name, cpf string, err error) {
	name, cpf, found, err = s.store.GetPayerProfile(ctx, accountID)
	if err != nil {
		return false, "", "", fmt.Errorf("donatetopup: get profile a=%d: %w", accountID, err)
	}
	return found, name, cpf, nil
}

// --- top-up orders ---

// CreateTopupOrder persists a PENDING order (persist-before-provider). It
// validates the request; a reused external_reference is reported as Invalid (the
// portal must generate a fresh UUID), a missing account as NotFound.
func (s *Service) CreateTopupOrder(ctx context.Context, o domain.TopupOrder) (Result, int64, error) {
	if o.AccountID <= 0 || o.Credits <= 0 || o.AmountCents <= 0 ||
		strings.TrimSpace(o.ExternalReference) == "" || o.PaymentMethod == 0 {
		return Invalid, 0, nil
	}
	id, err := s.store.CreateTopupOrder(ctx, o)
	switch {
	case err == nil:
		return OK, id, nil
	case errors.Is(err, store.ErrDuplicateExternalRef):
		return Invalid, 0, nil
	case errors.Is(err, store.ErrNotFound):
		return NotFound, 0, nil
	default:
		return Invalid, 0, fmt.Errorf("donatetopup: create order ref=%q: %w", o.ExternalReference, err)
	}
}

// ConfirmTopupOrder settles a paid order idempotently, returning the outcome and
// the resulting balance (current balance when already-confirmed).
func (s *Service) ConfirmTopupOrder(ctx context.Context, externalRef string) (ConfirmOutcome, int64, error) {
	outcome, balance, err := s.store.ConfirmTopupOrder(ctx, externalRef)
	if err != nil {
		return ConfirmNotFound, 0, fmt.Errorf("donatetopup: confirm ref=%q: %w", externalRef, err)
	}
	switch outcome {
	case store.TopupConfirmed:
		return Confirmed, int64(balance), nil
	case store.TopupAlreadyConfirmed:
		return AlreadyConfirmed, int64(balance), nil
	default:
		return ConfirmNotFound, 0, nil
	}
}

// GetTopupOrder returns the order's status and credits for the polling portal.
// An order not owned by accountID (or absent) reports status 0 (UNSPECIFIED)
// with zero values — the store's ownership filter treats it as not found.
func (s *Service) GetTopupOrder(ctx context.Context, externalRef string, accountID int64) (status int16, credits int32, newBalance int64, err error) {
	st, cr, bal, err := s.store.GetTopupOrder(ctx, externalRef, accountID)
	if errors.Is(err, store.ErrNotFound) {
		return 0, 0, 0, nil
	}
	if err != nil {
		return 0, 0, 0, fmt.Errorf("donatetopup: get order ref=%q: %w", externalRef, err)
	}
	return st, cr, int64(bal), nil
}

// --- helpers ---

// normalizeCPF strips every non-digit and accepts the result only when exactly
// 11 digits remain (length-only validation; no verifier-digit algorithm).
func normalizeCPF(cpf string) (string, bool) {
	var b strings.Builder
	for _, r := range cpf {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	if len(digits) != 11 {
		return "", false
	}
	return digits, true
}
