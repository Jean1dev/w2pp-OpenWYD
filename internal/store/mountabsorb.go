package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// Mount absorption persistence (0035_mount_absorb), sibling of mountrate.go.
// Postgres owns how much of a hit an adult mount eats instead of its owner, one
// number against players and one against monsters; the tmServer only ever reads
// it (via dbServer), once at boot.
//
// Mutations reuse npc_audit/npc_config_meta through auditAndBump, like every
// other overlay here — one moderation trail beats a parallel one nobody would
// think to read.

// ListMountAbsorb returns every configured lineage, in index order.
func (s *Store) ListMountAbsorb(ctx context.Context) ([]domain.MountAbsorb, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT mount_index, absorb_pvp, absorb_pve FROM mount_absorb ORDER BY mount_index`)
	if err != nil {
		return nil, fmt.Errorf("store: list mount absorb: %w", err)
	}
	defer rows.Close()

	var out []domain.MountAbsorb
	for rows.Next() {
		var a domain.MountAbsorb
		if err := rows.Scan(&a.MountIndex, &a.PvP, &a.PvE); err != nil {
			return nil, fmt.Errorf("store: scan mount absorb: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate mount absorb: %w", err)
	}
	return out, nil
}

// SetMountAbsorb writes both numbers of one lineage at once.
//
// Both together and not one at a time: they are the two halves of a single
// decision ("this mount is for PvE"), and a save that landed one and lost the
// other would leave a lineage nobody designed — strong against monsters and
// accidentally strong against people too.
func (s *Store) SetMountAbsorb(ctx context.Context, mountIndex, pvp, pve int16, moderatorID int64, moderator string) error {
	if mountIndex < domain.MountAdultLo || mountIndex > domain.MountAdultHi {
		return fmt.Errorf("store: %d is not an adult mount", mountIndex)
	}
	for _, v := range [...]int16{pvp, pve} {
		if v < 0 || v > 100 {
			return fmt.Errorf("store: absorb %d is out of 0..100", v)
		}
	}

	return s.inTx(ctx, func(tx pgx.Tx) error {
		before, err := fetchMountAbsorbJSON(ctx, tx, mountIndex)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO mount_absorb (mount_index, absorb_pvp, absorb_pve, updated_by)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (mount_index)
			 DO UPDATE SET absorb_pvp = EXCLUDED.absorb_pvp, absorb_pve = EXCLUDED.absorb_pve,
			               updated_by = EXCLUDED.updated_by, updated_at = now()`,
			mountIndex, pvp, pve, moderator); err != nil {
			return fmt.Errorf("store: upsert mount absorb %d: %w", mountIndex, err)
		}
		after, err := fetchMountAbsorbJSON(ctx, tx, mountIndex)
		if err != nil {
			return err
		}
		return auditAndBump(ctx, tx, nil, moderatorID, "set_mount_absorb", before, after)
	})
}

// ClearMountAbsorb drops the lineage's row, which returns it to the compiled
// default — absence is what "use the default" means everywhere in this overlay,
// so restoring is a delete and not a write of 25/25. Writing the default would
// be a configuration, and it would stop following the default if it ever moved.
func (s *Store) ClearMountAbsorb(ctx context.Context, mountIndex int16, moderatorID int64) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		before, err := fetchMountAbsorbJSON(ctx, tx, mountIndex)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM mount_absorb WHERE mount_index = $1`, mountIndex); err != nil {
			return fmt.Errorf("store: clear mount absorb %d: %w", mountIndex, err)
		}
		after, err := fetchMountAbsorbJSON(ctx, tx, mountIndex)
		if err != nil {
			return err
		}
		return auditAndBump(ctx, tx, nil, moderatorID, "clear_mount_absorb", before, after)
	})
}

// fetchMountAbsorbJSON snapshots a lineage for the audit trail. It answers
// 'null' for a lineage with no row, which is what makes "voltou ao padrão"
// readable in the log as something other than a write of two numbers.
func fetchMountAbsorbJSON(ctx context.Context, tx pgx.Tx, mountIndex int16) ([]byte, error) {
	var js []byte
	err := tx.QueryRow(ctx,
		`SELECT coalesce(
		     (SELECT jsonb_build_object('pvp', absorb_pvp, 'pve', absorb_pve)
		        FROM mount_absorb WHERE mount_index = $1),
		     'null'::jsonb)`, mountIndex).Scan(&js)
	if err != nil {
		return nil, fmt.Errorf("store: snapshot mount absorb %d: %w", mountIndex, err)
	}
	return js, nil
}

// MountConfigVersion is when the mount overlay last changed, as unix seconds —
// the most recent updated_at across ALL the mount tables (curve, absorption,
// attributes), or 0 when none has a row.
//
// A timestamp rather than a counter, and one number for all the tables, because of
// what the panel does with it. The tmServer reads these tables once at boot and
// reports back the number it read; the panel compares. Equal means the screen is
// showing what players are getting. Different means a save is sitting there
// waiting for a restart — and those two states are otherwise indistinguishable
// from the screen, which is exactly how someone spends four hundred âmagos
// testing a curve the server never loaded.
//
// The tables share one number because they share one restart: nobody restarts
// the game for the curve and not for the absorption.
func (s *Store) MountConfigVersion(ctx context.Context) (int64, error) {
	var v int64
	err := s.pool.QueryRow(ctx, `
		SELECT coalesce(extract(epoch FROM max(quando))::bigint, 0) FROM (
		    SELECT max(updated_at) AS quando FROM mount_growth_rate
		    UNION ALL
		    SELECT max(updated_at) AS quando FROM mount_absorb
		    UNION ALL
		    SELECT max(updated_at) AS quando FROM mount_bonus
		) AS t`).Scan(&v)
	if err != nil {
		return 0, fmt.Errorf("store: mount config version: %w", err)
	}
	return v, nil
}
