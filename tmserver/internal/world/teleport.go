package world

import (
	"math/rand"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/rng"
)

type teleRoute struct {
	dx, dy int16
	cost   int32
}

// teleportTable maps a rounded origin tile (x&0xFFFC, y&0xFFFC) to its
// destination + gold cost (GetTeleportPosition, GetFunc.cpp). The client sends an
// empty _MSG_ReqTeleport when it steps on a teleport tile; the server resolves
// the route from the player's position. Noatum is the hub: the three cities pay
// 700 to reach it; travel out of Noatum is free.
var teleportTable = map[[2]int16]teleRoute{
	{2116, 2100}: {1044, 1724, 700}, // Armia → Noatum
	{2480, 1716}: {1044, 1716, 700}, // Azran → Noatum
	{2456, 2016}: {1044, 1708, 700}, // Erion → Noatum
	{1044, 1724}: {2116, 2100, 0},   // Noatum → Armia
	{1044, 1716}: {2480, 1716, 0},   // Noatum → Azran
	{1044, 1708}: {2456, 2016, 0},   // Noatum → Erion
	{2548, 1740}: {2281, 3688, 0},   // Azran → Vale (Fada do Vale, item 3916)
	{1052, 1708}: {3650, 3110, 0},   // Noatum → Nippleheim
	{3648, 3108}: {1054, 1710, 0},   // Nippleheim → Noatum
	// Fields / dungeons (subset of GetTeleportPosition).
	{2140, 2068}: {2588, 2096, 0}, // Armia → Armia Field
	{2468, 1716}: {2248, 1556, 0}, // Azran → Azran Field
	{2364, 2284}: {144, 3788, 0},  // Armia Field → Dungeon 1
	{144, 3788}:  {2364, 2284, 0}, // Dungeon 1 → Armia Field
	{2668, 2156}: {148, 3774, 0},  // Armia Field → Dungeon 1 (alt)
	{144, 3772}:  {2668, 2156, 0}, // Dungeon 1 → Armia Field (alt)
	{1824, 1772}: {1172, 4080, 0}, // Azran Field → Underworld
	{1172, 4080}: {1824, 1772, 0}, // Underworld → Azran Field
	// GetFunc.cpp: both adjacent entrance blocks lead to floor 2; the return
	// deliberately lands in the eastern block, not the western entrance.
	{144, 3780}:  {1004, 4028, 0}, // Dungeon 1 → Dungeon 2
	{148, 3780}:  {1004, 4028, 0}, // Dungeon 1 → Dungeon 2
	{1004, 4028}: {148, 3780, 0},  // Dungeon 2 → Dungeon 1
	{408, 4072}:  {1004, 4064, 0}, // Dungeon 1 → Dungeon 2 (alt)
	{1004, 4064}: {408, 4072, 0},  // Dungeon 2 → Dungeon 1 (alt)
}

// TeleportDest resolves a teleport from (x,y): it rounds to the tile, looks up
// the route, and returns the destination (+rand%3 spread) and gold cost. ok is
// false when there is no teleport tile at that position.
func TeleportDest(x, y int16) (destX, destY int16, cost int32, ok bool) {
	return teleportDest(x, y, false, nil)
}

// TeleportDestWithAccess resolves routes whose access can depend on equipped items.
// The legacy Vale portal requires item 3916 in Equip[13].
func TeleportDestWithAccess(x, y int16, hasFairy bool, random *rng.MSVC) (destX, destY int16, cost int32, ok bool) {
	return teleportDest(x, y, hasFairy, random)
}

func teleportDest(x, y int16, hasFairy bool, random *rng.MSVC) (destX, destY int16, cost int32, ok bool) {
	r, found := teleportTable[[2]int16{x &^ 3, y &^ 3}] // round down to a multiple of 4
	if !found || (x&^3 == 2548 && y&^3 == 1740 && !hasFairy) {
		return 0, 0, 0, false
	}
	spread := func() int { return rand.Intn(3) }
	// Keep existing route randomness unchanged; the Vale branch is the one
	// newly ported from the legacy path and uses the world's MSVC stream.
	if random != nil && x&^3 == 2548 && y&^3 == 1740 {
		spread = func() int { return random.Intn(3) }
	}
	return r.dx + int16(spread()), r.dy + int16(spread()), r.cost, true
}
