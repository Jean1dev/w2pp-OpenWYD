package protocol

import "testing"

func TestAutoTradeBodySizes(t *testing.T) {
	// Body size + HeaderSize must equal the compiler-verified total packet size
	// (captura-wyd-autotrade.md, MSVC x86).
	tests := []struct {
		name      string
		bodySize  int
		fullTotal int
	}{
		{"SendAutoTrade", MsgSendAutoTradeBodySize, 196},
		{"ReqBuy", MsgReqBuyBodySize, 36},
		{"CreateMobTrade", createMobTradeSize - HeaderSize, 252},
		{"SendAutoTradeList", sendAutoTradeListWireSize, 528},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.bodySize+HeaderSize != tt.fullTotal {
				t.Errorf("%s: body %d + header %d = %d, want total %d", tt.name, tt.bodySize, HeaderSize, tt.bodySize+HeaderSize, tt.fullTotal)
			}
		})
	}
}

func TestSendAutoTradeRoundTrip(t *testing.T) {
	in := MsgSendAutoTradeBody{Title: "Vendo tudo", Tax: 5, Index: 42}
	in.Slots[0] = AutoTradeWireItem{
		Item:     WireItem{Index: 1234, Effects: [3]WireEffect{{Effect: 1, Value: 2}}},
		CarryPos: 7,
		Coin:     150000,
	}
	in.Slots[1] = AutoTradeWireItem{CarryPos: -1} // empty slot marker

	b := in.Encode()
	if len(b) != MsgSendAutoTradeBodySize {
		t.Fatalf("Encode len = %d, want %d", len(b), MsgSendAutoTradeBodySize)
	}
	// Hard-coded offsets (body = abs − 12): Title@0, Item[0]@24, CarryPos[0]@120,
	// Coin[0]@132, Tax@180, Index@182.
	if got := cTrimNUL(b[0:24]); got != "Vendo tudo" {
		t.Errorf("Title@0 = %q, want %q", got, "Vendo tudo")
	}
	if got := int16(le.Uint16(b[24:26])); got != 1234 {
		t.Errorf("Item[0].Index@24 = %d, want 1234", got)
	}
	if got := int8(b[120]); got != 7 {
		t.Errorf("CarryPos[0]@120 = %d, want 7", got)
	}
	if got := int8(b[121]); got != -1 {
		t.Errorf("CarryPos[1]@121 = %d, want -1", got)
	}
	if got := int32(le.Uint32(b[132:136])); got != 150000 {
		t.Errorf("Coin[0]@132 = %d, want 150000", got)
	}
	if got := int16(le.Uint16(b[180:182])); got != 5 {
		t.Errorf("Tax@180 = %d, want 5", got)
	}
	if got := int16(le.Uint16(b[182:184])); got != 42 {
		t.Errorf("Index@182 = %d, want 42", got)
	}

	var out MsgSendAutoTradeBody
	if err := out.Decode(b); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if out != in {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}

func TestSendAutoTradeEncodeListPadding(t *testing.T) {
	in := MsgSendAutoTradeBody{Title: "loja"}
	list := in.EncodeList()
	if len(list) != sendAutoTradeListWireSize {
		t.Fatalf("EncodeList len = %d, want %d (Size=528 quirk)", len(list), sendAutoTradeListWireSize)
	}
	// The meaningful 184 bytes lead; the remaining 332 are zero.
	head := in.Encode()
	for i := range head {
		if list[i] != head[i] {
			t.Fatalf("EncodeList[%d] = %d, want %d (leading struct)", i, list[i], head[i])
		}
	}
	for i := len(head); i < len(list); i++ {
		if list[i] != 0 {
			t.Fatalf("EncodeList[%d] = %d, want 0 (trailing pad)", i, list[i])
		}
	}
}

func TestReqBuyDecodeHonorsPadding(t *testing.T) {
	in := MsgReqBuyBody{
		Pos:      3,
		TargetID: 500,
		Price:    250000,
		Tax:      5,
		Item:     WireItem{Index: 777, Effects: [3]WireEffect{{Effect: 9, Value: 8}}},
	}
	b := in.Encode()
	if len(b) != MsgReqBuyBodySize {
		t.Fatalf("Encode len = %d, want %d", len(b), MsgReqBuyBodySize)
	}
	// Price must sit at body offset 8 (2-byte pad after the uint16 TargetID@4), not
	// at 6 — the whole point of the capture.
	if got := int32(le.Uint32(b[8:12])); got != 250000 {
		t.Errorf("Price@8 = %d, want 250000 (padding after TargetID)", got)
	}
	if b[6] != 0 || b[7] != 0 {
		t.Errorf("padding@6..7 = %d,%d, want 0,0", b[6], b[7])
	}
	if got := int16(le.Uint16(b[16:18])); got != 777 {
		t.Errorf("Item.Index@16 = %d, want 777", got)
	}

	var out MsgReqBuyBody
	if err := out.Decode(b); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if out != in {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}

func TestCreateMobTradeBodyTail(t *testing.T) {
	d := CreateMobData{MobID: 5, Name: "Seller", IsPlayer: true, PKPoint: 75}
	tab := make([]byte, 26)
	tab[0] = 0xAB
	body := EncodeCreateMobTradeBody(d, tab, "Minha Loja")
	if len(body) != createMobTradeSize-HeaderSize {
		t.Fatalf("body len = %d, want %d", len(body), createMobTradeSize-HeaderSize)
	}
	// Shares the CreateMob prefix: MobID@4 (body) must round-trip.
	if got := int(le.Uint16(body[4:6])); got != 5 {
		t.Errorf("MobID@4 = %d, want 5", got)
	}
	// Tab[26]@190, Desc[24]@216 (body offsets = abs − 12).
	if body[190] != 0xAB {
		t.Errorf("Tab[0]@190 = %d, want 0xAB", body[190])
	}
	if got := cTrimNUL(body[216 : 216+autoTradeTitleLen]); got != "Minha Loja" {
		t.Errorf("Desc@216 = %q, want %q", got, "Minha Loja")
	}
}
