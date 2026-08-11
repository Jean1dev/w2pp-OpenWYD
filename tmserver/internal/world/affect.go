package world

// MaxAffect is the number of affect (buff/debuff) slots per entity
// (STRUCT_AFFECT Affect[MAX_AFFECT], captura-wyd-affect-divina.md §A).
const MaxAffect = 32

// Affect types the score model reacts to (BASE_GetCurrentScore, captura §C).
const (
	AffectExpChest       = 39 // Baú de Experiência: +100 ExpBonus (Basedef.cpp:4451)
	AffectDivine         = 34 // Poção Divina: +20% MaxHp/MaxMp/Damage
	AffectVigor          = 35 // Poção de Vigor: +10% MaxHp/MaxMp
	AffectForceMobDamage = 30 // Frango Assado: +2000 flat mob-damage, 4h (Basedef.cpp:4427)
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
// the persisted slots) and the return to the selection screen must call this;
// otherwise a buff cast on one character survives the swap, applies to a
// different character and even gets persisted onto it by the next logout save.
func (e *Entity) ResetAffects() {
	e.Affect = [MaxAffect]Affect{}
	e.DivineEnd = 0
	e.Rsv = 0
	e.AffDamage, e.AffAC, e.AffMaxHP, e.AffMaxMP, e.AffRunSpeed, e.AffAttackSpeed, e.AffExpBonus = 0, 0, 0, 0, 0, 0, 0
	e.AffStr, e.AffInt, e.AffDex, e.AffCon, e.AffCritical = 0, 0, 0, 0, 0
	e.AffForceDamage, e.AffForceMobDamage = 0, 0
	e.AffHpAbs, e.AffMagic = 0, 0
	e.AffSpecial = [4]int16{}
	e.AffResist = [4]int16{}
	e.AffDamageMultiPct = 100
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

// ClearFirstAffect removes the first slot holding affect type t.
func (e *Entity) ClearFirstAffect(t uint8) bool {
	for i := range e.Affect {
		if e.Affect[i].Type == t {
			e.Affect[i] = Affect{}
			return true
		}
	}
	return false
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

// AffectDuration is the deliberate server-side tuning of how long a CAST affect
// lasts (issue #229) — the same class of divergence as the affect-25 index fix of
// issue #233. Its zero value is legacy-faithful: nothing is scaled or clamped, so
// the parity tests still exercise the original formula.
//
// It replaces the issue #92 attempt at the same goal, which halved the loader's
// AffectTime divisor. That knob was in the wrong place and was the wrong knob:
// the load-time division is integer, so it truncated the short tail of the table
// far past the intended half (issue #236), and it left untouched the term that
// actually dominates the duration — the (100+Special)/100 mastery multiplier,
// worth up to 5x at the Special cap, which is why the level-400 characters who
// filed #92, #202 and #229 kept seeing 30-100 min buffs.
//
// Only affects installed by a cast pass through here. Item, mount, cash-jewel and
// Divine affects write Entity.Affect directly with their own intentional
// durations (7/30 days, 1 h) and are untouched.
type AffectDuration struct {
	// ScalePct scales the legacy tick count. 0 or 100 leaves it alone.
	ScalePct int
	// MinTicks floors the result for NON-aggressive affects only. Short friendly
	// buffs (Escudo Dourado's raw AffectTime is 30, vs 600 for the long ones)
	// would otherwise scale down to nothing. Hostile affects are excluded so the
	// floor cannot LENGTHEN a short debuff like Enfraquecer (raw 5).
	MinTicks int
	// MaxTicks caps the result, cutting the mastery tail that no base-side
	// tuning could reach.
	MaxTicks int
}

// scale applies the policy to a legacy tick count. A landed affect never becomes
// 0 ticks — that would install a buff the very next sweep deletes.
func (d AffectDuration) scale(ticks, aggressive int) int {
	if d.ScalePct > 0 && d.ScalePct != 100 {
		ticks = ticks * d.ScalePct / 100
	}
	if d.MaxTicks > 0 && ticks > d.MaxTicks {
		ticks = d.MaxTicks
	}
	if aggressive == 0 && ticks < d.MinTicks {
		ticks = d.MinTicks
	}
	if ticks < 1 {
		ticks = 1
	}
	return ticks
}

// SetAffect is the legacy SetAffect (Server.cpp:9209): install a timed affect
// from a cast. Only PLAYERS take affects (`conn > MAX_USER` returns FALSE);
// aggressive casts bounce off RSV_BLOCK. time is the cast Delay (100+Special);
// the stored Time is (affectTime+1)*time/100 ticks (one tick = 8s of real time),
// then tuned by dur. A slot being reused whose PREVIOUS type is 1/3/10 gets the
// short 4-tick timer (the legacy's odd sType clamp) and skips the tuning, as does
// the "infinite" sentinel. Returns false when nothing was applied.
func (e *Entity) SetAffect(affectType, affectValue, affectTime, aggressive, time, level int, dur AffectDuration) bool {
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
	t, fixed := (affectTime+1)*time/100, false
	if prev == 1 || prev == 3 || prev == 10 {
		t, fixed = 4, true
	}
	if time >= 2139062143 {
		t, fixed = 2139062143, true
	}
	if !fixed {
		t = dur.scale(t, aggressive)
	}
	e.Affect[slot] = Affect{Type: uint8(affectType), Value: uint8(affectValue), Level: uint16(level), Time: uint32(t)}
	return true
}

// SetTick is the legacy SetTick (Server.cpp:9256): install a periodic affect
// (HoT 17 / DoT 20 / …). Unlike SetAffect, MOBS may carry ticks (only merchant
// NPCs are excluded); types 1/3/10 clamp to 2 ticks. Periodic affects share the
// AffectTime column with the cast buffs, so they take the same dur tuning.
func (e *Entity) SetTick(tickType, tickValue, affectTime, aggressive, delay, level int, dur AffectDuration) bool {
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
	t, fixed := delay*(affectTime+1)/100, false
	if delay >= 500000000 {
		t, fixed = 500000000, true
	}
	if t >= 3 && (tickType == 1 || tickType == 3 || tickType == 10) {
		t, fixed = 2, true
	}
	if !fixed {
		t = dur.scale(t, aggressive)
	}
	e.Affect[slot] = Affect{Type: uint8(tickType), Value: uint8(tickValue), Level: uint16(level), Time: uint32(t)}
	return true
}
