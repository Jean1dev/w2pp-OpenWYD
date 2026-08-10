// Package donaterevenue holds the read-only revenue reporting logic behind the
// painel de faturamento: how much money came in over a period, who paid, and
// where the donate credits went. It reads donate_topup_order (real money),
// donate_shop_audit (wallet movements), account and donate_payer_profile.
//
// It is NOT bin.v1.BillingService — that is the login entitlement gate, which
// web-platform-plan.md explicitly separates from the cash wallet.
//
// Every operation is moderator-gated exactly like donateshop, and every
// operation is a READ: this package writes nothing at all, not even an audit
// row, so it never interacts with the tmServer's single-owner loop.
//
// Two units flow through here and never mix: *Cents is real BRL in integer
// cents, *Credits is the in-game donate wallet unit. Revenue is recognized on
// confirmed_at, never created_at — the store enforces that; see
// internal/store/donate_revenue.go.
package donaterevenue

import (
	"context"
	"errors"
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

// Store is the persistence surface the service needs (satisfied by *store.Store).
type Store interface {
	AccountRole(ctx context.Context, id int64) (string, error)
	RevenueTotals(ctx context.Context, w store.RevenueWindow, accountID int64) (domain.RevenueTotals, error)
	RevenueByMethod(ctx context.Context, w store.RevenueWindow, accountID int64) ([]domain.RevenueByMethod, error)
	RevenueSeries(ctx context.Context, w store.RevenueWindow, bucket string, accountID int64) ([]domain.RevenuePoint, error)
	DonateLedgerTotals(ctx context.Context, w store.RevenueWindow, accountID int64) (domain.DonateLedgerTotals, error)
	ListTopupOrders(ctx context.Context, w store.RevenueWindow, f store.TopupOrderFilter, limit, offset int) ([]domain.TopupOrderRow, int, error)
	ListTopBuyers(ctx context.Context, w store.RevenueWindow, limit, offset int) ([]domain.TopBuyer, int, error)
	ListDonateLedger(ctx context.Context, w store.RevenueWindow, actions []string, accountID int64, limit, offset int) ([]domain.DonateLedgerEntry, int, error)
	SearchAccountsByNamePrefix(ctx context.Context, prefix string, limit int) ([]domain.AccountSummary, error)
}

// Result is the business outcome of an operation, mirroring donateshop.Result
// and carried in the response's AdminResult. Only infra failures are returned
// as errors.
type Result int

const (
	// OK means the operation succeeded.
	OK Result = iota
	// Forbidden means the caller is not a moderator/admin.
	Forbidden
	// Invalid means the request failed validation.
	Invalid
	// NotFound means the target does not exist. Reserved for contract symmetry:
	// no read here returns it, because an empty window is a legitimate result.
	NotFound
)

// Bucket is the requested series granularity, mirroring webv1.RevenueBucket.
type Bucket int

const (
	// BucketUnspecified skips the series entirely.
	BucketUnspecified Bucket = iota
	// BucketDay buckets per calendar day.
	BucketDay
	// BucketWeek buckets per ISO week (starts Monday).
	BucketWeek
	// BucketMonth buckets per calendar month.
	BucketMonth
)

// LedgerAction filters the donate ledger, mirroring webv1.DonateLedgerAction.
type LedgerAction int

const (
	// LedgerAny returns both movement kinds.
	LedgerAny LedgerAction = iota
	// LedgerPurchase returns only shop purchases (wallet debits).
	LedgerPurchase
	// LedgerCredit returns only moderator credits (wallet credits).
	LedgerCredit
)

// Summary bundles everything the KPI header needs in one round trip.
type Summary struct {
	Totals   domain.RevenueTotals
	Ledger   domain.DonateLedgerTotals
	ByMethod []domain.RevenueByMethod
	Series   []domain.RevenuePoint
	Window   Window
}

// Service implements the revenue reporting operations.
type Service struct {
	store Store
}

// New builds the service over the given store.
func New(s Store) *Service { return &Service{store: s} }

// Summary returns the KPI header, the per-gateway split and, when bucket is not
// Unspecified, the gap-filled time series. accountID 0 means all accounts.
func (s *Service) Summary(ctx context.Context, moderatorID, fromUnix, toUnix int64, bucket Bucket, accountID int64) (Result, Summary, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, Summary{}, err
	}
	if accountID < 0 {
		return Invalid, Summary{}, nil
	}
	w, echo, ok := normalizeWindow(fromUnix, toUnix)
	if !ok {
		return Invalid, Summary{}, nil
	}

	// Four independent reads over the same window. They stay sequential: no
	// other webserver service fans out, and the saving would be a few
	// milliseconds against a pool that also serves every other RPC.
	out := Summary{Window: echo}
	var err error
	if out.Totals, err = s.store.RevenueTotals(ctx, w, accountID); err != nil {
		return Invalid, Summary{}, fmt.Errorf("donaterevenue: revenue totals: %w", err)
	}
	if out.Ledger, err = s.store.DonateLedgerTotals(ctx, w, accountID); err != nil {
		return Invalid, Summary{}, fmt.Errorf("donaterevenue: ledger totals: %w", err)
	}
	if out.ByMethod, err = s.store.RevenueByMethod(ctx, w, accountID); err != nil {
		return Invalid, Summary{}, fmt.Errorf("donaterevenue: revenue by method: %w", err)
	}
	if keyword, want := bucketKeyword(bucket); want {
		if out.Series, err = s.store.RevenueSeries(ctx, w, keyword, accountID); err != nil {
			return Invalid, Summary{}, fmt.Errorf("donaterevenue: revenue series: %w", err)
		}
	}
	return OK, out, nil
}

// Orders returns one page of the order table. The CPF of every row is masked
// here; the raw digits never leave this function.
func (s *Service) Orders(ctx context.Context, moderatorID, fromUnix, toUnix int64, status, method int16, accountID int64, limit, offset int) (Result, []domain.TopupOrderRow, int, Window, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, nil, 0, Window{}, err
	}
	if accountID < 0 || status < 0 || method < 0 {
		return Invalid, nil, 0, Window{}, nil
	}
	w, echo, ok := normalizeWindow(fromUnix, toUnix)
	if !ok {
		return Invalid, nil, 0, Window{}, nil
	}
	limit, offset = normalizePage(limit, offset)

	rows, total, err := s.store.ListTopupOrders(ctx, w,
		store.TopupOrderFilter{Status: status, PaymentMethod: method, AccountID: accountID}, limit, offset)
	if err != nil {
		return Invalid, nil, 0, Window{}, fmt.Errorf("donaterevenue: list topup orders: %w", err)
	}
	for i := range rows {
		rows[i].PayerCPF = maskCPF(rows[i].PayerCPF)
	}
	return OK, rows, total, echo, nil
}

// TopBuyers ranks accounts by revenue inside the window, with their all-time
// aggregate alongside.
func (s *Service) TopBuyers(ctx context.Context, moderatorID, fromUnix, toUnix int64, limit, offset int) (Result, []domain.TopBuyer, int, Window, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, nil, 0, Window{}, err
	}
	w, echo, ok := normalizeWindow(fromUnix, toUnix)
	if !ok {
		return Invalid, nil, 0, Window{}, nil
	}
	limit, offset = normalizePage(limit, offset)

	rows, total, err := s.store.ListTopBuyers(ctx, w, limit, offset)
	if err != nil {
		return Invalid, nil, 0, Window{}, fmt.Errorf("donaterevenue: list top buyers: %w", err)
	}
	return OK, rows, total, echo, nil
}

// Spend returns one page of the donate wallet ledger. accountID matches the
// SUBJECT of each movement (whose wallet moved), never the raw audit column —
// see store.ledgerSubjectExpr for why those differ.
func (s *Service) Spend(ctx context.Context, moderatorID, fromUnix, toUnix int64, action LedgerAction, accountID int64, limit, offset int) (Result, []domain.DonateLedgerEntry, int, Window, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, nil, 0, Window{}, err
	}
	if accountID < 0 {
		return Invalid, nil, 0, Window{}, nil
	}
	w, echo, ok := normalizeWindow(fromUnix, toUnix)
	if !ok {
		return Invalid, nil, 0, Window{}, nil
	}
	limit, offset = normalizePage(limit, offset)

	rows, total, err := s.store.ListDonateLedger(ctx, w, ledgerActions(action), accountID, limit, offset)
	if err != nil {
		return Invalid, nil, 0, Window{}, fmt.Errorf("donaterevenue: list donate ledger: %w", err)
	}
	return OK, rows, total, echo, nil
}

// SearchAccounts resolves a typed login prefix to account identities so the
// panel can build an account_id filter.
func (s *Service) SearchAccounts(ctx context.Context, moderatorID int64, prefix string, limit int) (Result, []domain.AccountSummary, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, nil, err
	}
	p, ok := normalizePrefix(prefix)
	if !ok {
		return Invalid, nil, nil
	}
	rows, err := s.store.SearchAccountsByNamePrefix(ctx, p, normalizeSearchLimit(limit))
	if err != nil {
		return Invalid, nil, fmt.Errorf("donaterevenue: search accounts: %w", err)
	}
	return OK, rows, nil
}

// --- helpers ---

// ledgerActions maps the filter enum to the store's action keywords. LedgerAny
// yields nil, which the store reads as "both movement kinds".
func ledgerActions(a LedgerAction) []string {
	switch a {
	case LedgerPurchase:
		return []string{store.LedgerActionPurchase}
	case LedgerCredit:
		return []string{store.LedgerActionCredit}
	default:
		return nil
	}
}

// authorize checks the caller has a moderator/admin role. A missing account or a
// plain-player role yields Forbidden (never leaks whether the account exists).
// Mirrors donateshop.authorize — the panel is gated no differently from the rest
// of the admin surface.
func (s *Service) authorize(ctx context.Context, moderatorID int64) (Result, error) {
	if moderatorID <= 0 {
		return Forbidden, nil
	}
	role, err := s.store.AccountRole(ctx, moderatorID)
	if errors.Is(err, store.ErrNotFound) {
		return Forbidden, nil
	}
	if err != nil {
		return Invalid, fmt.Errorf("donaterevenue: role lookup %d: %w", moderatorID, err)
	}
	if role != "moderator" && role != "admin" {
		return Forbidden, nil
	}
	return OK, nil
}
