package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Os três portões do campo de treino (InitItem.csv:20-22): Primeira_Porta 458 em
// (2075,2015), Segunda_Porta 459 em (2143,1985) e Última_Porta 461 em
// (2081,1961). Cada um tem EF_KEYID 2, 3 e 4, que são as chaves 451, 452 e 453 —
// as mesmas que os três chefes do campo derrubam e que os Treinadores pedem
// (treinador.go).
//
// No legado eles nascem TRANCADOS (Server.cpp:7961-7980), são desenhados para
// quem chega perto (SendFunc.cpp:862) e voltam a trancar no timer de minuto
// (ProcessSecMinTimer.cpp:2650-2688, que relocka InitItem >= 17). Aqui todo
// portão nascia aberto e nenhum era desenhado, então o campo era um corredor sem
// portas: a chave não servia para nada e o "sem portão" do relato era literal.
//
// Escopo: só estes três. Ligar de uma vez todos os portões do InitItem fecharia
// portas pelo mapa inteiro que ninguém pediu — a mesma razão que deixou o Portão
// Orc Sul sozinho até agora (castelo_orc_gate.go). O desenho e o corpo do pacote
// são os dele, reusados.
//
// UNVERIFIED, como no Portão Orc: que o cliente 7662 impeça a passagem só com
// estes pacotes. Quem levanta o chão sob o portão é o cliente; o servidor não tem
// checagem de altura no movimento do jogador.
var portoesDoCampo = [3]struct {
	item int16
	x, y int16
}{
	{458, 2075, 2015},
	{459, 2143, 1985},
	{461, 2081, 1961},
}

// msgSemChave é _NN_No_Key (Language.txt:79), a fala que o legado dá a quem
// tenta um portão trancado sem a chave. O caminho genérico responde com um
// notify numérico, que o cliente não mostra.
const msgSemChave = "Você não possui uma chave."

// portoesDoCampoIDs devolve os ids de chão dos três portões, procurando-os na
// primeira chamada e deixando-os trancados — o boot semeia todo portão aberto
// (main.go seedWorldItems). Um portão ausente do InitItem fica com id -1.
func (d *Dispatcher) portoesDoCampoIDs(w *world.World) [3]int {
	if d.portoesCampoIDs[0] == 0 {
		for i := range d.portoesCampoIDs {
			d.portoesCampoIDs[i] = -1
		}
		w.ForEachStaticItem(func(g *world.GroundItem) {
			for i, p := range portoesDoCampo {
				if d.portoesCampoIDs[i] < 0 && g.Item.Index == p.item && g.X == p.x && g.Y == p.y {
					d.portoesCampoIDs[i] = g.ID
					g.State = world.StateLocked
				}
			}
		})
	}
	return d.portoesCampoIDs
}

// ehPortaoDoCampo diz se este id de chão é um dos três — o que faz a recusa sem
// chave falar em vez de calar (gate.go).
func (d *Dispatcher) ehPortaoDoCampo(w *world.World, id int) bool {
	for _, gid := range d.portoesDoCampoIDs(w) {
		if gid == id {
			return true
		}
	}
	return false
}

// syncPortoesDoCampo desenha para s os portões que entram na visão dele e retira
// os que saem — a metade de itens do GridMulticast, igual à do Portão Orc.
func (d *Dispatcher) syncPortoesDoCampo(w *world.World, s *world.Session, x, y int16) {
	if s == nil {
		return
	}
	for _, id := range d.portoesDoCampoIDs(w) {
		g := w.GroundItem(id)
		if g == nil {
			continue
		}
		key := gateSeenKey(g.ID)
		if chebyshev(x, y, g.X, g.Y) <= world.ViewRange {
			if w.MarkSeen(s, key) {
				w.SendTo(s, protocol.Header{Type: protocol.MsgCreateItem, ID: protocol.IDScene}, d.gateCreateBody(g))
			}
			continue
		}
		if w.Seen(s, key) {
			w.UnmarkSeen(s, key)
			w.SendTo(s, protocol.Header{Type: protocol.MsgDecayItem, ID: protocol.IDScene},
				protocol.EncodeDecayItemBody(uint16(world.GroundItemIDOffset+g.ID)))
		}
	}
}

// tickPortoesDoCampo volta a trancar, no minuto, o portão que alguém abriu — o
// relock do timer de minuto do legado. Sem ele a primeira chave abriria o campo
// para sempre, e a chave dos outros jogadores não valeria mais nada.
func (d *Dispatcher) tickPortoesDoCampo(w *world.World) {
	if d.tickCount%minutoTicks != 0 {
		return
	}
	for _, id := range d.portoesDoCampoIDs(w) {
		g := w.GroundItem(id)
		if g == nil || g.State == world.StateLocked {
			continue
		}
		g.State = world.StateLocked
		key := gateSeenKey(g.ID)
		// Trancar vai como CreateItem: o cliente já desenhou o portão aberto, e é
		// o CreateItem que carrega a altura que fecha a passagem (gateCreateBody).
		w.ForEachInViewAt(g.X, g.Y, -1, func(s *world.Session, _ *world.Entity) {
			w.MarkSeen(s, key)
			w.SendTo(s, protocol.Header{Type: protocol.MsgCreateItem, ID: protocol.IDScene}, d.gateCreateBody(g))
		})
		d.log.Info("portão do campo de treino trancado de novo", "gate", g.ID, "x", g.X, "y", g.Y)
	}
}
