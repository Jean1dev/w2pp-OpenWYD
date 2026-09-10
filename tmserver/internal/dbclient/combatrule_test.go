package dbclient

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/combatrule"
)

type fakeCombatRuleClient struct {
	dbv1.CombatRuleServiceClient
	resp *dbv1.GetCombatRuleResponse
}

func (f *fakeCombatRuleClient) GetCombatRule(context.Context, *dbv1.GetCombatRuleRequest, ...grpc.CallOption) (*dbv1.GetCombatRuleResponse, error) {
	return f.resp, nil
}

func TestCombatRuleFetch(t *testing.T) {
	tests := []struct {
		name string
		resp *dbv1.GetCombatRuleResponse
		want combatrule.Config
	}{
		{
			name: "configurada chega inteira",
			resp: &dbv1.GetCombatRuleResponse{
				Version: 6, Configured: true,
				WeaponIntMagicPct: 30, SpellDamageMulti: true, MobResistBase: 120,
			},
			want: combatrule.Config{Version: 6, Configured: true, Rules: combatrule.Rules{
				WeaponIntMagicPct: 30, SpellDamageMulti: true, MobResistBase: 120,
			}},
		},
		{
			// Os campos zerados de uma resposta sem regra não são uma regra: a
			// base de resistência 0 está fora da faixa, e o jogo recusaria tudo.
			name: "sem regra gravada é o padrão, e não o valor zero",
			resp: &dbv1.GetCombatRuleResponse{Version: 2},
			want: combatrule.Unconfigured(2),
		},
		{
			// E o padrão mesmo que os campos venham preenchidos com outra coisa.
			name: "configured=false ignora os campos",
			resp: &dbv1.GetCombatRuleResponse{Version: 3, WeaponIntMagicPct: 100, MobResistBase: 150},
			want: combatrule.Unconfigured(3),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := &CombatRuleSource{api: &fakeCombatRuleClient{resp: tt.resp}}
			got, err := src.Fetch(context.Background())
			if err != nil {
				t.Fatalf("Fetch: %v", err)
			}
			if got != tt.want {
				t.Errorf("Fetch = %+v, want %+v", got, tt.want)
			}
		})
	}
}
