package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

const (
	kingdomHekalotia = 7
	kingdomAkelonia  = 8
	minimumCapeCost  = 4
	maximumCapeCost  = 32
)

// QuoteKingdomCape returns the durable price snapshot used for optimistic purchases.
func (s *Store) QuoteKingdomCape(ctx context.Context) (domain.KingdomCapeQuote, error) {
	var q domain.KingdomCapeQuote
	err := s.pool.QueryRow(ctx, `SELECT version, hekalotia_cost, akelonia_cost FROM sapphire_balance WHERE singleton`).Scan(
		&q.Revision, &q.HekalotiaCost, &q.AkeloniaCost)
	if err != nil {
		return q, fmt.Errorf("store: quote kingdom cape: %w", err)
	}
	return q, nil
}

// PurchaseKingdomCape atomically persists payment/cape state and advances prices.
func (s *Store) PurchaseKingdomCape(ctx context.Context, accountID, expectedRevision int64, kingdom uint8, ch domain.Character) (domain.KingdomCapeQuote, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.KingdomCapeQuote{}, false, fmt.Errorf("store: begin kingdom cape purchase: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var q domain.KingdomCapeQuote
	err = tx.QueryRow(ctx, `SELECT version, hekalotia_cost, akelonia_cost FROM sapphire_balance WHERE singleton FOR UPDATE`).Scan(
		&q.Revision, &q.HekalotiaCost, &q.AkeloniaCost)
	if err != nil {
		return q, false, fmt.Errorf("store: lock kingdom cape quote: %w", err)
	}
	if q.Revision != expectedRevision {
		return q, false, nil
	}
	if kingdom != kingdomHekalotia && kingdom != kingdomAkelonia {
		return q, false, fmt.Errorf("store: invalid cape kingdom %d", kingdom)
	}

	var characterID int64
	err = tx.QueryRow(ctx, `UPDATE character SET clan=$3 WHERE account_id=$1 AND slot=$2 RETURNING id`, accountID, ch.Slot, ch.Clan).Scan(&characterID)
	if errors.Is(err, pgx.ErrNoRows) {
		return q, false, ErrNotFound
	}
	if err != nil {
		return q, false, fmt.Errorf("store: update cape character: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM item WHERE character_id=$1 AND owner_kind IN ('char_equip','char_carry')`, characterID); err != nil {
		return q, false, fmt.Errorf("store: clear cape purchase items: %w", err)
	}
	for _, it := range ch.Equip {
		if err := insertItem(ctx, tx, "char_equip", nil, &characterID, it); err != nil {
			return q, false, err
		}
	}
	for _, it := range ch.Carry {
		if err := insertItem(ctx, tx, "char_carry", nil, &characterID, it); err != nil {
			return q, false, err
		}
	}

	if kingdom == kingdomHekalotia {
		q.HekalotiaCost = clampCapeCost(q.HekalotiaCost + 1)
		q.AkeloniaCost = clampCapeCost(q.AkeloniaCost - 1)
	} else {
		q.HekalotiaCost = clampCapeCost(q.HekalotiaCost - 1)
		q.AkeloniaCost = clampCapeCost(q.AkeloniaCost + 1)
	}
	q.Revision++
	_, err = tx.Exec(ctx, `UPDATE sapphire_balance SET version=$1, hekalotia_cost=$2, akelonia_cost=$3 WHERE singleton`, q.Revision, q.HekalotiaCost, q.AkeloniaCost)
	if err != nil {
		return q, false, fmt.Errorf("store: update kingdom cape quote: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return q, false, fmt.Errorf("store: commit kingdom cape purchase: %w", err)
	}
	return q, true, nil
}

func clampCapeCost(cost int32) int32 {
	if cost < minimumCapeCost {
		return minimumCapeCost
	}
	if cost > maximumCapeCost {
		return maximumCapeCost
	}
	return cost
}
