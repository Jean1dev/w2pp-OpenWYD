package grpcsrv

import (
	"context"
	"testing"

	"google.golang.org/protobuf/proto"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// TestSetSemHorasDosChefesGuardaOQueEstava: um portal que ainda não conhece o
// campo manda a guerra de torres e não manda as horas; gravar zero traria os
// chefes a cada 15 s (e o banco recusaria). Ausente mantém o que está gravado.
func TestSetSemHorasDosChefesGuardaOQueEstava(t *testing.T) {
	adm := &fakeWorldEventAdmin{guardado: domain.WorldEventConfig{TowerWarEnabled: true, TowerWarHour: 21, BossRespawnHours: 48}}
	_, err := NewWorldEventAdmin(adm).SetWorldEventConfig(context.Background(), &webv1.SetWorldEventConfigRequest{
		ModeratorId: 1,
		Config:      &webv1.WorldEventConfig{TowerWarEnabled: proto.Bool(true), TowerWarHour: proto.Int32(22)},
	})
	if err != nil || adm.gravado == nil {
		t.Fatalf("SetWorldEventConfig err = %v, gravado = %v", err, adm.gravado)
	}
	if adm.gravado.BossRespawnHours != 48 {
		t.Errorf("chefes gravados em %d h, want os 48 h guardados", adm.gravado.BossRespawnHours)
	}
	if adm.gravado.TowerWarHour != 22 {
		t.Errorf("guerra de torres gravada às %dh, want as 22h pedidas", adm.gravado.TowerWarHour)
	}
}

// TestSetComHorasDosChefesGravaOPedido: presente vale como veio, e a leitura
// devolve o campo sempre presente.
func TestSetComHorasDosChefesGravaOPedido(t *testing.T) {
	adm := &fakeWorldEventAdmin{guardado: domain.WorldEventConfig{BossRespawnHours: 48}}
	_, err := NewWorldEventAdmin(adm).SetWorldEventConfig(context.Background(), &webv1.SetWorldEventConfigRequest{
		ModeratorId: 1,
		Config: &webv1.WorldEventConfig{
			TowerWarEnabled: proto.Bool(true), TowerWarHour: proto.Int32(20), BossRespawnHours: proto.Int32(12),
		},
	})
	if err != nil || adm.gravado == nil {
		t.Fatalf("SetWorldEventConfig err = %v, gravado = %v", err, adm.gravado)
	}
	if adm.gravado.BossRespawnHours != 12 {
		t.Errorf("chefes gravados em %d h, want 12 h", adm.gravado.BossRespawnHours)
	}
	if got := worldEventConfigToWebProto(domain.WorldEventConfig{BossRespawnHours: 7}); got.BossRespawnHours == nil || *got.BossRespawnHours != 7 {
		t.Errorf("leitura = %v, want presente e 7", got.BossRespawnHours)
	}
}
