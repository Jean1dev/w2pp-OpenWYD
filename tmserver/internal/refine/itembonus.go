package refine

import "github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"

// BonusItem is the slice of STRUCT_ITEMLIST that SetItemBonus reads for the
// item being rolled: nUnique, ReqLvl, nPos and the static stEffect pairs.
type BonusItem struct {
	Unique int
	ReqLvl int
	Pos    int
	Static []StaticEffect
}

// StaticEffect is one STRUCT_ITEMLIST.stEffect pair (sEffect, sValue).
type StaticEffect struct {
	Effect uint8
	Value  int16
}

// Effect ids SetItemBonus writes (ItemEffect.h).
const (
	efGrade0   = 100 // EF_GRADE0
	efGrade5   = 105 // EF_GRADE5
	efUnknown  = 59  // the "random noise" pair the client ignores
	efIncubate = 78  // EF_INCUBATE
)

// bonusTypes is g_pBonusType (Basedef.cpp:585).
var bonusTypes = [10]int{10, 30, 2, 5, 3, 10, 2, 8, 2, 8}

// bonusValues is g_pBonusValue[10][2][2] (Basedef.cpp:532) flattened, followed
// by g_pBonusType. SetItemBonus indexes it as [v20][lvdif][k] with lvdif up to
// 3 (2 on the quest path), past the declared [2]: the read lands on the next
// row and, for row 9, on g_pBonusType, which the compiler emits right after it.
// UNVERIFIED: that layout is MSVC's .data order for the two adjacent
// definitions; it is reproduced instead of clamping so the rolled ranges match.
var bonusValues = [...]int{
	10, 30, 2, 5,
	3, 10, 2, 8,
	2, 8, 2, 8,
	2, 6, 2, 6,
	2, 6, 2, 6,
	20, 45, 4, 8,
	7, 20, 5, 14,
	5, 14, 5, 14,
	4, 12, 4, 12,
	4, 12, 4, 12,
	// g_pBonusType, read by the out-of-bounds [9][2..3] rows.
	10, 30, 2, 5, 3, 10, 2, 8, 2, 8,
}

// ItemBonus is SetItemBonus (Server.cpp:1777-2717): it rolls dest's three
// effect slots from its level band. level is the reference level, quest is the
// legacy a3 flag (the quest/NPC path, which softens every roll) and dropBonus
// is the drop-rate bonus. rnd must be the MSVC rand() (0..32767); the calls run
// in the original order so a captured sequence reproduces byte-for-byte.
func ItemBonus(dest *world.Item, level int, quest bool, dropBonus int, info BonusItem, rnd func() int) {
	addDropBonus := dropBonus / 8
	if addDropBonus < 0 {
		addDropBonus = 0
	}
	if addDropBonus > 2 {
		addDropBonus = 2
	}

	add1, add2 := -1, -1
	if e := dest.Effects[0].Effect; e >= efGrade0 && e <= efGrade5 {
		add1 = int(e - efGrade0)
		add2 = int(dest.Effects[0].Value)
		dest.Effects[0] = world.Effect{}
	}
	if !quest && level >= 210 {
		level -= 47
	}

	nPos := info.Pos
	lvdif := (level - info.ReqLvl + 1) / 25
	if add1 != -1 {
		lvdif = add1
	}
	pForc := lvdif >= 4
	if lvdif < 0 {
		lvdif = 0
	}
	if lvdif > 3 {
		lvdif = 3
	}
	if quest && lvdif >= 3 {
		lvdif = 2
	}

	if nPos&0xFE != 0 && dest.Effects[0].Effect == 0 && nPos != 128 {
		rollItemBonus(dest, lvdif, pForc, quest, addDropBonus, add2, nPos, info.Unique, rnd)
	}

	for _, st := range info.Static {
		switch st.Effect {
		case efSanc:
			dest.Effects[0] = world.Effect{Effect: efSanc, Value: uint8(st.Value)}
		case efAmount:
			dest.Effects[0] = world.Effect{Effect: efAmount, Value: uint8(st.Value)}
		case efIncubate:
			v := rnd()%4 + int(st.Value)
			if v > 9 {
				v = 9
			}
			dest.Effects[0] = world.Effect{Effect: efIncubate, Value: uint8(v)}
		}
	}

	// Items whose three slots must never be empty get noise pairs.
	switch {
	case dest.Index == 412 || dest.Index == 413 || dest.Index == 419 || dest.Index == 420 || dest.Index == 753,
		dest.Index >= 447 && dest.Index <= 450, dest.Index >= 692 && dest.Index <= 695:
		for i := range dest.Effects {
			if dest.Effects[i].Effect == 0 {
				dest.Effects[i] = world.Effect{Effect: efUnknown, Value: uint8(rnd())}
			}
		}
	}
}

// rollItemBonus is the body of SetItemBonus guarded by the equip-slot check.
func rollItemBonus(dest *world.Item, lvdif int, pForc, quest bool, addDropBonus, add2, nPos, nUnique int, rnd func() int) {
	special := nUnique == 44 || nUnique == 47

	// First bonus kind (slot 1).
	v38, v36, v37 := efUnknown, 0, 0
	v35 := rnd() % 101
	var v34 int
	switch {
	case quest:
		v34 = v35 % 3
	case lvdif == 0:
		v34 = v35 % (8 - addDropBonus)
	case lvdif == 1, lvdif == 2:
		v34 = v35 % (6 - addDropBonus)
	default:
		v34 = v35 % 4
	}
	switch nPos {
	case 2:
		switch v34 {
		case 0:
			v38, v36 = 26, 3
		case 1:
			v38, v36 = 60, 2
		}
	case 4, 8:
		v38, v36, v37 = 71, 10, 1
	case 16:
		v38, v36 = 72, 5
	case 32:
		v38, v36, v37 = 73, 6, -1
	case 64, 192:
		if !special {
			switch v34 {
			case 0:
				v38, v36, v37 = 26, 3, 1
			case 1:
				v38, v36, v37 = 2, 9, -1
			case 2:
				v38, v36 = 74, 3
			}
		} else {
			switch v34 {
			case 0:
				v38, v36, v37 = 60, 4, -1
			case 1:
				v38, v36, v37 = 74, 3, 1
			}
		}
	}

	// Second bonus kind (slot 2).
	v33 := rnd() % 100
	if quest {
		v33 = 2 * v33 / 3
	}
	var v32 int
	switch {
	case quest:
		v32 = v33 % 4
	case lvdif == 0:
		v32 = v33 % 8
	case lvdif == 1, lvdif == 2:
		v32 = v33 % 6
	default:
		v32 = v33 % 4
	}
	v31, v30, v29 := efUnknown, 0, 0
	switch nPos {
	case 2:
		switch v32 {
		case 0:
			v31, v30 = 4, 10
		case 1:
			v31, v30, v29 = 3, 5, -1
		}
	case 4, 8:
		switch v32 {
		case 0:
			v31, v30, v29 = 60, 2, -1
		case 1:
			v31, v30, v29 = 2, 6, -1
		case 2:
			v31, v30, v29 = 3, 5, -1
		}
	case 16:
		switch v32 {
		case 0:
			v31, v30, v29 = 60, 2, -1
		case 1:
			v31, v30, v29 = 2, 6, -1
		case 2:
			v31, v30 = 74, 3
		case 3:
			v31, v30 = 54, 3
		}
	case 32:
		switch v32 {
		case 0:
			v31, v30, v29 = 60, 2, -1
		case 1:
			v31, v30 = 74, 3
		}
	case 64, 128, 192:
		if !special {
			switch v32 {
			case 0:
				v31, v30, v29 = 26, 3, 1
			case 1:
				v31, v30, v29 = 2, 9, -1
			case 2:
				v31, v30, v29 = 74, 3, 1
			}
		} else {
			switch v32 {
			case 0:
				v31, v30, v29 = 60, 4, -1
			case 1:
				v31, v30, v29 = 74, 3, 1
			}
		}
	}

	// Slot 1 magnitude.
	v35 = rnd() % 100
	v28 := bonusTier(lvdif, v35)
	if lvdif == 0 {
		switch {
		case v35 < 2:
			v28 = 4
		case v35 < 6:
			v28 = 3
		case v35 < 24:
			v28 = 2
		case v35 < 55:
			v28 = 1
		default:
			v28 = 0
		}
	}
	if pForc && v28 < 4 {
		v28 = 4
	}
	v28 += v37
	if quest && v28 == 0 {
		v28 = 1
	}
	switch {
	case dest.Effects[1].Effect == 0 && v28 > 0:
		dest.Effects[1] = world.Effect{Effect: uint8(v38), Value: uint8(v36 * v28)}
	case dest.Effects[1].Effect == 0 && nPos == 32:
		dest.Effects[1] = world.Effect{Effect: uint8(v38)}
	case dest.Effects[1].Effect == 0:
		dest.Effects[1] = world.Effect{Effect: efUnknown, Value: uint8(rnd() & 0x7F)}
	}

	// Slot 2 magnitude.
	v33 = rnd() % 100
	v27 := bonusTier(lvdif, v33)
	if lvdif == 0 {
		// Parity quirk: the level-0 branch tests the slot-1 roll (v35) and, when
		// it is below 2, writes the slot-1 tier instead of its own
		// (Server.cpp:2433-2450), leaving v27 at 0.
		v27 = 0
		if v35 >= 2 {
			switch {
			case v33 < 6:
				v27 = 3
			case v33 < 24:
				v27 = 2
			case v33 < 55:
				v27 = 1
			}
		}
	}
	if pForc && v27 < 3 {
		v27 = 3
	}
	if quest && v27 >= 5 {
		v27 = 4
	}
	v27 += v29
	if addDropBonus != 0 && v27 == 0 {
		v27 = addDropBonus
	}
	if quest && v27 == 0 {
		v27 = 1
	}
	switch {
	case dest.Effects[2].Effect == 0 && v27 > 0:
		dest.Effects[2] = world.Effect{Effect: uint8(v31), Value: uint8(v30 * v27)}
	case dest.Effects[2].Effect == 0:
		dest.Effects[2] = world.Effect{Effect: efUnknown, Value: uint8(rnd() & 0x7F)}
	}

	if dest.Effects[0].Effect != 0 {
		return
	}
	// Slot 0: the refine level (or an occasional extra stat).
	v26 := rnd() % 100
	if quest {
		v26 /= 2
	}
	v25, v24, v23, v22 := 6, 22, 75, 90
	if lvdif >= 2 {
		v25, v24, v23, v22 = 6, 35, 85, 100
	}
	switch {
	case v26 < v25:
		dest.Effects[0] = world.Effect{Effect: efSanc, Value: 2}
		if add2 > 2 {
			if v := gradeSanc(add2, rnd()%100); v != 0 {
				dest.Effects[0].Value = uint8(v)
			}
		}
	case v26 < v24:
		dest.Effects[0] = world.Effect{Effect: efSanc, Value: 1}
	case v26 < v23:
		dest.Effects[0] = world.Effect{Effect: efSanc, Value: 0}
	case v26 < v22:
		v20 := rnd() % 10
		lo := bonusValues[v20*4+lvdif*2]
		hi := bonusValues[v20*4+lvdif*2+1]
		v14 := rnd()%(hi+1-lo) + lo
		dest.Effects[0] = world.Effect{Effect: uint8(bonusTypes[v20]), Value: uint8(v14)}
	default:
		dest.Effects[0] = world.Effect{Effect: efUnknown, Value: uint8(rnd() & 0x7F)}
	}
}

// bonusTier maps a 0..99 roll to a bonus multiplier for lvdif 1..3 (and the
// unreachable >=4 band, kept for parity). lvdif 0 is handled by the caller,
// whose two slots differ.
func bonusTier(lvdif, r int) int {
	switch lvdif {
	case 1:
		switch {
		case r < 1:
			return 5
		case r < 5:
			return 4
		case r < 24:
			return 3
		case r < 65:
			return 2
		}
		return 1
	case 2:
		switch {
		case r < 2:
			return 5
		case r < 16:
			return 4
		case r < 60:
			return 3
		}
		return 2
	case 3:
		switch {
		case r < 2:
			return 6
		case r < 9:
			return 5
		case r < 45:
			return 4
		case r < 75:
			return 3
		}
		return 2
	}
	if lvdif >= 4 {
		switch {
		case r < 2:
			return 7
		case r < 10:
			return 6
		case r < 30:
			return 5
		case r < 75:
			return 4
		}
		return 3
	}
	return 0
}

// gradeSanc is the EF_GRADE upgrade of a +2 roll (Server.cpp:2558-2652): the
// grade carried in add2 may lift the refine to 3..7. 0 keeps the +2.
func gradeSanc(add2, r int) int {
	// Each row lists, from the best outcome down, the roll ceiling for it.
	var cuts []struct{ below, sanc int }
	switch add2 {
	case 3:
		cuts = []struct{ below, sanc int }{{30, 3}}
	case 4:
		cuts = []struct{ below, sanc int }{{10, 4}, {40, 3}}
	case 5:
		cuts = []struct{ below, sanc int }{{10, 5}, {30, 4}, {60, 3}}
	case 6:
		cuts = []struct{ below, sanc int }{{10, 6}, {20, 5}, {40, 4}, {60, 3}}
	case 7:
		cuts = []struct{ below, sanc int }{{4, 7}, {10, 6}, {20, 5}, {35, 4}, {60, 3}}
	}
	for _, c := range cuts {
		if r < c.below {
			return c.sanc
		}
	}
	return 0
}
