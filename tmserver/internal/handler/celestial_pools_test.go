package handler

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// loginPools entra no mundo com o personagem dado e devolve o MaxHp/MaxMp do
// primeiro UpdateScore da entrada (STRUCT_SCORE: MaxHp@16, MaxMp@20). É esse o
// pacote que dá os pools vivos ao cliente; o MSG_CNFCharacterLogin é montado do
// estado gravado e não serve de prova.
func loginPools(t *testing.T, st world.CharacterState) (hp, mp int32) {
	t.Helper()
	db := newDB()
	db.loadResult = st
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
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		h, payload, ok := readMaybeHeaderRaw(t, c)
		if !ok || h.Type != protocol.MsgUpdateScore || len(payload) < 24 {
			continue
		}
		return int32(binary.LittleEndian.Uint32(payload[16:20])), int32(binary.LittleEndian.Uint32(payload[20:24]))
	}
	t.Fatal("nenhum UpdateScore chegou na entrada")
	return 0, 0
}

// TestCelestialGravadoErradoVoltaConsertado: um Celestial que nasceu antes do
// porte do +MAX_LEVEL tem no banco os pools de um personagem de nível baixo. O
// login o reconstrói pela fórmula do legado (BASE_GetHpMp), como o original faz
// em todo carregamento — é isso que conserta quem já existe.
func TestCelestialGravadoErradoVoltaConsertado(t *testing.T) {
	// FM Celestial nível 10, sem pontos: o banco tem 70/95 (base + 10 níveis).
	hp, mp := loginPools(t, world.CharacterState{
		Slot: 0, Name: "Celeste", Class: 1, X: 5, Y: 5, Level: 10, ClassMaster: classMasterCelestial,
		Str: 5, Int: 8, Dex: 5, Con: 5, HP: 70, MaxHP: 70, MP: 95, MaxMP: 95,
	})
	if hp != 60+(10+399)*1 || mp != 65+(10+399)*3 {
		t.Errorf("Celestial no login: HP/MP %d/%d, want %d/%d (os 399 níveis do legado)", hp, mp, 60+409, 65+409*3)
	}
}

// TestArchNaoTemOPoolRecalculado: Mortal e Arch mantêm o que está gravado. O
// recálculo apagaria as concessões permanentes que este servidor guarda direto
// no pool (os cristais do Arch).
func TestArchNaoTemOPoolRecalculado(t *testing.T) {
	hp, mp := loginPools(t, world.CharacterState{
		Slot: 0, Name: "Arcanjo", Class: 1, X: 5, Y: 5, Level: 10, ClassMaster: classMasterArch,
		Str: 5, Int: 8, Dex: 5, Con: 5, HP: 150, MaxHP: 150, MP: 175, MaxMP: 175,
	})
	if hp != 150 || mp != 175 {
		t.Errorf("Arch no login: HP/MP %d/%d, want 150/175 (o gravado)", hp, mp)
	}
}
