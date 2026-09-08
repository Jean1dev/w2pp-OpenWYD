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
