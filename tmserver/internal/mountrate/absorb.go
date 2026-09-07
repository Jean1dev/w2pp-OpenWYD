package mountrate

import "github.com/jeanluca/w2pp-openwyd/internal/domain"

// Absorb is one lineage's pair: how much of a hit the mount eats instead of its
// owner, against a player and against a monster.
//
// Both numbers live in one value because they are the two halves of one decision
// about what the mount is for — a lineage configured on one axis only would be a
// mount nobody designed.
type Absorb struct {
	PvP int8 // 0..100
	PvE int8 // 0..100
}

// AbsorbTable maps an ADULT mount index (2360..2389) to its pair. A lineage
// absent from the table has no configuration, and the caller's default applies.
type AbsorbTable map[int16]Absorb

// Percent returns how much of a hit the mount at mountIndex absorbs, and whether
// a configuration exists for it. byPlayer picks the axis: true when the blow
// came from another player, false when it came from a monster.
//
// The caller keeps its own default for the false case rather than this package
// inventing one, the same split Table.Rate uses: the default is a game rule and
// belongs with the game, not with the storage shape.
func (t AbsorbTable) Percent(mountIndex int16, byPlayer bool) (int, bool) {
	if t == nil {
		return 0, false
	}
	a, ok := t[mountIndex]
	if !ok {
		return 0, false
	}
	if byPlayer {
		return int(a.PvP), true
	}
	return int(a.PvE), true
}

// IsAdultMount reports whether an item index is one of the adult lineages an
// absorption pair can be set for. Shared with the growth curve because both
// overlays cover exactly the same thirty rows.
func IsAdultMount(index int16) bool {
	return index >= domain.MountAdultLo && index <= domain.MountAdultHi
}
