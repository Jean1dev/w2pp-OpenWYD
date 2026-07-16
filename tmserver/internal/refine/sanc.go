// Package refine ports the dust-refine core of _MSG_UseItem ("refino com
// poeiras"): the packed sanc encoding and the success-rate tables. Like
// internal/combine it is pure — no world state, no I/O — because the encoding
// and the rates are parity-critical economy constants (parity-tests.md §4.0).
//
// The refine level does NOT live in the item's cValue as a plain number. It is
// packed together with a second counter (Basedef.cpp:2282, BASE_SetItemSanc):
//
//	levels 0..9   cValue = level + 10*pity     (pity 0..20, so 0..209)
//	levels 10..15 cValue = base + gem          (base 230/234/238/242/246/250, gem 0..3)
//
// Reading a cValue back as-is therefore reports a wildly wrong level for any
// item that has ever failed a refine.
//
// This package reports the TRUE level (0..15). The legacy BASE_GetItemSanc
// instead returns the non-contiguous REF_* sentinels (Basedef.h:256): REF_10=10,
// REF_11=12, REF_12=15, REF_13=18, REF_14=22, REF_15=27. Every comparison the
// dust path makes (== 9, >= 6, >= REF_11) yields the same answer in either
// space, with one translation: `sanc >= REF_11` becomes `level >= 11`.
package refine

import "github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"

// Effect ids that can carry a refine level. BASE_GetItemSanc prefers the
// [116,125] range over EF_SANC when an item somehow has both.
const (
	efSanc      = 43  // EF_SANC (ItemEffect.h:100)
	efSancAltLo = 116 // the alternate carriers (ItemEffect.h), used by some minted items
	efSancAltHi = 125
)

// Mount-growth items (2330..2389) are rejected outright by every
// BASE_GetItemSanc-family function (Basedef.cpp:2138) — they reuse the effect
// slots for growth data, so a sanc read there would be garbage.
const (
	growthLo = 2330
	growthHi = 2390
)

// gemBase is the first packed cValue of a +10 item; each level spans 4 values,
// one per gem index (Basedef.cpp:2294-2308).
const (
	gemBase    = 230
	gemPerLvl  = 4
	minGemLvl  = 10
	maxSancLvl = 15
	maxPity    = 20
)

func isGrowth(it world.Item) bool { return it.Index >= growthLo && it.Index < growthHi }

// readSlot returns the effect slot BASE_GetItemSanc and BASE_GetItemGem read
// (Basedef.cpp:2141-2160): ALL slots are scanned for the [116,125] range first,
// and only then for EF_SANC. -1 when the item carries neither.
func readSlot(it world.Item) int {
	for i, ef := range it.Effects {
		if ef.Effect >= efSancAltLo && ef.Effect <= efSancAltHi {
			return i
		}
	}
	for i, ef := range it.Effects {
		if ef.Effect == efSanc {
			return i
		}
	}
	return -1
}

// writeSlot returns the effect slot BASE_SetItemSanc writes and
// BASE_GetItemSancSuccess reads (Basedef.cpp:2312, :2225): the first slot that
// is EF_SANC *or* in [116,125], scanning 0→1→2. -1 when there is none, which is
// the legacy's FALSE return — the level is silently not stored.
//
// Parity quirk: this is deliberately NOT readSlot. On an item carrying both an
// EF_SANC pair and a [116,125] pair, the legacy reads the level from one slot
// and the pity from another. The bootstrap in the dust path normalizes items to
// a single pair, so this is only reachable for items minted elsewhere (a legacy
// save or a DB import). Reproduced rather than tidied.
func writeSlot(it world.Item) int {
	for i, ef := range it.Effects {
		if ef.Effect == efSanc || (ef.Effect >= efSancAltLo && ef.Effect <= efSancAltHi) {
			return i
		}
	}
	return -1
}

// decodeLevel maps a packed cValue to a true refine level 0..15
// (BASE_GetItemSanc, Basedef.cpp:2162-2180).
func decodeLevel(v uint8) int {
	if v >= gemBase && v <= gemBase+(maxSancLvl-minGemLvl+1)*gemPerLvl-1 {
		return minGemLvl + int(v-gemBase)/gemPerLvl
	}
	// Everything else (including the unreachable 210..229 and the 254/255 tail)
	// folds by %10, which is what strips the pity back off.
	return int(v) % 10
}

// encode packs a level and its aux counter into a cValue (BASE_SetItemSanc,
// Basedef.cpp:2282-2310). aux is the pity counter below +10 and the gem index
// from +10 up.
func encode(level, aux int) uint8 {
	// Legacy quirk: an out-of-range level wipes the item to +0 — it does NOT
	// clamp to +15 (Basedef.cpp:2285).
	if level < 0 || level > maxSancLvl {
		level = 0
	}
	if aux < 0 {
		aux = 0
	}
	if aux > maxPity {
		aux = maxPity
	}
	if level >= minGemLvl {
		// Overflows past +15 with a large aux, exactly as the legacy's assignment
		// to an unsigned char does.
		return uint8(gemBase + (level-minGemLvl)*gemPerLvl + aux)
	}
	return uint8(level + 10*aux)
}

// Level returns an item's true refine level, 0..15 (BASE_GetItemSanc). 0 means
// unrefined.
func Level(it world.Item) int {
	if isGrowth(it) {
		return 0
	}
	i := readSlot(it)
	if i < 0 {
		return 0
	}
	return decodeLevel(it.Effects[i].Value)
}

// Pity returns the accumulated "success" counter (BASE_GetItemSancSuccess).
// Failed refines raise it and it feeds back into the next attempt's rate. From
// +10 up the same field holds the gem index instead, so the two must not be
// mixed — the dust path only ever reads Pity below +10.
func Pity(it world.Item) int {
	if isGrowth(it) {
		return 0
	}
	i := writeSlot(it)
	if i < 0 {
		return 0
	}
	return int(it.Effects[i].Value) / 10
}

// Gem returns the gem index of a +10..+15 item, or -1 below +10
// (BASE_GetItemGem).
func Gem(it world.Item) int {
	if isGrowth(it) {
		// The legacy returns FALSE here, i.e. gem 0 — not the -1 that means "no
		// gem" (Basedef.cpp:2186). Reproduced.
		return 0
	}
	i := readSlot(it)
	if i < 0 {
		return -1
	}
	v := it.Effects[i].Value
	if v < gemBase {
		return -1
	}
	return int(v-gemBase) % gemPerLvl
}

// Set stores a level and aux counter in the item's sanc slot, reporting whether
// a slot was available (BASE_SetItemSanc). It never allocates one: the caller
// plants the EF_SANC pair first (Bootstrap).
func Set(it *world.Item, level, aux int) bool {
	i := writeSlot(*it)
	if i < 0 {
		return false
	}
	it.Effects[i].Value = encode(level, aux)
	return true
}

// Bootstrap plants a zeroed EF_SANC pair in the first slot free to hold one, so
// that a never-refined item has somewhere to store its level
// (_MSG_UseItem.cpp:814-840). A slot counts as free when it is empty or already
// carries a sanc pair. It reports false when all three slots hold real effects —
// the legacy refuses to refine such an item at all.
func Bootstrap(it *world.Item) bool {
	for i, ef := range it.Effects {
		if ef.Effect == 0 || ef.Effect == efSanc || (ef.Effect >= efSancAltLo && ef.Effect <= efSancAltHi) {
			it.Effects[i] = world.Effect{Effect: efSanc, Value: 0}
			return true
		}
	}
	return false
}
