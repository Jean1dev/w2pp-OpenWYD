package protocol

import (
	"encoding/binary"
	"testing"
)

// MSG_CreateItem is 30 bytes in the legacy (Basedef.h:1957-1971): the 12-byte
// header, GridX/GridY/ItemID as shorts, the 8-byte STRUCT_ITEM, then Rotate,
// State, Height and Create. A short body makes the client read past the frame.
func TestEncodeCreateItemBody(t *testing.T) {
	b := EncodeCreateItemBody(CreateItemData{
		GridX: 2487, GridY: 2129, ItemID: 10042,
		Item:   WireItem{Index: 462, Effects: [3]WireEffect{{Effect: 1, Value: 2}}},
		Rotate: 1, State: 3, Height: 0x34,
	})
	if len(b)+HeaderSize != 30 {
		t.Fatalf("MSG_CreateItem = %d bytes, want 30", len(b)+HeaderSize)
	}
	le := binary.LittleEndian
	if le.Uint16(b[0:]) != 2487 || le.Uint16(b[2:]) != 2129 || le.Uint16(b[4:]) != 10042 {
		t.Errorf("grid/id = %d,%d,%d", le.Uint16(b[0:]), le.Uint16(b[2:]), le.Uint16(b[4:]))
	}
	if le.Uint16(b[6:]) != 462 || b[8] != 1 || b[9] != 2 {
		t.Errorf("item = % x", b[6:14])
	}
	if b[14] != 1 || b[15] != 3 || b[16] != 0x34 || b[17] != 0 {
		t.Errorf("rotate/state/height/create = % x", b[14:18])
	}
}

// MSG_DecayItem is the header plus ItemID and a zero short.
func TestEncodeDecayItemBody(t *testing.T) {
	b := EncodeDecayItemBody(10042)
	if len(b)+HeaderSize != 16 || binary.LittleEndian.Uint16(b) != 10042 || b[2] != 0 || b[3] != 0 {
		t.Errorf("MSG_DecayItem body = % x", b)
	}
}
