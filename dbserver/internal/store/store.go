package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/domain"
)

// Store persists migrated data to PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

// New wraps a pgx pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// SaveAccount inserts an account with its characters, items and affects in a
// single transaction, returning the new account id. The whole account is
// atomic: any failure rolls back (no partial accounts).
func (s *Store) SaveAccount(ctx context.Context, acc domain.Account) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("store: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var accountID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO account
			(name, pass_hash, pin_hash, block_pass_hash, real_name, email, telephone,
			 address, ssn1, ssn2, donate_balance, cargo_coin, is_blocked, year, year_day)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		RETURNING id`,
		acc.Name, acc.PassHash, acc.PinHash, acc.BlockPassHash, acc.RealName, acc.Email,
		acc.Telephone, acc.Address, acc.SSN1, acc.SSN2, acc.DonateBalance, acc.CargoCoin,
		acc.IsBlocked, acc.Year, acc.YearDay,
	).Scan(&accountID); err != nil {
		return 0, fmt.Errorf("store: insert account %q: %w", acc.Name, err)
	}

	for _, it := range acc.Cargo {
		if err := insertItem(ctx, tx, "account_cargo", &accountID, nil, it); err != nil {
			return 0, err
		}
	}

	for _, ch := range acc.Characters {
		charID, err := insertCharacter(ctx, tx, accountID, ch)
		if err != nil {
			return 0, err
		}
		for _, it := range ch.Equip {
			if err := insertItem(ctx, tx, "char_equip", nil, &charID, it); err != nil {
				return 0, err
			}
		}
		for _, it := range ch.Carry {
			if err := insertItem(ctx, tx, "char_carry", nil, &charID, it); err != nil {
				return 0, err
			}
		}
		for _, a := range ch.Affects {
			if _, err := tx.Exec(ctx,
				`INSERT INTO affect (character_id, type, value, level, time) VALUES ($1,$2,$3,$4,$5)`,
				charID, a.Type, a.Value, a.Level, a.Time); err != nil {
				return 0, fmt.Errorf("store: insert affect: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("store: commit account %q: %w", acc.Name, err)
	}
	return accountID, nil
}

func insertCharacter(ctx context.Context, tx pgx.Tx, accountID int64, ch domain.Character) (int64, error) {
	var id int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO character
			(account_id, slot, name, class, clan, guild_id, guild_level, level, exp, coin,
			 str, int, dex, con, score_bonus, special_bonus, skill_bonus,
			 max_hp, max_mp, hp, mp, critical, regen_hp, regen_mp,
			 resist_fire, resist_ice, resist_thunder, resist_magic,
			 learned_skill, magic, save_x, save_y, citizen, class_master,
			 skill_bar, short_skill)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,
			 $22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,$34,$35,$36)
		RETURNING id`,
		accountID, ch.Slot, ch.Name, ch.Class, ch.Clan, ch.GuildID, ch.GuildLevel, ch.Level, ch.Exp, ch.Coin,
		ch.Str, ch.Int, ch.Dex, ch.Con, ch.ScoreBonus, ch.SpecialBonus, ch.SkillBonus,
		ch.MaxHp, ch.MaxMp, ch.Hp, ch.Mp, ch.Critical, ch.RegenHP, ch.RegenMP,
		ch.ResistFire, ch.ResistIce, ch.ResistThunder, ch.ResistMagic,
		ch.LearnedSkill, ch.Magic, ch.SaveX, ch.SaveY, ch.Citizen, ch.ClassMaster,
		byteArrToInt16(ch.SkillBar[:]), byteArrToInt16(ch.ShortSkill[:]),
	).Scan(&id); err != nil {
		return 0, fmt.Errorf("store: insert character %q: %w", ch.Name, err)
	}
	return id, nil
}

func insertItem(ctx context.Context, tx pgx.Tx, kind string, accountID, charID *int64, it domain.Item) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO item
			(owner_kind, account_id, character_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		kind, accountID, charID, it.Slot, it.Index,
		it.Eff1, it.EffV1, it.Eff2, it.EffV2, it.Eff3, it.EffV3); err != nil {
		return fmt.Errorf("store: insert %s item slot %d: %w", kind, it.Slot, err)
	}
	return nil
}

// byteArrToInt16 widens a byte slice to []int16 for a smallint[] column.
func byteArrToInt16(b []uint8) []int16 {
	out := make([]int16, len(b))
	for i, v := range b {
		out[i] = int16(v)
	}
	return out
}
