package handler

import (
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestCombineVariantRoutesInstalled(t *testing.T) {
	d := New(Config{Log: slog.New(slog.DiscardHandler)})
	for _, ty := range []protocol.Type{protocol.MsgCombineItemAilyn, protocol.MsgCombineItemEhre, protocol.MsgCombineItemTiny, protocol.MsgCombineItemShany, protocol.MsgCombineItemAgatha, protocol.MsgCombineItemLindy, protocol.MsgCombineItemAlquimia} {
		if d.routes[ty] == nil {
			t.Errorf("combine route %#x not installed", ty)
		}
	}
}

func TestEhreEffectAndSoulResults(t *testing.T) {
	it := world.Item{}
	ehreAddEffect(&it, 70, 2, 20)
	ehreAddEffect(&it, 70, 20, 20)
	if it.Effects[1] != (world.Effect{Effect: 70, Value: 20}) {
		t.Fatalf("effect=%+v, want capped MP+20", it.Effects[1])
	}
	if got := ehreSoul(2441, 2442, 2443); got != 10 {
		t.Fatalf("soul=%d, want SOUL_ID(10)", got)
	}
}
