package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// O Portão Orc Leste — InitItem 463, EF_KEYID 6, no arco em (2518,2106) — é a
// segunda entrada do Castelo Orc, e ficava ABERTA.
//
// O Portão Sul (462) já nasce trancado e desenhado, e nunca abre: a corrida
// entra por teleporte, e uma porta aberta deixaria um segundo grupo entrar atrás
// do primeiro (castelo_orc_gate.go, regra da equipe de 11/09/2026). O Leste
// nascia aberto e nem era desenhado — como todo InitItem que este servidor não
// trata —, então bastava andar até o arco para entrar no castelo a pé, sem
// corrida nenhuma. Fechá-lo é o que faltava para a regra valer.
//
// Ele nunca abre, pela mesma razão do Sul, e isso não tira nada de ninguém: a
// chave dele (466, Chave Portão Orc Leste) não tem regra de drop nem código —
// não existe em jogo.
//
// Os dois portões INTERNOS ficam como estão, de propósito: o 464 (Último Portão
// Orc Sul, 2504,2145) e o 468 (Último Portão Orc Leste, 2528,2134) estão entre a
// entrada da corrida (2494,2128) e o Grão Lorde (2533,2120) / o Chefe
// (2533,2157). As chaves deles (467 e 469) também não existem, então trancá-los
// deixaria o grupo dentro do castelo e sem passagem até o boss.
//
// UNVERIFIED, como no Sul: que o cliente 7662 impeça a passagem só com estes
// pacotes. Ele desenha o portão a partir deles (confirmado em jogo em 11/09);
// quem levanta o chão sob a porta é o cliente, e o servidor não tem checagem de
// altura no movimento do jogador.
const itemPortaoOrcLeste = 463

var casteloOrcLestePos = [2]int16{2518, 2106}

// msgPortaoOrcLesteTrancado é o que o clique responde. O legado abriria com a
// chave; aqui a porta é fechada para sempre, e dizer isso é melhor do que o
// código numérico do notify, que o cliente não desenha.
const msgPortaoOrcLesteTrancado = "O Portão Orc Leste está trancado. A entrada é pela quest do Castelo Orc."

// casteloOrcPortaoLeste devolve o Portão Orc Leste, procurando-o na primeira
// chamada e deixando-o trancado — o boot semeia todo portão do InitItem aberto
// (main.go seedWorldItems). nil quando o InitItem não o semeou (testes, montagem
// quebrada).
func (d *Dispatcher) casteloOrcPortaoLeste(w *world.World) *world.GroundItem {
	if d.casteloOrcLesteID == 0 {
		d.casteloOrcLesteID = -1
		w.ForEachStaticItem(func(g *world.GroundItem) {
			if d.casteloOrcLesteID < 0 && g.Item.Index == itemPortaoOrcLeste &&
				g.X == casteloOrcLestePos[0] && g.Y == casteloOrcLestePos[1] {
				d.casteloOrcLesteID = g.ID
			}
		})
		if g := w.GroundItem(d.casteloOrcLesteID); g != nil {
			g.State = world.StateLocked
		}
	}
	if d.casteloOrcLesteID < 0 {
		return nil
	}
	return w.GroundItem(d.casteloOrcLesteID)
}

// syncCasteloOrcLeste desenha o portão para quem chega perto e o retira de quem
// se afasta — a metade de itens do GridMulticast, igual à do Portão Sul.
func (d *Dispatcher) syncCasteloOrcLeste(w *world.World, s *world.Session, x, y int16) {
	g := d.casteloOrcPortaoLeste(w)
	if g == nil || s == nil {
		return
	}
	key := gateSeenKey(g.ID)
	if chebyshev(x, y, g.X, g.Y) <= world.ViewRange {
		if w.MarkSeen(s, key) {
			w.SendTo(s, protocol.Header{Type: protocol.MsgCreateItem, ID: protocol.IDScene}, d.gateCreateBody(g))
		}
		return
	}
	if w.Seen(s, key) {
		w.UnmarkSeen(s, key)
		w.SendTo(s, protocol.Header{Type: protocol.MsgDecayItem, ID: protocol.IDScene},
			protocol.EncodeDecayItemBody(uint16(world.GroundItemIDOffset+g.ID)))
	}
}
