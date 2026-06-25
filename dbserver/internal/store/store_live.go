package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/domain"
)

// ErrNotFound is returned when a queried account or character does not exist.
var ErrNotFound = errors.New("store: not found")

// AccountAuth is the minimum account data needed to authenticate a login: the
// id, the stored argon2id password hash and the blocked flag. The caller
// verifies the password (store never sees plaintext beyond the hash).
type AccountAuth struct {
	ID        int64
	PassHash  string
	IsBlocked bool
}

// AccountByName fetches the auth row for a canonical (lowercase) account name.
// Returns ErrNotFound when no such account exists.
func (s *Store) AccountByName(ctx context.Context, name string) (AccountAuth, error) {
	var a AccountAuth
	err := s.pool.QueryRow(ctx,
		`SELECT id, pass_hash, is_blocked FROM account WHERE name = $1`, name).
		Scan(&a.ID, &a.PassHash, &a.IsBlocked)
	if errors.Is(err, pgx.ErrNoRows) {
		return AccountAuth{}, ErrNotFound
	}
	if err != nil {
		return AccountAuth{}, fmt.Errorf("store: account by name %q: %w", name, err)
	}
	return a, nil
}

// AccountAuthByID fetches the auth row for an account id (used to confirm a
// password before a destructive operation). Returns ErrNotFound if absent.
func (s *Store) AccountAuthByID(ctx context.Context, id int64) (AccountAuth, error) {
	a := AccountAuth{ID: id}
	err := s.pool.QueryRow(ctx,
		`SELECT pass_hash, is_blocked FROM account WHERE id = $1`, id).
		Scan(&a.PassHash, &a.IsBlocked)
	if errors.Is(err, pgx.ErrNoRows) {
		return AccountAuth{}, ErrNotFound
	}
	if err != nil {
		return AccountAuth{}, fmt.Errorf("store: account by id %d: %w", id, err)
	}
	return a, nil
}

// ListCharacters returns the character-selection projection (slot, name, class,
// level, exp, guild) for an account, ordered by slot.
func (s *Store) ListCharacters(ctx context.Context, accountID int64) ([]domain.Character, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT slot, name, class, guild_id, level, exp, coin,
		        max_hp, hp, max_mp, mp, str, int, dex, con
		   FROM character WHERE account_id = $1 ORDER BY slot`, accountID)
	if err != nil {
		return nil, fmt.Errorf("store: list characters: %w", err)
	}
	defer rows.Close()

	var out []domain.Character
	for rows.Next() {
		var ch domain.Character
		if err := rows.Scan(&ch.Slot, &ch.Name, &ch.Class, &ch.GuildID, &ch.Level, &ch.Exp, &ch.Coin,
			&ch.MaxHp, &ch.Hp, &ch.MaxMp, &ch.Mp, &ch.Str, &ch.Int, &ch.Dex, &ch.Con); err != nil {
			return nil, fmt.Errorf("store: scan character summary: %w", err)
		}
		out = append(out, ch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list characters: %w", err)
	}
	return out, nil
}

// LoadCharacter loads one character's full state (stats + equip + carry +
// affects) by account and slot. Returns ErrNotFound when the slot is empty.
func (s *Store) LoadCharacter(ctx context.Context, accountID int64, slot int) (domain.Character, error) {
	var ch domain.Character
	var charID int64
	var skillBar, shortSkill []int16
	err := s.pool.QueryRow(ctx, `
		SELECT id, slot, name, class, clan, guild_id, guild_level, level, exp, coin,
		       str, int, dex, con, score_bonus, special_bonus, skill_bonus,
		       max_hp, max_mp, hp, mp, critical, regen_hp, regen_mp,
		       resist_fire, resist_ice, resist_thunder, resist_magic,
		       learned_skill, magic, save_x, save_y, last_city, citizen, class_master,
		       skill_bar, short_skill
		  FROM character WHERE account_id = $1 AND slot = $2`, accountID, slot).
		Scan(&charID, &ch.Slot, &ch.Name, &ch.Class, &ch.Clan, &ch.GuildID, &ch.GuildLevel,
			&ch.Level, &ch.Exp, &ch.Coin, &ch.Str, &ch.Int, &ch.Dex, &ch.Con,
			&ch.ScoreBonus, &ch.SpecialBonus, &ch.SkillBonus, &ch.MaxHp, &ch.MaxMp, &ch.Hp, &ch.Mp,
			&ch.Critical, &ch.RegenHP, &ch.RegenMP, &ch.ResistFire, &ch.ResistIce, &ch.ResistThunder,
			&ch.ResistMagic, &ch.LearnedSkill, &ch.Magic, &ch.SaveX, &ch.SaveY, &ch.LastCity, &ch.Citizen,
			&ch.ClassMaster, &skillBar, &shortSkill)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Character{}, ErrNotFound
	}
	if err != nil {
		return domain.Character{}, fmt.Errorf("store: load character a=%d slot=%d: %w", accountID, slot, err)
	}
	int16ArrToByteArr(skillBar, ch.SkillBar[:])
	int16ArrToByteArr(shortSkill, ch.ShortSkill[:])

	if ch.Equip, err = s.loadItems(ctx, charID, "char_equip"); err != nil {
		return domain.Character{}, err
	}
	if ch.Carry, err = s.loadItems(ctx, charID, "char_carry"); err != nil {
		return domain.Character{}, err
	}
	if ch.Affects, err = s.loadAffects(ctx, charID); err != nil {
		return domain.Character{}, err
	}
	return ch, nil
}

func (s *Store) loadItems(ctx context.Context, charID int64, kind string) ([]domain.Item, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3, expires_at
		  FROM item WHERE character_id = $1 AND owner_kind = $2 ORDER BY slot`, charID, kind)
	if err != nil {
		return nil, fmt.Errorf("store: load %s: %w", kind, err)
	}
	defer rows.Close()
	var out []domain.Item
	for rows.Next() {
		var it domain.Item
		var exp *time.Time
		if err := rows.Scan(&it.Slot, &it.Index, &it.Eff1, &it.EffV1, &it.Eff2, &it.EffV2, &it.Eff3, &it.EffV3, &exp); err != nil {
			return nil, fmt.Errorf("store: scan %s item: %w", kind, err)
		}
		it.ExpiresAt = expirySeconds(exp)
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) loadAffects(ctx context.Context, charID int64) ([]domain.Affect, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT type, value, level, time FROM affect WHERE character_id = $1`, charID)
	if err != nil {
		return nil, fmt.Errorf("store: load affects: %w", err)
	}
	defer rows.Close()
	var out []domain.Affect
	for rows.Next() {
		var a domain.Affect
		if err := rows.Scan(&a.Type, &a.Value, &a.Level, &a.Time); err != nil {
			return nil, fmt.Errorf("store: scan affect: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// CreateCharacter inserts a new character into a free slot and returns its id.
// The caller is responsible for validating the slot is free (the UNIQUE
// (account_id, slot) and name constraints also enforce it).
func (s *Store) CreateCharacter(ctx context.Context, accountID int64, ch domain.Character) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("store: begin create character: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	id, err := insertCharacter(ctx, tx, accountID, ch)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("store: commit create character: %w", err)
	}
	return id, nil
}

// DeleteCharacter removes a character (and its items/affects via ON DELETE
// CASCADE) by account and slot. Returns ErrNotFound when the slot is empty.
func (s *Store) DeleteCharacter(ctx context.Context, accountID int64, slot int) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM character WHERE account_id = $1 AND slot = $2`, accountID, slot)
	if err != nil {
		return fmt.Errorf("store: delete character a=%d slot=%d: %w", accountID, slot, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SaveCharacter persists a character's live state for in-game saves
// (_MSG_DBSaveMob / SavingQuit). The character must already exist (matched by
// account_id + slot); equip, carry and affects are replaced atomically.
//
// This is a PARTIAL update on purpose: it touches only the columns the in-world
// Entity authoritatively tracks this phase (world.CharacterSave) — clan,
// guild_id, level, coin, str/int/dex/con, hp/max_hp. Everything else (class,
// exp, mp/max_mp, guild_level, bonuses, regen/resist, skills, magic, save_x/y,
// citizen, class_master, skill bars) is left UNTOUCHED so an in-game save never
// wipes imported data the world does not simulate. Widening to the full
// STRUCT_MOB is UNVERIFIED and waits on capture (PROGRESS Fase 4).
func (s *Store) SaveCharacter(ctx context.Context, accountID int64, ch domain.Character) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin save character: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var charID int64
	err = tx.QueryRow(ctx, `
		UPDATE character SET
			clan=$3, guild_id=$4, level=$5, coin=$6,
			str=$7, int=$8, dex=$9, con=$10, hp=$11, max_hp=$12, last_city=$13
		WHERE account_id=$1 AND slot=$2
		RETURNING id`,
		accountID, ch.Slot, ch.Clan, ch.GuildID, ch.Level, ch.Coin,
		ch.Str, ch.Int, ch.Dex, ch.Con, ch.Hp, ch.MaxHp, ch.LastCity,
	).Scan(&charID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("store: update character %q: %w", ch.Name, err)
	}

	if _, err := tx.Exec(ctx,
		`DELETE FROM item WHERE character_id=$1 AND owner_kind IN ('char_equip','char_carry')`, charID); err != nil {
		return fmt.Errorf("store: clear items: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM affect WHERE character_id=$1`, charID); err != nil {
		return fmt.Errorf("store: clear affects: %w", err)
	}
	for _, it := range ch.Equip {
		if err := insertItem(ctx, tx, "char_equip", nil, &charID, it); err != nil {
			return err
		}
	}
	for _, it := range ch.Carry {
		if err := insertItem(ctx, tx, "char_carry", nil, &charID, it); err != nil {
			return err
		}
	}
	for _, a := range ch.Affects {
		if _, err := tx.Exec(ctx,
			`INSERT INTO affect (character_id, type, value, level, time) VALUES ($1,$2,$3,$4,$5)`,
			charID, a.Type, a.Value, a.Level, a.Time); err != nil {
			return fmt.Errorf("store: insert affect: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit save character %q: %w", ch.Name, err)
	}
	return nil
}

// LoadCargo loads the account-shared cargo: the gold (account.cargo_coin) and the
// stored items (item rows with owner_kind='account_cargo'). The cargo is keyed by
// account — every character of the account shares the same vault — so it is read
// once per session, independent of the character slot. A missing account returns
// ErrNotFound; an account with no stored items returns (coin, nil).
func (s *Store) LoadCargo(ctx context.Context, accountID int64) (int32, []domain.Item, error) {
	var coin int32
	err := s.pool.QueryRow(ctx,
		`SELECT cargo_coin FROM account WHERE id = $1`, accountID).Scan(&coin)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil, ErrNotFound
	}
	if err != nil {
		return 0, nil, fmt.Errorf("store: load cargo coin a=%d: %w", accountID, err)
	}
	items, err := s.loadAccountItems(ctx, accountID, "account_cargo")
	if err != nil {
		return 0, nil, err
	}
	return coin, items, nil
}

// loadAccountItems mirrors loadItems but keys on account_id (cargo is account-,
// not character-scoped).
func (s *Store) loadAccountItems(ctx context.Context, accountID int64, kind string) ([]domain.Item, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3, expires_at
		  FROM item WHERE account_id = $1 AND owner_kind = $2 ORDER BY slot`, accountID, kind)
	if err != nil {
		return nil, fmt.Errorf("store: load %s: %w", kind, err)
	}
	defer rows.Close()
	var out []domain.Item
	for rows.Next() {
		var it domain.Item
		var exp *time.Time
		if err := rows.Scan(&it.Slot, &it.Index, &it.Eff1, &it.EffV1, &it.Eff2, &it.EffV2, &it.Eff3, &it.EffV3, &exp); err != nil {
			return nil, fmt.Errorf("store: scan %s item: %w", kind, err)
		}
		it.ExpiresAt = expirySeconds(exp)
		out = append(out, it)
	}
	return out, rows.Err()
}

// SaveCargo persists the account-shared cargo gold + items as a replace-all,
// mirroring SaveCharacter's atomic item swap (anti-dup: the old set is deleted
// and the new set re-inserted in one transaction). A missing account returns
// ErrNotFound.
func (s *Store) SaveCargo(ctx context.Context, accountID int64, coin int32, items []domain.Item) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin save cargo: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `UPDATE account SET cargo_coin = $2 WHERE id = $1`, accountID, coin)
	if err != nil {
		return fmt.Errorf("store: update cargo coin a=%d: %w", accountID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM item WHERE account_id = $1 AND owner_kind = 'account_cargo'`, accountID); err != nil {
		return fmt.Errorf("store: clear cargo items a=%d: %w", accountID, err)
	}
	for _, it := range items {
		if err := insertItem(ctx, tx, "account_cargo", &accountID, nil, it); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit save cargo a=%d: %w", accountID, err)
	}
	return nil
}

// int16ArrToByteArr narrows a smallint[] column back into a fixed byte array,
// the inverse of byteArrToInt16. Extra elements are ignored; missing ones stay
// zero.
func int16ArrToByteArr(src []int16, dst []uint8) {
	for i := range dst {
		if i < len(src) {
			dst[i] = uint8(src[i])
		}
	}
}
