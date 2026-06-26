package protocol

import (
	"encoding/binary"
	"testing"
)

func TestEncodeUpdateScore(t *testing.T) {
	b := EncodeUpdateScore(ScoreData{
		Level: 50, Ac: 120, Damage: 200, AttackRun: 0x58,
		MaxHp: 1500, MaxMp: 400, Hp: 1490, Mp: 390,
		Str: 80, Int: 12, Dex: 34, Con: 56,
	})
	if len(b) != updateScoreSize-HeaderSize { // 152 - 12 = 140
		t.Fatalf("UpdateScore body = %d, want %d", len(b), updateScoreSize-HeaderSize)
	}
	le := binary.LittleEndian
	if le.Uint32(b[0:]) != 50 || le.Uint32(b[4:]) != 120 || le.Uint32(b[8:]) != 200 {
		t.Errorf("Level/Ac/Damage = %d/%d/%d", le.Uint32(b[0:]), le.Uint32(b[4:]), le.Uint32(b[8:]))
	}
	if b[13] != 0x58 { // AttackRun @13
		t.Errorf("AttackRun = %#x, want 0x58", b[13])
	}
	if le.Uint32(b[16:]) != 1500 || le.Uint32(b[24:]) != 1490 {
		t.Errorf("MaxHp/Hp = %d/%d", le.Uint32(b[16:]), le.Uint32(b[24:]))
	}
	if le.Uint16(b[32:]) != 80 || le.Uint16(b[38:]) != 56 {
		t.Errorf("Str/Con = %d/%d", le.Uint16(b[32:]), le.Uint16(b[38:]))
	}
	// Status bars duplicated near the tail.
	if le.Uint32(b[124:]) != 1490 || le.Uint32(b[128:]) != 390 {
		t.Errorf("CurrHp/CurrMp = %d/%d", le.Uint32(b[124:]), le.Uint32(b[128:]))
	}
}
