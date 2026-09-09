package handler

import (
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// TestEntrarNoMundoConfirmaAMana: a rajada do login tem de trazer a confirmação
// de HP/MP, e não só o UpdateScore.
//
// O cliente do WYD desconta a mana LOCALMENTE ao lançar e só reconcilia a barra
// com um MSG_SetHpMp. O login mandava UpdateScore e mais nada, e UpdateScore não
// substitui essa reconciliação — então um cliente que chegava com a conta
// desandada de uma sessão anterior continuava errado depois de relogar.
//
// E o estado é absorvente: uma barra suficientemente negativa faz o PRÓPRIO
// cliente recusar as magias ("Mana insuficiente" é mensagem dele), e aí ele para
// de mandar pacote, de modo que nenhuma correção que dependa de um pedido dele
// chega mais. Entrar no mundo é o único ponto de reconciliação garantido.
//
// O login é feito à mão de propósito: o enterWorld dos outros testes drena a
// rajada inteira, que é justamente onde este pacote vive.
func TestEntrarNoMundoConfirmaAMana(t *testing.T) {
	addr, stop, _ := startServerSummon(t, summonDB(30), nil, 0, 0)
	defer stop()
	c := dial(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgAccountLogin, loginBody("tester", "secret", protocol.AppVersion))
	if ty, _ := read(t, c); ty != protocol.MsgCNFAccountLogin {
		t.Fatalf("login de conta falhou: %#x", ty)
	}
	var body protocol.MsgCharacterLoginBody
	send(t, c, protocol.MsgCharacterLogin, body.Encode())
	if ty, _ := read(t, c); ty != protocol.MsgCNFCharacterLogin {
		t.Fatalf("login de personagem falhou: %#x", ty)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		h, _, ok := readMaybeHeaderRaw(t, c)
		if !ok {
			continue
		}
		if h.Type == protocol.MsgSetHpMp {
			return
		}
	}
	t.Error("a rajada do login não trouxe MSG_SetHpMp: um cliente que chegue com a " +
		"própria conta de mana desandada nunca é reconciliado")
}
