package grpcsrv

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
)

func TestSaveGuildFameGrava(t *testing.T) {
	fs := &fakeStore{}
	resp, err := New(fs).SaveGuildFame(context.Background(), &dbv1.SaveGuildFameRequest{GuildId: 7, Fame: 1100})
	if err != nil || !resp.GetOk() {
		t.Fatalf("SaveGuildFame = (%v, %v), want ok", resp, err)
	}
	if fs.fama[7] != 1100 {
		t.Errorf("fama gravada = %d, want 1100", fs.fama[7])
	}
}

// TestSaveGuildFameGuildaInexistente: ok=false, não erro — a guilda pode ter sido
// desfeita entre o fim da guerra e a gravação, e isso não é falha do dbServer.
func TestSaveGuildFameGuildaInexistente(t *testing.T) {
	resp, err := New(&fakeStore{}).SaveGuildFame(context.Background(),
		&dbv1.SaveGuildFameRequest{GuildId: guildaInexistente, Fame: 5})
	if err != nil || resp.GetOk() {
		t.Fatalf("SaveGuildFame = (%v, %v), want ok=false sem erro", resp, err)
	}
}

// TestSaveGuildFameRecusaIdForaDoUshort: uint16(65537) é a guilda 1. Truncar
// gravaria fama numa guilda que ninguém mencionou.
func TestSaveGuildFameRecusaIdForaDoUshort(t *testing.T) {
	fs := &fakeStore{}
	for _, id := range []uint32{0, 65536, 65537} {
		_, err := New(fs).SaveGuildFame(context.Background(), &dbv1.SaveGuildFameRequest{GuildId: id, Fame: 5})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("guild id %d: err = %v, want InvalidArgument", id, err)
		}
	}
	if len(fs.fama) != 0 {
		t.Errorf("gravou fama com id inválido: %v", fs.fama)
	}
}
