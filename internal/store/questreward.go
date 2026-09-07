package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// QuestRewardVersion returns the monotonic version of the quest-trophy payouts.
func (s *Store) QuestRewardVersion(ctx context.Context) (int64, error) {
	var v int64
	err := s.pool.QueryRow(ctx, `SELECT version FROM quest_reward_meta WHERE id = TRUE`).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("store: quest reward version: %w", err)
	}
	return v, nil
}

// QuestRewards returns every edited trophy and the version it belongs to. An
// empty result is the normal state: the payouts then come from the content file
// as they always did.
func (s *Store) QuestRewards(ctx context.Context) (domain.QuestRewardConfig, error) {
	var cfg domain.QuestRewardConfig
	if err := s.inTx(ctx, func(tx pgx.Tx) error {
		// One transaction so the version and the rows describe the same
		// generation — a read straddling an edit would hand tmServer a version
		// it never actually ran.
		if err := tx.QueryRow(ctx,
			`SELECT version FROM quest_reward_meta WHERE id = TRUE`).Scan(&cfg.Version); err != nil &&
			!errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("store: quest reward version: %w", err)
		}
		rows, err := tx.Query(ctx, `
			SELECT tier, mortal_exp, arch_exp, coin, mortal_min, mortal_max, arch_min, arch_max
			  FROM quest_reward
			 ORDER BY tier`)
		if err != nil {
			return fmt.Errorf("store: quest rewards: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var q domain.QuestReward
			if err := rows.Scan(&q.Tier, &q.MortalExp, &q.ArchExp, &q.Coin,
				&q.MortalMin, &q.MortalMax, &q.ArchMin, &q.ArchMax); err != nil {
				return fmt.Errorf("store: scan quest reward: %w", err)
			}
			cfg.Tiers = append(cfg.Tiers, q)
		}
		return rows.Err()
	}); err != nil {
		return domain.QuestRewardConfig{}, err
	}
	return cfg, nil
}

// SetQuestReward writes one trophy and bumps the version, in one transaction. It
// returns the row as it stood before so the caller can put the pair in the
// panel's audit log; when there was no row, the second return is false and the
// caller says "vinha do conteúdo" rather than inventing a previous value it
// cannot know.
func (s *Store) SetQuestReward(ctx context.Context, q domain.QuestReward, moderatorID int64) (before domain.QuestReward, had bool, err error) {
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`SELECT version FROM quest_reward_meta WHERE id = TRUE FOR UPDATE`); err != nil {
			return fmt.Errorf("store: lock quest reward meta: %w", err)
		}
		before = domain.QuestReward{Tier: q.Tier}
		scanErr := tx.QueryRow(ctx, `
			SELECT tier, mortal_exp, arch_exp, coin, mortal_min, mortal_max, arch_min, arch_max
			  FROM quest_reward WHERE tier = $1`, q.Tier).
			Scan(&before.Tier, &before.MortalExp, &before.ArchExp, &before.Coin,
				&before.MortalMin, &before.MortalMax, &before.ArchMin, &before.ArchMax)
		switch {
		case errors.Is(scanErr, pgx.ErrNoRows):
			before, had = domain.QuestReward{Tier: q.Tier}, false
		case scanErr != nil:
			return fmt.Errorf("store: read quest reward: %w", scanErr)
		default:
			had = true
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO quest_reward
			    (tier, mortal_exp, arch_exp, coin, mortal_min, mortal_max, arch_min, arch_max,
			     updated_by, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
			ON CONFLICT (tier) DO UPDATE SET
				mortal_exp = EXCLUDED.mortal_exp,
				arch_exp   = EXCLUDED.arch_exp,
				coin       = EXCLUDED.coin,
				mortal_min = EXCLUDED.mortal_min,
				mortal_max = EXCLUDED.mortal_max,
				arch_min   = EXCLUDED.arch_min,
				arch_max   = EXCLUDED.arch_max,
				updated_by = EXCLUDED.updated_by,
				updated_at = now()`,
			q.Tier, q.MortalExp, q.ArchExp, q.Coin,
			q.MortalMin, q.MortalMax, q.ArchMin, q.ArchMax, nullableID(moderatorID)); err != nil {
			return fmt.Errorf("store: upsert quest reward: %w", err)
		}
		return bumpQuestRewardVersion(ctx, tx)
	})
	return before, had, err
}

// DeleteQuestReward drops one trophy's override, so it goes back to the content
// file. It reports what was removed, and whether there was anything to remove.
func (s *Store) DeleteQuestReward(ctx context.Context, tier int32, moderatorID int64) (before domain.QuestReward, had bool, err error) {
	_ = moderatorID // the audit entry is the panel's; the row is simply gone.
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`SELECT version FROM quest_reward_meta WHERE id = TRUE FOR UPDATE`); err != nil {
			return fmt.Errorf("store: lock quest reward meta: %w", err)
		}
		scanErr := tx.QueryRow(ctx, `
			SELECT tier, mortal_exp, arch_exp, coin, mortal_min, mortal_max, arch_min, arch_max
			  FROM quest_reward WHERE tier = $1`, tier).
			Scan(&before.Tier, &before.MortalExp, &before.ArchExp, &before.Coin,
				&before.MortalMin, &before.MortalMax, &before.ArchMin, &before.ArchMax)
		switch {
		case errors.Is(scanErr, pgx.ErrNoRows):
			before, had = domain.QuestReward{Tier: tier}, false
		case scanErr != nil:
			return fmt.Errorf("store: read quest reward: %w", scanErr)
		default:
			had = true
		}
		if _, err := tx.Exec(ctx, `DELETE FROM quest_reward WHERE tier = $1`, tier); err != nil {
			return fmt.Errorf("store: delete quest reward: %w", err)
		}
		return bumpQuestRewardVersion(ctx, tx)
	})
	return before, had, err
}

func bumpQuestRewardVersion(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx,
		`UPDATE quest_reward_meta SET version = version + 1 WHERE id = TRUE`); err != nil {
		return fmt.Errorf("store: bump quest reward version: %w", err)
	}
	return nil
}
