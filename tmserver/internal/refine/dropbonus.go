package refine

import (
	"github.com/jeanluca/w2pp-openwyd/internal/itemeffect"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// The drop-time bonus roll: SetItemBonus (Server.cpp:1777-2717).
//
// Every equipment piece a mob drops in the legacy server passes through this
// before it reaches the killer's inventory. It is what makes two copies of the
// same sword differ — the catalog gives an item its base numbers, this gives one
// copy its own three effect pairs.
//
// The rewrite never had it. putMobDrop handed the mob's configured Carry entry
// straight to the player, so a dropped item came out identical to an NPC-bought
// one and every "atributo adicional" read zero.
//
// Ported as behaviour, not as text: the original is a decompiled 940-line
// function carrying three defects, each named and corrected at the line where it
// would have been reproduced. Correcting them does not cost RNG parity — the
// number of rand() draws and their order are unchanged.

// Effect ids this roll writes. The ones already named elsewhere in the package
// (efSanc, efAmount) are reused rather than redeclared.
const (
	efUnique   = 59 // EF_UNIQUE: the game's way of saying "this slot holds nothing"
	efIncubate = 78 // EF_INCUBATE (ItemEffect.h:138)

	// EF_GRADE0..EF_GRADE5 (ItemEffect.h:153-158). A caller marks an item by
	// parking one of these in slot 0 before the roll: the effect id carries the
	// level distance to use, the value the refine ceiling to allow. It is the
	// only way an item comes out above +2, and nothing a common mob drops has it.
	efGrade0 = 100
	efGrade5 = 105
)

// nPos bits the roll covers (Basedef.h). Face (bit 0) and everything from
// accessory up (bit 8+) are outside the gate, and so is a bare shield.
const (
	posElmo     = 2
	posArmadura = 4
	posCalca    = 8
	posLuva     = 16
	posBota     = 32
	posArma     = 64
	posEscudo   = 128
	posDuasMaos = 192

	// posGate is `nPos & 0xFE` in the original: any of the elmo..escudo bits.
	posGate = 0xFE
)

// Weapon classes that roll magic instead of raw damage: nUnique 44 (lanças e
// tridentes) and 47 (cajados, varinhas e bastões), 80 and 112 catalog rows.
const (
	uniqueLanca  = 44
	uniqueCajado = 47
)

// bonusTipo is g_pBonusType (Basedef.cpp:585): the ten effects the special-bonus
// branch can grant. Rows 5-9 repeat rows 0-4's effects with wider value ranges.
var bonusTipo = [10]uint8{10, 30, 2, 5, 3, 10, 2, 8, 2, 8}

// bonusFaixa is g_pBonusValue (Basedef.cpp:532): {mínimo, máximo} per type, per
// level-distance row.
//
// DEFEITO 2 DO ORIGINAL. The array is declared [10][2][2] — two rows — and the
// code indexes it with lvdif, which reaches 3. At lvdif 2 and 3 the legacy reads
// into the NEXT type's rows, and at type 9 past the array entirely, into
// g_pBonusType. That is a memory bug, not a design: it pairs one type's effect
// id with another type's value range, and the last type with whatever follows
// the table in memory.
//
// Reproducing it would mean encoding a memory layout as game rules, so the index
// is clamped to the rows that exist. lvdif 0 and 1 behave exactly as the legacy
// did; lvdif 2 and 3 reuse row 1 instead of reading garbage.
var bonusFaixa = [10][2][2]int{
	{{10, 30}, {2, 5}},
	{{3, 10}, {2, 8}},
	{{2, 8}, {2, 8}},
	{{2, 6}, {2, 6}},
	{{2, 6}, {2, 6}},
	{{20, 45}, {4, 8}},
	{{7, 20}, {5, 14}},
	{{5, 14}, {5, 14}},
	{{4, 12}, {4, 12}},
	{{4, 12}, {4, 12}},
}

// Base is what the roll needs to know about the item from the catalog.
type Base struct {
	Unique  int                     // nUnique: the weapon class, for the arma branch
	ReqLvl  int                     // ReqLvl: the level distance is measured against this
	Pos     int                     // nPos: which of the tables applies
	Efeitos []itemeffect.BaseEffect // the catalog's own effects, for the tail overrides
	Indice  int                     // sIndex, for the material-signature tail
}

// Drop applies the bonus roll to dest in place (SetItemBonus).
//
// nivel is the killing mob's level, dropBonus the killer's accumulated bonus
// (fairy, grade-5 equipment, gem), cristal the a3 flag the crystal path passes.
// roll(n) must behave like rand()%n; the draw order below is the legacy's,
// because a captured RNG sequence only reproduces if every draw happens.
func Drop(dest *world.Item, base Base, nivel, dropBonus int, cristal bool, roll func(int) int) {
	adicional := dropBonus / 8
	if adicional < 0 {
		adicional = 0
	}
	if adicional > 2 {
		adicional = 2
	}

	// A grade marker overrides the level distance and unlocks refine above +2.
	// It is consumed here: the slot has to be empty for the roll below to write.
	marcaDist, marcaTeto := -1, -1
	if e := dest.Effects[0].Effect; e >= efGrade0 && e <= efGrade5 {
		marcaDist = int(e - efGrade0)
		marcaTeto = int(dest.Effects[0].Value)
		dest.Effects[0] = world.Effect{}
	}

	// An end-game patch in the original: past level 210 the mob counts as 47
	// levels lower, which pulls high-level drops back down a rung.
	if !cristal && nivel >= 210 {
		nivel -= 47
	}

	dist := (nivel - base.ReqLvl + 1) / 25
	if marcaDist != -1 {
		dist = marcaDist
	}
	// Measured before the clamp: a mob 99+ levels above the item guarantees a
	// floor on both bonuses.
	piso := dist >= 4
	if dist < 0 {
		dist = 0
	}
	if dist > 3 {
		dist = 3
	}
	if cristal && dist >= 3 {
		dist = 2
	}

	nPos := base.Pos
	if nPos&posGate != 0 && dest.Effects[0].Effect == 0 && nPos != posEscudo {
		rolaBonus(dest, base, nPos, dist, adicional, marcaTeto, piso, cristal, roll)
	}

	aplicaCatalogo(dest, base, roll)
	assinaMaterial(dest, base.Indice, roll)
}

// rolaBonus is the guarded block: the two bonus slots and then the refine slot.
func rolaBonus(dest *world.Item, base Base, nPos, dist, adicional, marcaTeto int, piso, cristal bool, roll func(int) int) {
	// Draw 1 — which effect goes in slot 1. A smaller divisor is better odds,
	// because the good outcomes are the low remainders.
	div1 := 8 - adicional
	switch {
	case cristal:
		div1 = 3
	case dist == 1, dist == 2:
		div1 = 6 - adicional
	case dist >= 3:
		div1 = 4
	}
	ef1, mult1, degrau1 := efeitoSlot1(nPos, base.Unique, roll(101)%div1)

	// Draw 2 — which effect goes in slot 2. The drop bonus does NOT apply here;
	// it only ever widened the odds of the first bonus.
	sorte2 := roll(100)
	if cristal {
		sorte2 = 2 * sorte2 / 3
	}
	div2 := 8
	switch {
	case cristal:
		div2 = 4
	case dist == 1, dist == 2:
		div2 = 6
	case dist >= 3:
		div2 = 4
	}
	ef2, mult2, degrau2 := efeitoSlot2(nPos, base.Unique, sorte2%div2)

	// Draw 3 — how big the first bonus is.
	q1 := escada(dist, roll(100))
	if piso && q1 < 4 {
		q1 = 4
	}
	q1 += degrau1
	if cristal && q1 == 0 {
		q1 = 1
	}
	if dest.Effects[1].Effect == 0 {
		switch {
		case q1 > 0:
			dest.Effects[1] = world.Effect{Effect: ef1, Value: uint8(mult1 * q1)}
		case nPos == posBota:
			// Boots and only boots write the effect with a zero value instead of
			// the nothing-marker, which is why a dropped boot can read
			// "dano adicional 0" out loud.
			dest.Effects[1] = world.Effect{Effect: ef1, Value: 0}
		default:
			dest.Effects[1] = world.Effect{Effect: efUnique, Value: uint8(roll(128))}
		}
	}

	// Draw 4 — how big the second bonus is.
	//
	// DEFEITO 1 DO ORIGINAL. At dist 0 the legacy tested the PREVIOUS draw here
	// (the one already spent on the first bonus) and, in the branch it guarded,
	// assigned to the first bonus's magnitude — which had already been written
	// into the item. The result was that 2% of same-level drops silently lost
	// their second bonus. The ladder is otherwise a twin of the one above, so
	// reading it with its own draw is both the fix and the simpler code.
	q2 := escada(dist, roll(100))
	if piso && q2 < 3 {
		q2 = 3
	}
	if cristal && q2 >= 5 {
		q2 = 4
	}
	q2 += degrau2
	if adicional != 0 && q2 == 0 {
		q2 = adicional
	}
	if cristal && q2 == 0 {
		q2 = 1
	}
	if dest.Effects[2].Effect == 0 {
		if q2 > 0 {
			dest.Effects[2] = world.Effect{Effect: ef2, Value: uint8(mult2 * q2)}
		} else {
			dest.Effects[2] = world.Effect{Effect: efUnique, Value: uint8(roll(128))}
		}
	}

	rolaRefino(dest, dist, marcaTeto, cristal, roll)
}

// rolaRefino is draw 5: slot 0 becomes a refine level, a special bonus, or
// nothing. It is the valuable one — refine +2 is 6% of every drop, and no
// unmarked item can come out above that.
func rolaRefino(dest *world.Item, dist, marcaTeto int, cristal bool, roll func(int) int) {
	if dest.Effects[0].Effect != 0 {
		return
	}
	sorte := roll(100)
	if cristal {
		sorte /= 2
	}
	// The legacy assigns defaults and then overwrites them for every value dist
	// can hold, so only these two sets are ever used.
	dois, um, zero, especial := 6, 22, 75, 90
	if dist >= 2 {
		dois, um, zero, especial = 6, 35, 85, 100
	}

	switch {
	case sorte < dois:
		// Written raw, not through Set: the packed gem encoding only starts at
		// +10, and nothing this branch produces reaches it.
		dest.Effects[0] = world.Effect{Effect: efSanc, Value: 2}
		if marcaTeto > 2 {
			subirGrade(dest, marcaTeto, roll(100))
		}
	case sorte < um:
		dest.Effects[0] = world.Effect{Effect: efSanc, Value: 1}
	case sorte < zero:
		dest.Effects[0] = world.Effect{Effect: efSanc, Value: 0}
	case sorte < especial:
		tipo := roll(10)
		linha := dist
		if linha > 1 {
			linha = 1 // ver bonusFaixa: defeito 2
		}
		lo := bonusFaixa[tipo][linha][0]
		hi := bonusFaixa[tipo][linha][1]
		dest.Effects[0] = world.Effect{Effect: bonusTipo[tipo], Value: uint8(roll(hi+1-lo) + lo)}
	default:
		dest.Effects[0] = world.Effect{Effect: efUnique, Value: uint8(roll(128))}
	}
}

// subirGrade is the only path to a refine above +2 on a rolled item: a grade
// marker whose ceiling was above 2 gets a second draw against its own table.
func subirGrade(dest *world.Item, teto, sorte int) {
	nivel := 2
	switch teto {
	case 3:
		if sorte < 30 {
			nivel = 3
		}
	case 4:
		switch {
		case sorte < 10:
			nivel = 4
		case sorte < 40:
			nivel = 3
		}
	case 5:
		switch {
		case sorte < 10:
			nivel = 5
		case sorte < 30:
			nivel = 4
		case sorte < 60:
			nivel = 3
		}
	case 6:
		switch {
		case sorte < 10:
			nivel = 6
		case sorte < 20:
			nivel = 5
		case sorte < 40:
			nivel = 4
		case sorte < 60:
			nivel = 3
		}
	case 7:
		switch {
		case sorte < 4:
			nivel = 7
		case sorte < 10:
			nivel = 6
		case sorte < 20:
			nivel = 5
		case sorte < 35:
			nivel = 4
		case sorte < 60:
			nivel = 3
		}
	}
	dest.Effects[0].Value = uint8(nivel)
}

// escada turns a 0..99 draw into a magnitude step, per level distance. The
// value written to the item is this step times the effect's multiplier, so step
// 0 means the item gets nothing.
//
// DEFEITO 3 DO ORIGINAL: both ladders carry a full table for dist >= 4 that can
// never run, because dist is clamped to 3 before the switch. It is left out
// rather than ported, so nobody reads it as live behaviour.
func escada(dist, sorte int) int {
	switch dist {
	case 1:
		switch {
		case sorte < 1:
			return 5
		case sorte < 5:
			return 4
		case sorte < 24:
			return 3
		case sorte < 65:
			return 2
		default:
			return 1
		}
	case 2:
		switch {
		case sorte < 2:
			return 5
		case sorte < 16:
			return 4
		case sorte < 60:
			return 3
		default:
			return 2
		}
	case 3:
		switch {
		case sorte < 2:
			return 6
		case sorte < 9:
			return 5
		case sorte < 45:
			return 4
		case sorte < 75:
			return 3
		default:
			return 2
		}
	default: // dist 0
		switch {
		case sorte < 2:
			return 4
		case sorte < 6:
			return 3
		case sorte < 24:
			return 2
		case sorte < 55:
			return 1
		default:
			return 0
		}
	}
}

// efeitoSlot1 is the first switch(nPos): which effect the first bonus grants,
// what one step of it is worth, and a per-piece adjustment to the step.
//
// Armadura, calça, luva and bota do not consult the draw at all — they always
// grant the same effect. When one of those comes out empty it is the magnitude
// ladder that did it, never the choice of effect.
func efeitoSlot1(nPos, unique, sel int) (ef uint8, mult, degrau int) {
	switch nPos {
	case posElmo:
		switch sel {
		case 0:
			return 26, 3, 0 // EF_ATTSPEED
		case 1:
			return 60, 2, 0 // EF_MAGIC
		}
	case posArmadura, posCalca:
		return 71, 10, 1 // EF_CRITICAL2
	case posLuva:
		return 72, 5, 0 // EF_ACADD2
	case posBota:
		return 73, 6, -1 // EF_DAMAGE2
	case posArma, posDuasMaos:
		if unique != uniqueLanca && unique != uniqueCajado {
			switch sel {
			case 0:
				return 26, 3, 1 // EF_ATTSPEED
			case 1:
				return 2, 9, -1 // EF_DAMAGE
			case 2:
				return 74, 3, 0 // EF_SPECIALALL
			}
		} else {
			switch sel {
			case 0:
				return 60, 4, -1 // EF_MAGIC
			case 1:
				return 74, 3, 1 // EF_SPECIALALL
			}
		}
	}
	return efUnique, 0, 0
}

// efeitoSlot2 is the second switch(nPos), for the second bonus. Its case for a
// bare shield is dead in the original too: the gate in Drop rejects nPos 128
// before this is ever reached.
func efeitoSlot2(nPos, unique, sel int) (ef uint8, mult, degrau int) {
	switch nPos {
	case posElmo:
		switch sel {
		case 0:
			return 4, 10, 0 // EF_HP
		case 1:
			return 3, 5, -1 // EF_AC
		}
	case posArmadura, posCalca:
		switch sel {
		case 0:
			return 60, 2, -1 // EF_MAGIC
		case 1:
			return 2, 6, -1 // EF_DAMAGE
		case 2:
			return 3, 5, -1 // EF_AC
		}
	case posLuva:
		switch sel {
		case 0:
			return 60, 2, -1 // EF_MAGIC
		case 1:
			return 2, 6, -1 // EF_DAMAGE
		case 2:
			return 74, 3, 0 // EF_SPECIALALL
		case 3:
			return 54, 3, 0 // EF_RESISTALL
		}
	case posBota:
		switch sel {
		case 0:
			return 60, 2, -1 // EF_MAGIC
		case 1:
			return 74, 3, 0 // EF_SPECIALALL
		}
	case posArma, posDuasMaos:
		if unique != uniqueLanca && unique != uniqueCajado {
			switch sel {
			case 0:
				return 26, 3, 1 // EF_ATTSPEED
			case 1:
				return 2, 9, -1 // EF_DAMAGE
			case 2:
				return 74, 3, 1 // EF_SPECIALALL
			}
		} else {
			switch sel {
			case 0:
				return 60, 4, -1 // EF_MAGIC
			case 1:
				return 74, 3, 1 // EF_SPECIALALL
			}
		}
	}
	return efUnique, 0, 0
}

// aplicaCatalogo is the tail loop: three catalog effects overwrite slot 0 after
// the roll, so an item whose catalog fixes its refine keeps it. It runs for
// every item, including the ones the gate above rejected.
//
// EF_AMOUNT is handled here because the legacy handles it here, but the content
// parser does not currently carry that id from ItemList.csv (94 rows use it), so
// today the branch never fires. That gap is older than this function and is not
// widened by it.
func aplicaCatalogo(dest *world.Item, base Base, roll func(int) int) {
	for _, e := range base.Efeitos {
		switch e.Eff {
		case efSanc:
			dest.Effects[0] = world.Effect{Effect: efSanc, Value: uint8(e.Val)}
		case efAmount:
			dest.Effects[0] = world.Effect{Effect: efAmount, Value: uint8(e.Val)}
		case efIncubate:
			v := roll(4) + int(e.Val)
			if v > 9 {
				v = 9
			}
			dest.Effects[0] = world.Effect{Effect: efIncubate, Value: uint8(v)}
		}
	}
}

// materiaisAssinados are the thirteen items the legacy stamps with random bytes
// in every empty slot: the two Oriharucon and Lactolerium forms, Imp, and the
// two Círculo Divino families. They are the refine materials — the most traded
// items in the game.
//
// Nothing in the original reads those bytes back. It is a stamp with no reader:
// per-item identity was written and then never used. The serial column
// (0033_item_serial) is the same idea with the reader in place.
func assinaMaterial(dest *world.Item, indice int, roll func(int) int) {
	assinado := false
	switch {
	case indice == 412 || indice == 413 || indice == 419 || indice == 420 || indice == 753:
		assinado = true
	case indice >= 447 && indice <= 450:
		assinado = true
	case indice >= 692 && indice <= 695:
		assinado = true
	}
	if !assinado {
		return
	}
	for i := range dest.Effects {
		if dest.Effects[i].Effect == 0 {
			// The legacy stores a raw rand() in a byte field, which is its low
			// eight bits — the same value rand()%256 gives.
			dest.Effects[i] = world.Effect{Effect: efUnique, Value: uint8(roll(256))}
		}
	}
}
