package handler

import (
	"encoding/binary"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// TestWriteAttackerStatusOneLayout pins the echo's status fields to the
// MSG_Attack layout — CurrentHp@4, CurrentExp@12, CurrentMp@40, ReqMp@46 —
// whatever type the frame arrived as.
//
// Basedef.h declares AttackOne/Two with HP and MP swapped, and following it
// told the client its mana was its HP on every single-target skill: the client
// reads the attacker's mana at body+40 for all three types, and the legacy
// writes all three through MSG_Attack* (_MSG_Attack.cpp:23, :254, :1744).
func TestWriteAttackerStatusOneLayout(t *testing.T) {
	const (
		hp    = int32(1234)
		mp    = int32(5678)
		exp   = int64(229572)
		reqMp = int16(42)
	)
	payload := make([]byte, protocol.MsgAttackDamOffset)
	writeAttackerStatus(payload, hp, mp, exp, reqMp)

	if got := int32(binary.LittleEndian.Uint32(payload[4:8])); got != hp {
		t.Errorf("CurrentHp@4 = %d, want %d", got, hp)
	}
	if got := int64(binary.LittleEndian.Uint64(payload[12:20])); got != exp {
		t.Errorf("CurrentExp@12 = %d, want %d", got, exp)
	}
	if got := int32(binary.LittleEndian.Uint32(payload[40:44])); got != mp {
		t.Errorf("CurrentMp@40 = %d, want %d", got, mp)
	}
	if got := int16(binary.LittleEndian.Uint16(payload[46:48])); got != reqMp {
		t.Errorf("ReqMp@46 = %d, want %d", got, reqMp)
	}
}

// TestWriteAttackerStatusIgnoresShortBody: a frame too small to hold the fixed
// fields is left untouched rather than panicking.
func TestWriteAttackerStatusIgnoresShortBody(t *testing.T) {
	payload := make([]byte, protocol.MsgAttackDamOffset-1)
	writeAttackerStatus(payload, 1, 2, 3, 4)
	for i, b := range payload {
		if b != 0 {
			t.Fatalf("byte %d = %d, want the short body left alone", i, b)
		}
	}
}
