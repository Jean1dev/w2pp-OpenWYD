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
		Special: [4]int16{5, 6, 7, 8},
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
	// Special[4] @40-46.
	for i, want := range []uint16{5, 6, 7, 8} {
		if got := le.Uint16(b[40+i*2:]); got != want {
			t.Errorf("Special[%d] = %d, want %d", i, got, want)
		}
	}
	// Status bars duplicated near the tail.
	if le.Uint32(b[124:]) != 1490 || le.Uint32(b[128:]) != 390 {
		t.Errorf("CurrHp/CurrMp = %d/%d", le.Uint32(b[124:]), le.Uint32(b[128:]))
	}
}

func TestEncodeUpdateScoreTail(t *testing.T) {
	var s ScoreData
	s.Critical, s.SaveMana = 3, 10
	s.Affect[0] = PackAffect(AffectData{Type: 11, Time: 0x1234}) // buff icon slot 0
	s.Affect[31] = PackAffect(AffectData{Type: 34, Time: 3000000})
	s.Guild, s.GuildLevel = 77, 9
	s.Resist = [4]int8{1, -2, 3, 4}
	s.Magic = 42
	b := EncodeUpdateScore(s)
	le := binary.LittleEndian
	if b[48] != 3 || b[49] != 10 {
		t.Errorf("Critical/SaveMana @48/49 = %d/%d", b[48], b[49])
	}
	// Affect icon = (Type<<8) | (Time&0xFF); Time clamps at 2550000 first.
	if got := le.Uint16(b[50:]); got != (11<<8)|0x34 {
		t.Errorf("Affect[0] @50 = %#x, want %#x", got, (11<<8)|0x34)
	}
	if got := le.Uint16(b[50+31*2:]); got != (34<<8)|uint16(2550000&0xFF) {
		t.Errorf("Affect[31] = %#x, want clamped time byte", got)
	}
	if le.Uint16(b[114:]) != 77 || le.Uint16(b[116:]) != 9 {
		t.Errorf("Guild/GuildLevel @114/116 = %d/%d", le.Uint16(b[114:]), le.Uint16(b[116:]))
	}
	if int8(b[118]) != 1 || int8(b[119]) != -2 {
		t.Errorf("Resist @118 = %d,%d", int8(b[118]), int8(b[119]))
	}
	if le.Uint32(b[132:]) != 42 {
		t.Errorf("Magic @132 = %d, want 42", le.Uint32(b[132:]))
	}
	// The legacy sends the trailing Special[4] bytes as its 0xCC stack quirk.
	if b[136] != 0xCC || b[139] != 0xCC {
		t.Errorf("tail Special quirk = %#x %#x, want 0xCC", b[136], b[139])
	}
}

func TestEncodeSetHpDam(t *testing.T) {
	b := EncodeSetHpDam(950, -50)
	if len(b) != setHpDamSize-HeaderSize { // 20 - 12 = 8
		t.Fatalf("SetHpDam body = %d, want 8", len(b))
	}
	le := binary.LittleEndian
	if le.Uint32(b[0:]) != 950 {
		t.Errorf("Hp = %d, want 950", le.Uint32(b[0:]))
	}
	if int32(le.Uint32(b[4:])) != -50 {
		t.Errorf("Dam = %d, want -50", int32(le.Uint32(b[4:])))
	}
}

func TestEncodeSetHpMp(t *testing.T) {
	b := EncodeSetHpMp(900, 120, 950, 300)
	if len(b) != setHpMpSize-HeaderSize { // 28 - 12 = 16
		t.Fatalf("SetHpMp body = %d, want 16", len(b))
	}
	le := binary.LittleEndian
	if le.Uint32(b[0:]) != 900 || le.Uint32(b[4:]) != 120 {
		t.Errorf("Hp/Mp = %d/%d, want 900/120", le.Uint32(b[0:]), le.Uint32(b[4:]))
	}
	if le.Uint32(b[8:]) != 950 || le.Uint32(b[12:]) != 300 {
		t.Errorf("ReqHp/ReqMp = %d/%d, want 950/300", le.Uint32(b[8:]), le.Uint32(b[12:]))
	}
}
