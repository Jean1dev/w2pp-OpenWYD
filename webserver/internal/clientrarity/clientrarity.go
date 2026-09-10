// Package clientrarity decides the rarity tier the client's item tooltip shows
// — the border colour and the "Item nível X" line GamePatch.dll draws — and
// writes it as the table the DLL reads (GamePatchItens.bin).
//
// The rule lives here, in Go, and not in the DLL, so there is one place to read
// and test it and, later, one place for the staff panel to feed. It is built
// from what the catalog already says about each item:
//
//   - Grade 5-8 is an Ancient ("(Anct)") item: Divino.
//   - EF_MOBTYPE 3 is Celestial-only equipment: Mítico.
//   - EF_MOBTYPE 1 is Arch-only equipment, and Grade 4 is "(Le)": Lendário.
//   - Grades 3, 2 and 1 are the armour letters (A), (M) and (N): Épico, Raro
//     and Comum.
//   - Anything else — every Mortal weapon, which carries no letter, and the few
//     ungraded armours — goes by its level requirement, topping out at Épico:
//     Lendário is kept for (Le) and Arch.
//
// Refinement raises it further, per item, so it cannot be in the per-index
// table: the table carries the floor each refinement guarantees (+11 and +12
// Épico, +13 Lendário, +14 Mítico, +15 Divino) and GamePatch.dll takes the
// higher of the two for the item under the mouse.
//
// Only equipment is classified: armour (helm, body, legs, gloves, boots) and
// weapons and shields. Mounts are not here; GamePatch.dll ranks them from
// GamePatch.txt, by name, because their tiers were chosen one by one.
package clientrarity

import (
	"encoding/binary"
	"fmt"
)

// Tier is a rarity step, in the order the tooltip ranks them. The byte values
// are the file format: GamePatch.dll maps each to its label and colour.
type Tier uint8

// The tiers. None leaves the client's own tooltip untouched.
const (
	None Tier = iota
	Comum
	Incomum
	Raro
	Epico
	Lendario
	Mitico
	Divino
)

// String is the label the tooltip prints, as the DLL spells it.
func (t Tier) String() string {
	switch t {
	case Comum:
		return "Comum"
	case Incomum:
		return "Incomum"
	case Raro:
		return "Raro"
	case Epico:
		return "Épico"
	case Lendario:
		return "Lendário"
	case Mitico:
		return "Mítico"
	case Divino:
		return "Divino"
	}
	return ""
}

// Item is what the rule reads from one catalog record.
type Item struct {
	Index    int
	Name     string
	ReqLevel int
	Pos      int
	Grade    int
	MobType  int // EF_MOBTYPE: 1 Arch, 2 Mortal, 3 Celestial, 0 anyone
}

// The equipment slots, as ItemList's nPos bit mask.
const (
	posHelm   = 2
	posBody   = 4
	posLegs   = 8
	posGloves = 16
	posBoots  = 32
	posRight  = 64
	posLeft   = 128
	posBoth   = 192
)

func isEquipment(pos int) bool {
	switch pos {
	case posHelm, posBody, posLegs, posGloves, posBoots, posRight, posLeft, posBoth:
		return true
	}
	return false
}

// The EF_MOBTYPE values (Basedef.h: ARCH 1, CELESTIAL 3), which BASE_CanEquip
// uses to keep the item on that evolution.
const (
	mobTypeArch      = 1
	mobTypeCelestial = 3
)

// exceptions are items the rule would rank wrong, decided one by one. The
// Demolidor Celestial is a Mortal-only weapon (EF_MOBTYPE 2) despite the name,
// and at level 312 the rule would make it Épico; it is Incomum. Its Ancient
// versions (3781-3784) stay Divino with every other Ancient.
var exceptions = map[int]Tier{
	3596: Incomum, // Demolidor_Celestial
}

// Classify is the tier of one catalog item.
func Classify(it Item) Tier {
	if !isEquipment(it.Pos) {
		return None
	}
	if t, ok := exceptions[it.Index]; ok {
		return t
	}
	switch {
	case it.Grade >= 5 && it.Grade <= 8:
		return Divino
	case it.MobType == mobTypeCelestial:
		return Mitico
	case it.MobType == mobTypeArch, it.Grade == 4:
		return Lendario
	case it.Grade == 3:
		return Epico
	case it.Grade == 2:
		return Raro
	case it.Grade == 1:
		return Comum
	}
	return byLevel(it.ReqLevel)
}

// RefineFloor is the lowest tier an equipment item refined to +10, +11 … +15
// shows, indexed by refinement minus 10. Below +11 the catalog tier stands.
var RefineFloor = [6]Tier{
	None,     // +10
	Epico,    // +11
	Epico,    // +12
	Lendario, // +13
	Mitico,   // +14
	Divino,   // +15
}

// WithRefine is the tier an item of catalog tier t shows at the given
// refinement: the higher of the two. Items outside the table stay outside.
func WithRefine(t Tier, refine int) Tier {
	if t == None || refine < 10 || refine > 15 {
		return t
	}
	return max(t, RefineFloor[refine-10])
}

// byLevel ranks equipment the catalog gives no letter to.
func byLevel(level int) Tier {
	switch {
	case level < 100:
		return Comum
	case level < 150:
		return Incomum
	case level < 200:
		return Raro
	}
	return Epico
}

// The client ItemList.bin: 6500 records of 140 bytes under a flat XOR 0x5A,
// plus 4 trailing bytes. Name[64], then shorts: mesh, texture, vfx, ReqLvl
// (+70), ReqStr/Int/Dex/Con; twelve (code, value) effect pairs at +80; int
// price at +128; then shorts nUnique, nPos (+134), Extra, Grade (+138).
const (
	itemCount      = 6500
	itemRecord     = 140
	itemListSize   = itemCount*itemRecord + 4
	itemXOR        = 0x5A
	offReqLevel    = 70
	offEffects     = 80
	effectSlots    = 12
	offPos         = 134
	offGrade       = 138
	efMobType      = 112
	nameBytes      = 64
	fileMagic      = "GPRI"
	fileHeaderSize = 4 + 2 + 1 + len(RefineFloor)
)

// ReadItemList decodes the catalog records the rule needs. Empty records are
// skipped.
func ReadItemList(il []byte) ([]Item, error) {
	if len(il) != itemListSize {
		return nil, fmt.Errorf("clientrarity: o ItemList.bin tem %d bytes, esperava %d", len(il), itemListSize)
	}
	rec := make([]byte, itemRecord)
	short := func(off int) int { return int(int16(binary.LittleEndian.Uint16(rec[off:]))) }
	var items []Item
	for i := 0; i < itemCount; i++ {
		for j, b := range il[i*itemRecord : (i+1)*itemRecord] {
			rec[j] = b ^ itemXOR
		}
		if rec[0] == 0 {
			continue
		}
		it := Item{
			Index:    i,
			ReqLevel: short(offReqLevel),
			Pos:      int(uint16(short(offPos))),
			Grade:    short(offGrade),
		}
		n := 0
		for n < nameBytes && rec[n] != 0 {
			n++
		}
		it.Name = string(rec[:n])
		for s := 0; s < effectSlots; s++ {
			if short(offEffects+4*s) == efMobType {
				it.MobType = short(offEffects + 4*s + 2)
				break
			}
		}
		items = append(items, it)
	}
	return items, nil
}

// Table is GamePatchItens.bin: the magic "GPRI", the item count as a
// little-endian uint16, the number of refinement floors (6) and the floors
// for +10 … +15 as Tier bytes, then one Tier byte per item index.
func Table(items []Item) []byte {
	out := make([]byte, fileHeaderSize+itemCount)
	copy(out, fileMagic)
	binary.LittleEndian.PutUint16(out[4:], itemCount)
	out[6] = byte(len(RefineFloor))
	for i, t := range RefineFloor {
		out[7+i] = byte(t)
	}
	for _, it := range items {
		if it.Index >= 0 && it.Index < itemCount {
			out[fileHeaderSize+it.Index] = byte(Classify(it))
		}
	}
	return out
}

// Count is how many items fall in each tier — the generator's report.
func Count(items []Item) map[Tier]int {
	c := map[Tier]int{}
	for _, it := range items {
		if t := Classify(it); t != None {
			c[t]++
		}
	}
	return c
}
