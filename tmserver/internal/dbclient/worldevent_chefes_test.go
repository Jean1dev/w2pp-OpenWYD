package dbclient

import (
	"context"
	"testing"

	"google.golang.org/protobuf/proto"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
)

// TestRenascimentoDosChefesPresencaNoFio: ausente é um dbServer de antes da
// migração 0056 e tem de rodar o padrão decidido (24 h), nunca o zero — que
// traria os chefes de volta a cada 15 s durante um deploy.
func TestRenascimentoDosChefesPresencaNoFio(t *testing.T) {
	tests := []struct {
		nome string
		cfg  *dbv1.WorldEventConfig
		quer int32
	}{
		{"ausente é o padrão decidido", &dbv1.WorldEventConfig{NoticeEnabled: true}, 24},
		{"sem config nenhuma é o padrão decidido", nil, 24},
		{"presente vale como veio", &dbv1.WorldEventConfig{BossRespawnHours: proto.Int32(6)}, 6},
	}
	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			c := &WorldEventConfig{api: &fakeWorldEventAPI{snapshotResp: &dbv1.GetWorldEventConfigResponse{
				Version: 1, Config: tt.cfg,
			}}}
			snap, err := c.Snapshot(context.Background())
			if err != nil {
				t.Fatalf("Snapshot: %v", err)
			}
			if snap.Event.BossRespawnHours != tt.quer {
				t.Errorf("chefes voltam em %d h, want %d h", snap.Event.BossRespawnHours, tt.quer)
			}
		})
	}
}
