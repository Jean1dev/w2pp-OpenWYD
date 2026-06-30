package world

// MaxAffect is the number of affect (buff/debuff) slots per entity
// (STRUCT_AFFECT Affect[MAX_AFFECT], captura-wyd-affect-divina.md §A).
const MaxAffect = 32

// Affect types the score model reacts to (BASE_GetCurrentScore, captura §C).
const (
	AffectDivine = 34 // Poção Divina: +20% MaxHp/MaxMp/Damage
	AffectVigor  = 35 // Poção de Vigor: +10% MaxHp/MaxMp
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
