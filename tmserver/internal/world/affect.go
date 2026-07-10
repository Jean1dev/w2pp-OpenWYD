package world

// MaxAffect is the number of affect (buff/debuff) slots per entity
// (STRUCT_AFFECT Affect[MAX_AFFECT], captura-wyd-affect-divina.md §A).
const MaxAffect = 32

// Affect types the score model reacts to (BASE_GetCurrentScore, captura §C).
const (
	AffectExpChest = 39 // Baú de Experiência: +100 ExpBonus (Basedef.cpp:4451)
	AffectDivine   = 34 // Poção Divina: +20% MaxHp/MaxMp/Damage
	AffectVigor    = 35 // Poção de Vigor: +10% MaxHp/MaxMp
)

// Affect mirrors STRUCT_AFFECT (8 bytes): a timed buff/debuff. Type==0 is an empty
// slot. For the Divine buff the real deadline is Entity.DivineEnd (wall-clock); the
// Affect.Time field is only the client icon timer.
type Affect struct {
	Type  uint8
	Value uint8
	Level uint16
	Time  uint32
}

// EmptyAffect returns the slot already holding affect type t, else the first free
// slot, else -1 when full — mirrors GetEmptyAffect (GetFunc.cpp:734).
func (e *Entity) EmptyAffect(t uint8) int {
	for i := range e.Affect {
		if e.Affect[i].Type == t {
			return i
		}
	}
	for i := range e.Affect {
		if e.Affect[i].Type == 0 {
			return i
		}
	}
	return -1
}

// ResetAffects wipes every buff/debuff slot and the derived affect state
// (caches + Rsv + DivineEnd). The player Entity lives as long as the
// CONNECTION, not the character (allocated once per TCP connection, reused
// across character-select swaps), so both character-login (before rehydrating
// the persisted slots) and the return to the selection screen must call this —
// otherwise a buff cast on one character (e.g. the Baú de Experiência, issue
// #47) survives the swap, applies to a different character and even gets
// persisted onto it by the next characterLogout save.
func (e *Entity) ResetAffects() {
	e.Affect = [MaxAffect]Affect{}
	e.DivineEnd = 0
	e.Rsv = 0
	e.AffDamage, e.AffAC, e.AffMaxHP, e.AffMaxMP, e.AffCon, e.AffExpBonus = 0, 0, 0, 0, 0, 0
	e.AffSpecial = [4]int16{}
	e.AffResist = [4]int16{}
}

// HasAnyAffect reports whether any slot holds a buff/debuff (Type != 0).
func (e *Entity) HasAnyAffect() bool {
	for i := range e.Affect {
		if e.Affect[i].Type != 0 {
			return true
		}
	}
	return false
}

// HasAffect reports whether any slot holds affect type t.
func (e *Entity) HasAffect(t uint8) bool {
	for i := range e.Affect {
		if e.Affect[i].Type == t {
			return true
		}
	}
	return false
}

// ClearAffect removes every slot holding affect type t.
func (e *Entity) ClearAffect(t uint8) {
	for i := range e.Affect {
		if e.Affect[i].Type == t {
			e.Affect[i] = Affect{}
		}
	}
}

// Rsv state flags (Basedef.h:244-251) — recomputed from the active affects by
// the score refresh; combat and the affect engine read them.
const (
	RsvFrost uint8 = 0x01
	RsvDrain uint8 = 0x02
	RsvParry uint8 = 0x08
	RsvHide  uint8 = 0x10
	RsvHaste uint8 = 0x20
	RsvCast  uint8 = 0x40
	RsvBlock uint8 = 0x80
)

// SetAffect is the legacy SetAffect (Server.cpp:9209): install a timed affect
// from a cast. Only PLAYERS take affects (`conn > MAX_USER` returns FALSE);
// aggressive casts bounce off RSV_BLOCK. time is the cast Delay (100+Special);
// the stored Time is (affectTime+1)*time/100 ticks (one tick = 8s of real time).
// A slot being reused whose PREVIOUS type is 1/3/10 gets the short 4-tick timer
// (the legacy's odd sType clamp). Returns false when nothing was applied.
func (e *Entity) SetAffect(affectType, affectValue, affectTime, aggressive, time, level int) bool {
	if !IsPlayer(e.ID) || e.Merchant == 1 {
		return false
	}
	if e.Rsv&RsvBlock != 0 && aggressive != 0 {
		return false
	}
	if affectType <= 0 {
		return false
	}
	slot := e.EmptyAffect(uint8(affectType))
	if slot < 0 {
		return false
	}
	prev := e.Affect[slot].Type
	t := (affectTime + 1) * time / 100
	if prev == 1 || prev == 3 || prev == 10 {
		t = 4
	}
	if time >= 2139062143 {
		t = 2139062143
	}
	e.Affect[slot] = Affect{Type: uint8(affectType), Value: uint8(affectValue), Level: uint16(level), Time: uint32(t)}
	return true
}

// SetTick is the legacy SetTick (Server.cpp:9256): install a periodic affect
// (HoT 17 / DoT 20 / …). Unlike SetAffect, MOBS may carry ticks (only merchant
// NPCs are excluded); types 1/3/10 clamp to 2 ticks.
func (e *Entity) SetTick(tickType, tickValue, affectTime, aggressive, delay, level int) bool {
	if e.Merchant == 1 && !IsPlayer(e.ID) {
		return false
	}
	if e.Rsv&RsvBlock != 0 && aggressive != 0 {
		return false
	}
	if tickType <= 0 {
		return false
	}
	slot := e.EmptyAffect(uint8(tickType))
	if slot < 0 {
		return false
	}
	t := delay * (affectTime + 1) / 100
	if delay >= 500000000 {
		t = 500000000
	}
	if t >= 3 && (tickType == 1 || tickType == 3 || tickType == 10) {
		t = 2
	}
	e.Affect[slot] = Affect{Type: uint8(tickType), Value: uint8(tickValue), Level: uint16(level), Time: uint32(t)}
	return true
}
