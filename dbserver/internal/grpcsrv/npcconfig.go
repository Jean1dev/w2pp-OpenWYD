package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// NpcConfigStore is the read surface the NPC-config service needs (satisfied by
// *store.Store). tmServer only reads NPC config through here; all writes go
// through the web-api.
type NpcConfigStore interface {
	NPCConfigVersion(ctx context.Context) (int64, error)
	ListNPCDefinitions(ctx context.Context) ([]domain.NPCDefinition, error)
	ItemPriceOverrides(ctx context.Context) ([]domain.ItemPriceOverride, error)
	ListMobTemplateStats(ctx context.Context) ([]domain.MobTemplateStat, error)
	ListItemStats(ctx context.Context) ([]domain.ItemStat, error)
	ListMountGrowthRates(ctx context.Context) ([]domain.MountGrowthRate, error)
	ListMountAbsorb(ctx context.Context) ([]domain.MountAbsorb, error)
	ListMountBonus(ctx context.Context) ([]domain.MountBonus, error)
	MountConfigVersion(ctx context.Context) (int64, error)
}

// NpcConfigServer implements dbv1.NpcConfigServiceServer. It is the read-only
// bridge tmServer polls to reload moderator-edited NPC config (npc-editing-plan.md).
type NpcConfigServer struct {
	dbv1.UnimplementedNpcConfigServiceServer
	store NpcConfigStore
}

// NewNpcConfig builds the NPC-config service over the given store.
func NewNpcConfig(s NpcConfigStore) *NpcConfigServer { return &NpcConfigServer{store: s} }

// NpcConfigVersion returns the monotonic config version for the tmServer poll.
func (s *NpcConfigServer) NpcConfigVersion(ctx context.Context, _ *dbv1.NpcConfigVersionRequest) (*dbv1.NpcConfigVersionResponse, error) {
	v, err := s.store.NPCConfigVersion(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "npc config version: %v", err)
	}
	return &dbv1.NpcConfigVersionResponse{Version: v}, nil
}

// ListNpcDefinitions returns the full definition snapshot plus global price
// overrides, tagged with the version they were read at.
func (s *NpcConfigServer) ListNpcDefinitions(ctx context.Context, _ *dbv1.ListNpcDefinitionsRequest) (*dbv1.ListNpcDefinitionsResponse, error) {
	// Read the version FIRST so a concurrent write can only make the snapshot look
	// older than it is (the tmServer re-polls and reloads), never newer — which
	// would let a stale snapshot masquerade as current and skip the next reload.
	version, err := s.store.NPCConfigVersion(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "npc config version: %v", err)
	}
	defs, err := s.store.ListNPCDefinitions(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list npc definitions: %v", err)
	}
	prices, err := s.store.ItemPriceOverrides(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "item price overrides: %v", err)
	}
	return &dbv1.ListNpcDefinitionsResponse{
		Version:        version,
		Definitions:    npcDefinitionsToProto(defs),
		PriceOverrides: itemPricesToProto(prices),
	}, nil
}

func npcDefinitionsToProto(defs []domain.NPCDefinition) []*dbv1.NpcDefinition {
	out := make([]*dbv1.NpcDefinition, 0, len(defs))
	for _, d := range defs {
		shop := make([]*dbv1.NpcShopItem, 0, len(d.Shop))
		for _, it := range d.Shop {
			shop = append(shop, &dbv1.NpcShopItem{
				Slot: int32(it.Slot), ItemIndex: it.ItemIndex, Quantity: int32(it.Quantity),
				Eff1: int32(it.Eff1), Effv1: int32(it.EffV1),
				Eff2: int32(it.Eff2), Effv2: int32(it.EffV2),
				Eff3: int32(it.Eff3), Effv3: int32(it.EffV3),
			})
		}
		out = append(out, &dbv1.NpcDefinition{
			Id: d.ID, Slug: d.Slug, TemplateName: d.TemplateName, DisplayName: d.DisplayName,
			Enabled: d.Enabled, MapId: d.MapID, PosX: d.PosX, PosY: d.PosY,
			RouteType: int32(d.RouteType), Merchant: int32(d.Merchant), Shop: shop,
			Origin: d.Origin, GeneratorIndex: d.GeneratorIndex, FollowerTemplate: d.FollowerTemplate,
			MinuteGenerate: d.MinuteGenerate, MinGroup: d.MinGroup, MaxGroup: d.MaxGroup,
			MaxNumMob: d.MaxNumMob, Formation: int32(d.Formation),
			SegX: d.SegX[:], SegY: d.SegY[:], SegRange: d.SegRange[:], SegWait: d.SegWait[:],
			FightAction: d.FightAction[:], DieAction: d.DieAction[:],
		})
	}
	return out
}

func itemPricesToProto(prices []domain.ItemPriceOverride) []*dbv1.ItemPrice {
	out := make([]*dbv1.ItemPrice, 0, len(prices))
	for _, p := range prices {
		out = append(out, &dbv1.ItemPrice{ItemIndex: p.ItemIndex, Price: p.Price})
	}
	return out
}

// ListMobTemplateStats returns every moderator-edited mob/NPC template stat
// override (mob-template-editing-plan.md). Deliberately independent of
// ListNpcDefinitions: no version field, because tmServer applies this only
// once at boot — no hot-reload for this feature.
func (s *NpcConfigServer) ListMobTemplateStats(ctx context.Context, _ *dbv1.ListMobTemplateStatsRequest) (*dbv1.ListMobTemplateStatsResponse, error) {
	stats, err := s.store.ListMobTemplateStats(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list mob template stats: %v", err)
	}
	return &dbv1.ListMobTemplateStatsResponse{Overrides: mobTemplateStatsToProto(stats)}, nil
}

func mobTemplateStatsToProto(stats []domain.MobTemplateStat) []*dbv1.MobTemplateStat {
	out := make([]*dbv1.MobTemplateStat, 0, len(stats))
	for _, st := range stats {
		equip := make([]*dbv1.MobTemplateEquipItem, 0, len(st.Equip))
		for _, it := range st.Equip {
			equip = append(equip, &dbv1.MobTemplateEquipItem{
				Slot: int32(it.Slot), ItemIndex: it.ItemIndex,
				Eff1: int32(it.Eff1), Effv1: int32(it.EffV1),
				Eff2: int32(it.Eff2), Effv2: int32(it.EffV2),
				Eff3: int32(it.Eff3), Effv3: int32(it.EffV3),
			})
		}
		out = append(out, &dbv1.MobTemplateStat{
			TemplateName: st.TemplateName, DisplayName: st.DisplayName,
			Clan: int32(st.Clan), Merchant: int32(st.Merchant), Class: int32(st.Class),
			Coin: st.Coin, Exp: st.Exp, Spx: st.SPX, Spy: st.SPY,
			Level: st.Level, Ac: st.AC, Damage: st.Damage, ChaosRate: int32(st.ChaosRate),
			AttackRun: int32(st.AttackRun), Direction: int32(st.Direction),
			Str: int32(st.Str), Intel: int32(st.Int), Dex: int32(st.Dex), Con: int32(st.Con),
			Special1: int32(st.Special[0]), Special2: int32(st.Special[1]),
			Special3: int32(st.Special[2]), Special4: int32(st.Special[3]),
			MaxHp: st.MaxHp, Hp: st.Hp, MaxMp: st.MaxMp, Mp: st.Mp,
			LearnedSkill: st.LearnedSkill, ScoreBonus: int32(st.ScoreBonus),
			SkillBar1: int32(st.SkillBar[0]), SkillBar2: int32(st.SkillBar[1]),
			SkillBar3: int32(st.SkillBar[2]), SkillBar4: int32(st.SkillBar[3]),
			RegenHp: int32(st.RegenHP), RegenMp: int32(st.RegenMP),
			Resist1: int32(st.Resist[0]), Resist2: int32(st.Resist[1]),
			Resist3: int32(st.Resist[2]), Resist4: int32(st.Resist[3]),
			Equip: equip,
		})
	}
	return out
}

// ListItemStats returns every moderator-edited item base stat override
// (0023_item_stats), the item-side sibling of ListMobTemplateStats. No version
// field for the same reason: tmServer applies these once at boot, because they
// feed the equip score model and a live swap would leave two players wearing
// the same item with different stats.
func (s *NpcConfigServer) ListItemStats(ctx context.Context, _ *dbv1.ListItemStatsRequest) (*dbv1.ListItemStatsResponse, error) {
	stats, err := s.store.ListItemStats(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list item stats: %v", err)
	}
	return &dbv1.ListItemStatsResponse{Overrides: itemStatsToProto(stats)}, nil
}

// itemStatsToProto widens each int16 to the wire's int32. proto3 has no 16-bit
// scalar, so the narrowing lives in the tmServer client instead; nothing here
// can overflow, since every source value already fits in int16.
func itemStatsToProto(stats []domain.ItemStat) []*dbv1.ItemStat {
	out := make([]*dbv1.ItemStat, 0, len(stats))
	for _, st := range stats {
		out = append(out, &dbv1.ItemStat{
			ItemIndex:  st.ItemIndex,
			ReqLevel:   int32(st.ReqLevel),
			ReqStr:     int32(st.ReqStr),
			ReqInt:     int32(st.ReqInt),
			ReqDex:     int32(st.ReqDex),
			ReqCon:     int32(st.ReqCon),
			Damage:     int32(st.Damage),
			Damageadd:  int32(st.DamageAdd),
			Ac:         int32(st.AC),
			Acadd:      int32(st.ACAdd),
			Magic:      int32(st.Magic),
			Magicadd:   int32(st.MagicAdd),
			Critical:   int32(st.Critical),
			Critical2:  int32(st.Critical2),
			Runspeed:   int32(st.RunSpeed),
			Str:        int32(st.Str),
			Intel:      int32(st.Int),
			Dex:        int32(st.Dex),
			Con:        int32(st.Con),
			Hp:         int32(st.Hp),
			Hpadd:      int32(st.HpAdd),
			Hpadd2:     int32(st.HpAdd2),
			Mp:         int32(st.Mp),
			Mpadd:      int32(st.MpAdd),
			Mpadd2:     int32(st.MpAdd2),
			Resist1:    int32(st.Resist1),
			Resist2:    int32(st.Resist2),
			Resist3:    int32(st.Resist3),
			Resist4:    int32(st.Resist4),
			Resistall:  int32(st.ResistAll),
			Special1:   int32(st.Special1),
			Special2:   int32(st.Special2),
			Special3:   int32(st.Special3),
			Special4:   int32(st.Special4),
			Specialall: int32(st.SpecialAll),
			Itemlevel:  int32(st.ItemLevel),
			Itemtype:   int32(st.ItemType),
			Mobtype:    int32(st.MobType),
			Wtype:      int32(st.WType),
			Pos:        int32(st.Pos),
			Sanc:       int32(st.Sanc),
			Nosanc:     int32(st.NoSanc),
			Incubate:   int32(st.Incubate),
			Incudelay:  int32(st.IncuDelay),
			EfRange:    int32(st.Range),
			EfVolatile: int32(st.Volatile),
		})
	}
	return out
}

// ListMountGrowthRates returns the configured mount growth curves
// (0030_mount_growth_rate). A lineage with no rows simply does not appear, and
// the tmServer keeps its compiled default for that mount — absence means "not
// configured", never "zero chance".
func (s *NpcConfigServer) ListMountGrowthRates(ctx context.Context, _ *dbv1.ListMountGrowthRatesRequest) (*dbv1.ListMountGrowthRatesResponse, error) {
	rates, err := s.store.ListMountGrowthRates(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list mount growth rates: %v", err)
	}
	out := make([]*dbv1.MountGrowthRate, 0, len(rates))
	for _, r := range rates {
		out = append(out, &dbv1.MountGrowthRate{
			MountIndex: int32(r.MountIndex),
			Band:       int32(r.Band),
			Rate:       int32(r.Rate),
		})
	}
	return &dbv1.ListMountGrowthRatesResponse{Rates: out}, nil
}

// ListMountAbsorb returns the configured absorption pairs (0035_mount_absorb).
// A lineage with no row simply does not appear, and the tmServer keeps the
// compiled default (25/25, the legacy's flat share) for that mount — absence
// means "not configured", never "absorbs nothing".
func (s *NpcConfigServer) ListMountAbsorb(ctx context.Context, _ *dbv1.ListMountAbsorbRequest) (*dbv1.ListMountAbsorbResponse, error) {
	rows, err := s.store.ListMountAbsorb(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list mount absorb: %v", err)
	}
	out := make([]*dbv1.MountAbsorb, 0, len(rows))
	for _, a := range rows {
		out = append(out, &dbv1.MountAbsorb{
			MountIndex: int32(a.MountIndex),
			AbsorbPvp:  int32(a.PvP),
			AbsorbPve:  int32(a.PvE),
		})
	}
	return &dbv1.ListMountAbsorbResponse{Absorb: out}, nil
}

// MountConfigVersion answers when the mount overlay last changed, as unix
// seconds. Zero means nobody has ever configured a lineage.
func (s *NpcConfigServer) MountConfigVersion(ctx context.Context, _ *dbv1.MountConfigVersionRequest) (*dbv1.MountConfigVersionResponse, error) {
	v, err := s.store.MountConfigVersion(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mount config version: %v", err)
	}
	return &dbv1.MountConfigVersionResponse{Version: v}, nil
}

// ListMountBonus returns the configured mount attributes (0043_mount_bonus).
// Only the rows someone saved: the tmServer owns the compiled table and falls
// back to it for every lineage absent here.
func (s *NpcConfigServer) ListMountBonus(ctx context.Context, _ *dbv1.ListMountBonusRequest) (*dbv1.ListMountBonusResponse, error) {
	rows, err := s.store.ListMountBonus(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list mount bonus: %v", err)
	}
	out := make([]*dbv1.MountBonus, 0, len(rows))
	for _, b := range rows {
		out = append(out, &dbv1.MountBonus{
			MountIndex: int32(b.MountIndex),
			Attack:     int32(b.Attack),
			Magic:      int32(b.Magic),
			Evasion:    int32(b.Evasion),
			Resist:     int32(b.Resist),
		})
	}
	return &dbv1.ListMountBonusResponse{Bonus: out}, nil
}
