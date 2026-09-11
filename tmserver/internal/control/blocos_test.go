package control

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	gamev1 "github.com/jeanluca/w2pp-openwyd/api/game/v1"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// mundoComBlocos is mundoRodando with two NPCGener blocks, the first raised.
func mundoComBlocos(t *testing.T) *world.World {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	w := world.New(world.Config{GridDim: 64}, log, world.NopPersistence{}, nil)
	tmpl := func(name string) []byte {
		b := make([]byte, 816)
		copy(b, name)
		b[92+16], b[92+24] = 100, 100 // MaxHp, Hp
		return b
	}
	w.RegisterGenerators([]*world.Generator{
		{Name: "Torre_de_Thor", MaxNumMob: 1, SegX: [5]int16{10}, SegY: [5]int16{10}, LeaderTmpl: tmpl("Torre")},
		{Name: "Kefra", MaxNumMob: 1, SegX: [5]int16{50}, SegY: [5]int16{50}, LeaderTmpl: tmpl("Kefra")},
	})
	w.GenerateMob(0)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); _ = w.Serve(ctx, ln) }()
	t.Cleanup(func() { cancel(); <-done })
	return w
}

func servidorComBlocos(t *testing.T) *Server {
	t.Helper()
	s, err := NewServer(mundoComBlocos(t), tokenDeTeste,
		slog.New(slog.NewTextHandler(io.Discard, nil)), teleporteNulo, Overlays{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return s
}

func TestListBlocksAchaPorNomeNumeroEPerto(t *testing.T) {
	s := servidorComBlocos(t)
	ctx := context.Background()

	resp, err := s.ListBlocks(ctx, &gamev1.ListBlocksRequest{Name: "thor"})
	if err != nil {
		t.Fatalf("ListBlocks: %v", err)
	}
	if resp.GetTotal() != 1 || resp.GetBlocks()[0].GetIndex() != 0 || resp.GetBlocks()[0].GetAlive() != 1 {
		t.Fatalf("busca por nome = %+v, want o bloco 0 com 1 vivo", resp)
	}

	// 0 is a real block: an index that is set, even to 0, narrows; absent is any.
	um := int32(1)
	resp, _ = s.ListBlocks(ctx, &gamev1.ListBlocksRequest{Index: &um})
	if len(resp.GetBlocks()) != 1 || resp.GetBlocks()[0].GetName() != "Kefra" || resp.GetBlocks()[0].GetAlive() != 0 {
		t.Errorf("busca pelo número 1 = %+v", resp.GetBlocks())
	}
	resp, _ = s.ListBlocks(ctx, &gamev1.ListBlocksRequest{})
	if resp.GetTotal() != 2 {
		t.Errorf("sem filtro achou %d, want 2", resp.GetTotal())
	}

	resp, _ = s.ListBlocks(ctx, &gamev1.ListBlocksRequest{X: 48, Y: 48, Radius: 5})
	if len(resp.GetBlocks()) != 1 || resp.GetBlocks()[0].GetIndex() != 1 {
		t.Errorf("perto de (48,48) = %+v, want só o Kefra", resp.GetBlocks())
	}
}

func TestBlockCommandSoAbreOsComandosDeBloco(t *testing.T) {
	s := servidorComBlocos(t)
	ctx := context.Background()

	if _, err := s.BlockCommand(ctx, &gamev1.BlockCommandRequest{Line: "npc"}); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("sem runner: %v, want FailedPrecondition", err)
	}

	var got struct {
		by, line string
		x, y     int16
	}
	s.SetBlockRunner(func(_ *world.World, by string, x, y int16, line string) []string {
		got.by, got.line, got.x, got.y = by, line, x, y
		return []string{"ok"}
	})

	// The door opens onto the block commands only, not the whole GM bus.
	for _, linha := range []string{"kick Fulano", "setgold 999", ""} {
		if _, err := s.BlockCommand(ctx, &gamev1.BlockCommandRequest{Line: linha}); status.Code(err) != codes.InvalidArgument {
			t.Errorf("%q: %v, want InvalidArgument", linha, err)
		}
	}

	resp, err := s.BlockCommand(ctx, &gamev1.BlockCommandRequest{Line: " gerar 1 aqui ", X: 20, Y: 21, By: "admin"})
	if err != nil {
		t.Fatalf("BlockCommand: %v", err)
	}
	if len(resp.GetLines()) != 1 || got.line != "gerar 1 aqui" || got.by != "admin" || got.x != 20 || got.y != 21 {
		t.Errorf("runner recebeu %+v, resposta %v", got, resp.GetLines())
	}

	resp, _ = s.BlockCommand(ctx, &gamev1.BlockCommandRequest{Line: "criar Kefra", X: 999, Y: 5})
	if len(resp.GetLines()) != 1 || resp.GetLines()[0] != "Coordenada fora do mapa." {
		t.Errorf("fora do mapa: %v", resp.GetLines())
	}
}
