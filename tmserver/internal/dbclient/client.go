// Package dbclient adapts the dbServer gRPC AccountService (api/db/v1) to the
// world.Persistence port. tmServer's loop/handlers depend only on the port; this
// adapter does the blocking gRPC calls (off the loop via World.Go, or inline at
// shutdown) and maps proto messages to/from the world's types
// (migration-plan.md §3.5).
package dbclient

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Client is a world.Persistence backed by the dbServer.
type Client struct {
	api dbv1.AccountServiceClient
}

// New wraps a gRPC connection as a Persistence backend.
func New(conn grpc.ClientConnInterface) *Client {
	return &Client{api: dbv1.NewAccountServiceClient(conn)}
}

var _ world.Persistence = (*Client)(nil)

// AccountLogin authenticates and, on success, fetches the character-selection
// list so the world can present it immediately.
func (c *Client) AccountLogin(ctx context.Context, name, password string) (world.LoginOutcome, error) {
	resp, err := c.api.AccountLogin(ctx, &dbv1.AccountLoginRequest{
		AccountName:   name,
		Password:      password,
		ClientVersion: int32(protocol.AppVersion),
	})
	if err != nil {
		return world.LoginOutcome{}, fmt.Errorf("dbclient: account login: %w", err)
	}
	out := world.LoginOutcome{
		Result:    loginResultFromProto(resp.GetResult()),
		AccountID: resp.GetAccountId(),
		Role:      resp.GetRole(),
	}
	if out.Result != world.LoginOK {
		return out, nil
	}
	out.Characters, err = c.ListCharacters(ctx, out.AccountID)
	if err != nil {
		return world.LoginOutcome{}, err
	}
	// Load the account-shared cargo in the same off-loop round-trip as the
	// character list — it is account-scoped, so it is fetched once per login.
	out.Cargo, err = c.LoadCargo(ctx, out.AccountID)
	if err != nil {
		return world.LoginOutcome{}, err
	}
	// Fetch the donate web-shop mailbox (issue #34) in the same round-trip too,
	// rather than a second loop re-entry after login completes. A fetch failure
	// here is non-fatal to the login — the mailbox is simply retried next login.
	if pending, err := c.ListPendingDeliveries(ctx, out.AccountID); err == nil {
		out.PendingDeliveries = pending
	}
	return out, nil
}

// ListCharacters fetches the character-selection projection for an account.
func (c *Client) ListCharacters(ctx context.Context, accountID int64) ([]world.CharSummary, error) {
	list, err := c.api.ListCharacters(ctx, &dbv1.ListCharactersRequest{AccountId: accountID})
	if err != nil {
		return nil, fmt.Errorf("dbclient: list characters: %w", err)
	}
	out := make([]world.CharSummary, 0, len(list.GetCharacters()))
	for _, ch := range list.GetCharacters() {
		out = append(out, world.CharSummary{
			Slot:    int(ch.GetSlot()),
			Name:    ch.GetName(),
			Class:   int(ch.GetClass()),
			Level:   int(ch.GetLevel()),
			Exp:     ch.GetExp(),
			GuildID: uint16(ch.GetGuildId()),
			Coin:    ch.GetCoin(),
			MaxHp:   ch.GetMaxHp(),
			Hp:      ch.GetHp(),
			MaxMp:   ch.GetMaxMp(),
			Mp:      ch.GetMp(),
			Str:     int16(ch.GetStr()),
			Int:     int16(ch.GetInt()),
			Dex:     int16(ch.GetDex()),
			Con:     int16(ch.GetCon()),
		})
	}
	return out, nil
}

// CreateCharacter creates a character in a free slot.
func (c *Client) CreateCharacter(ctx context.Context, accountID int64, slot int, name string, class int) (bool, error) {
	resp, err := c.api.CreateCharacter(ctx, &dbv1.CreateCharacterRequest{
		AccountId: accountID,
		Slot:      int32(slot),
		Name:      name,
		Class:     int32(class),
	})
	if err != nil {
		return false, fmt.Errorf("dbclient: create character: %w", err)
	}
	return resp.GetOk(), nil
}

// CreateArchCharacter creates an ARCH twin in the first free account slot.
func (c *Client) CreateArchCharacter(ctx context.Context, accountID int64, name string, class, mortalFace, mortalSlot int) (int, bool, error) {
	resp, err := c.api.CreateArchCharacter(ctx, &dbv1.CreateArchCharacterRequest{
		AccountId:  accountID,
		Name:       name,
		Class:      int32(class),
		MortalFace: int32(mortalFace),
		MortalSlot: int32(mortalSlot),
	})
	if err != nil {
		return 0, false, fmt.Errorf("dbclient: create arch character: %w", err)
	}
	return int(resp.GetSlot()), resp.GetOk(), nil
}

// DeleteCharacter deletes a character after password confirmation.
func (c *Client) DeleteCharacter(ctx context.Context, accountID int64, slot int, _, password string) (bool, error) {
	resp, err := c.api.DeleteCharacter(ctx, &dbv1.DeleteCharacterRequest{
		AccountId: accountID,
		Slot:      int32(slot),
		Password:  password,
	})
	if err != nil {
		return false, fmt.Errorf("dbclient: delete character: %w", err)
	}
	return resp.GetOk(), nil
}

// SetPin sets/changes the account's numeric PIN (hashed argon2id on the dbServer).
func (c *Client) SetPin(ctx context.Context, accountID int64, pin string) (bool, error) {
	resp, err := c.api.SetPin(ctx, &dbv1.SetPinRequest{AccountId: accountID, Pin: pin})
	if err != nil {
		return false, fmt.Errorf("dbclient: set pin: %w", err)
	}
	return resp.GetOk(), nil
}

// VerifyPin checks a numeric PIN against the account's stored hash.
func (c *Client) VerifyPin(ctx context.Context, accountID int64, pin string) (world.PinResult, error) {
	resp, err := c.api.VerifyPin(ctx, &dbv1.VerifyPinRequest{AccountId: accountID, Pin: pin})
	if err != nil {
		return world.PinNoAccount, fmt.Errorf("dbclient: verify pin: %w", err)
	}
	return pinResultFromProto(resp.GetResult()), nil
}

func pinResultFromProto(r dbv1.PinResult) world.PinResult {
	switch r {
	case dbv1.PinResult_PIN_RESULT_OK:
		return world.PinOK
	case dbv1.PinResult_PIN_RESULT_NOT_SET:
		return world.PinNotSet
	case dbv1.PinResult_PIN_RESULT_BAD_PIN:
		return world.PinBadPin
	default:
		return world.PinNoAccount
	}
}

// LoadCharacter loads a character's state for world injection.
func (c *Client) LoadCharacter(ctx context.Context, accountID int64, slot int) (world.CharacterState, error) {
	resp, err := c.api.LoadCharacter(ctx, &dbv1.LoadCharacterRequest{
		AccountId: accountID,
		Slot:      int32(slot),
	})
	if err != nil {
		return world.CharacterState{}, fmt.Errorf("dbclient: load character: %w", err)
	}
	return characterStateFromProto(resp.GetCharacter()), nil
}

// SaveOnShutdown persists the world's snapshot of a character.
func (c *Client) SaveOnShutdown(ctx context.Context, save world.CharacterSave) error {
	_, err := c.api.SaveCharacter(ctx, &dbv1.SaveCharacterRequest{
		AccountId: save.AccountID,
		Character: characterSaveToProto(save),
	})
	if err != nil {
		return fmt.Errorf("dbclient: save character: %w", err)
	}
	return nil
}

// LoadCargo loads the account-shared warehouse (gold + items) for world
// injection. Items are placed positionally into the fixed Cargo array.
func (c *Client) LoadCargo(ctx context.Context, accountID int64) (world.CargoState, error) {
	resp, err := c.api.LoadCargo(ctx, &dbv1.LoadCargoRequest{AccountId: accountID})
	if err != nil {
		return world.CargoState{}, fmt.Errorf("dbclient: load cargo: %w", err)
	}
	st := world.CargoState{AccountID: accountID, Coin: resp.GetCargoCoin()}
	for _, it := range resp.GetItems() {
		slot := int(it.GetSlot())
		if slot < 0 || slot >= world.MaxCargo {
			continue
		}
		st.Items[slot] = world.Item{
			Index: int16(it.GetIndex()),
			Effects: [3]world.Effect{
				{Effect: uint8(it.GetEff1()), Value: uint8(it.GetEffv1())},
				{Effect: uint8(it.GetEff2()), Value: uint8(it.GetEffv2())},
				{Effect: uint8(it.GetEff3()), Value: uint8(it.GetEffv3())},
			},
		}
	}
	return st, nil
}

// SaveCargo persists the account-shared warehouse (replace-all on the dbServer).
func (c *Client) SaveCargo(ctx context.Context, save world.CargoSave) error {
	_, err := c.api.SaveCargo(ctx, &dbv1.SaveCargoRequest{
		AccountId: save.AccountID,
		CargoCoin: save.Coin,
		Items:     savedItemsToProto(save.Items),
	})
	if err != nil {
		return fmt.Errorf("dbclient: save cargo: %w", err)
	}
	return nil
}

// ListPendingDeliveries fetches the account's pending item grants from the
// delivery_queue mailbox (donate web shop, issue #34).
func (c *Client) ListPendingDeliveries(ctx context.Context, accountID int64) ([]world.Delivery, error) {
	resp, err := c.api.ListPendingDeliveries(ctx, &dbv1.ListPendingDeliveriesRequest{AccountId: accountID})
	if err != nil {
		return nil, fmt.Errorf("dbclient: list pending deliveries: %w", err)
	}
	out := make([]world.Delivery, 0, len(resp.GetDeliveries()))
	for _, d := range resp.GetDeliveries() {
		out = append(out, world.Delivery{ID: d.GetId(), Item: itemFromProto(d.GetItem())})
	}
	return out, nil
}

// SaveCargoWithDeliveries persists the cargo and acks the drained mailbox rows in
// one dbServer transaction (the anti-dup boundary for the drain).
func (c *Client) SaveCargoWithDeliveries(ctx context.Context, save world.CargoSave, deliveredIDs, lostIDs []int64) error {
	_, err := c.api.SaveCargoWithDeliveries(ctx, &dbv1.SaveCargoWithDeliveriesRequest{
		AccountId:    save.AccountID,
		CargoCoin:    save.Coin,
		Items:        savedItemsToProto(save.Items),
		DeliveredIds: deliveredIDs,
		LostIds:      lostIDs,
	})
	if err != nil {
		return fmt.Errorf("dbclient: save cargo with deliveries: %w", err)
	}
	return nil
}

// SetAccountBlocked flips account.is_blocked by name (GM ban/unban, issue #122).
func (c *Client) SetAccountBlocked(ctx context.Context, name string, blocked bool) error {
	if _, err := c.api.SetAccountBlocked(ctx, &dbv1.SetAccountBlockedRequest{
		AccountName: name,
		Blocked:     blocked,
	}); err != nil {
		return fmt.Errorf("dbclient: set account blocked: %w", err)
	}
	return nil
}

// CreateGuild allocates a persistent legacy guild id and makes the character leader.
func (c *Client) CreateGuild(ctx context.Context, accountID int64, slot int, characterName, guildName string, clan, citizen uint8, serverIndex int) (world.GuildRecord, bool, error) {
	resp, err := c.api.CreateGuild(ctx, &dbv1.CreateGuildRequest{
		AccountId:     accountID,
		Slot:          int32(slot),
		CharacterName: characterName,
		GuildName:     guildName,
		Clan:          int32(clan),
		Citizen:       int32(citizen),
		ServerIndex:   int32(serverIndex),
	})
	if err != nil {
		return world.GuildRecord{}, false, fmt.Errorf("dbclient: create guild: %w", err)
	}
	return guildFromProto(resp.GetGuild()), resp.GetOk(), nil
}

// SetGuildMember persists one character's guild membership/rank.
func (c *Client) SetGuildMember(ctx context.Context, accountID int64, slot int, characterName string, guildID uint16, guildLevel uint8) error {
	resp, err := c.api.SetGuildMember(ctx, &dbv1.SetGuildMemberRequest{
		AccountId:     accountID,
		Slot:          int32(slot),
		CharacterName: characterName,
		GuildId:       uint32(guildID),
		GuildLevel:    int32(guildLevel),
	})
	if err != nil {
		return fmt.Errorf("dbclient: set guild member: %w", err)
	}
	if !resp.GetOk() {
		return fmt.Errorf("dbclient: set guild member rejected")
	}
	return nil
}

// LeaveGuild clears one character's persistent guild membership.
func (c *Client) LeaveGuild(ctx context.Context, accountID int64, slot int) error {
	resp, err := c.api.LeaveGuild(ctx, &dbv1.LeaveGuildRequest{AccountId: accountID, Slot: int32(slot)})
	if err != nil {
		return fmt.Errorf("dbclient: leave guild: %w", err)
	}
	if !resp.GetOk() {
		return fmt.Errorf("dbclient: leave guild rejected")
	}
	return nil
}

// PromoteGuildMember assigns the first available sub-leader rank.
func (c *Client) PromoteGuildMember(ctx context.Context, guildID uint16, accountID int64, slot int) (uint8, bool, error) {
	resp, err := c.api.PromoteGuildMember(ctx, &dbv1.PromoteGuildMemberRequest{
		GuildId:   uint32(guildID),
		AccountId: accountID,
		Slot:      int32(slot),
	})
	if err != nil {
		return 0, false, fmt.Errorf("dbclient: promote guild member: %w", err)
	}
	return uint8(resp.GetGuildLevel()), resp.GetOk(), nil
}

// TransferGuildLeader persists a guild leadership transfer.
func (c *Client) TransferGuildLeader(ctx context.Context, guildID uint16, oldAccountID int64, oldSlot int, newAccountID int64, newSlot int) error {
	resp, err := c.api.TransferGuildLeader(ctx, &dbv1.TransferGuildLeaderRequest{
		GuildId:      uint32(guildID),
		OldAccountId: oldAccountID,
		OldSlot:      int32(oldSlot),
		NewAccountId: newAccountID,
		NewSlot:      int32(newSlot),
	})
	if err != nil {
		return fmt.Errorf("dbclient: transfer guild leader: %w", err)
	}
	if !resp.GetOk() {
		return fmt.Errorf("dbclient: transfer guild leader rejected")
	}
	return nil
}

// SetGuildRelation persists a directed guild ally/war relation.
func (c *Client) SetGuildRelation(ctx context.Context, guildID, targetGuildID uint16, kind world.GuildRelationKind) error {
	resp, err := c.api.SetGuildRelation(ctx, &dbv1.SetGuildRelationRequest{
		GuildId:       uint32(guildID),
		TargetGuildId: uint32(targetGuildID),
		Kind:          relationKindToProto(kind),
	})
	if err != nil {
		return fmt.Errorf("dbclient: set guild relation: %w", err)
	}
	if !resp.GetOk() {
		return fmt.Errorf("dbclient: set guild relation rejected")
	}
	return nil
}

// ListGuilds loads the guild registry snapshot.
func (c *Client) ListGuilds(ctx context.Context) ([]world.GuildRecord, error) {
	resp, err := c.api.ListGuilds(ctx, &dbv1.ListGuildsRequest{})
	if err != nil {
		return nil, fmt.Errorf("dbclient: list guilds: %w", err)
	}
	out := make([]world.GuildRecord, 0, len(resp.GetGuilds()))
	for _, g := range resp.GetGuilds() {
		out = append(out, guildFromProto(g))
	}
	return out, nil
}

// ListGuildRelations loads every directed guild relation.
func (c *Client) ListGuildRelations(ctx context.Context) ([]world.GuildRelation, error) {
	resp, err := c.api.ListGuildRelations(ctx, &dbv1.ListGuildRelationsRequest{})
	if err != nil {
		return nil, fmt.Errorf("dbclient: list guild relations: %w", err)
	}
	out := make([]world.GuildRelation, 0, len(resp.GetRelations()))
	for _, r := range resp.GetRelations() {
		out = append(out, relationFromProto(r))
	}
	return out, nil
}

// LoadGuildZones loads city/guild-zone state.
func (c *Client) LoadGuildZones(ctx context.Context) ([]world.GuildZone, error) {
	resp, err := c.api.LoadGuildZones(ctx, &dbv1.LoadGuildZonesRequest{})
	if err != nil {
		return nil, fmt.Errorf("dbclient: load guild zones: %w", err)
	}
	out := make([]world.GuildZone, 0, len(resp.GetZones()))
	for _, z := range resp.GetZones() {
		out = append(out, zoneFromProto(z))
	}
	return out, nil
}

// SaveGuildZone persists one city/guild-zone row.
func (c *Client) SaveGuildZone(ctx context.Context, zone world.GuildZone) error {
	resp, err := c.api.SaveGuildZone(ctx, &dbv1.SaveGuildZoneRequest{Zone: zoneToProto(zone)})
	if err != nil {
		return fmt.Errorf("dbclient: save guild zone: %w", err)
	}
	if !resp.GetOk() {
		return fmt.Errorf("dbclient: save guild zone rejected")
	}
	return nil
}

// LoadGuildTowerState loads the current GTorre owner.
func (c *Client) LoadGuildTowerState(ctx context.Context) (world.GuildTowerState, error) {
	resp, err := c.api.LoadGuildTowerState(ctx, &dbv1.LoadGuildTowerStateRequest{})
	if err != nil {
		return world.GuildTowerState{}, fmt.Errorf("dbclient: load guild tower state: %w", err)
	}
	return towerStateFromProto(resp.GetState()), nil
}

// SaveGuildTowerState persists the current GTorre owner.
func (c *Client) SaveGuildTowerState(ctx context.Context, state world.GuildTowerState) error {
	resp, err := c.api.SaveGuildTowerState(ctx, &dbv1.SaveGuildTowerStateRequest{State: towerStateToProto(state)})
	if err != nil {
		return fmt.Errorf("dbclient: save guild tower state: %w", err)
	}
	if !resp.GetOk() {
		return fmt.Errorf("dbclient: save guild tower state rejected")
	}
	return nil
}

// LoadCastleQuestState loads the current Castle/Zakum quest state.
func (c *Client) LoadCastleQuestState(ctx context.Context) (world.CastleQuestState, error) {
	resp, err := c.api.LoadCastleQuestState(ctx, &dbv1.LoadCastleQuestStateRequest{})
	if err != nil {
		return world.CastleQuestState{}, fmt.Errorf("dbclient: load castle quest state: %w", err)
	}
	return castleQuestStateFromProto(resp.GetState()), nil
}

// SaveCastleQuestState persists the current Castle/Zakum quest state.
func (c *Client) SaveCastleQuestState(ctx context.Context, state world.CastleQuestState) error {
	resp, err := c.api.SaveCastleQuestState(ctx, &dbv1.SaveCastleQuestStateRequest{State: castleQuestStateToProto(state)})
	if err != nil {
		return fmt.Errorf("dbclient: save castle quest state: %w", err)
	}
	if !resp.GetOk() {
		return fmt.Errorf("dbclient: save castle quest state rejected")
	}
	return nil
}

func loginResultFromProto(r dbv1.LoginResult) world.LoginResult {
	switch r {
	case dbv1.LoginResult_LOGIN_RESULT_OK:
		return world.LoginOK
	case dbv1.LoginResult_LOGIN_RESULT_BAD_PASSWORD:
		return world.LoginBadPassword
	case dbv1.LoginResult_LOGIN_RESULT_BLOCKED:
		return world.LoginBlocked
	case dbv1.LoginResult_LOGIN_RESULT_ALREADY_PLAYING:
		return world.LoginAlreadyPlaying
	default:
		return world.LoginNoAccount
	}
}

func guildFromProto(g *dbv1.Guild) world.GuildRecord {
	if g == nil {
		return world.GuildRecord{}
	}
	return world.GuildRecord{
		ID:      uint16(g.GetId()),
		Name:    g.GetName(),
		Clan:    uint8(g.GetClan()),
		Fame:    g.GetFame(),
		Citizen: uint8(g.GetCitizen()),
	}
}

func relationKindToProto(k world.GuildRelationKind) dbv1.GuildRelationKind {
	switch k {
	case world.GuildRelationAlly:
		return dbv1.GuildRelationKind_GUILD_RELATION_KIND_ALLY
	case world.GuildRelationWar:
		return dbv1.GuildRelationKind_GUILD_RELATION_KIND_WAR
	default:
		return dbv1.GuildRelationKind_GUILD_RELATION_KIND_NONE
	}
}

func relationKindFromProto(k dbv1.GuildRelationKind) world.GuildRelationKind {
	switch k {
	case dbv1.GuildRelationKind_GUILD_RELATION_KIND_ALLY:
		return world.GuildRelationAlly
	case dbv1.GuildRelationKind_GUILD_RELATION_KIND_WAR:
		return world.GuildRelationWar
	default:
		return world.GuildRelationNone
	}
}

func relationFromProto(r *dbv1.GuildRelation) world.GuildRelation {
	if r == nil {
		return world.GuildRelation{}
	}
	return world.GuildRelation{
		GuildID:       uint16(r.GetGuildId()),
		TargetGuildID: uint16(r.GetTargetGuildId()),
		Kind:          relationKindFromProto(r.GetKind()),
	}
}

func zoneToProto(z world.GuildZone) *dbv1.GuildZone {
	return &dbv1.GuildZone{
		Zone:           int32(z.Zone),
		ChargeGuild:    uint32(z.ChargeGuild),
		ChallengeGuild: uint32(z.ChallengeGuild),
		Clan:           int32(z.Clan),
		Victory:        int32(z.Victory),
		CityTax:        int32(z.CityTax),
		ChallengeMoney: z.ChallengeMoney,
		TaxVault:       z.TaxVault,
	}
}

func zoneFromProto(z *dbv1.GuildZone) world.GuildZone {
	if z == nil {
		return world.GuildZone{}
	}
	return world.GuildZone{
		Zone:           int(z.GetZone()),
		ChargeGuild:    uint16(z.GetChargeGuild()),
		ChallengeGuild: uint16(z.GetChallengeGuild()),
		Clan:           uint8(z.GetClan()),
		Victory:        uint8(z.GetVictory()),
		CityTax:        uint8(z.GetCityTax()),
		ChallengeMoney: z.GetChallengeMoney(),
		TaxVault:       z.GetTaxVault(),
	}
}

func towerStateToProto(st world.GuildTowerState) *dbv1.GuildTowerState {
	return &dbv1.GuildTowerState{OwnerGuild: uint32(st.OwnerGuild), UpdatedAtUnix: st.UpdatedAtUnix}
}

func towerStateFromProto(st *dbv1.GuildTowerState) world.GuildTowerState {
	if st == nil {
		return world.GuildTowerState{}
	}
	return world.GuildTowerState{OwnerGuild: uint16(st.GetOwnerGuild()), UpdatedAtUnix: st.GetUpdatedAtUnix()}
}

func castleQuestStateToProto(st world.CastleQuestState) *dbv1.CastleQuestState {
	return &dbv1.CastleQuestState{Level: st.Level, TimeLeft: st.TimeLeft, Clear: st.Clear, LeaderName: st.LeaderName}
}

func castleQuestStateFromProto(st *dbv1.CastleQuestState) world.CastleQuestState {
	if st == nil {
		return world.CastleQuestState{}
	}
	return world.CastleQuestState{Level: st.GetLevel(), TimeLeft: st.GetTimeLeft(), Clear: st.GetClear(), LeaderName: st.GetLeaderName()}
}

// characterStateFromProto maps the loaded character to the world injection shape.
//
// UNVERIFIED: the contract (api/db/v1 Character) does not carry position (X/Y)
// or the derived combat scores (Damage/AC/Master), so those stay zero
// until the full STRUCT_MOB snapshot is captured (PROGRESS Fase 4). Position
// especially must be resolved before live play.
func characterStateFromProto(c *dbv1.Character) world.CharacterState {
	st := world.CharacterState{
		Slot:        int(c.GetSlot()),
		Name:        c.GetName(),
		Class:       int(c.GetClass()),
		Level:       int(c.GetLevel()),
		Exp:         c.GetExp(),
		HP:          c.GetHp(),
		MaxHP:       c.GetMaxHp(),
		MP:          c.GetMp(),
		MaxMP:       c.GetMaxMp(),
		Coin:        c.GetCoin(),
		Clan:        uint8(c.GetClan()),
		GuildID:     uint16(c.GetGuildId()),
		GuildLevel:  uint8(c.GetGuildLevel()),
		Citizen:     uint8(c.GetCitizen()),
		ClassMaster: uint8(c.GetClassMaster()),
		Soul:        uint8(c.GetSoul()),
		Str:         int16(c.GetStr()),
		Int:         int16(c.GetInt()),
		Dex:         int16(c.GetDex()),
		Con:         int16(c.GetCon()),
		LastCity:    int16(c.GetLastCity()),

		ScoreBonus:      uint16(c.GetScoreBonus()),
		SpecialBonus:    uint16(c.GetSpecialBonus()),
		LearnedSkill:    c.GetLearnedSkill(),
		SecLearnedSkill: c.GetSecLearnedSkill(),
		Magic:           int16(c.GetMagic()),
	}
	for i, v := range c.GetSpecial() {
		if i >= len(st.BaseSpecial) {
			break
		}
		st.BaseSpecial[i] = int16(v)
	}
	for i, v := range c.GetSkillBar() {
		if i >= len(st.SkillBar) {
			break
		}
		st.SkillBar[i] = uint8(v)
	}
	for i, v := range c.GetShortSkill() {
		if i >= len(st.ShortSkill) {
			break
		}
		st.ShortSkill[i] = uint8(v)
	}
	// The Divine buff persists as an affect row whose Time holds the absolute Unix
	// deadline (DivineEnd); reconstruct it so login can re-apply (captura §B).
	// Every other row is a live buff slot (Time in 8s affect ticks).
	for _, a := range c.GetAffects() {
		if a.GetType() == world.AffectDivine {
			st.DivineEnd = int64(a.GetTime())
			continue
		}
		st.Affects = append(st.Affects, world.Affect{
			Type:  uint8(a.GetType()),
			Value: uint8(a.GetValue()),
			Level: uint16(a.GetLevel()),
			Time:  a.GetTime(),
		})
	}
	for _, it := range c.GetEquip() {
		slot := int(it.GetSlot())
		if slot < 0 || slot >= world.MaxEquip {
			continue
		}
		st.Equip[slot] = itemFromProto(it)
	}
	for _, it := range c.GetCarry() {
		slot := int(it.GetSlot())
		if slot < 0 || slot >= world.MaxCarry {
			continue
		}
		st.Carry[slot] = itemFromProto(it)
	}
	return st
}

// itemFromProto maps one persisted item to the in-world STRUCT_ITEM shape.
func itemFromProto(it *dbv1.Item) world.Item {
	return world.Item{
		Index: int16(it.GetIndex()),
		Effects: [3]world.Effect{
			{Effect: uint8(it.GetEff1()), Value: uint8(it.GetEffv1())},
			{Effect: uint8(it.GetEff2()), Value: uint8(it.GetEffv2())},
			{Effect: uint8(it.GetEff3()), Value: uint8(it.GetEffv3())},
		},
		ExpiresAt: it.GetExpiresAt(),
	}
}

func characterSaveToProto(s world.CharacterSave) *dbv1.Character {
	c := &dbv1.Character{
		Slot:       int32(s.Slot),
		Clan:       int32(s.Clan),
		GuildId:    uint32(s.GuildID),
		GuildLevel: int32(s.GuildLevel),
		Level:      s.Level,
		Exp:        s.Exp,
		Coin:       s.Coin,
		Str:        int32(s.Str),
		Int:        int32(s.Int),
		Dex:        int32(s.Dex),
		Con:        int32(s.Con),
		Hp:         s.HP,
		MaxHp:      s.MaxHP,
		Mp:         s.MP,
		MaxMp:      s.MaxMP,
		LastCity:   int32(s.LastCity),
		Carry:      savedItemsToProto(s.Carry),
		Equip:      savedItemsToProto(s.Equip),

		ScoreBonus:      int32(s.ScoreBonus),
		SpecialBonus:    int32(s.SpecialBonus),
		LearnedSkill:    s.LearnedSkill,
		SecLearnedSkill: s.SecLearnedSkill,
		Soul:            int32(s.Soul),
		Special:         make([]int32, len(s.BaseSpecial)),
		SkillBar:        make([]uint32, len(s.SkillBar)),
		ShortSkill:      make([]uint32, len(s.ShortSkill)),
	}
	for i, v := range s.BaseSpecial {
		c.Special[i] = int32(v)
	}
	for i, v := range s.SkillBar {
		c.SkillBar[i] = uint32(v)
	}
	for i, v := range s.ShortSkill {
		c.ShortSkill[i] = uint32(v)
	}
	// Persist the Divine buff (only while still active) as one affect row carrying the
	// absolute deadline in Time, so it survives relog (captura §B). The other live
	// buff slots persist as raw rows (Time in 8s affect ticks).
	if s.DivineEnd > time.Now().Unix() {
		c.Affects = append(c.Affects, &dbv1.Affect{Type: int32(world.AffectDivine), Level: 1, Time: uint32(s.DivineEnd)})
	}
	for _, a := range s.Affects {
		if a.Type == 0 || a.Type == world.AffectDivine {
			continue
		}
		c.Affects = append(c.Affects, &dbv1.Affect{
			Type:  int32(a.Type),
			Value: int32(a.Value),
			Level: int32(a.Level),
			Time:  a.Time,
		})
	}
	return c
}

func savedItemsToProto(items []world.SavedItem) []*dbv1.Item {
	if len(items) == 0 {
		return nil
	}
	out := make([]*dbv1.Item, 0, len(items))
	for _, it := range items {
		out = append(out, &dbv1.Item{
			Slot:      int32(it.Slot),
			Index:     int32(it.Index),
			Eff1:      int32(it.Eff1),
			Effv1:     int32(it.EffV1),
			Eff2:      int32(it.Eff2),
			Effv2:     int32(it.EffV2),
			Eff3:      int32(it.Eff3),
			Effv3:     int32(it.EffV3),
			ExpiresAt: it.ExpiresAt,
		})
	}
	return out
}
