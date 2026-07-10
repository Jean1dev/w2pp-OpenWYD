package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// Donate web shop persistence (issue #34). Postgres owns the shop catalog
// (donate_shop_item) and the donate wallet (account.donate_balance); a purchase
// debits the wallet and enqueues the item into delivery_queue in one transaction.
// The tmServer never reads the catalog — it only drains delivery_queue into the
// account cargo inside its single-owner loop. Crediting/debiting donate_balance
// is a PARTIAL account update, so no tmServer save clobbers it (SaveCharacter
// never touches account; SaveCargo writes only cargo_coin).

// ErrInsufficientDonate is returned by BuyDonateItem when the account's donate
// balance is below the item price.
var ErrInsufficientDonate = errors.New("store: insufficient donate balance")

// ErrShopItemDisabled is returned by BuyDonateItem when the target offer exists
// but is not enabled for sale.
var ErrShopItemDisabled = errors.New("store: donate shop item disabled")

// itemPayload is the delivery_queue.payload shape for kind='item': everything the
// tmServer drain needs to materialize a STRUCT_ITEM into the account cargo.
// ExpiresAt is absolute Unix-seconds (0 = permanent).
type itemPayload struct {
	ItemIndex int32 `json:"item_index"`
	Eff1      uint8 `json:"eff1"`
	EffV1     uint8 `json:"effv1"`
	Eff2      uint8 `json:"eff2"`
	EffV2     uint8 `json:"effv2"`
	Eff3      uint8 `json:"eff3"`
	EffV3     uint8 `json:"effv3"`
	ExpiresAt int64 `json:"expires_at"`
}

// ListDonateShopItems returns every shop offer (enabled or not), ordered by id —
// the moderation table. ListEnabledDonateShopItems is the player-facing vitrine.
func (s *Store) ListDonateShopItems(ctx context.Context) ([]domain.DonateShopItem, error) {
	return s.queryDonateShopItems(ctx, `
		SELECT id, item_index, eff1, effv1, eff2, effv2, eff3, effv3, price, title, description, enabled, expires_days
		FROM donate_shop_item ORDER BY id`)
}

// ListEnabledDonateShopItems returns only the offers on sale (the web vitrine).
func (s *Store) ListEnabledDonateShopItems(ctx context.Context) ([]domain.DonateShopItem, error) {
	return s.queryDonateShopItems(ctx, `
		SELECT id, item_index, eff1, effv1, eff2, effv2, eff3, effv3, price, title, description, enabled, expires_days
		FROM donate_shop_item WHERE enabled = TRUE ORDER BY id`)
}

func (s *Store) queryDonateShopItems(ctx context.Context, sql string) ([]domain.DonateShopItem, error) {
	rows, err := s.pool.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("store: list donate shop items: %w", err)
	}
	defer rows.Close()
	var out []domain.DonateShopItem
	for rows.Next() {
		d, err := scanDonateShopItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// GetDonateShopItem loads one offer by id. Returns ErrNotFound when absent.
func (s *Store) GetDonateShopItem(ctx context.Context, id int64) (domain.DonateShopItem, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, item_index, eff1, effv1, eff2, effv2, eff3, effv3, price, title, description, enabled, expires_days
		FROM donate_shop_item WHERE id = $1`, id)
	d, err := scanDonateShopItem(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DonateShopItem{}, ErrNotFound
	}
	if err != nil {
		return domain.DonateShopItem{}, fmt.Errorf("store: get donate shop item %d: %w", id, err)
	}
	return d, nil
}

// scanRow is the read surface shared by pgx.Row and pgx.Rows (both have Scan).
type scanRow interface {
	Scan(dest ...any) error
}

func scanDonateShopItem(row scanRow) (domain.DonateShopItem, error) {
	var d domain.DonateShopItem
	err := row.Scan(&d.ID, &d.ItemIndex, &d.Eff1, &d.EffV1, &d.Eff2, &d.EffV2, &d.Eff3, &d.EffV3,
		&d.Price, &d.Title, &d.Description, &d.Enabled, &d.ExpiresDays)
	return d, err
}

// UpsertDonateShopItem inserts a new offer (d.ID == 0) or updates the existing
// one by id, writing an audit row in the same transaction. Returns the offer id;
// updating a missing id returns ErrNotFound.
func (s *Store) UpsertDonateShopItem(ctx context.Context, d domain.DonateShopItem, moderatorID int64) (int64, error) {
	var id int64
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if d.ID == 0 {
			if err := tx.QueryRow(ctx, `
				INSERT INTO donate_shop_item
					(item_index, eff1, effv1, eff2, effv2, eff3, effv3, price, title, description, enabled, expires_days, updated_by, updated_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13, now())
				RETURNING id`,
				d.ItemIndex, d.Eff1, d.EffV1, d.Eff2, d.EffV2, d.Eff3, d.EffV3,
				d.Price, d.Title, d.Description, d.Enabled, d.ExpiresDays, nullableID(moderatorID),
			).Scan(&id); err != nil {
				return fmt.Errorf("store: insert donate shop item: %w", err)
			}
			after, _ := fetchDonateItemJSON(ctx, tx, id)
			return donateAudit(ctx, tx, &id, moderatorID, "create", nil, after)
		}

		before, _ := fetchDonateItemJSON(ctx, tx, d.ID)
		if before == nil {
			return ErrNotFound
		}
		id = d.ID
		if _, err := tx.Exec(ctx, `
			UPDATE donate_shop_item SET
				item_index=$2, eff1=$3, effv1=$4, eff2=$5, effv2=$6, eff3=$7, effv3=$8,
				price=$9, title=$10, description=$11, enabled=$12, expires_days=$13,
				updated_by=$14, updated_at=now()
			WHERE id=$1`,
			id, d.ItemIndex, d.Eff1, d.EffV1, d.Eff2, d.EffV2, d.Eff3, d.EffV3,
			d.Price, d.Title, d.Description, d.Enabled, d.ExpiresDays, nullableID(moderatorID),
		); err != nil {
			return fmt.Errorf("store: update donate shop item %d: %w", id, err)
		}
		after, _ := fetchDonateItemJSON(ctx, tx, id)
		return donateAudit(ctx, tx, &id, moderatorID, "update", before, after)
	})
	return id, err
}

// SetDonateShopItemEnabled toggles whether an offer is on sale. Returns
// ErrNotFound if the offer does not exist.
func (s *Store) SetDonateShopItemEnabled(ctx context.Context, id int64, enabled bool, moderatorID int64) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		before, _ := fetchDonateItemJSON(ctx, tx, id)
		if before == nil {
			return ErrNotFound
		}
		if _, err := tx.Exec(ctx,
			`UPDATE donate_shop_item SET enabled=$2, updated_by=$3, updated_at=now() WHERE id=$1`,
			id, enabled, nullableID(moderatorID)); err != nil {
			return fmt.Errorf("store: set donate shop item %d enabled: %w", id, err)
		}
		after, _ := fetchDonateItemJSON(ctx, tx, id)
		return donateAudit(ctx, tx, &id, moderatorID, "set_enabled", before, after)
	})
}

// DeleteDonateShopItem removes an offer. Returns ErrNotFound if absent.
func (s *Store) DeleteDonateShopItem(ctx context.Context, id int64, moderatorID int64) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		before, _ := fetchDonateItemJSON(ctx, tx, id)
		if before == nil {
			return ErrNotFound
		}
		if _, err := tx.Exec(ctx, `DELETE FROM donate_shop_item WHERE id = $1`, id); err != nil {
			return fmt.Errorf("store: delete donate shop item %d: %w", id, err)
		}
		return donateAudit(ctx, tx, &id, moderatorID, "delete", before, nil)
	})
}

// DonateBalance returns an account's donate wallet balance. Returns ErrNotFound
// when the account does not exist.
func (s *Store) DonateBalance(ctx context.Context, accountID int64) (int32, error) {
	var bal int32
	err := s.pool.QueryRow(ctx, `SELECT donate_balance FROM account WHERE id = $1`, accountID).Scan(&bal)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("store: donate balance a=%d: %w", accountID, err)
	}
	return bal, nil
}

// CreditDonateBalance adds amount to an account's donate wallet (the manual/admin
// credit path the future payment webhook reuses) and returns the new balance. It
// is a partial UPDATE — never a full-row write — so it is safe against tmServer
// saves. Returns ErrNotFound if the account is absent.
func (s *Store) CreditDonateBalance(ctx context.Context, accountID int64, amount int32, moderatorID int64, reason string) (int32, error) {
	var newBal int32
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx,
			`UPDATE account SET donate_balance = donate_balance + $2 WHERE id = $1 RETURNING donate_balance`,
			accountID, amount).Scan(&newBal)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("store: credit donate a=%d: %w", accountID, err)
		}
		after, _ := json.Marshal(map[string]any{
			"account_id": accountID, "amount": amount, "balance": newBal, "reason": reason,
		})
		return donateAudit(ctx, tx, nil, moderatorID, "credit_balance", nil, after)
	})
	return newBal, err
}

// BuyDonateItem debits the account's donate balance by the offer price and
// enqueues the item into delivery_queue, transactionally, returning the new
// balance. The tmServer delivers the queued item into the account cargo on the
// next login (web-platform-plan.md §mailbox). Returns ErrNotFound (unknown offer
// or account), ErrShopItemDisabled (offer not on sale), or ErrInsufficientDonate.
func (s *Store) BuyDonateItem(ctx context.Context, accountID, shopItemID int64) (int32, error) {
	var newBal int32
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		var it domain.DonateShopItem
		err := tx.QueryRow(ctx, `
			SELECT id, item_index, eff1, effv1, eff2, effv2, eff3, effv3, price, title, description, enabled, expires_days
			FROM donate_shop_item WHERE id = $1`, shopItemID).
			Scan(&it.ID, &it.ItemIndex, &it.Eff1, &it.EffV1, &it.Eff2, &it.EffV2, &it.Eff3, &it.EffV3,
				&it.Price, &it.Title, &it.Description, &it.Enabled, &it.ExpiresDays)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("store: buy: load offer %d: %w", shopItemID, err)
		}
		if !it.Enabled {
			return ErrShopItemDisabled
		}

		// Lock the wallet row, then check funds before debiting so the outcome is
		// unambiguous (insufficient vs missing account).
		var bal int32
		err = tx.QueryRow(ctx, `SELECT donate_balance FROM account WHERE id = $1 FOR UPDATE`, accountID).Scan(&bal)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("store: buy: load balance a=%d: %w", accountID, err)
		}
		if bal < it.Price {
			return ErrInsufficientDonate
		}
		if err := tx.QueryRow(ctx,
			`UPDATE account SET donate_balance = donate_balance - $2 WHERE id = $1 RETURNING donate_balance`,
			accountID, it.Price).Scan(&newBal); err != nil {
			return fmt.Errorf("store: buy: debit a=%d: %w", accountID, err)
		}

		payload, err := json.Marshal(itemPayload{
			ItemIndex: it.ItemIndex,
			Eff1:      it.Eff1, EffV1: it.EffV1, Eff2: it.Eff2, EffV2: it.EffV2, Eff3: it.Eff3, EffV3: it.EffV3,
			ExpiresAt: expiresDaysToUnix(it.ExpiresDays),
		})
		if err != nil {
			return fmt.Errorf("store: buy: marshal payload: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO delivery_queue (account_id, kind, payload, source)
			VALUES ($1, 'item', $2, $3)`,
			accountID, payload, fmt.Sprintf("donate_shop:%d", it.ID)); err != nil {
			return fmt.Errorf("store: buy: enqueue delivery a=%d: %w", accountID, err)
		}
		after, _ := json.Marshal(map[string]any{
			"account_id": accountID, "shop_item_id": it.ID, "price": it.Price, "balance": newBal,
		})
		return donateAudit(ctx, tx, &it.ID, accountID, "purchase", nil, after)
	})
	return newBal, err
}

// PendingItemDeliveries returns the account's pending item grants from the
// mailbox, oldest first. The tmServer calls it inside its loop at login, applies
// each to the account cargo, then acks via SaveCargoWithDeliveries.
func (s *Store) PendingItemDeliveries(ctx context.Context, accountID int64) ([]domain.Delivery, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, payload FROM delivery_queue
		WHERE account_id = $1 AND status = 'pending' AND kind = 'item'
		ORDER BY id`, accountID)
	if err != nil {
		return nil, fmt.Errorf("store: pending deliveries a=%d: %w", accountID, err)
	}
	defer rows.Close()
	var out []domain.Delivery
	for rows.Next() {
		var id int64
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, fmt.Errorf("store: scan delivery: %w", err)
		}
		var p itemPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("store: decode delivery %d payload: %w", id, err)
		}
		out = append(out, domain.Delivery{
			ID:        id,
			AccountID: accountID,
			Kind:      domain.DeliveryItem,
			Item: domain.Item{
				Index: int16(p.ItemIndex),
				Eff1:  p.Eff1, EffV1: p.EffV1, Eff2: p.Eff2, EffV2: p.EffV2, Eff3: p.Eff3, EffV3: p.EffV3,
				ExpiresAt: p.ExpiresAt,
			},
		})
	}
	return out, rows.Err()
}

// SaveCargoWithDeliveries persists the account cargo (gold + items, replace-all
// like SaveCargo) AND marks the drained mailbox rows delivered/lost, all in one
// transaction. Committing both together is the anti-dup boundary: a crash before
// commit leaves the rows 'pending' and the cargo unchanged, so the next login
// re-drains cleanly with no duplication. Returns ErrNotFound if the account is
// absent.
func (s *Store) SaveCargoWithDeliveries(ctx context.Context, accountID int64, coin int32, items []domain.Item, deliveredIDs, lostIDs []int64) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE account SET cargo_coin = $2 WHERE id = $1`, accountID, coin)
		if err != nil {
			return fmt.Errorf("store: drain: update cargo coin a=%d: %w", accountID, err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		if _, err := tx.Exec(ctx,
			`DELETE FROM item WHERE account_id = $1 AND owner_kind = 'account_cargo'`, accountID); err != nil {
			return fmt.Errorf("store: drain: clear cargo items a=%d: %w", accountID, err)
		}
		for _, it := range items {
			if err := insertItem(ctx, tx, "account_cargo", &accountID, nil, it); err != nil {
				return err
			}
		}
		if err := markDeliveries(ctx, tx, "delivered", deliveredIDs); err != nil {
			return err
		}
		return markDeliveries(ctx, tx, "lost", lostIDs)
	})
}

func markDeliveries(ctx context.Context, tx pgx.Tx, status string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx,
		`UPDATE delivery_queue SET status = $1 WHERE id = ANY($2)`, status, ids); err != nil {
		return fmt.Errorf("store: mark deliveries %s: %w", status, err)
	}
	return nil
}

// --- helpers ---

// expiresDaysToUnix converts a shop offer's expires_days into an absolute expiry
// timestamp (Unix seconds); 0 days = permanent (0).
func expiresDaysToUnix(days int32) int64 {
	if days <= 0 {
		return 0
	}
	return time.Now().Add(time.Duration(days) * 24 * time.Hour).Unix()
}

// donateAudit writes one row to donate_shop_audit. Unlike auditAndBump there is
// no config version to bump — the tmServer never reads the donate catalog.
func donateAudit(ctx context.Context, tx pgx.Tx, shopItemID *int64, accountID int64, action string, before, after []byte) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO donate_shop_audit (shop_item_id, account_id, action, before, after)
		VALUES ($1,$2,$3,$4,$5)`,
		shopItemID, accountID, action, nullableJSON(before), nullableJSON(after)); err != nil {
		return fmt.Errorf("store: write donate audit: %w", err)
	}
	return nil
}

func fetchDonateItemJSON(ctx context.Context, tx pgx.Tx, id int64) ([]byte, error) {
	var js []byte
	err := tx.QueryRow(ctx, `SELECT to_jsonb(d) FROM donate_shop_item d WHERE id = $1`, id).Scan(&js)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return js, err
}
