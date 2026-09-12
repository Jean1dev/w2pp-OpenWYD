package store

import (
	"context"
	"fmt"
)

// Shop-points wallet (0060_shop_points): the currency an open personal shop pays
// its owner, 3 points per quarter-hour and 7 with a Fada Azul.
//
// Separate from account.donate_balance on purpose — that wallet is money somebody
// paid and is what the revenue panel sums; this one is time. See the migration.

// AddShopPoints credits (or debits, with a negative delta) the account's
// shop-points wallet and appends the movement to the audit trail, in one
// transaction. It returns the balance AFTER the movement.
//
// The balance is computed by the database (balance + $2), never written back as a
// total the caller worked out: two characters on the same account can finish a
// quarter-hour in the same instant, and a read-modify-write would silently drop
// one of the two credits.
//
// A negative delta that would take the wallet below zero fails the table's CHECK
// and rolls the whole thing back, audit row included — a spend can never leave a
// receipt for points that were not actually taken.
func (s *Store) AddShopPoints(ctx context.Context, accountID int64, delta int32, characterName, reason string) (int32, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("store: abrir transação de pontos de lojinha: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var saldo int32
	// The wallet row is created on first credit: an account that never kept a
	// shop open has no row, and requiring a seed would mean every account ever
	// created carries one.
	err = tx.QueryRow(ctx, `
		INSERT INTO shop_points (account_id, balance, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (account_id) DO UPDATE
			SET balance = shop_points.balance + $2, updated_at = now()
		RETURNING balance`, accountID, delta).Scan(&saldo)
	if err != nil {
		return 0, fmt.Errorf("store: creditar pontos de lojinha: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO shop_points_audit (account_id, character_name, delta, balance_after, reason)
		VALUES ($1, $2, $3, $4, $5)`,
		accountID, characterName, delta, saldo, reason); err != nil {
		return 0, fmt.Errorf("store: gravar extrato de pontos de lojinha: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("store: confirmar pontos de lojinha: %w", err)
	}
	return saldo, nil
}

// ShopPoints reads one account's balance. A missing row is zero, not an error:
// an account that never opened a shop simply has no points.
func (s *Store) ShopPoints(ctx context.Context, accountID int64) (int32, error) {
	var saldo int32
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE((SELECT balance FROM shop_points WHERE account_id = $1), 0)`,
		accountID).Scan(&saldo)
	if err != nil {
		return 0, fmt.Errorf("store: ler pontos de lojinha: %w", err)
	}
	return saldo, nil
}
