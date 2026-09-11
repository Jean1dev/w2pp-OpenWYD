package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// A loja do Unicórnio Puro vende armas por Emblema Orc em vez de gold. Regra
// nossa, pedida em 2026-09-11: nem o legado nem o port tinham loja cobrada em
// item. No legado o Unicórnio Puro (NPCGener #3748, ao lado do campo de treino)
// é Merchant 110, que nenhum handler trata: o clique não fazia nada.
const (
	// merchantUnicornioPuro é o Merchant do template Unicornio_Puro. Nenhum
	// outro template do jogo usa 110, e é por ele, não pelo nome, que a loja é
	// reconhecida: o painel pode trocar o nome exibido de um NPC.
	merchantUnicornioPuro = 110

	// emblemaOrc é o item 524, Emblema_Orc. No legado só serve de entrega da
	// quest de novato 4 (_MSG_Quest.cpp:2058).
	emblemaOrc = 524

	// precoEmEmblemas é quanto cada arma custa.
	precoEmEmblemas = 1
)

// armasDoUnicornioPuro é o estoque padrão: a família do 939, toda sem requisito
// de nível nem de atributo (ItemList.csv 938-945), +0 e sem adicional.
var armasDoUnicornioPuro = [...]int16{
	938, // Martelo de Pedra
	939, // Katana
	940, // Cajado Orc
	941, // Foice do Mago
	942, // Foice de Argos
	943, // Arco de Caveira
	944, // Lança de Taurus
	945, // Tsurugi
}

const (
	msgFaltaEmblema   = "Você precisa de 1 Emblema Orc para comprar esta arma."
	msgCompraEmblema  = "Comprado por 1 Emblema Orc."
	merchantDeLojaCli = 1 // o Merchant que faz o cliente abrir a janela de loja
)

// ehLojaDeEmblema diz se npc é a loja do Unicórnio Puro.
func ehLojaDeEmblema(npc *world.Entity) bool {
	return npc != nil && npc.Merchant == merchantUnicornioPuro
}

// merchantParaOCliente é o Merchant que o CreateMob anuncia. O cliente decide
// pelo Merchant o que o clique manda: 1 abre a janela de loja (_MSG_REQShopList),
// e 110 cairia em _MSG_Quest, que ninguém trata. O servidor continua vendo 110,
// que é como a loja de emblema é reconhecida.
func merchantParaOCliente(e *world.Entity) uint8 {
	if ehLojaDeEmblema(e) {
		return merchantDeLojaCli
	}
	return e.Merchant
}

// abastecerLojaDeEmblema põe o estoque padrão no Unicórnio Puro quando ele não
// tem nenhum. Com -npc-editing ligado, que é o estado de produção, o Carry do
// template não chega ao jogo: o applyShop zera o Carry e escreve o que está no
// npc_shop_item. Por isso o estoque não pode morar no arquivo do template. Se a
// equipe cadastrar outro estoque pelo painel, vale o do painel.
func abastecerLojaDeEmblema(npc *world.Entity) {
	if !ehLojaDeEmblema(npc) {
		return
	}
	for _, it := range npc.Carry {
		if it.Index != 0 {
			return
		}
	}
	for i, idx := range armasDoUnicornioPuro {
		npc.Carry[protocol.ShopSlot(i)] = world.Item{Index: idx}
	}
}

// slotDoEmblema é a primeira casa acessível da mochila com um Emblema Orc, ou -1.
func slotDoEmblema(e *world.Entity) int {
	limite := activeCarryLimit(e)
	for i := 0; i < limite && i < len(e.Carry); i++ {
		if e.Carry[i].Index == emblemaOrc {
			return i
		}
	}
	return -1
}

// comprarComEmblema fecha a compra na loja de emblema: tira 1 Emblema Orc da
// mochila no lugar do gold. Devolve false, sem mexer em nada, quando o jogador
// não tem emblema.
func (d *Dispatcher) comprarComEmblema(w *world.World, s *world.Session, e *world.Entity) bool {
	slot := slotDoEmblema(e)
	if slot < 0 {
		sendClientMessage(w, s, msgFaltaEmblema)
		return false
	}
	consumeOneItem(&e.Carry[slot])
	d.sendSlot(w, s, world.ItemPlaceCarry, slot, e.Carry[slot])
	return true
}
