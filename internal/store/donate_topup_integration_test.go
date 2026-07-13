//go:build integration

// Integration tests for the donate top-up store. They require a real database
// and are excluded from the default build. Run with:
//
//	W2PP_TEST_DSN=postgres://postgres:dev@localhost:5432/postgres go test -tags=integration ./internal/store/
package store

import (
	"context"
	"errors"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// TestTopupCreateConfirmHappyPath: create PENDING, confirm once -> CONFIRMED and
// the balance rises by credits.
func TestTopupCreateConfirmHappyPath(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	s := New(pool)

	var accID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO account (name, pass_hash, donate_balance) VALUES ('topup_happy','x',100) RETURNING id`).
		Scan(&accID); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM account WHERE id = $1`, accID) })

	id, err := s.CreateTopupOrder(ctx, domain.TopupOrder{
		ExternalReference: "ref-happy", AccountID: accID, Credits: 30, AmountCents: 500, PaymentMethod: 1,
	})
	if err != nil || id == 0 {
		t.Fatalf("CreateTopupOrder = (%d, %v)", id, err)
	}

	// Pending: GetTopupOrder reports status 1, credits, balance 0.
	st, cr, bal, err := s.GetTopupOrder(ctx, "ref-happy", accID)
	if err != nil || st != TopupStatusPending || cr != 30 || bal != 0 {
		t.Fatalf("get pending = (%d,%d,%d,%v)", st, cr, bal, err)
	}

	out, newBal, err := s.ConfirmTopupOrder(ctx, "ref-happy")
	if err != nil || out != TopupConfirmed || newBal != 130 {
		t.Fatalf("confirm = (%v,%d,%v), want (Confirmed,130,nil)", out, newBal, err)
	}

	// Paid: GetTopupOrder reports status 2 with the current balance.
	st, _, bal, err = s.GetTopupOrder(ctx, "ref-happy", accID)
	if err != nil || st != TopupStatusPaid || bal != 130 {
		t.Errorf("get paid = (%d,bal=%d,%v), want (2,130,nil)", st, bal, err)
	}
}

// TestTopupConfirmIdempotent is the critical test: confirming twice credits once.
func TestTopupConfirmIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	s := New(pool)

	var accID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO account (name, pass_hash, donate_balance) VALUES ('topup_idem','x',0) RETURNING id`).
		Scan(&accID); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM account WHERE id = $1`, accID) })

	if _, err := s.CreateTopupOrder(ctx, domain.TopupOrder{
		ExternalReference: "ref-idem", AccountID: accID, Credits: 50, AmountCents: 990, PaymentMethod: 1,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	out, bal, err := s.ConfirmTopupOrder(ctx, "ref-idem")
	if err != nil || out != TopupConfirmed || bal != 50 {
		t.Fatalf("first confirm = (%v,%d,%v), want (Confirmed,50,nil)", out, bal, err)
	}
	// Second confirm: ALREADY_CONFIRMED, balance unchanged, no re-credit.
	out, bal, err = s.ConfirmTopupOrder(ctx, "ref-idem")
	if err != nil || out != TopupAlreadyConfirmed || bal != 50 {
		t.Fatalf("second confirm = (%v,%d,%v), want (AlreadyConfirmed,50,nil)", out, bal, err)
	}

	var got int32
	if err := pool.QueryRow(ctx, `SELECT donate_balance FROM account WHERE id = $1`, accID).Scan(&got); err != nil {
		t.Fatalf("read balance: %v", err)
	}
	if got != 50 {
		t.Errorf("balance = %d after double-confirm, want 50 (credited once)", got)
	}
}

// TestTopupConfirmUnknownRef: confirming a nonexistent reference is NOT_FOUND and
// credits nothing.
func TestTopupConfirmUnknownRef(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	s := New(pool)

	out, bal, err := s.ConfirmTopupOrder(ctx, "does-not-exist")
	if err != nil || out != TopupNotFound || bal != 0 {
		t.Errorf("confirm unknown = (%v,%d,%v), want (NotFound,0,nil)", out, bal, err)
	}
}

// TestTopupDuplicateExternalRef: a reused external_reference is rejected.
func TestTopupDuplicateExternalRef(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	s := New(pool)

	var accID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO account (name, pass_hash, donate_balance) VALUES ('topup_dup','x',0) RETURNING id`).
		Scan(&accID); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM account WHERE id = $1`, accID) })

	o := domain.TopupOrder{ExternalReference: "ref-dup", AccountID: accID, Credits: 10, AmountCents: 500, PaymentMethod: 1}
	if _, err := s.CreateTopupOrder(ctx, o); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := s.CreateTopupOrder(ctx, o); !errors.Is(err, ErrDuplicateExternalRef) {
		t.Errorf("duplicate create err = %v, want ErrDuplicateExternalRef", err)
	}
}

// TestTopupCreateMissingAccount: an unknown account_id is ErrNotFound.
func TestTopupCreateMissingAccount(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	s := New(pool)

	_, err := s.CreateTopupOrder(ctx, domain.TopupOrder{
		ExternalReference: "ref-noacc", AccountID: 999999999, Credits: 10, AmountCents: 500, PaymentMethod: 1,
	})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("create missing account err = %v, want ErrNotFound", err)
	}
}

// TestTopupPayerProfileUpsert: save then get; a second save overwrites.
func TestTopupPayerProfileUpsert(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	s := New(pool)

	var accID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO account (name, pass_hash) VALUES ('topup_profile','x') RETURNING id`).
		Scan(&accID); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM account WHERE id = $1`, accID) })

	// No profile yet.
	if _, _, found, err := s.GetPayerProfile(ctx, accID); err != nil || found {
		t.Fatalf("get unset = (found=%v, %v)", found, err)
	}
	if err := s.SavePayerProfile(ctx, accID, "Jean", "12345678909"); err != nil {
		t.Fatalf("save: %v", err)
	}
	name, cpf, found, err := s.GetPayerProfile(ctx, accID)
	if err != nil || !found || name != "Jean" || cpf != "12345678909" {
		t.Fatalf("get = (%q,%q,%v,%v)", name, cpf, found, err)
	}
	// Overwrite.
	if err := s.SavePayerProfile(ctx, accID, "Jean Luca", "98765432100"); err != nil {
		t.Fatalf("resave: %v", err)
	}
	name, cpf, _, _ = s.GetPayerProfile(ctx, accID)
	if name != "Jean Luca" || cpf != "98765432100" {
		t.Errorf("after overwrite = (%q,%q), want (Jean Luca, 98765432100)", name, cpf)
	}
}

// TestTopupGetOrderOwnership: a different account cannot read an order.
func TestTopupGetOrderOwnership(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	s := New(pool)

	var owner, other int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO account (name, pass_hash, donate_balance) VALUES ('topup_owner','x',0) RETURNING id`).Scan(&owner); err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO account (name, pass_hash, donate_balance) VALUES ('topup_other','x',0) RETURNING id`).Scan(&other); err != nil {
		t.Fatalf("seed other: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM account WHERE id = ANY($1)`, []int64{owner, other}) })

	if _, err := s.CreateTopupOrder(ctx, domain.TopupOrder{
		ExternalReference: "ref-own", AccountID: owner, Credits: 10, AmountCents: 500, PaymentMethod: 1,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	// The owner reads it; the other account gets ErrNotFound.
	if _, _, _, err := s.GetTopupOrder(ctx, "ref-own", owner); err != nil {
		t.Errorf("owner get err = %v, want nil", err)
	}
	if _, _, _, err := s.GetTopupOrder(ctx, "ref-own", other); !errors.Is(err, ErrNotFound) {
		t.Errorf("other-account get err = %v, want ErrNotFound", err)
	}
}

// TestTopupCreditCardSmoke: an order with PaymentMethod=CREDIT_CARD persists and
// confirms identically — the method is just a column.
func TestTopupCreditCardSmoke(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	s := New(pool)

	var accID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO account (name, pass_hash, donate_balance) VALUES ('topup_cc','x',0) RETURNING id`).Scan(&accID); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM account WHERE id = $1`, accID) })

	if _, err := s.CreateTopupOrder(ctx, domain.TopupOrder{
		ExternalReference: "ref-cc", AccountID: accID, Credits: 25, AmountCents: 4990, PaymentMethod: 2,
	}); err != nil {
		t.Fatalf("create credit-card order: %v", err)
	}
	var method int16
	if err := pool.QueryRow(ctx,
		`SELECT payment_method FROM donate_topup_order WHERE external_reference = 'ref-cc'`).Scan(&method); err != nil {
		t.Fatalf("read method: %v", err)
	}
	if method != 2 {
		t.Errorf("payment_method = %d, want 2 (CREDIT_CARD)", method)
	}
	if out, bal, err := s.ConfirmTopupOrder(ctx, "ref-cc"); err != nil || out != TopupConfirmed || bal != 25 {
		t.Errorf("confirm cc = (%v,%d,%v), want (Confirmed,25,nil)", out, bal, err)
	}
}
