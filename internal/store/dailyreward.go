package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// Daily reward persistence (issue #35). Postgres owns the reward catalog
// (daily_reward_item) and the once-per-account-per-UTC-day claim gate
// (daily_reward_claim); claiming enqueues the item into delivery_queue in one
// transaction, reusing the exact mailbox donate_shop writes into (0008). The
// tmServer never reads the catalog or the claim table — it only drains
// delivery_queue, identically for donate and daily-reward grants.

// ErrAlreadyClaimedToday is returned by ClaimDailyReward when the account
// already has a daily_reward_claim row for today's UTC calendar day.
var ErrAlreadyClaimedToday = errors.New("store: daily reward already claimed today")

const dailyRewardItemCols = `id, item_index, eff1, effv1, eff2, effv2, eff3, effv3, title, description, enabled, expires_days`

// ListDailyRewardItems returns every reward offer (enabled or not) — the
// moderation table. ListEnabledDailyRewardItems is the player-facing vitrine.
func (s *Store) ListDailyRewardItems(ctx context.Context) ([]domain.DailyRewardItem, error) {
	return s.queryDailyRewardItems(ctx, `SELECT `+dailyRewardItemCols+` FROM daily_reward_item ORDER BY id`)
}

// ListEnabledDailyRewardItems returns only the offers claimable today (the web vitrine).
func (s *Store) ListEnabledDailyRewardItems(ctx context.Context) ([]domain.DailyRewardItem, error) {
	return s.queryDailyRewardItems(ctx, `SELECT `+dailyRewardItemCols+` FROM daily_reward_item WHERE enabled = TRUE ORDER BY id`)
}

func (s *Store) queryDailyRewardItems(ctx context.Context, sql string) ([]domain.DailyRewardItem, error) {
	rows, err := s.pool.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("store: list daily reward items: %w", err)
	}
	defer rows.Close()
	var out []domain.DailyRewardItem
	for rows.Next() {
		d, err := scanDailyRewardItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// GetDailyRewardItem loads one offer by id. Returns ErrNotFound when absent.
func (s *Store) GetDailyRewardItem(ctx context.Context, id int64) (domain.DailyRewardItem, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+dailyRewardItemCols+` FROM daily_reward_item WHERE id = $1`, id)
	d, err := scanDailyRewardItem(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DailyRewardItem{}, ErrNotFound
	}
	if err != nil {
		return domain.DailyRewardItem{}, fmt.Errorf("store: get daily reward item %d: %w", id, err)
	}
	return d, nil
}

func scanDailyRewardItem(row scanRow) (domain.DailyRewardItem, error) {
	var d domain.DailyRewardItem
	err := row.Scan(&d.ID, &d.ItemIndex, &d.Eff1, &d.EffV1, &d.Eff2, &d.EffV2, &d.Eff3, &d.EffV3,
		&d.Title, &d.Description, &d.Enabled, &d.ExpiresDays)
	return d, err
}

// UpsertDailyRewardItem inserts a new offer (d.ID == 0) or updates the
// existing one by id, writing an audit row in the same transaction. Returns
// the offer id; updating a missing id returns ErrNotFound.
func (s *Store) UpsertDailyRewardItem(ctx context.Context, d domain.DailyRewardItem, moderatorID int64) (int64, error) {
	var id int64
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if d.ID == 0 {
			if err := tx.QueryRow(ctx, `
				INSERT INTO daily_reward_item
					(item_index, eff1, effv1, eff2, effv2, eff3, effv3, title, description, enabled, expires_days, updated_by, updated_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12, now())
				RETURNING id`,
				d.ItemIndex, d.Eff1, d.EffV1, d.Eff2, d.EffV2, d.Eff3, d.EffV3,
				d.Title, d.Description, d.Enabled, d.ExpiresDays, nullableID(moderatorID),
			).Scan(&id); err != nil {
				return fmt.Errorf("store: insert daily reward item: %w", err)
			}
			after, _ := fetchDailyRewardItemJSON(ctx, tx, id)
			return dailyRewardAudit(ctx, tx, &id, moderatorID, "create", nil, after)
		}

		before, _ := fetchDailyRewardItemJSON(ctx, tx, d.ID)
		if before == nil {
			return ErrNotFound
		}
		id = d.ID
		if _, err := tx.Exec(ctx, `
			UPDATE daily_reward_item SET
				item_index=$2, eff1=$3, effv1=$4, eff2=$5, effv2=$6, eff3=$7, effv3=$8,
				title=$9, description=$10, enabled=$11, expires_days=$12,
				updated_by=$13, updated_at=now()
			WHERE id=$1`,
			id, d.ItemIndex, d.Eff1, d.EffV1, d.Eff2, d.EffV2, d.Eff3, d.EffV3,
			d.Title, d.Description, d.Enabled, d.ExpiresDays, nullableID(moderatorID),
		); err != nil {
			return fmt.Errorf("store: update daily reward item %d: %w", id, err)
		}
		after, _ := fetchDailyRewardItemJSON(ctx, tx, id)
		return dailyRewardAudit(ctx, tx, &id, moderatorID, "update", before, after)
	})
	return id, err
}

// SetDailyRewardItemEnabled toggles whether an offer is claimable. Returns
// ErrNotFound if the offer does not exist.
func (s *Store) SetDailyRewardItemEnabled(ctx context.Context, id int64, enabled bool, moderatorID int64) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		before, _ := fetchDailyRewardItemJSON(ctx, tx, id)
		if before == nil {
			return ErrNotFound
		}
		if _, err := tx.Exec(ctx,
			`UPDATE daily_reward_item SET enabled=$2, updated_by=$3, updated_at=now() WHERE id=$1`,
			id, enabled, nullableID(moderatorID)); err != nil {
			return fmt.Errorf("store: set daily reward item %d enabled: %w", id, err)
		}
		after, _ := fetchDailyRewardItemJSON(ctx, tx, id)
		return dailyRewardAudit(ctx, tx, &id, moderatorID, "set_enabled", before, after)
	})
}

// DeleteDailyRewardItem removes an offer. Returns ErrNotFound if absent.
func (s *Store) DeleteDailyRewardItem(ctx context.Context, id int64, moderatorID int64) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		before, _ := fetchDailyRewardItemJSON(ctx, tx, id)
		if before == nil {
			return ErrNotFound
		}
		if _, err := tx.Exec(ctx, `DELETE FROM daily_reward_item WHERE id = $1`, id); err != nil {
			return fmt.Errorf("store: delete daily reward item %d: %w", id, err)
		}
		return dailyRewardAudit(ctx, tx, &id, moderatorID, "delete", before, nil)
	})
}

// DailyRewardClaimedToday reports whether the account has already claimed a
// daily reward for today's UTC calendar day, and if so which item (and its
// title, for the UI) they picked. claimed is false with zero/empty
// itemID/title when there is no claim yet today.
func (s *Store) DailyRewardClaimedToday(ctx context.Context, accountID int64) (claimed bool, itemID int64, title string, err error) {
	var rewardItemID *int64
	err = s.pool.QueryRow(ctx, `
		SELECT c.reward_item_id, COALESCE(i.title, '')
		FROM daily_reward_claim c
		LEFT JOIN daily_reward_item i ON i.id = c.reward_item_id
		WHERE c.account_id = $1 AND c.claim_date = (now() AT TIME ZONE 'UTC')::date`,
		accountID).Scan(&rewardItemID, &title)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, 0, "", nil
	}
	if err != nil {
		return false, 0, "", fmt.Errorf("store: daily reward claimed today a=%d: %w", accountID, err)
	}
	if rewardItemID != nil {
		itemID = *rewardItemID
	}
	return true, itemID, title, nil
}

// ClaimDailyReward redeems one reward for the account today: verifies the
// offer exists and is enabled, inserts the claim row (racing on the
// (account_id, claim_date) UNIQUE constraint rather than an explicit
// SELECT ... FOR UPDATE — see the doc comment below), and enqueues the item
// into delivery_queue, all in one transaction. Returns ErrNotFound (unknown
// offer or account), ErrShopItemDisabled (offer not enabled), or
// ErrAlreadyClaimedToday.
//
// Design note: donate's BuyDonateItem needs SELECT ... FOR UPDATE because it
// performs a genuine read-modify-write on a mutable balance (two concurrent
// buys must not both see the pre-debit balance). A daily claim has no mutable
// counter to race on — "has this account already claimed today" IS the
// question the UNIQUE(account_id, claim_date) index answers atomically.
// Catching the resulting unique_violation is therefore race-safe under
// concurrent double-submits without taking any row lock.
func (s *Store) ClaimDailyReward(ctx context.Context, accountID, rewardItemID int64) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		it, err := scanDailyRewardItem(tx.QueryRow(ctx,
			`SELECT `+dailyRewardItemCols+` FROM daily_reward_item WHERE id = $1`, rewardItemID))
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("store: claim: load offer %d: %w", rewardItemID, err)
		}
		if !it.Enabled {
			return ErrShopItemDisabled
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO daily_reward_claim (account_id, reward_item_id, claim_date)
			VALUES ($1, $2, (now() AT TIME ZONE 'UTC')::date)`,
			accountID, it.ID); err != nil {
			return classifyClaimInsertErr(err)
		}

		payload, err := json.Marshal(itemPayload{
			ItemIndex: it.ItemIndex,
			Eff1:      it.Eff1, EffV1: it.EffV1, Eff2: it.Eff2, EffV2: it.EffV2, Eff3: it.Eff3, EffV3: it.EffV3,
			ExpiresAt: expiresDaysToUnix(it.ExpiresDays),
		})
		if err != nil {
			return fmt.Errorf("store: claim: marshal payload: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO delivery_queue (account_id, kind, payload, source)
			VALUES ($1, 'item', $2, $3)`,
			accountID, payload, fmt.Sprintf("daily_reward:%d", it.ID)); err != nil {
			return fmt.Errorf("store: claim: enqueue delivery a=%d: %w", accountID, err)
		}
		after, _ := json.Marshal(map[string]any{
			"account_id": accountID, "reward_item_id": it.ID,
		})
		return dailyRewardAudit(ctx, tx, &it.ID, accountID, "claim", nil, after)
	})
}

// classifyClaimInsertErr maps the daily_reward_claim INSERT's constraint
// violations to sentinel errors: unique_violation (23505) means the account
// already claimed today; foreign_key_violation (23503) means the account_id
// does not exist.
func classifyClaimInsertErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return ErrAlreadyClaimedToday
		case "23503":
			return ErrNotFound
		}
	}
	return fmt.Errorf("store: insert daily reward claim: %w", err)
}

// --- helpers ---

func dailyRewardAudit(ctx context.Context, tx pgx.Tx, rewardItemID *int64, accountID int64, action string, before, after []byte) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO daily_reward_audit (reward_item_id, account_id, action, before, after)
		VALUES ($1,$2,$3,$4,$5)`,
		rewardItemID, accountID, action, nullableJSON(before), nullableJSON(after)); err != nil {
		return fmt.Errorf("store: write daily reward audit: %w", err)
	}
	return nil
}

func fetchDailyRewardItemJSON(ctx context.Context, tx pgx.Tx, id int64) ([]byte, error) {
	var js []byte
	err := tx.QueryRow(ctx, `SELECT to_jsonb(d) FROM daily_reward_item d WHERE id = $1`, id).Scan(&js)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return js, err
}
