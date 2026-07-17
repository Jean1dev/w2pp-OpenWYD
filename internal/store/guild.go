package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

const (
	legacyGuildPerServer = 4096
	guildAllocLockBase   = 0x77326775696c6400
)

// CreateGuild allocates a legacy ushort guild id, creates the guild row, debits
// the creation cost, and makes the requesting character its leader in one
// transaction.
func (s *Store) CreateGuild(ctx context.Context, accountID int64, slot int, characterName, guildName string, clan, citizen uint8, serverIndex int, cost int32) (domain.Guild, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Guild{}, fmt.Errorf("store: begin create guild: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if serverIndex < 0 {
		serverIndex = 0
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(guildAllocLockBase+serverIndex)); err != nil {
		return domain.Guild{}, fmt.Errorf("store: lock guild allocator: %w", err)
	}

	var charID int64
	var currentGuild int
	var coin int32
	err = tx.QueryRow(ctx, `
		SELECT id, guild_id, coin
		  FROM character
		 WHERE account_id = $1 AND slot = $2 AND name = $3
		 FOR UPDATE`,
		accountID, slot, characterName,
	).Scan(&charID, &currentGuild, &coin)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Guild{}, ErrNotFound
	}
	if err != nil {
		return domain.Guild{}, fmt.Errorf("store: lock character for guild create: %w", err)
	}
	if currentGuild != 0 || cost < 0 || coin < cost {
		return domain.Guild{}, ErrConflict
	}

	minID := serverIndex*legacyGuildPerServer + 1
	maxID := serverIndex*legacyGuildPerServer + legacyGuildPerServer - 1
	var guildID int
	err = tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(id), $1 - 1) + 1 FROM guild WHERE id BETWEEN $1 AND $2`,
		minID, maxID,
	).Scan(&guildID)
	if err != nil {
		return domain.Guild{}, fmt.Errorf("store: allocate guild id: %w", err)
	}
	if guildID > maxID || guildID >= 65536 {
		return domain.Guild{}, ErrNoFreeSlot
	}

	guild := domain.Guild{ID: uint16(guildID), Name: guildName, Clan: clan, Citizen: citizen}
	if _, err := tx.Exec(ctx, `
		INSERT INTO guild(id, name, clan, fame, citizen)
		VALUES ($1, $2, $3, 0, $4)`,
		guildID, guildName, clan, citizen,
	); err != nil {
		return domain.Guild{}, fmt.Errorf("store: insert guild %q: %w", guildName, err)
	}
	if err := setGuildMemberTx(ctx, tx, accountID, slot, charID, characterName, uint16(guildID), 9); err != nil {
		return domain.Guild{}, err
	}
	if err := spendCharacterCoinTx(ctx, tx, charID, cost); err != nil {
		return domain.Guild{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Guild{}, fmt.Errorf("store: commit create guild %q: %w", guildName, err)
	}
	return guild, nil
}

// SetGuildMember sets a character's guild id/level and mirrors it in
// guild_member. A guild id of 0 removes the membership.
func (s *Store) SetGuildMember(ctx context.Context, accountID int64, slot int, characterName string, guildID uint16, guildLevel uint8) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin set guild member: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var charID int64
	err = tx.QueryRow(ctx, `
		SELECT id FROM character
		 WHERE account_id = $1 AND slot = $2 AND name = $3
		 FOR UPDATE`,
		accountID, slot, characterName,
	).Scan(&charID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("store: lock character for guild member: %w", err)
	}
	if err := setGuildMemberTx(ctx, tx, accountID, slot, charID, characterName, guildID, guildLevel); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit set guild member: %w", err)
	}
	return nil
}

// LeaveGuild removes a character from its guild.
func (s *Store) LeaveGuild(ctx context.Context, accountID int64, slot int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin leave guild: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var charID int64
	err = tx.QueryRow(ctx,
		`SELECT id FROM character WHERE account_id = $1 AND slot = $2 FOR UPDATE`,
		accountID, slot,
	).Scan(&charID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("store: lock character for leave guild: %w", err)
	}
	if err := setGuildMemberTx(ctx, tx, accountID, slot, charID, "", 0, 0); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit leave guild: %w", err)
	}
	return nil
}

// PromoteGuildMember assigns the first available sub-leader level (6, 7, 8) and
// debits the leader in the same transaction.
func (s *Store) PromoteGuildMember(ctx context.Context, guildID uint16, leaderAccountID int64, leaderSlot int, accountID int64, slot int, cost int32) (uint8, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("store: begin promote guild member: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	leaderID, _, leaderGuild, leaderLevel, leaderCoin, err := lockGuildCharacterWithCoin(ctx, tx, leaderAccountID, leaderSlot)
	if err != nil {
		return 0, err
	}
	if leaderGuild != int(guildID) || leaderLevel != 9 || cost < 0 || leaderCoin < cost {
		return 0, ErrConflict
	}

	var charID int64
	var name string
	var currentGuild int
	var currentLevel int
	err = tx.QueryRow(ctx, `
		SELECT id, name, guild_id, guild_level
		  FROM character
		 WHERE account_id = $1 AND slot = $2
		 FOR UPDATE`,
		accountID, slot,
	).Scan(&charID, &name, &currentGuild, &currentLevel)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("store: lock character for guild promotion: %w", err)
	}
	if currentGuild != int(guildID) || currentLevel != 0 {
		return 0, ErrConflict
	}

	rows, err := tx.Query(ctx, `
		SELECT guild_level FROM guild_member
		 WHERE guild_id = $1 AND guild_level BETWEEN 6 AND 8
		 FOR UPDATE`,
		guildID,
	)
	if err != nil {
		return 0, fmt.Errorf("store: list guild subleaders: %w", err)
	}
	used := [3]bool{}
	for rows.Next() {
		var level int
		if err := rows.Scan(&level); err != nil {
			rows.Close()
			return 0, fmt.Errorf("store: scan guild subleader: %w", err)
		}
		if level >= 6 && level <= 8 {
			used[level-6] = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("store: list guild subleaders: %w", err)
	}
	rows.Close()

	level := uint8(0)
	for i, ok := range used {
		if !ok {
			level = uint8(6 + i)
			break
		}
	}
	if level == 0 {
		return 0, ErrNoFreeSlot
	}
	if err := spendCharacterCoinTx(ctx, tx, leaderID, cost); err != nil {
		return 0, err
	}
	if err := setGuildMemberTx(ctx, tx, accountID, slot, charID, name, guildID, level); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("store: commit promote guild member: %w", err)
	}
	return level, nil
}

// TransferGuildLeader moves level 9 from oldAccountID/slot to newAccountID/slot.
func (s *Store) TransferGuildLeader(ctx context.Context, guildID uint16, oldAccountID int64, oldSlot int, newAccountID int64, newSlot int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin transfer guild leader: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	oldID, oldName, oldGuild, oldLevel, err := lockGuildCharacter(ctx, tx, oldAccountID, oldSlot)
	if err != nil {
		return err
	}
	newID, newName, newGuild, _, err := lockGuildCharacter(ctx, tx, newAccountID, newSlot)
	if err != nil {
		return err
	}
	if oldGuild != int(guildID) || newGuild != int(guildID) || oldLevel != 9 {
		return ErrConflict
	}
	if err := setGuildMemberTx(ctx, tx, oldAccountID, oldSlot, oldID, oldName, guildID, 0); err != nil {
		return err
	}
	if err := setGuildMemberTx(ctx, tx, newAccountID, newSlot, newID, newName, guildID, 9); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit transfer guild leader: %w", err)
	}
	return nil
}

func lockGuildCharacter(ctx context.Context, tx pgx.Tx, accountID int64, slot int) (int64, string, int, int, error) {
	var charID int64
	var name string
	var guildID, guildLevel int
	err := tx.QueryRow(ctx, `
		SELECT id, name, guild_id, guild_level
		  FROM character
		 WHERE account_id = $1 AND slot = $2
		 FOR UPDATE`,
		accountID, slot,
	).Scan(&charID, &name, &guildID, &guildLevel)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", 0, 0, ErrNotFound
	}
	if err != nil {
		return 0, "", 0, 0, fmt.Errorf("store: lock guild character: %w", err)
	}
	return charID, name, guildID, guildLevel, nil
}

func lockGuildCharacterWithCoin(ctx context.Context, tx pgx.Tx, accountID int64, slot int) (int64, string, int, int, int32, error) {
	var charID int64
	var name string
	var guildID, guildLevel int
	var coin int32
	err := tx.QueryRow(ctx, `
		SELECT id, name, guild_id, guild_level, coin
		  FROM character
		 WHERE account_id = $1 AND slot = $2
		 FOR UPDATE`,
		accountID, slot,
	).Scan(&charID, &name, &guildID, &guildLevel, &coin)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", 0, 0, 0, ErrNotFound
	}
	if err != nil {
		return 0, "", 0, 0, 0, fmt.Errorf("store: lock guild character with coin: %w", err)
	}
	return charID, name, guildID, guildLevel, coin, nil
}

func spendCharacterCoinTx(ctx context.Context, tx pgx.Tx, charID int64, cost int32) error {
	if cost == 0 {
		return nil
	}
	tag, err := tx.Exec(ctx, `UPDATE character SET coin = coin - $2 WHERE id = $1 AND coin >= $2`, charID, cost)
	if err != nil {
		return fmt.Errorf("store: spend character coin: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	return nil
}

func setGuildMemberTx(ctx context.Context, tx pgx.Tx, accountID int64, slot int, charID int64, name string, guildID uint16, guildLevel uint8) error {
	if _, err := tx.Exec(ctx,
		`UPDATE character SET guild_id = $3, guild_level = $4 WHERE account_id = $1 AND slot = $2`,
		accountID, slot, guildID, guildLevel,
	); err != nil {
		return fmt.Errorf("store: update character guild fields: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM guild_member WHERE character_id = $1`, charID); err != nil {
		return fmt.Errorf("store: clear guild member: %w", err)
	}
	if guildID == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO guild_member(guild_id, character_id, account_id, slot, name, guild_level, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())`,
		guildID, charID, accountID, slot, name, guildLevel,
	); err != nil {
		return fmt.Errorf("store: insert guild member: %w", err)
	}
	return nil
}

// SetGuildRelation upserts or removes one directed ally/war relation.
func (s *Store) SetGuildRelation(ctx context.Context, guildID, targetGuildID uint16, kind domain.GuildRelationKind) error {
	if guildID == 0 || guildID == targetGuildID {
		return ErrConflict
	}
	if kind == domain.GuildRelationNone || targetGuildID == 0 {
		tag, err := s.pool.Exec(ctx,
			`DELETE FROM guild_relation WHERE guild_id = $1 AND ($2 = 0 OR kind = $2)`, guildID, kind)
		if err != nil {
			return fmt.Errorf("store: clear guild relation: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO guild_relation(guild_id, target_guild_id, kind, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (guild_id, kind)
		DO UPDATE SET target_guild_id = EXCLUDED.target_guild_id, updated_at = now()`,
		guildID, targetGuildID, kind,
	)
	if err != nil {
		return fmt.Errorf("store: set guild relation: %w", err)
	}
	return nil
}

// ListGuilds returns every registered guild ordered by id.
func (s *Store) ListGuilds(ctx context.Context) ([]domain.Guild, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, clan, fame, citizen FROM guild ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("store: list guilds: %w", err)
	}
	defer rows.Close()
	var out []domain.Guild
	for rows.Next() {
		var g domain.Guild
		if err := rows.Scan(&g.ID, &g.Name, &g.Clan, &g.Fame, &g.Citizen); err != nil {
			return nil, fmt.Errorf("store: scan guild: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// ListGuildRelations returns every directed guild relation ordered by guild id.
func (s *Store) ListGuildRelations(ctx context.Context) ([]domain.GuildRelation, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT guild_id, target_guild_id, kind FROM guild_relation ORDER BY guild_id, target_guild_id`)
	if err != nil {
		return nil, fmt.Errorf("store: list guild relations: %w", err)
	}
	defer rows.Close()
	var out []domain.GuildRelation
	for rows.Next() {
		var r domain.GuildRelation
		if err := rows.Scan(&r.GuildID, &r.TargetGuildID, &r.Kind); err != nil {
			return nil, fmt.Errorf("store: scan guild relation: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// LoadGuildZones loads the five guild/city zones.
func (s *Store) LoadGuildZones(ctx context.Context) ([]domain.GuildZone, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT zone, charge_guild, challenge_guild, clan, victory, city_tax, challenge_money, tax_vault
		  FROM guild_zone ORDER BY zone`)
	if err != nil {
		return nil, fmt.Errorf("store: load guild zones: %w", err)
	}
	defer rows.Close()
	var out []domain.GuildZone
	for rows.Next() {
		var z domain.GuildZone
		if err := rows.Scan(&z.Zone, &z.ChargeGuild, &z.ChallengeGuild, &z.Clan, &z.Victory, &z.CityTax, &z.ChallengeMoney, &z.TaxVault); err != nil {
			return nil, fmt.Errorf("store: scan guild zone: %w", err)
		}
		out = append(out, z)
	}
	return out, rows.Err()
}

// SaveGuildZone persists one guild/city zone.
func (s *Store) SaveGuildZone(ctx context.Context, z domain.GuildZone) error {
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO guild_zone(zone, charge_guild, challenge_guild, clan, victory, city_tax, challenge_money, tax_vault, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
		ON CONFLICT (zone)
		DO UPDATE SET charge_guild = EXCLUDED.charge_guild,
		              challenge_guild = EXCLUDED.challenge_guild,
		              clan = EXCLUDED.clan,
		              victory = EXCLUDED.victory,
		              city_tax = EXCLUDED.city_tax,
		              challenge_money = EXCLUDED.challenge_money,
		              tax_vault = EXCLUDED.tax_vault,
		              updated_at = now()`,
		z.Zone, z.ChargeGuild, z.ChallengeGuild, z.Clan, z.Victory, z.CityTax, z.ChallengeMoney, z.TaxVault,
	)
	if err != nil {
		return fmt.Errorf("store: save guild zone %d: %w", z.Zone, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// LoadGuildTowerState loads the single GTorre owner row.
func (s *Store) LoadGuildTowerState(ctx context.Context) (domain.GuildTowerState, error) {
	var st domain.GuildTowerState
	err := s.pool.QueryRow(ctx,
		`SELECT owner_guild, updated_at_unix FROM guild_tower_state WHERE id = 1`,
	).Scan(&st.OwnerGuild, &st.UpdatedAtUnix)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.GuildTowerState{}, ErrNotFound
	}
	if err != nil {
		return domain.GuildTowerState{}, fmt.Errorf("store: load guild tower state: %w", err)
	}
	return st, nil
}

// SaveGuildTowerState persists the single GTorre owner row.
func (s *Store) SaveGuildTowerState(ctx context.Context, st domain.GuildTowerState) error {
	if st.UpdatedAtUnix == 0 {
		st.UpdatedAtUnix = time.Now().Unix()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO guild_tower_state(id, owner_guild, updated_at_unix)
		VALUES (1, $1, $2)
		ON CONFLICT (id)
		DO UPDATE SET owner_guild = EXCLUDED.owner_guild,
		              updated_at_unix = EXCLUDED.updated_at_unix`,
		st.OwnerGuild, st.UpdatedAtUnix,
	)
	if err != nil {
		return fmt.Errorf("store: save guild tower state: %w", err)
	}
	return nil
}

// LoadCastleQuestState loads the single Castle/Zakum quest row.
func (s *Store) LoadCastleQuestState(ctx context.Context) (domain.CastleQuestState, error) {
	var st domain.CastleQuestState
	err := s.pool.QueryRow(ctx,
		`SELECT level, time_left, clear, leader_name FROM castle_quest_state WHERE id = 1`,
	).Scan(&st.Level, &st.TimeLeft, &st.Clear, &st.LeaderName)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CastleQuestState{}, ErrNotFound
	}
	if err != nil {
		return domain.CastleQuestState{}, fmt.Errorf("store: load castle quest state: %w", err)
	}
	return st, nil
}

// SaveCastleQuestState persists the single Castle/Zakum quest row.
func (s *Store) SaveCastleQuestState(ctx context.Context, st domain.CastleQuestState) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO castle_quest_state(id, level, time_left, clear, leader_name, updated_at)
		VALUES (1, $1, $2, $3, $4, now())
		ON CONFLICT (id)
		DO UPDATE SET level = EXCLUDED.level,
		              time_left = EXCLUDED.time_left,
		              clear = EXCLUDED.clear,
		              leader_name = EXCLUDED.leader_name,
		              updated_at = now()`,
		st.Level, st.TimeLeft, st.Clear, st.LeaderName,
	)
	if err != nil {
		return fmt.Errorf("store: save castle quest state: %w", err)
	}
	return nil
}
