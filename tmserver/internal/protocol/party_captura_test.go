package protocol

import (
	"encoding/hex"
	"testing"
)

// The real client's invite, captured from the server log on 2026-09-08: TesteUP
// (conn 2) inviting teste (conn 1). Thirty-two bytes, not the thirty-six
// sizeof(MSG_SendReqParty) implies.
//
// It is written as raw hex on purpose. Every other test of this message builds
// its body with Encode and hands it straight back to Decode, which agrees with
// itself no matter what the client does — and that is exactly how a decoder
// that rejected every real invite stayed green.
const conviteReal = "ff00be0089048904020054657374655550000000000000000000000001000000"

func TestDecodeAceitaOConviteRealDoCliente(t *testing.T) {
	b, err := hex.DecodeString(conviteReal)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 32 {
		t.Fatalf("a captura tem %d bytes, esperava 32", len(b))
	}

	var m MsgSendReqPartyBody
	if err := m.Decode(b); err != nil {
		t.Fatalf("o convite real do cliente foi recusado: %v", err)
	}
	if m.PartyID != 2 {
		t.Errorf("PartyID = %d, want 2 (a conexão de quem convida)", m.PartyID)
	}
	if got := string(m.MobName[:7]); got != "TesteUP" {
		t.Errorf("MobName = %q, want TesteUP", got)
	}
	if m.Unk != 1 {
		t.Errorf("Unk = %d, want 1 (a conexão do convidado)", m.Unk)
	}
	// Absent from the wire, so it must read as zero rather than as garbage from
	// whatever happened to follow the buffer.
	if m.Target != 0 {
		t.Errorf("Target = %d, want 0 — o cliente não mandou esse campo", m.Target)
	}
}

// Shorter than the client ever sends is still a malformed packet.
func TestDecodeRecusaMaisCurtoQueOCliente(t *testing.T) {
	b, _ := hex.DecodeString(conviteReal)
	for _, n := range []int{0, 10, 26, 31} {
		var m MsgSendReqPartyBody
		if err := m.Decode(b[:n]); err == nil {
			t.Errorf("%d bytes foram aceitos; o mínimo é %d", n, MsgSendReqPartyBodyMin)
		}
	}
}

// The server still WRITES the full struct, because that is what the legacy
// sends and what the client reads on the way back.
func TestEncodeContinuaMandandoOTamanhoCheio(t *testing.T) {
	m := MsgSendReqPartyBody{PartyID: 2, Unk: 1}
	if got := len(m.Encode()); got != MsgSendReqPartyBodySize {
		t.Errorf("Encode gerou %d bytes, want %d (SendFunc.cpp manda sizeof)",
			got, MsgSendReqPartyBodySize)
	}
}

// And a full-length body still decodes, Target included — the server's own
// Encode has to survive a round trip.
func TestDecodeAindaLeOTamanhoCheio(t *testing.T) {
	m := MsgSendReqPartyBody{PartyID: 7, Unk: 9, Target: 5}
	var v MsgSendReqPartyBody
	if err := v.Decode(m.Encode()); err != nil {
		t.Fatal(err)
	}
	if v.PartyID != 7 || v.Unk != 9 || v.Target != 5 {
		t.Errorf("ida e volta deu %+v", v)
	}
}

// MSG_Trade carries MSVC natural-alignment padding, because the struct sits
// outside the pack(1) blocks of Basedef.h (last pop at 1850, struct at 2435).
// The body is 144 bytes, not the 142 the fields add up to:
//
//	Item[15] 0..120 · InvenPos[15] 120..135 · <pad> 135
//	TradeMoney 136..140 · MyCheck 140 · <pad> 141 · OpponentID 142..144
//
// The two filler bytes were missing, so money, the confirmation flag and the
// opponent id were each read early. Nothing errored — the old length was
// SHORTER than what the client sends, so the guard passed and the numbers were
// quietly wrong, which is what a trade that "does not work" looks like.
//
// Built as raw bytes on purpose: an Encode→Decode round trip agrees with itself
// at any offsets, and that is exactly why the existing trade tests stayed green
// through the bug.
func TestTradeLeOsCamposNosOffsetsDoLegado(t *testing.T) {
	b := make([]byte, 144)
	// Um item no slot 0, para provar que a área de itens não se moveu.
	le.PutUint16(b[0:2], 1234)
	b[tradeOffInvenPos] = 7                            // InvenPos[0]
	le.PutUint32(b[tradeOffMoney:tradeOffMoney+4], 5_000_000) // dinheiro
	b[tradeOffMyCheck] = 1                             // confirmado
	le.PutUint16(b[tradeOffOpponentID:tradeOffOpponentID+2], 42)
	// Os dois bytes de preenchimento ficam em zero, como o compilador os deixa.

	var m MsgTradeBody
	if err := m.Decode(b); err != nil {
		t.Fatal(err)
	}
	if m.Item[0].Index != 1234 {
		t.Errorf("Item[0].Index = %d, want 1234", m.Item[0].Index)
	}
	if m.InvenPos[0] != 7 {
		t.Errorf("InvenPos[0] = %d, want 7", m.InvenPos[0])
	}
	if m.TradeMoney != 5_000_000 {
		t.Errorf("TradeMoney = %d, want 5000000 — offset do dinheiro errado", m.TradeMoney)
	}
	if m.MyCheck != 1 {
		t.Errorf("MyCheck = %d, want 1 — a confirmação é lida do byte errado", m.MyCheck)
	}
	if m.OpponentID != 42 {
		t.Errorf("OpponentID = %d, want 42", m.OpponentID)
	}
}

// sizeof(MSG_Trade) menos o cabeçalho: 144. Se isto mudar, os offsets acima
// mudaram junto e a troca volta a ler campo trocado.
func TestTradeTemOTamanhoDoLegado(t *testing.T) {
	if MsgTradeBodySize != 144 {
		t.Errorf("MsgTradeBodySize = %d, want 144 (sizeof com o preenchimento do MSVC)", MsgTradeBodySize)
	}
	if got := len((&MsgTradeBody{}).Encode()); got != 144 {
		t.Errorf("Encode gerou %d bytes, want 144", got)
	}
}

// O parceiro fica no fim, então um corpo que pare antes dele ainda serve — a
// mesma tolerância do convite de grupo, e pelo mesmo motivo.
func TestTradeAceitaCorpoSemOParceiro(t *testing.T) {
	b := make([]byte, tradeBodyMin)
	le.PutUint32(b[tradeOffMoney:tradeOffMoney+4], 99)
	b[tradeOffMyCheck] = 1

	var m MsgTradeBody
	if err := m.Decode(b); err != nil {
		t.Fatalf("corpo de %d bytes recusado: %v", len(b), err)
	}
	if m.TradeMoney != 99 || m.MyCheck != 1 {
		t.Errorf("dinheiro/confirmação = %d/%d, want 99/1", m.TradeMoney, m.MyCheck)
	}
	if m.OpponentID != 0 {
		t.Errorf("OpponentID = %d, want 0 — não veio no pacote", m.OpponentID)
	}
}
