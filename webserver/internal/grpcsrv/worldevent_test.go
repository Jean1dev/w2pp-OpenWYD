package grpcsrv

import (
	"context"
	"testing"

	"google.golang.org/protobuf/proto"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/worldevent"
)

// fakeWorldEventAdmin answers Get with a stored config and records what Set
// was asked to write.
type fakeWorldEventAdmin struct {
	guardado domain.WorldEventConfig
	getRes   worldevent.Result
	gravado  *domain.WorldEventConfig
}

func (f *fakeWorldEventAdmin) Get(context.Context, int64) (worldevent.Result, int64, domain.WorldEventConfig, error) {
	return f.getRes, 3, f.guardado, nil
}

func (f *fakeWorldEventAdmin) Set(_ context.Context, _ int64, cfg domain.WorldEventConfig) (worldevent.Result, error) {
	f.gravado = &cfg
	return worldevent.OK, nil
}

// TestSetSemCamposDaTorreGuardaOQueEstava is the rolling-deploy hazard on the
// write path: a portal that predates the Tower War fields sends neither, and a
// plain replace would switch the daily war off at midnight whenever somebody
// saved double EXP.
func TestSetSemCamposDaTorreGuardaOQueEstava(t *testing.T) {
	adm := &fakeWorldEventAdmin{guardado: domain.WorldEventConfig{TowerWarEnabled: true, TowerWarHour: 21}}
	ack, err := NewWorldEventAdmin(adm).SetWorldEventConfig(context.Background(), &webv1.SetWorldEventConfigRequest{
		ModeratorId: 1, Config: &webv1.WorldEventConfig{DoubleExpEnabled: true},
	})
	if err != nil || ack.GetResult() != webv1.AdminResult_ADMIN_RESULT_OK {
		t.Fatalf("SetWorldEventConfig = (%v, %v), want OK", ack, err)
	}
	if adm.gravado == nil {
		t.Fatal("nada foi gravado")
	}
	if !adm.gravado.DoubleExpEnabled {
		t.Error("o XP em dobro pedido não foi gravado")
	}
	if !adm.gravado.TowerWarEnabled || adm.gravado.TowerWarHour != 21 {
		t.Errorf("guerra de torres gravada = %v às %dh, want a guardada (ligada às 21h)",
			adm.gravado.TowerWarEnabled, adm.gravado.TowerWarHour)
	}
}

// TestSetComCamposDaTorreGravaOPedido: presente e zerado é uma escolha — desligar
// a guerra, ou pô-la à meia-noite — e tem de ser gravada como veio.
func TestSetComCamposDaTorreGravaOPedido(t *testing.T) {
	adm := &fakeWorldEventAdmin{guardado: domain.WorldEventConfig{TowerWarEnabled: true, TowerWarHour: 21}}
	_, err := NewWorldEventAdmin(adm).SetWorldEventConfig(context.Background(), &webv1.SetWorldEventConfigRequest{
		ModeratorId: 1,
		Config:      &webv1.WorldEventConfig{TowerWarEnabled: proto.Bool(false), TowerWarHour: proto.Int32(0)},
	})
	if err != nil || adm.gravado == nil {
		t.Fatalf("SetWorldEventConfig err = %v, gravado = %v", err, adm.gravado)
	}
	if adm.gravado.TowerWarEnabled || adm.gravado.TowerWarHour != 0 {
		t.Errorf("guerra de torres gravada = %v às %dh, want desligada às 0h", adm.gravado.TowerWarEnabled, adm.gravado.TowerWarHour)
	}
}

// TestSetSemCamposNaoGravaQuandoALeituraRecusa: to keep what is stored the
// server has to read it first, under the same authorization; a refusal there
// answers the caller and writes nothing.
func TestSetSemCamposNaoGravaQuandoALeituraRecusa(t *testing.T) {
	adm := &fakeWorldEventAdmin{getRes: worldevent.Forbidden}
	ack, err := NewWorldEventAdmin(adm).SetWorldEventConfig(context.Background(), &webv1.SetWorldEventConfigRequest{
		ModeratorId: 9, Config: &webv1.WorldEventConfig{},
	})
	if err != nil || ack.GetResult() != webv1.AdminResult_ADMIN_RESULT_FORBIDDEN {
		t.Fatalf("SetWorldEventConfig = (%v, %v), want FORBIDDEN", ack, err)
	}
	if adm.gravado != nil {
		t.Error("gravou apesar da recusa")
	}
}

func TestGetMandaATorrePresente(t *testing.T) {
	adm := &fakeWorldEventAdmin{guardado: domain.WorldEventConfig{TowerWarEnabled: false, TowerWarHour: 0}}
	resp, err := NewWorldEventAdmin(adm).GetWorldEventConfig(context.Background(), &webv1.GetWorldEventConfigRequest{ModeratorId: 1})
	if err != nil {
		t.Fatalf("GetWorldEventConfig: %v", err)
	}
	cfg := resp.GetConfig()
	if cfg.TowerWarEnabled == nil || cfg.TowerWarHour == nil {
		t.Fatalf("guerra de torres veio ausente: %+v", cfg)
	}
	if cfg.GetTowerWarEnabled() || cfg.GetTowerWarHour() != 0 {
		t.Errorf("guerra de torres = %v às %dh, want desligada às 0h", cfg.GetTowerWarEnabled(), cfg.GetTowerWarHour())
	}
}
