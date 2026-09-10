package handler

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestManaNegativaGravadaNaoChegaAoCliente: um personagem com MP negativo no
// banco não pode entrar no mundo mostrando mana negativa.
//
// O servidor já não produz MP negativo em jogo — toda escrita tem guarda ou
// piso. Mas houve um período em que o debuff de monstro drenava a mana antes de
// existir piso, e o que foi gravado ali ficou. O carregamento protegia o HP
// (quem entra morto é revivido) e não o MP, e o valor cru ia para o cliente em
// todo login: -19929 logo na mensagem de boas-vindas, com o cliente animando a
// barra devagar a partir dali.
func TestManaNegativaGravadaNaoChegaAoCliente(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Drenado", Class: 2, X: 5, Y: 5,
		HP: 2708, MaxHP: 2708, MP: -19929, MaxMP: 5133, Level: 322,
	}
	addr, stop, _ := startServerSummon(t, db, nil, 0, 0)
	defer stop()
	c := dial(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgAccountLogin, loginBody("tester", "secret", protocol.AppVersion))
	if ty, _ := read(t, c); ty != protocol.MsgCNFAccountLogin {
		t.Fatalf("login de conta falhou: %#x", ty)
	}
	var body protocol.MsgCharacterLoginBody
	send(t, c, protocol.MsgCharacterLogin, body.Encode())

	visto := false
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		h, payload, ok := readMaybeHeaderRaw(t, c)
		if !ok {
			continue
		}
		var mp int32
		switch {
		case h.Type == protocol.MsgCNFCharacterLogin && len(payload) >= 128:
			mp = int32(binary.LittleEndian.Uint32(payload[124:128])) // mob@4 + CurrentScore@92 + Mp@28
		case h.Type == protocol.MsgSetHpMp && len(payload) >= 8:
			mp = int32(binary.LittleEndian.Uint32(payload[4:8])) // Hp@0, Mp@4
		default:
			continue
		}
		visto = true
		if mp < 0 {
			t.Errorf("%#x levou Mp=%d ao cliente; o valor gravado negativo não foi saneado no login", h.Type, mp)
		}
	}
	if !visto {
		t.Fatal("nenhum pacote com mana chegou no login")
	}
}
