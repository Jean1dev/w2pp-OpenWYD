package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// NPC editing persistence (npc-editing-plan.md). Postgres owns the NPC
// *definition*; the tmServer only ever reads it (via dbServer). Every mutation
// runs in a transaction that also writes an audit row and bumps
// npc_config_meta.version so the tmServer's cheap version poll detects the
// change.

// NPCConfigVersion returns the monotonically increasing config version. The
// tmServer polls this to decide whether to reload the full definition set.
func (s *Store) NPCConfigVersion(ctx context.Context) (int64, error) {
	var v int64
	err := s.pool.QueryRow(ctx, `SELECT version FROM npc_config_meta WHERE id = TRUE`).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil // meta row not seeded yet (pre-0005 or empty) — treat as version 0
	}
	if err != nil {
		return 0, fmt.Errorf("store: npc config version: %w", err)
	}
	return v, nil
}

// ListNPCDefinitions returns every NPC definition with its shop items, ordered by
// id. This is the full snapshot the tmServer materializes into live entities.
func (s *Store) ListNPCDefinitions(ctx context.Context) ([]domain.NPCDefinition, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, slug, template_name, display_name, enabled, map_id, pos_x, pos_y, route_type, merchant,
		       origin, COALESCE(generator_index,-1), follower_template, minute_generate,
		       min_group, max_group, max_num_mob, formation, generator_data
		FROM npc_definition ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("store: list npc definitions: %w", err)
	}
	defer rows.Close()

	byID := make(map[int64]int) // id → index in out (to attach shop items)
	var out []domain.NPCDefinition
	for rows.Next() {
		var d domain.NPCDefinition
		var generatorData []byte
		if err := rows.Scan(&d.ID, &d.Slug, &d.TemplateName, &d.DisplayName, &d.Enabled,
			&d.MapID, &d.PosX, &d.PosY, &d.RouteType, &d.Merchant, &d.Origin,
			&d.GeneratorIndex, &d.FollowerTemplate, &d.MinuteGenerate, &d.MinGroup,
			&d.MaxGroup, &d.MaxNumMob, &d.Formation, &generatorData); err != nil {
			return nil, fmt.Errorf("store: scan npc definition: %w", err)
		}
		if len(generatorData) > 0 {
			var gd generatorJSON
			if err := json.Unmarshal(generatorData, &gd); err != nil {
				return nil, fmt.Errorf("store: decode generator %q: %w", d.Slug, err)
			}
			d.SegX, d.SegY, d.SegRange, d.SegWait = gd.SegX, gd.SegY, gd.SegRange, gd.SegWait
			d.FightAction, d.DieAction = gd.FightAction, gd.DieAction
		}
		byID[d.ID] = len(out)
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate npc definitions: %w", err)
	}

	shopRows, err := s.pool.Query(ctx, `
		SELECT npc_id, slot, item_index, quantity, eff1, effv1, eff2, effv2, eff3, effv3
		FROM npc_shop_item ORDER BY npc_id, slot`)
	if err != nil {
		return nil, fmt.Errorf("store: list npc shop items: %w", err)
	}
	defer shopRows.Close()
	for shopRows.Next() {
		var npcID int64
		var it domain.NPCShopItem
		if err := shopRows.Scan(&npcID, &it.Slot, &it.ItemIndex, &it.Quantity,
			&it.Eff1, &it.EffV1, &it.Eff2, &it.EffV2, &it.Eff3, &it.EffV3); err != nil {
			return nil, fmt.Errorf("store: scan npc shop item: %w", err)
		}
		normalizeShopQuantity(&it)
		if idx, ok := byID[npcID]; ok {
			out[idx].Shop = append(out[idx].Shop, it)
		}
	}
	if err := shopRows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate npc shop items: %w", err)
	}
	return out, nil
}

// GetNPCDefinition loads one definition (with shop items) by id. Returns
// ErrNotFound when absent.
func (s *Store) GetNPCDefinition(ctx context.Context, id int64) (domain.NPCDefinition, error) {
	var d domain.NPCDefinition
	err := s.pool.QueryRow(ctx, `
		SELECT id, slug, template_name, display_name, enabled, map_id, pos_x, pos_y, route_type, merchant,
		       origin, COALESCE(generator_index,-1)
		FROM npc_definition WHERE id = $1`, id).
		Scan(&d.ID, &d.Slug, &d.TemplateName, &d.DisplayName, &d.Enabled,
			&d.MapID, &d.PosX, &d.PosY, &d.RouteType, &d.Merchant, &d.Origin, &d.GeneratorIndex)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NPCDefinition{}, ErrNotFound
	}
	if err != nil {
		return domain.NPCDefinition{}, fmt.Errorf("store: get npc definition %d: %w", id, err)
	}
	shop, err := s.loadShopItems(ctx, s.pool, id)
	if err != nil {
		return domain.NPCDefinition{}, err
	}
	d.Shop = shop
	return d, nil
}

// ItemPriceOverrides returns every global price override. The tmServer merges
// these over its content-catalog base prices.
func (s *Store) ItemPriceOverrides(ctx context.Context) ([]domain.ItemPriceOverride, error) {
	rows, err := s.pool.Query(ctx, `SELECT item_index, price FROM item_price ORDER BY item_index`)
	if err != nil {
		return nil, fmt.Errorf("store: list item prices: %w", err)
	}
	defer rows.Close()
	var out []domain.ItemPriceOverride
	for rows.Next() {
		var p domain.ItemPriceOverride
		if err := rows.Scan(&p.ItemIndex, &p.Price); err != nil {
			return nil, fmt.Errorf("store: scan item price: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// AccountRole returns the account's role ('player'/'moderator'/'admin'). The
// web-api uses it to authorize NPC-admin RPCs. Returns ErrNotFound if absent.
func (s *Store) AccountRole(ctx context.Context, id int64) (string, error) {
	var role string
	err := s.pool.QueryRow(ctx, `SELECT role FROM account WHERE id = $1`, id).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("store: account role %d: %w", id, err)
	}
	return role, nil
}

// UpsertNPCDefinition inserts a new definition or updates the existing one with
// the same slug (position, visibility, template, merchant type). Shop items are
// managed separately via SetNPCShop. Returns the definition id. Bumps the config
// version and writes an audit row, all in one transaction.
func (s *Store) UpsertNPCDefinition(ctx context.Context, d domain.NPCDefinition, moderatorID int64) (int64, error) {
	var id int64
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		before, _ := fetchDefJSON(ctx, tx, d.Slug)
		qerr := tx.QueryRow(ctx, `
			INSERT INTO npc_definition
				(slug, template_name, display_name, enabled, map_id, pos_x, pos_y, route_type, merchant, updated_by, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10, now())
			ON CONFLICT (slug) DO UPDATE SET
				template_name = EXCLUDED.template_name,
				display_name  = EXCLUDED.display_name,
				enabled       = EXCLUDED.enabled,
				map_id        = EXCLUDED.map_id,
				pos_x         = EXCLUDED.pos_x,
				pos_y         = EXCLUDED.pos_y,
				route_type    = EXCLUDED.route_type,
				merchant      = EXCLUDED.merchant,
				updated_by    = EXCLUDED.updated_by,
				updated_at    = now()
			RETURNING id`,
			d.Slug, d.TemplateName, d.DisplayName, d.Enabled, d.MapID, d.PosX, d.PosY,
			d.RouteType, d.Merchant, nullableID(moderatorID),
		).Scan(&id)
		if qerr != nil {
			return fmt.Errorf("store: upsert npc definition %q: %w", d.Slug, qerr)
		}
		action := "update"
		if before == nil {
			action = "create"
		}
		after, _ := fetchDefJSONByID(ctx, tx, id)
		if err := auditAndBump(ctx, tx, &id, moderatorID, action, before, after); err != nil {
			return err
		}
		return nil
	})
	return id, err
}

// SetNPCShop replaces a merchant NPC's shop stock. Returns ErrNotFound if the
// definition does not exist.
func (s *Store) SetNPCShop(ctx context.Context, npcID int64, items []domain.NPCShopItem, moderatorID int64) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := ensureDefExists(ctx, tx, npcID); err != nil {
			return err
		}
		before, _ := fetchShopJSON(ctx, tx, npcID)
		if _, err := tx.Exec(ctx, `DELETE FROM npc_shop_item WHERE npc_id = $1`, npcID); err != nil {
			return fmt.Errorf("store: clear npc shop %d: %w", npcID, err)
		}
		for _, it := range items {
			normalizeShopQuantity(&it)
			if _, err := tx.Exec(ctx, `
				INSERT INTO npc_shop_item (npc_id, slot, item_index, quantity, eff1, effv1, eff2, effv2, eff3, effv3)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
				npcID, it.Slot, it.ItemIndex, normalizedQuantity(it.Quantity),
				it.Eff1, it.EffV1, it.Eff2, it.EffV2, it.Eff3, it.EffV3,
			); err != nil {
				return fmt.Errorf("store: insert npc shop item (npc %d slot %d): %w", npcID, it.Slot, err)
			}
		}
		after, _ := fetchShopJSON(ctx, tx, npcID)
		return auditAndBump(ctx, tx, &npcID, moderatorID, "set_shop", before, after)
	})
}

// SetNPCVisibility toggles whether an NPC appears in the world. Returns
// ErrNotFound if the definition does not exist.
func (s *Store) SetNPCVisibility(ctx context.Context, npcID int64, enabled bool, moderatorID int64) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := ensureDefExists(ctx, tx, npcID); err != nil {
			return err
		}
		before, _ := fetchDefJSONByID(ctx, tx, npcID)
		if _, err := tx.Exec(ctx,
			`UPDATE npc_definition SET enabled = $2, updated_by = $3, updated_at = now() WHERE id = $1`,
			npcID, enabled, nullableID(moderatorID)); err != nil {
			return fmt.Errorf("store: set npc visibility %d: %w", npcID, err)
		}
		after, _ := fetchDefJSONByID(ctx, tx, npcID)
		return auditAndBump(ctx, tx, &npcID, moderatorID, "set_visibility", before, after)
	})
}

// SetItemPrice sets (or clears, when price < 0) the global override price for an
// item index.
func (s *Store) SetItemPrice(ctx context.Context, itemIndex int32, price int64, moderatorID int64) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if price < 0 {
			if _, err := tx.Exec(ctx, `DELETE FROM item_price WHERE item_index = $1`, itemIndex); err != nil {
				return fmt.Errorf("store: clear item price %d: %w", itemIndex, err)
			}
		} else if _, err := tx.Exec(ctx, `
			INSERT INTO item_price (item_index, price, updated_by, updated_at)
			VALUES ($1,$2,$3, now())
			ON CONFLICT (item_index) DO UPDATE SET price = EXCLUDED.price, updated_by = EXCLUDED.updated_by, updated_at = now()`,
			itemIndex, price, nullableID(moderatorID)); err != nil {
			return fmt.Errorf("store: set item price %d: %w", itemIndex, err)
		}
		after, _ := json.Marshal(domain.ItemPriceOverride{ItemIndex: itemIndex, Price: price})
		return auditAndBump(ctx, tx, nil, moderatorID, "set_price", nil, after)
	})
}

// DeleteNPCDefinition removes a definition (its shop items cascade). Returns
// ErrNotFound if absent.
func (s *Store) DeleteNPCDefinition(ctx context.Context, npcID int64, moderatorID int64) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		var origin string
		err := tx.QueryRow(ctx, `SELECT origin FROM npc_definition WHERE id = $1`, npcID).Scan(&origin)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("store: read npc definition origin: %w", err)
		}
		if origin == "content" {
			return ErrContentOwned
		}
		before, err := fetchDefJSONByID(ctx, tx, npcID)
		if err != nil {
			return err
		}
		if before == nil {
			return ErrNotFound
		}
		if _, err := tx.Exec(ctx, `DELETE FROM npc_definition WHERE id = $1`, npcID); err != nil {
			return fmt.Errorf("store: delete npc definition %d: %w", npcID, err)
		}
		return auditAndBump(ctx, tx, &npcID, moderatorID, "delete", before, nil)
	})
}

// SeedNPCDefinitions bulk-imports definitions (with shop items) in one
// transaction, skipping any slug that already exists (idempotent — safe to
// re-run). It bumps the config version once at the end and writes no per-row
// audit (this is a system import, not moderator activity). Returns the number of
// definitions actually inserted.
func (s *Store) SeedNPCDefinitions(ctx context.Context, defs []domain.NPCDefinition) (int, error) {
	inserted := 0
	indices := make(map[int32]string, len(defs))
	for _, d := range defs {
		if previous, exists := indices[d.GeneratorIndex]; exists {
			return 0, fmt.Errorf("store: duplicate content generator index %d for %q and %q", d.GeneratorIndex, previous, d.Slug)
		}
		indices[d.GeneratorIndex] = d.Slug
	}
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		for _, d := range defs {
			gd, marshalErr := json.Marshal(generatorJSON{
				SegX: d.SegX, SegY: d.SegY, SegRange: d.SegRange, SegWait: d.SegWait,
				FightAction: d.FightAction, DieAction: d.DieAction,
			})
			if marshalErr != nil {
				return fmt.Errorf("store: encode generator %q: %w", d.Slug, marshalErr)
			}
			var id int64
			var created bool
			err := tx.QueryRow(ctx, `
				INSERT INTO npc_definition
					(slug, template_name, display_name, enabled, map_id, pos_x, pos_y, route_type, merchant,
					 origin, generator_index, follower_template, minute_generate, min_group, max_group, max_num_mob, formation, generator_data)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'content',$10,$11,$12,$13,$14,$15,$16,$17)
				ON CONFLICT (slug) DO UPDATE SET origin='content', generator_index=EXCLUDED.generator_index,
				 follower_template=EXCLUDED.follower_template, minute_generate=EXCLUDED.minute_generate,
				 min_group=EXCLUDED.min_group, max_group=EXCLUDED.max_group,
				 max_num_mob=EXCLUDED.max_num_mob, formation=EXCLUDED.formation, generator_data=EXCLUDED.generator_data
				RETURNING id, (xmax = 0)`,
				d.Slug, d.TemplateName, d.DisplayName, d.Enabled, d.MapID, d.PosX, d.PosY, d.RouteType, d.Merchant,
				d.GeneratorIndex, d.FollowerTemplate, d.MinuteGenerate, d.MinGroup, d.MaxGroup, d.MaxNumMob, d.Formation, gd,
			).Scan(&id, &created)
			if err != nil {
				return fmt.Errorf("store: seed npc definition %q: %w", d.Slug, err)
			}
			for _, it := range d.Shop {
				normalizeShopQuantity(&it)
				if _, err := tx.Exec(ctx, `
					INSERT INTO npc_shop_item (npc_id, slot, item_index, quantity, eff1, effv1, eff2, effv2, eff3, effv3)
					VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
					ON CONFLICT (npc_id, slot) DO NOTHING`,
					id, it.Slot, it.ItemIndex, normalizedQuantity(it.Quantity),
					it.Eff1, it.EffV1, it.Eff2, it.EffV2, it.Eff3, it.EffV3,
				); err != nil {
					return fmt.Errorf("store: seed shop item (%q slot %d): %w", d.Slug, it.Slot, err)
				}
			}
			if created {
				inserted++
			}
		}
		if len(defs) > 0 {
			if _, err := tx.Exec(ctx, `UPDATE npc_config_meta SET version = version + 1 WHERE id = TRUE`); err != nil {
				return fmt.Errorf("store: bump npc config version: %w", err)
			}
		}
		return nil
	})
	return inserted, err
}

type generatorJSON struct {
	SegX        [5]int32  `json:"seg_x"`
	SegY        [5]int32  `json:"seg_y"`
	SegRange    [5]int32  `json:"seg_range"`
	SegWait     [5]int32  `json:"seg_wait"`
	FightAction [4]string `json:"fight_action"`
	DieAction   [4]string `json:"die_action"`
}

// --- helpers ---

// inTx runs fn in a transaction, rolling back on error.
func (s *Store) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) loadShopItems(ctx context.Context, q pgxQuerier, npcID int64) ([]domain.NPCShopItem, error) {
	rows, err := q.Query(ctx, `
		SELECT slot, item_index, quantity, eff1, effv1, eff2, effv2, eff3, effv3
		FROM npc_shop_item WHERE npc_id = $1 ORDER BY slot`, npcID)
	if err != nil {
		return nil, fmt.Errorf("store: load shop items %d: %w", npcID, err)
	}
	defer rows.Close()
	var out []domain.NPCShopItem
	for rows.Next() {
		var it domain.NPCShopItem
		if err := rows.Scan(&it.Slot, &it.ItemIndex, &it.Quantity, &it.Eff1, &it.EffV1, &it.Eff2, &it.EffV2, &it.Eff3, &it.EffV3); err != nil {
			return nil, fmt.Errorf("store: scan shop item: %w", err)
		}
		normalizeShopQuantity(&it)
		out = append(out, it)
	}
	return out, rows.Err()
}

const efAmount = 61

func normalizedQuantity(q int16) int16 {
	if q <= 0 {
		return 1
	}
	return q
}

func normalizeShopQuantity(it *domain.NPCShopItem) {
	if it.Quantity <= 0 {
		it.Quantity = 1
	}
	if it.Eff1 == efAmount {
		if it.EffV1 > 0 {
			it.Quantity = int16(it.EffV1)
		}
		it.Eff1, it.EffV1 = 0, 0
	}
	if it.Eff2 == efAmount {
		if it.EffV2 > 0 {
			it.Quantity = int16(it.EffV2)
		}
		it.Eff2, it.EffV2 = 0, 0
	}
	if it.Eff3 == efAmount {
		if it.EffV3 > 0 {
			it.Quantity = int16(it.EffV3)
		}
		it.Eff3, it.EffV3 = 0, 0
	}
}

// pgxQuerier is the read surface shared by *pgxpool.Pool and pgx.Tx.
type pgxQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// auditAndBump writes the audit row and increments the config version. Called at
// the end of every mutating transaction.
func auditAndBump(ctx context.Context, tx pgx.Tx, npcID *int64, moderatorID int64, action string, before, after []byte) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO npc_audit (npc_id, account_id, action, before, after)
		VALUES ($1,$2,$3,$4,$5)`,
		npcID, moderatorID, action, nullableJSON(before), nullableJSON(after)); err != nil {
		return fmt.Errorf("store: write npc audit: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE npc_config_meta SET version = version + 1 WHERE id = TRUE`); err != nil {
		return fmt.Errorf("store: bump npc config version: %w", err)
	}
	return nil
}

func ensureDefExists(ctx context.Context, tx pgx.Tx, npcID int64) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM npc_definition WHERE id = $1)`, npcID).Scan(&exists); err != nil {
		return fmt.Errorf("store: check npc definition %d: %w", npcID, err)
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}

// fetchDefJSON returns the definition row as JSON keyed by slug (nil if absent),
// used to snapshot the "before" state for the audit trail.
func fetchDefJSON(ctx context.Context, tx pgx.Tx, slug string) ([]byte, error) {
	var js []byte
	err := tx.QueryRow(ctx, `SELECT to_jsonb(d) FROM npc_definition d WHERE slug = $1`, slug).Scan(&js)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return js, err
}

func fetchDefJSONByID(ctx context.Context, tx pgx.Tx, id int64) ([]byte, error) {
	var js []byte
	err := tx.QueryRow(ctx, `SELECT to_jsonb(d) FROM npc_definition d WHERE id = $1`, id).Scan(&js)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return js, err
}

func fetchShopJSON(ctx context.Context, tx pgx.Tx, npcID int64) ([]byte, error) {
	var js []byte
	err := tx.QueryRow(ctx,
		`SELECT coalesce(jsonb_agg(to_jsonb(si) ORDER BY si.slot), '[]'::jsonb)
		 FROM npc_shop_item si WHERE npc_id = $1`, npcID).Scan(&js)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return js, err
}

// nullableID renders a zero moderator id as SQL NULL (system/seed writes).
func nullableID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

// nullableJSON renders empty JSON bytes as SQL NULL.
func nullableJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
