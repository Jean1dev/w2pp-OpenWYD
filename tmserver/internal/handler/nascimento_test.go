package handler

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestPersonagemNovoNasceNoCampoSoComOCorpo: the first login of a character the
// dbserver just created (level 0, no experience, never seeded) puts it in the
// training field, wearing the class gear from its template and with an empty
// bag — the template's potions stay behind.
func TestPersonagemNovoNasceNoCampoSoComOCorpo(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{Slot: 0, Name: "Novato", Class: 1, Level: 0, LastCity: 2, HP: 100, MaxHP: 100}
	tmpl := make([]byte, content.BaseMobSize)
	copy(tmpl[0:16], "Template")
	binary.LittleEndian.PutUint16(tmpl[140:], 11)     // Equip[0]: o corpo da classe
	binary.LittleEndian.PutUint16(tmpl[140+48:], 816) // Equip[6]: a arma
	binary.LittleEndian.PutUint16(tmpl[268:], 401)    // Carry[0]: poção de HP
	tmpl[268+2], tmpl[268+3] = efAmount, 120
	binary.LittleEndian.PutUint16(tmpl[268+8:], 4144) // Carry[1]: Baú de Experiência
	addr, stop := startServerBaseMobs(t, db, map[int][]byte{1: tmpl})
	defer stop()
	c := loginAndSelect(t, addr)
	defer c.Close()

	var body protocol.MsgCharacterLoginBody
	send(t, c, protocol.MsgCharacterLogin, body.Encode())
	ty, payload := read(t, c)
	if ty != protocol.MsgCNFCharacterLogin {
		t.Fatalf("got %#x, want CNFCharacterLogin", ty)
	}
	x := int16(binary.LittleEndian.Uint16(payload[0:]))
	y := int16(binary.LittleEndian.Uint16(payload[2:]))
	if x != campoDeTreinoNascimentoX || y != campoDeTreinoNascimentoY {
		t.Errorf("nasceu em (%d,%d), want o campo de treino (%d,%d)", x, y, campoDeTreinoNascimentoX, campoDeTreinoNascimentoY)
	}
	mob := payload[4:] // STRUCT_MOB
	if corpo := binary.LittleEndian.Uint16(mob[140:]); corpo != 11 {
		t.Errorf("Equip[0] = %d, want o corpo da classe (11)", corpo)
	}
	if arma := binary.LittleEndian.Uint16(mob[140+48:]); arma != 816 {
		t.Errorf("Equip[6] = %d, want a arma do template (816)", arma)
	}
	for i := 0; i < world.MaxCarry; i++ {
		if idx := binary.LittleEndian.Uint16(mob[268+8*i:]); idx != 0 {
			t.Errorf("a bolsa tem o item %d no slot %d, want vazia", idx, i)
		}
	}
	if ouro := int32(binary.LittleEndian.Uint32(mob[28:])); ouro != 0 {
		t.Errorf("entrou com %d de ouro, want 0", ouro)
	}
}

// TestNascimentoNoCampoDeTreino: a new character enters the world in the
// training field — the tile and the ring EmptyCellNear may move it to all carry
// the field's AttributeMap bit — and anyone else keeps entering in their city.
func TestNascimentoNoCampoDeTreino(t *testing.T) {
	x, y := pontoDeEntrada(0, true)
	if x != campoDeTreinoNascimentoX || y != campoDeTreinoNascimentoY {
		t.Fatalf("novo entra em (%d,%d), want (%d,%d)", x, y, campoDeTreinoNascimentoX, campoDeTreinoNascimentoY)
	}

	attr, err := os.ReadFile(filepath.Join("..", "..", "..", "Release", "TMsrv", "run", "AttributeMap.dat"))
	if err != nil {
		t.Skipf("AttributeMap.dat indisponível: %v", err)
	}
	const dim = 1024 // uma célula por 4×4 tiles
	noCampo := func(x, y int) bool { return attr[(y/4)*dim+x/4]&attrCampoDeTreino != 0 }
	for dy := -4; dy <= 4; dy++ {
		for dx := -4; dx <= 4; dx++ {
			if px, py := int(x)+dx, int(y)+dy; !noCampo(px, py) {
				t.Errorf("(%d,%d), perto do nascimento, está fora do campo de treino", px, py)
			}
		}
	}

	// Quem já existe continua entrando na cidade — fora do campo.
	cx, cy := pontoDeEntrada(0, false)
	if noCampo(int(cx), int(cy)) {
		t.Errorf("personagem antigo entra em (%d,%d), dentro do campo de treino", cx, cy)
	}
}
