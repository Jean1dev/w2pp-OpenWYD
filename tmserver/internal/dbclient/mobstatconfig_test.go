package dbclient

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
)

type fakeMobStatAPI struct {
	dbv1.NpcConfigServiceClient // embed so we only need to implement what we use
	resp                        *dbv1.ListMobTemplateStatsResponse
}

func (f *fakeMobStatAPI) ListMobTemplateStats(context.Context, *dbv1.ListMobTemplateStatsRequest, ...grpc.CallOption) (*dbv1.ListMobTemplateStatsResponse, error) {
	return f.resp, nil
}

func TestMobStatSourceFetchMapsByTemplateName(t *testing.T) {
	src := &MobStatSource{api: &fakeMobStatAPI{resp: &dbv1.ListMobTemplateStatsResponse{
		Overrides: []*dbv1.MobTemplateStat{
			{
				TemplateName: "Karkarian", DisplayName: "Karkarian Rebalanceado",
				Clan: 3, Merchant: 0, Class: 1, Coin: 5000, Exp: 123456,
				Spx: 10, Spy: 20, Level: 80, Ac: 40, Damage: 300, ChaosRate: 1,
				AttackRun: 5, Direction: 2, Str: 100, Intel: 10, Dex: 50, Con: 90,
				Special1: 1, Special2: 2, Special3: 3, Special4: 4,
				MaxHp: 50000, Hp: 50000, MaxMp: 1000, Mp: 1000,
				LearnedSkill: 7, ScoreBonus: 3,
				SkillBar1: 1, SkillBar2: 2, SkillBar3: 3, SkillBar4: 4,
				RegenHp: 10, RegenMp: 5,
				Resist1: 1, Resist2: -1, Resist3: 2, Resist4: -2,
				Equip: []*dbv1.MobTemplateEquipItem{
					{Slot: 0, ItemIndex: 1100, Eff1: 1, Effv1: 9},
				},
			},
		},
	}}}

	got, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d overrides, want 1", len(got))
	}
	ov, ok := got["Karkarian"]
	if !ok {
		t.Fatal("missing override for Karkarian")
	}
	if ov.DisplayName != "Karkarian Rebalanceado" || ov.Level != 80 || ov.Hp != 50000 || ov.Exp != 123456 {
		t.Errorf("override = %+v, want DisplayName/Level/Hp/Exp to match", ov)
	}
	if ov.Resist != [4]int8{1, -1, 2, -2} {
		t.Errorf("Resist = %+v, want {1,-1,2,-2}", ov.Resist)
	}
	if len(ov.Equip) != 1 || ov.Equip[0].Slot != 0 || ov.Equip[0].Index != 1100 || ov.Equip[0].Eff[0] != [2]uint8{1, 9} {
		t.Errorf("Equip = %+v, want one slot 0 index 1100 eff {1,9}", ov.Equip)
	}
}
