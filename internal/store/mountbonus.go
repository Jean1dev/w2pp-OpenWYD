package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/mountbonus"
)

// Mount attribute persistence (0043_mount_bonus), sibling of mountabsorb.go.
// Postgres owns what the panel changed on top of the compiled table; the
// tmServer only ever reads it (via dbServer), once at boot.

// ListMountBonus returns every configured lineage, in index order.
func (s *Store) ListMountBonus(ctx context.Context) ([]domain.MountBonus, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT mount_index, attack, magic, evasion, resist FROM mount_bonus ORDER BY mount_index`)
	if err != nil {
		return nil, fmt.Errorf("store: list mount bonus: %w", err)
	}
	defer rows.Close()

	var out []domain.MountBonus
	for rows.Next() {
		var b domain.MountBonus
		if err := rows.Scan(&b.MountIndex, &b.Attack, &b.Magic, &b.Evasion, &b.Resist); err != nil {
			return nil, fmt.Errorf("store: scan mount bonus: %w", err)
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate mount bonus: %w", err)
	}
	return out, nil
}

// SetMountBonus writes all four numbers of one lineage at once — they are one
// decision about what the mount is, like the absorption pair.
func (s *Store) SetMountBonus(ctx context.Context, b domain.MountBonus, moderatorID int64, moderator string) error {
	if !mountbonus.IsAdult(b.MountIndex) {
		return fmt.Errorf("store: %d is not an adult mount", b.MountIndex)
	}
	if !(mountbonus.Bonus{Attack: b.Attack, Magic: b.Magic, Evasion: b.Evasion, Resist: b.Resist}).Valid() {
		return fmt.Errorf("store: mount bonus %+v is out of range", b)
	}

	return s.inTx(ctx, func(tx pgx.Tx) error {
		before, err := fetchMountBonusJSON(ctx, tx, b.MountIndex)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO mount_bonus (mount_index, attack, magic, evasion, resist, updated_by)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (mount_index)
			 DO UPDATE SET attack = EXCLUDED.attack, magic = EXCLUDED.magic,
			               evasion = EXCLUDED.evasion, resist = EXCLUDED.resist,
			               updated_by = EXCLUDED.updated_by, updated_at = now()`,
			b.MountIndex, b.Attack, b.Magic, b.Evasion, b.Resist, moderator); err != nil {
			return fmt.Errorf("store: upsert mount bonus %d: %w", b.MountIndex, err)
		}
		after, err := fetchMountBonusJSON(ctx, tx, b.MountIndex)
		if err != nil {
			return err
		}
		return auditAndBump(ctx, tx, nil, moderatorID, "set_mount_bonus", before, after)
	})
}

// ClearMountBonus drops the lineage's row, returning it to the compiled table.
// A delete and not a write of the default values, for the reason every overlay
// here gives: a written default stops following the default if it ever moves.
func (s *Store) ClearMountBonus(ctx context.Context, mountIndex int16, moderatorID int64) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		before, err := fetchMountBonusJSON(ctx, tx, mountIndex)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM mount_bonus WHERE mount_index = $1`, mountIndex); err != nil {
			return fmt.Errorf("store: clear mount bonus %d: %w", mountIndex, err)
		}
		after, err := fetchMountBonusJSON(ctx, tx, mountIndex)
		if err != nil {
			return err
		}
		return auditAndBump(ctx, tx, nil, moderatorID, "clear_mount_bonus", before, after)
	})
}

// fetchMountBonusJSON snapshots a lineage for the audit trail, 'null' when it
// has no row.
func fetchMountBonusJSON(ctx context.Context, tx pgx.Tx, mountIndex int16) ([]byte, error) {
	var js []byte
	err := tx.QueryRow(ctx,
		`SELECT coalesce(
		     (SELECT jsonb_build_object('attack', attack, 'magic', magic,
		                                'evasion', evasion, 'resist', resist)
		        FROM mount_bonus WHERE mount_index = $1),
		     'null'::jsonb)`, mountIndex).Scan(&js)
	if err != nil {
		return nil, fmt.Errorf("store: snapshot mount bonus %d: %w", mountIndex, err)
	}
	return js, nil
}
