package dbclient

import (
	"context"
	"testing"
)

func TestSaveGuildFameMandaIdEFama(t *testing.T) {
	api := &fakeAPI{}
	if err := newClient(api).SaveGuildFame(context.Background(), 7, 1100); err != nil {
		t.Fatalf("SaveGuildFame: %v", err)
	}
	if api.famaReq == nil || api.famaReq.GetGuildId() != 7 || api.famaReq.GetFame() != 1100 {
		t.Errorf("pedido = %+v, want guilda 7 com fama 1100", api.famaReq)
	}
}

// TestSaveGuildFameRecusadaViraErro: ok=false (a guilda não existe mais) volta
// como erro, para quem chamou poder registrar que a fama não foi gravada — calado,
// ela sumiria de novo no próximo reinício sem ninguém saber por quê.
func TestSaveGuildFameRecusadaViraErro(t *testing.T) {
	api := &fakeAPI{famaRecusada: true}
	if err := newClient(api).SaveGuildFame(context.Background(), 7, 1100); err == nil {
		t.Error("ok=false voltou sem erro")
	}
}
