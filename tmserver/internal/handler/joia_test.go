package handler

import (
	"net"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// baseJoiaEntity is a known-score entity for the affect-8 score-math tests.
func baseJoiaEntity() *world.Entity {
	return &world.Entity{MaxHP: 1000, MaxMP: 1000, AC: 100, Damage: 200, Magic: 250}
}

// TestJoiaAffect8Bits ports Basedef.cpp:4478-4548: each Level bit of affect type
// 8 (the Jóias PvP) applies one jewel's bonus into the cached Aff* fields.
func TestJoiaAffect8Bits(t *testing.T) {
	tests := []struct {
		name  string
		bit   uint
		check func(t *testing.T, e *world.Entity)
	}{
		{"resistencia", 1, func(t *testing.T, e *world.Entity) {
			for k, r := range e.AffResist {
				if r != 25 {
					t.Errorf("AffResist[%d] = %d, want 25", k, r)
				}
			}
		}},
		{"revelacao_cast", 2, func(t *testing.T, e *world.Entity) {
			if e.Rsv&world.RsvCast == 0 {
				t.Errorf("Rsv = %#x, want RsvCast set", e.Rsv)
			}
		}},
		{"absorcao", 3, func(t *testing.T, e *world.Entity) {
			if e.AffHpAbs != 20 {
				t.Errorf("AffHpAbs = %d, want 20", e.AffHpAbs)
			}
		}},
		{"protecao", 4, func(t *testing.T, e *world.Entity) {
			if e.AffMaxHP != 100 || e.AffAC != 10 {
				t.Errorf("AffMaxHP/AffAC = %d/%d, want 100/10", e.AffMaxHP, e.AffAC)
			}
		}},
		{"poder", 5, func(t *testing.T, e *world.Entity) {
			if e.AffMaxHP != 100 || e.AffDamage != 20 || e.AffMagic != 40 {
				t.Errorf("AffMaxHP/AffDamage/AffMagic = %d/%d/%d, want 100/20/40", e.AffMaxHP, e.AffDamage, e.AffMagic)
			}
		}},
		{"magia_mp_to_hp", 7, func(t *testing.T, e *world.Entity) {
			if e.AffMaxHP != 500 || e.AffMaxMP != -500 {
				t.Errorf("AffMaxHP/AffMaxMP = %d/%d, want 500/-500", e.AffMaxHP, e.AffMaxMP)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := baseJoiaEntity()
			e.Affect[0] = world.Affect{Type: affectPvP, Level: uint16(1) << tt.bit}
			applyAffectScore(e)
			tt.check(t, e)
		})
	}
}

// TestJoiaAffect8NoOpBits asserts the two jewels with no server-side stat
// (Sagacidade bit 0, Precisão bit 6 — Accuracy is a dead field in the legacy)
// leave the score untouched: they exist only as buff icons.
func TestJoiaAffect8NoOpBits(t *testing.T) {
	for _, bit := range []uint{0, 6} {
		e := baseJoiaEntity()
		e.Affect[0] = world.Affect{Type: affectPvP, Level: uint16(1) << bit}
		applyAffectScore(e)
		if e.Rsv != 0 || e.AffMaxHP != 0 || e.AffMaxMP != 0 || e.AffAC != 0 ||
			e.AffDamage != 0 || e.AffMagic != 0 || e.AffHpAbs != 0 || e.AffResist != [4]int16{} {
			t.Fatalf("bit %d changed score state: %+v", bit, e)
		}
	}
}

// TestJoiaAffect8Stacks confirms multiple jewels OR their bits into the one
// affect-8 slot and all bonuses accumulate.
func TestJoiaAffect8Stacks(t *testing.T) {
	e := baseJoiaEntity()
	e.Affect[0] = world.Affect{Type: affectPvP, Level: (1 << 1) | (1 << 4)} // Resistência + Proteção
	applyAffectScore(e)
	if e.AffResist[0] != 25 {
		t.Errorf("AffResist[0] = %d, want 25", e.AffResist[0])
	}
	if e.AffMaxHP != 100 || e.AffAC != 10 {
		t.Errorf("AffMaxHP/AffAC = %d/%d, want 100/10", e.AffMaxHP, e.AffAC)
	}
}

func TestEffectiveMagicPoder(t *testing.T) {
	e := &world.Entity{Magic: 250, AffMagic: 40}
	if got := effectiveMagic(e); got != 290 {
		t.Errorf("effectiveMagic = %d, want 290", got)
	}
}

func TestHpAbsHeal(t *testing.T) {
	tests := []struct {
		dmg  int
		abs  int32
		want int32
	}{
		{1000, 20, 200}, // 20% of 1000
		{10, 20, 2},     // (10*20+1)/100 = 2
		{2000, 20, 350}, // 400 capped to 350
		{100, 0, 0},     // no buff
	}
	for _, tt := range tests {
		if got := hpAbsHeal(tt.dmg, tt.abs); got != tt.want {
			t.Errorf("hpAbsHeal(%d, %d) = %d, want %d", tt.dmg, tt.abs, got, tt.want)
		}
	}
}

// useCarry sends a _MSG_UseItem for the carry slot.
func useCarry(t *testing.T, c net.Conn, slot int) {
	t.Helper()
	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: int32(slot)}
	send(t, c, protocol.MsgUseItem, body.Encode())
}

// joiaDB seeds a character with the given jewels laid out in consecutive carry
// slots starting at 0.
func joiaDB(jewels ...int16) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000, MP: 500, MaxMP: 500}
	for i, idx := range jewels {
		st.Carry[i] = world.Item{Index: idx}
	}
	db.loadResult = st
	return db
}

// TestUseJoiaPvP uses the Jóia da Resistência (3201) and asserts the server
// consumes it and pushes an affect-8 slot carrying that jewel's Level bit.
func TestUseJoiaPvP(t *testing.T) {
	addr, stop := startServerClockVol(t, joiaDB(3201), map[int]int{3201: volJoiaPvP})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useCarry(t, c, 0)

	sawItem, sawAffect, sawScore := false, false, false
	for range 6 {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			break
		}
		switch ty {
		case protocol.MsgSendItem:
			sawItem = true
			if le16(payload[4:6]) != 0 {
				t.Errorf("carry slot 0 = %d, want empty after use", le16(payload[4:6]))
			}
		case protocol.MsgSendAffect:
			sawAffect = true
			if payload[0] != affectPvP {
				t.Errorf("affect type = %d, want %d", payload[0], affectPvP)
			}
			if lvl := le16(payload[2:4]); lvl != (1 << 1) {
				t.Errorf("affect level = %#x, want bit 1", lvl)
			}
		case protocol.MsgUpdateScore:
			sawScore = true
		}
	}
	if !sawItem || !sawAffect || !sawScore {
		t.Errorf("missing message(s): item=%v affect=%v score=%v", sawItem, sawAffect, sawScore)
	}
}

// TestUseJoiaPvPStacks uses two different jewels and asserts their bits OR into
// the same affect-8 slot.
func TestUseJoiaPvPStacks(t *testing.T) {
	addr, stop := startServerClockVol(t, joiaDB(3201, 3202), map[int]int{3201: volJoiaPvP, 3202: volJoiaPvP})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useCarry(t, c, 0)
	drain(t, c)
	useCarry(t, c, 1)

	wantLevel := uint16((1 << 1) | (1 << 2))
	for range 6 {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			break
		}
		if ty == protocol.MsgSendAffect {
			if payload[0] != affectPvP {
				t.Fatalf("affect type = %d, want %d", payload[0], affectPvP)
			}
			if lvl := le16(payload[2:4]); lvl != wantLevel {
				t.Fatalf("stacked level = %#x, want %#x", lvl, wantLevel)
			}
			return
		}
	}
	t.Fatal("no MsgSendAffect after stacking two jewels")
}

// TestUseJoiaRecovery uses the Jóia da Recuperação (3203) with an active skill
// buff (affect type 5) and asserts the buff is cleared.
func TestUseJoiaRecovery(t *testing.T) {
	db := joiaDB(3203)
	st := db.loadResult
	st.Affects = []world.Affect{{Type: 5, Level: 1, Time: 100}}
	db.loadResult = st

	addr, stop := startServerClockVol(t, db, map[int]int{3203: volJoiaRecovery})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	drain(t, c) // discard the login-time affect snapshot (carries the type-5 buff)
	useCarry(t, c, 0)

	for range 6 {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			break
		}
		if ty == protocol.MsgSendAffect {
			if payload[0] != 0 { // slot 0 (the type-5 buff) must be cleared
				t.Fatalf("affect slot 0 type = %d, want 0 after recovery cleanse", payload[0])
			}
			return
		}
	}
	t.Fatal("no MsgSendAffect after recovery cleanse")
}

// TestUseJoiaArmazenagem uses the Jóia da Armazenagem (3207): it has no
// server-side stat, so it is only consumed — no score/affect push.
func TestUseJoiaArmazenagem(t *testing.T) {
	addr, stop := startServerClockVol(t, joiaDB(3207), map[int]int{3207: volJoiaRecovery})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useCarry(t, c, 0)

	item := expect(t, c, protocol.MsgSendItem)
	if le16(item[4:6]) != 0 {
		t.Errorf("carry slot 0 = %d, want empty after use", le16(item[4:6]))
	}
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("unexpected extra message %#x after storage jewel (should only consume)", ty)
	}
}

// drain reads and discards any immediately-available frames.
func drain(t *testing.T, c net.Conn) {
	t.Helper()
	for range 6 {
		if _, _, ok := readMaybe(t, c); !ok {
			break
		}
	}
}
