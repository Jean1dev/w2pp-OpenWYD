package dbclient

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

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
				PvpSkillPct: 60, PvpMeleePct: 80,
				SpellIntAccuracyPct: proto.Int32(30), MaxMissStreak: proto.Int32(4),
			},
			want: combatrule.Config{Version: 6, Configured: true, Rules: combatrule.Rules{
				WeaponIntMagicPct: 30, SpellDamageMulti: true, MobResistBase: 120,
				PvPSkillPct: 60, PvPMeleePct: 80,
				SpellIntAccuracyPct: 30, MaxMissStreak: 4,
			}},
		},
		{
			// Um dbServer anterior aos campos de PvP não os manda, e o zero que
			// chega está fora da faixa: passado adiante, invalidaria a regra
			// inteira e o jogo voltaria ao padrão com a regra gravada no painel.
			// Ele também é anterior aos de precisão, que chegam ausentes e viram o
			// padrão decidido — o mesmo que a 0046 dá a uma linha antiga.
			name: "PvP ausente é o legado, não zero",
			resp: &dbv1.GetCombatRuleResponse{
				Version: 7, Configured: true,
				WeaponIntMagicPct: 100, SpellDamageMulti: true, MobResistBase: 150,
			},
			want: combatrule.Config{Version: 7, Configured: true, Rules: kersefComPrecisaoPadrao()},
		},
		{
			// Um dbServer com o PvP e sem a precisão: os dois campos ausentes são o
			// padrão decidido (50%, 2), não o zero do legado.
			name: "precisão ausente é o padrão decidido",
			resp: &dbv1.GetCombatRuleResponse{
				Version: 8, Configured: true,
				WeaponIntMagicPct: 100, SpellDamageMulti: true, MobResistBase: 150,
				PvpSkillPct: 100, PvpMeleePct: 100,
			},
			want: combatrule.Config{Version: 8, Configured: true, Rules: kersefComPrecisaoPadrao()},
		},
		{
			// E um 0 PRESENTE é o legado gravado de propósito — o atalho do Kersef.
			// Lido como ausente, ele viraria 50% e 2 e o Kersef não chegaria ao jogo.
			name: "precisão presente em zero é o legado, não o padrão",
			resp: &dbv1.GetCombatRuleResponse{
				Version: 9, Configured: true,
				WeaponIntMagicPct: 100, SpellDamageMulti: true, MobResistBase: 150,
				PvpSkillPct: 100, PvpMeleePct: 100,
				SpellIntAccuracyPct: proto.Int32(0), MaxMissStreak: proto.Int32(0),
			},
			want: combatrule.Config{Version: 9, Configured: true, Rules: combatrule.Kersef()},
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

// kersefComPrecisaoPadrao is Kersef as read from a dbServer that predates the
// precision fields: everything it sent, plus the decided default for the two it
// did not.
func kersefComPrecisaoPadrao() combatrule.Rules {
	r := combatrule.Kersef()
	r.SpellIntAccuracyPct = combatrule.Default().SpellIntAccuracyPct
	r.MaxMissStreak = combatrule.Default().MaxMissStreak
	return r
}
