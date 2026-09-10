package handler

import (
	"encoding/binary"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestBMTransformadoEntraTransformado: um BM que sai do jogo transformado e volta
// com o afeto ainda valendo tem de aparecer com o corpo da fera para ELE MESMO.
//
// O corpo que o cliente desenha no próprio personagem vem do Equip[0] do
// CNFCharacterLogin. O legado reescreve MOB.Equip[0].sIndex com a malha da fera
// dentro do GetCurrentScore (Basedef.cpp:4106) antes de mandar o login, então o
// pacote já sai com a fera. O port calculava a malha só para os OUTROS
// (EquipVisual) e montava o login com o equipamento gravado: o dono nascia com o
// corpo normal e o ícone da transformação aceso.
func TestBMTransformadoEntraTransformado(t *testing.T) {
	db := newDB()
	st := world.CharacterState{
		Slot: 0, Name: "Beast", Class: 2, X: 5, Y: 5,
		HP: 500, MaxHP: 500, MP: 500, MaxMP: 500, Level: 50,
		Affects: []world.Affect{{Type: affectTransform, Value: 2, Level: 100, Time: 50}},
	}
	st.Equip[0] = world.Item{Index: 21}
	db.loadResult = st

	addr, stop := startServerBaseMobs(t, db, map[int][]byte{2: plainMobTemplate("Beast")})
	defer stop()

	c := dial(t, addr)
	defer c.Close()
	send(t, c, protocol.MsgAccountLogin, loginBody("tester", "secret", protocol.AppVersion))
	if ty, _ := read(t, c); ty != protocol.MsgCNFAccountLogin {
		t.Fatalf("account login falhou: %#x", ty)
	}
	var body protocol.MsgCharacterLoginBody
	send(t, c, protocol.MsgCharacterLogin, body.Encode())
	ty, payload := read(t, c)
	if ty != protocol.MsgCNFCharacterLogin {
		t.Fatalf("character login falhou: %#x", ty)
	}
	const equip0 = 4 + 140 // STRUCT_MOB @ body4, Equip[0] @ +140, sIndex primeiro
	if got := binary.LittleEndian.Uint16(payload[equip0 : equip0+2]); got != 23 {
		t.Errorf("Equip[0] no login = %d; com o Urso ativo o próprio cliente tem de receber a malha 23", got)
	}
}
