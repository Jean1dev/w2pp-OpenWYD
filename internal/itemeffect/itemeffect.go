// Package itemeffect holds the meaning of an ItemList.csv effect: the EF_*
// tokens the score model understands, and the ids they map to.
//
// It lives at the root rather than inside tmserver/internal/content because two
// services need it now. The tmServer parses the catalog and applies the numbers;
// the webServer reads the same catalog to show a moderator what an item grants
// today, before they change it. Go's internal rule keeps the webServer out of
// tmserver/internal, and a second copy of these ids would be worse than the
// import: a copy that drifted would not fail, it would quietly attribute a
// number to the wrong effect, which is close to unfindable in game.
//
// tmserver/internal/content aliases these types, so nothing that already speaks
// in content.BaseEffect had to change.
package itemeffect

import (
	"strconv"
	"strings"
)

// BaseEffect is one static item effect (STRUCT_ITEMLIST.stEffect): an EF_* effect
// id and its value — the item's inherent stats (weapon damage, armor AC, attribute
// bonuses). These come from the catalog by item index, distinct from the per-item
// instance refines stored on STRUCT_ITEM.
type BaseEffect struct {
	Eff uint8
	Val int16
}

// Req is what a character must have to equip an item: ItemList.csv column 3,
// written "Lvl.Str.Int.Dex.Con".
type Req struct {
	Lvl, Str, Int, Dex, Con int16
}

// efName maps the ItemList.csv EF_<name> tokens to their STRUCT_EFFECT.cEffect ids
// (the same ids the instance refines use). Only effects the score model can represent
// are mapped. EF_WTYPE is included because some Huntress affects key off the
// equipped weapon type, but it still does not fold into CurrentScore. The purely
// visual/requirement ones (EF_CLASS/EF_GRID/EF_RANGE/EF_REGEN*/…) are ignored. The ids
// match ItemEffect.h. EF_SANC carries an item's refine level (the joias), consumed as a
// multiplier by the handler rather than a flat stat.
//
// EF_CRITICAL/EF_CRITICAL2 ARE score stats (Basedef.cpp:3209 derives MOB.Critical from
// them) — they were misfiled as visual here, which is why crit gear granted nothing
// (issue #102). Most crit lives in the catalog: the class body items and the armor sets
// carry EF_CRITICAL directly.
//
// EF_RESIST1..4/EF_RESISTALL (ItemEffect.h:106-111) are the same bug class (issue #211):
// they ARE score stats too (CMob.cpp:640-643 derives MOB.Resist[0..3] from them via
// BASE_GetMobAbility), so leaving them off this whitelist silently dropped every
// immunity/resist item's effect at parse time — the items looked correct in the catalog
// but granted nothing on the character.
//
// EF_MAGIC/EF_MAGICADD (ItemEffect.h:117,124-125) are the same bug class again (issue #223):
// Basedef.cpp:3194-3195 derives the caster's Magic score from BASE_GetMobAbility(EF_MAGIC)+
// BASE_GetMobAbility(EF_MAGICADD), so an "Ataque Mágico" item's bonus was silently dropped at
// parse time — the item looked correct in the catalog but granted no extra magic damage.
var efName = map[string]uint8{
	"EF_DAMAGE": 2, "EF_AC": 3, "EF_HP": 4, "EF_MP": 5,
	"EF_STR": 7, "EF_INT": 8, "EF_DEX": 9, "EF_CON": 10,
	"EF_SPECIAL1": 11, "EF_SPECIAL2": 12, "EF_SPECIAL3": 13, "EF_SPECIAL4": 14,
	"EF_SPECIALALL": 74,
	"EF_POS":        17, "EF_WTYPE": 21, "EF_CRITICAL": 42, "EF_SANC": 43,
	"EF_HPADD": 45, "EF_MPADD": 46,
	"EF_RESIST1": 49, "EF_RESIST2": 50, "EF_RESIST3": 51, "EF_RESIST4": 52, "EF_ACADD": 53, "EF_RESISTALL": 54,
	"EF_MAGIC":     60,
	"EF_DAMAGEADD": 67, "EF_MAGICADD": 68, "EF_HPADD2": 69, "EF_MPADD2": 70, "EF_CRITICAL2": 71,
	"EF_ITEMLEVEL": 87, "EF_MOBTYPE": 112, "EF_RUNSPEED": 29,
	// EF_ITEMTYPE is not a score stat either: it is read only by the combine
	// matchers (GetFunc.cpp:487 Agatha) as a recipe gate, exactly like EF_NOSANC
	// gates the refine path. Without it in this whitelist those gates read 0 for
	// every item and silently pass/fail the wrong way.
	"EF_ITEMTYPE": 113,
	// Refine gates (_MSG_UseItem.cpp dust path): EF_NOSANC marks an item that can
	// never be refined; the two incubation effects drive the mount-egg branch.
	"EF_NOSANC": 126, "EF_INCUBATE": 78, "EF_INCUDELAY": 84,
	// EF_NOTRADE gates the trade window and the personal shop
	// (_MSG_Trade.cpp:180, _MSG_SendAutoTrade.cpp:85). It is the same bug class as
	// EF_ITEMTYPE above: 156 rows of ItemList.csv carry it, and while it was missing
	// from this table every one of those items read 0 and traded freely.
	"EF_NOTRADE": 127,
}

// EffectID returns the STRUCT_EFFECT id for an EF_<name> token, and whether the
// score model understands it.
//
// Exported so the moderator override path (tmserver/internal/itemstat) can name
// effects the way ItemList.csv does, resolved against this table rather than a
// second copy of the ids. A copy that drifted would not fail: it would quietly
// grant the wrong stat, which is close to unfindable in game.
func EffectID(name string) (uint8, bool) {
	id, ok := efName[name]
	return id, ok
}

// ID returns the STRUCT_EFFECT id for an EF_<name> token, and whether the score
// model understands it.
func ID(name string) (uint8, bool) {
	id, ok := efName[name]
	return id, ok
}

// Names returns every EF_* token the score model understands, unordered. Useful
// to a caller that has to prove its own table covers the same set.
func Names() []string {
	out := make([]string, 0, len(efName))
	for n := range efName {
		out = append(out, n)
	}
	return out
}

// ParsePairs reads the effects out of one ItemList.csv row.
//
// The row is scanned for an EF_* token with a number after it, wherever it sits:
// the legacy CSV has no fixed column for effects, and an unrecognised token is
// skipped rather than treated as an error, because the file carries plenty this
// model has no use for.
func ParsePairs(fields []string) []BaseEffect {
	var out []BaseEffect
	for i := 0; i+1 < len(fields); i++ {
		id, ok := efName[strings.TrimSpace(fields[i])]
		if !ok {
			continue
		}
		v, err := strconv.Atoi(strings.TrimSpace(fields[i+1]))
		if err != nil {
			continue
		}
		out = append(out, BaseEffect{Eff: id, Val: int16(v)})
	}
	return out
}

// Named tokens the game reads but the score model must NOT: EF_RANGE is attack
// reach and EF_VOLATILE is the use-item class. They are absent from efName on
// purpose — folding either into CurrentScore is a bug — so they are named here
// for the callers that read them through their own maps.
const (
	// EFRange is attack reach. Read at mob spawn as the largest EF_RANGE over a
	// template's equips (tmserver/internal/world/api.go).
	EFRange = "EF_RANGE"
	// EFVolatile is what an item does when used: potion, divine, scroll, stone.
	EFVolatile = "EF_VOLATILE"
)

// PairValue reads the value of one EF_<name> pair from a catalog row.
//
// The pairs are found by NAME anywhere in the row rather than at a fixed column,
// which is why the item catalog does not depend on the column mapping being
// pinned down. Three places were scanning for a token with the same loop; this
// is that loop, once.
//
// The second result separates "the row does not carry it" from "it carries
// zero", which the maps built from this cannot express — a missing key and a
// zero value read identically.
func PairValue(fields []string, name string) (int16, bool) {
	for i := 0; i+1 < len(fields); i++ {
		if strings.TrimSpace(fields[i]) != name {
			continue
		}
		v, err := strconv.Atoi(strings.TrimSpace(fields[i+1]))
		if err != nil {
			return 0, false
		}
		return int16(v), true
	}
	return 0, false
}

// ParseReq reads the "Lvl.Str.Int.Dex.Con" requirement column. The second
// result is false for a malformed column or an all-zero requirement — the
// caller stores nothing in that case, so "no requirement" stays a single state
// rather than two that compare differently.
func ParseReq(field string) (Req, bool) {
	parts := strings.Split(strings.TrimSpace(field), ".")
	if len(parts) != 5 {
		return Req{}, false
	}
	var v [5]int16
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return Req{}, false
		}
		v[i] = int16(n)
	}
	req := Req{Lvl: v[0], Str: v[1], Int: v[2], Dex: v[3], Con: v[4]}
	if req == (Req{}) {
		return Req{}, false
	}
	return req, true
}
