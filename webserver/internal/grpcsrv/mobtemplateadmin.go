package grpcsrv

import (
	"context"
	"math"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/mobtemplateadmin"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/mobtemplates"
)

// MobTemplateAdmin is the moderator mob-template-stat-editing surface the
// server depends on (satisfied by *mobtemplateadmin.Service). Kept as an
// interface so the server is unit-testable.
type MobTemplateAdmin interface {
	ListTemplates(ctx context.Context, moderatorID int64) (mobtemplateadmin.Result, []mobtemplates.File, error)
	Get(ctx context.Context, moderatorID int64, templateName string) (mobtemplateadmin.Result, domain.MobTemplateStat, bool, error)
	Upsert(ctx context.Context, moderatorID int64, st domain.MobTemplateStat) (mobtemplateadmin.Result, error)
	SetEquip(ctx context.Context, moderatorID int64, templateName string, items []domain.MobTemplateEquipItem) (mobtemplateadmin.Result, error)
	Delete(ctx context.Context, moderatorID int64, templateName string) (mobtemplateadmin.Result, error)
}

// MobTemplateAdminServer implements webv1.MobTemplateAdminServiceServer.
type MobTemplateAdminServer struct {
	webv1.UnimplementedMobTemplateAdminServiceServer
	admin MobTemplateAdmin
}

// NewMobTemplateAdmin builds the MobTemplateAdminService over the given admin logic.
func NewMobTemplateAdmin(a MobTemplateAdmin) *MobTemplateAdminServer {
	return &MobTemplateAdminServer{admin: a}
}

// ListMobTemplates returns every mob template file scanned from the content
// tree, for the moderator UI's template_name picker.
func (s *MobTemplateAdminServer) ListMobTemplates(ctx context.Context, req *webv1.ListMobTemplatesRequest) (*webv1.ListMobTemplatesResponse, error) {
	res, tmpls, err := s.admin.ListTemplates(ctx, req.GetModeratorId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list mob templates: %v", err)
	}
	out := make([]*webv1.MobTemplateFile, 0, len(tmpls))
	for _, t := range tmpls {
		out = append(out, &webv1.MobTemplateFile{
			TemplateName: t.TemplateName, DisplayName: t.DisplayName, Merchant: int32(t.Merchant),
		})
	}
	return &webv1.ListMobTemplatesResponse{Result: mobStatResultToProto(res), Templates: out}, nil
}

// GetMobTemplateStat returns the stat override for a template, falling back
// to the raw template file's current values when no override exists yet.
func (s *MobTemplateAdminServer) GetMobTemplateStat(ctx context.Context, req *webv1.GetMobTemplateStatRequest) (*webv1.GetMobTemplateStatResponse, error) {
	res, st, overridden, err := s.admin.Get(ctx, req.GetModeratorId(), req.GetTemplateName())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get mob template stat: %v", err)
	}
	resp := &webv1.GetMobTemplateStatResponse{Result: mobStatResultToProto(res)}
	if res == mobtemplateadmin.OK {
		resp.Stat = adminMobTemplateStatToProto(st, overridden)
	}
	return resp, nil
}

// UpsertMobTemplateStat creates or replaces the full stat override for a template.
func (s *MobTemplateAdminServer) UpsertMobTemplateStat(ctx context.Context, req *webv1.UpsertMobTemplateStatRequest) (*webv1.UpsertMobTemplateStatResponse, error) {
	if !mobTemplateStatInRange(req.GetStat()) {
		return &webv1.UpsertMobTemplateStatResponse{Result: webv1.AdminResult_ADMIN_RESULT_INVALID}, nil
	}
	st := protoToMobTemplateStat(req.GetStat())
	res, err := s.admin.Upsert(ctx, req.GetModeratorId(), st)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "upsert mob template stat: %v", err)
	}
	return &webv1.UpsertMobTemplateStatResponse{Result: mobStatResultToProto(res)}, nil
}

// SetMobTemplateEquip replaces a template's Equip[] slot overrides.
func (s *MobTemplateAdminServer) SetMobTemplateEquip(ctx context.Context, req *webv1.SetMobTemplateEquipRequest) (*webv1.AdminAck, error) {
	items := make([]domain.MobTemplateEquipItem, 0, len(req.GetItems()))
	for _, it := range req.GetItems() {
		items = append(items, protoToMobTemplateEquipItem(it))
	}
	res, err := s.admin.SetEquip(ctx, req.GetModeratorId(), req.GetTemplateName(), items)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "set mob template equip: %v", err)
	}
	return &webv1.AdminAck{Result: mobStatResultToProto(res)}, nil
}

// DeleteMobTemplateStat removes the override, reverting to the raw file defaults.
func (s *MobTemplateAdminServer) DeleteMobTemplateStat(ctx context.Context, req *webv1.DeleteMobTemplateStatRequest) (*webv1.AdminAck, error) {
	res, err := s.admin.Delete(ctx, req.GetModeratorId(), req.GetTemplateName())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "delete mob template stat: %v", err)
	}
	return &webv1.AdminAck{Result: mobStatResultToProto(res)}, nil
}

func adminMobTemplateStatToProto(st domain.MobTemplateStat, overridden bool) *webv1.AdminMobTemplateStat {
	equip := make([]*webv1.AdminMobTemplateEquipItem, 0, len(st.Equip))
	for _, it := range st.Equip {
		equip = append(equip, &webv1.AdminMobTemplateEquipItem{
			Slot: int32(it.Slot), ItemIndex: it.ItemIndex,
			Eff1: int32(it.Eff1), Effv1: int32(it.EffV1),
			Eff2: int32(it.Eff2), Effv2: int32(it.EffV2),
			Eff3: int32(it.Eff3), Effv3: int32(it.EffV3),
		})
	}
	return &webv1.AdminMobTemplateStat{
		TemplateName: st.TemplateName, Overridden: overridden, DisplayName: st.DisplayName,
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
	}
}

// mobTemplateStatInRange checks every AdminMobTemplateStat field against the
// real STRUCT_MOB wire type it narrows into (protoToMobTemplateStat and, for
// spx/spy/str/intel/dex/con/special*, tmserver/internal/dbclient downstream),
// so an out-of-range value (e.g. clan=300, resist=200) is rejected here
// instead of silently truncating/wrapping past this boundary.
func mobTemplateStatInRange(p *webv1.AdminMobTemplateStat) bool {
	if p == nil {
		return false
	}
	u8 := func(v int32) bool { return v >= 0 && v <= math.MaxUint8 }
	i8 := func(v int32) bool { return v >= math.MinInt8 && v <= math.MaxInt8 }
	i16 := func(v int32) bool { return v >= math.MinInt16 && v <= math.MaxInt16 }
	u16 := func(v int32) bool { return v >= 0 && v <= math.MaxUint16 }
	return u8(p.GetClan()) && u8(p.GetMerchant()) && u8(p.GetClass()) &&
		i16(p.GetSpx()) && i16(p.GetSpy()) &&
		u8(p.GetChaosRate()) && u8(p.GetAttackRun()) && u8(p.GetDirection()) &&
		i16(p.GetStr()) && i16(p.GetIntel()) && i16(p.GetDex()) && i16(p.GetCon()) &&
		i16(p.GetSpecial1()) && i16(p.GetSpecial2()) && i16(p.GetSpecial3()) && i16(p.GetSpecial4()) &&
		u16(p.GetScoreBonus()) &&
		u8(p.GetSkillBar1()) && u8(p.GetSkillBar2()) && u8(p.GetSkillBar3()) && u8(p.GetSkillBar4()) &&
		u16(p.GetRegenHp()) && u16(p.GetRegenMp()) &&
		i8(p.GetResist1()) && i8(p.GetResist2()) && i8(p.GetResist3()) && i8(p.GetResist4())
}

func protoToMobTemplateStat(p *webv1.AdminMobTemplateStat) domain.MobTemplateStat {
	st := domain.MobTemplateStat{
		TemplateName: p.GetTemplateName(), DisplayName: p.GetDisplayName(),
		Clan: uint8(p.GetClan()), Merchant: uint8(p.GetMerchant()), Class: uint8(p.GetClass()),
		Coin: p.GetCoin(), Exp: p.GetExp(), SPX: p.GetSpx(), SPY: p.GetSpy(),
		Level: p.GetLevel(), AC: p.GetAc(), Damage: p.GetDamage(), ChaosRate: uint8(p.GetChaosRate()),
		AttackRun: uint8(p.GetAttackRun()), Direction: uint8(p.GetDirection()),
		Str: int16(p.GetStr()), Int: int16(p.GetIntel()), Dex: int16(p.GetDex()), Con: int16(p.GetCon()),
		Special: [4]int16{int16(p.GetSpecial1()), int16(p.GetSpecial2()), int16(p.GetSpecial3()), int16(p.GetSpecial4())},
		MaxHp:   p.GetMaxHp(), Hp: p.GetHp(), MaxMp: p.GetMaxMp(), Mp: p.GetMp(),
		LearnedSkill: p.GetLearnedSkill(), ScoreBonus: uint16(p.GetScoreBonus()),
		SkillBar: [4]uint8{uint8(p.GetSkillBar1()), uint8(p.GetSkillBar2()), uint8(p.GetSkillBar3()), uint8(p.GetSkillBar4())},
		RegenHP:  uint16(p.GetRegenHp()), RegenMP: uint16(p.GetRegenMp()),
		Resist: [4]int8{int8(p.GetResist1()), int8(p.GetResist2()), int8(p.GetResist3()), int8(p.GetResist4())},
	}
	for _, it := range p.GetEquip() {
		st.Equip = append(st.Equip, protoToMobTemplateEquipItem(it))
	}
	return st
}

func protoToMobTemplateEquipItem(it *webv1.AdminMobTemplateEquipItem) domain.MobTemplateEquipItem {
	return domain.MobTemplateEquipItem{
		Slot: int16(it.GetSlot()), ItemIndex: it.GetItemIndex(),
		Eff1: uint8(it.GetEff1()), EffV1: uint8(it.GetEffv1()),
		Eff2: uint8(it.GetEff2()), EffV2: uint8(it.GetEffv2()),
		Eff3: uint8(it.GetEff3()), EffV3: uint8(it.GetEffv3()),
	}
}

// mobStatResultToProto maps mobtemplateadmin.Result to the shared AdminResult enum
// (named to avoid colliding with npcadmin's resultToProto in this package).
func mobStatResultToProto(r mobtemplateadmin.Result) webv1.AdminResult {
	switch r {
	case mobtemplateadmin.OK:
		return webv1.AdminResult_ADMIN_RESULT_OK
	case mobtemplateadmin.Forbidden:
		return webv1.AdminResult_ADMIN_RESULT_FORBIDDEN
	case mobtemplateadmin.NotFound:
		return webv1.AdminResult_ADMIN_RESULT_NOT_FOUND
	default:
		return webv1.AdminResult_ADMIN_RESULT_INVALID
	}
}
