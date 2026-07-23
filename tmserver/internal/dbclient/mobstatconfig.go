package dbclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/mobstat"
)

// MobStatSource fetches the moderator-edited mob/NPC template stat overrides
// (mob-template-editing-plan.md) from the dbServer's NpcConfigService. It is
// deliberately independent of NpcConfig/npccfg: a stat override applies to ANY
// npc/<template_name> file, not just the DB-managed merchant subset NpcConfig
// materializes, and is fetched once at boot (no version poll — no hot-reload
// for this feature, matching EDITAPPMOB's own restart-to-apply behavior).
type MobStatSource struct {
	api dbv1.NpcConfigServiceClient
}

// NewMobStatSource wraps a gRPC connection as a MobStatSource.
func NewMobStatSource(conn grpc.ClientConnInterface) *MobStatSource {
	return &MobStatSource{api: dbv1.NewNpcConfigServiceClient(conn)}
}

// Fetch returns every template stat override, keyed by template_name, ready
// to apply over raw template bytes via mobstat.Apply.
func (c *MobStatSource) Fetch(ctx context.Context) (map[string]mobstat.Override, error) {
	resp, err := c.api.ListMobTemplateStats(ctx, &dbv1.ListMobTemplateStatsRequest{})
	if err != nil {
		return nil, fmt.Errorf("dbclient: list mob template stats: %w", err)
	}
	out := make(map[string]mobstat.Override, len(resp.GetOverrides()))
	for _, st := range resp.GetOverrides() {
		ov := mobstat.Override{
			DisplayName: st.GetDisplayName(),
			Clan:        uint8(st.GetClan()), Merchant: uint8(st.GetMerchant()), Class: uint8(st.GetClass()),
			Coin: st.GetCoin(), Exp: st.GetExp(), SPX: int16(st.GetSpx()), SPY: int16(st.GetSpy()),
			Level: st.GetLevel(), AC: st.GetAc(), Damage: st.GetDamage(), ChaosRate: uint8(st.GetChaosRate()),
			AttackRun: uint8(st.GetAttackRun()), Direction: uint8(st.GetDirection()),
			Str: int16(st.GetStr()), Int: int16(st.GetIntel()), Dex: int16(st.GetDex()), Con: int16(st.GetCon()),
			Special: [4]int16{int16(st.GetSpecial1()), int16(st.GetSpecial2()), int16(st.GetSpecial3()), int16(st.GetSpecial4())},
			MaxHp:   st.GetMaxHp(), Hp: st.GetHp(), MaxMp: st.GetMaxMp(), Mp: st.GetMp(),
			LearnedSkill: st.GetLearnedSkill(), ScoreBonus: uint16(st.GetScoreBonus()),
			SkillBar: [4]uint8{uint8(st.GetSkillBar1()), uint8(st.GetSkillBar2()), uint8(st.GetSkillBar3()), uint8(st.GetSkillBar4())},
			RegenHP:  uint16(st.GetRegenHp()), RegenMP: uint16(st.GetRegenMp()),
			Resist: [4]int8{int8(st.GetResist1()), int8(st.GetResist2()), int8(st.GetResist3()), int8(st.GetResist4())},
		}
		for _, it := range st.GetEquip() {
			ov.Equip = append(ov.Equip, mobstat.EquipItem{
				Slot: int(it.GetSlot()), Index: uint16(it.GetItemIndex()),
				Eff: [3][2]uint8{
					{uint8(it.GetEff1()), uint8(it.GetEffv1())},
					{uint8(it.GetEff2()), uint8(it.GetEffv2())},
					{uint8(it.GetEff3()), uint8(it.GetEffv3())},
				},
			})
		}
		out[st.GetTemplateName()] = ov
	}
	return out, nil
}
