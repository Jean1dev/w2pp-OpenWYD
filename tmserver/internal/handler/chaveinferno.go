package handler

import "github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"

// A Chave do Inferno não abre porta nenhuma: ela leva quem a usa para dentro do
// Inferno, e some. É o molde do Pedido de Caça (useHuntingScroll) com um destino
// só — decidido com a equipe em 12/09/2026, e por isso um item NOVO, no índice
// 3222, que o catálogo trazia como um dos 78 "Cupom da Sorte" sem uso.
//
// O destino é a entrada da área do Inferno (Imp, Golem e Lagarto do Inferno, com
// o Rei Taurus ao fundo), o ponto das imagens que a equipe mandou. O
// AttributeMap marca a vizinhança inteira como caminhável.
const (
	itemChaveDoInferno = 3222
	chaveInfernoX      = 1960
	chaveInfernoY      = 1592
)

// useChaveInferno gasta uma chave e teleporta o jogador para o Inferno.
//
// A chave é gasta ANTES do teleporte e só depois de o destino ser aceito, na
// ordem do legado para consumíveis de teleporte: um destino recusado devolve o
// slot e a chave continua na bolsa.
func (d *Dispatcher) useChaveInferno(w *world.World, s *world.Session, e *world.Entity, src int) {
	if e.Carry[src].Index != itemChaveDoInferno {
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}
	x, y := int16(chaveInfernoX), int16(chaveInfernoY)
	if fx, fy, ok := w.EmptyCellNear(x, y); ok {
		x, y = fx, fy
	}
	consumeOneItem(&e.Carry[src])
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	d.doTeleport(w, s, x, y)
	d.log.Info("chave do inferno usada", "conn", s.Conn, "account", s.AccountName, "x", x, "y", y)
}
