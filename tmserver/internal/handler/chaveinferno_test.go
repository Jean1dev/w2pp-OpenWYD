package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestChaveDoInfernoTeleporta: a chave leva para o Inferno e some da bolsa.
func TestChaveDoInfernoTeleporta(t *testing.T) {
	db := carryDB(world.Item{Index: itemChaveDoInferno})
	addr, stop := startServerClockVol(t, db, map[int]int{itemChaveDoInferno: volChaveInferno})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useItemFrame(t, c, 0)
	payload := expect(t, c, protocol.MsgSendItem)
	if got := le16(payload[4:6]); got != 0 {
		t.Errorf("a chave continuou na bolsa (item %d), want slot vazio", got)
	}
	action := expectAction(t, c)
	if action.TargetX != chaveInfernoX || action.TargetY != chaveInfernoY {
		t.Errorf("teleporte para (%d,%d), want o Inferno (%d,%d)",
			action.TargetX, action.TargetY, chaveInfernoX, chaveInfernoY)
	}
}

// TestChaveDoInfernoSoOItemCerto: o consumível 245 é desta chave e de mais
// nada. Um item com o mesmo código e outro índice devolve o slot e não
// teleporta — a guarda existe porque o código do consumível vem do catálogo,
// que a Mesa de Itens do painel pode editar.
func TestChaveDoInfernoSoOItemCerto(t *testing.T) {
	const outro = 3223
	db := carryDB(world.Item{Index: outro})
	addr, stop := startServerClockVol(t, db, map[int]int{outro: volChaveInferno})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useItemFrame(t, c, 0)
	payload := expect(t, c, protocol.MsgSendItem)
	if got := le16(payload[4:6]); got != outro {
		t.Errorf("slot devolvido com o item %d, want %d intacto", got, outro)
	}
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("houve resposta %#x depois da recusa, want nenhuma (sem teleporte)", ty)
	}
}
