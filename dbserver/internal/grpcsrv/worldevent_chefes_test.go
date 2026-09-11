package grpcsrv

import (
	"context"
	"testing"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// TestRenascimentoDosChefesVaiPresente: o campo sai sempre marcado como
// enviado — é a presença que diz ao tmServer que este dbServer já conhece a
// migração 0056.
func TestRenascimentoDosChefesVaiPresente(t *testing.T) {
	st := &fakeWorldEventStore{cfg: domain.WorldEventConfig{BossRespawnHours: 36}}
	resp, err := NewWorldEventConfig(st).GetWorldEventConfig(context.Background(), &dbv1.GetWorldEventConfigRequest{})
	if err != nil {
		t.Fatalf("GetWorldEventConfig: %v", err)
	}
	cfg := resp.GetConfig()
	if cfg.BossRespawnHours == nil || *cfg.BossRespawnHours != 36 {
		t.Errorf("boss_respawn_hours = %v, want presente e 36", cfg.BossRespawnHours)
	}
}
