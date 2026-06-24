package world

import "math/rand"

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
}

// TeleportDest resolves a teleport from (x,y): it rounds to the tile, looks up
// the route, and returns the destination (+rand%3 spread) and gold cost. ok is
// false when there is no teleport tile at that position.
func TeleportDest(x, y int16) (destX, destY int16, cost int32, ok bool) {
	r, found := teleportTable[[2]int16{x &^ 3, y &^ 3}] // round down to a multiple of 4
	if !found {
		return 0, 0, 0, false
	}
	return r.dx + int16(rand.Intn(3)), r.dy + int16(rand.Intn(3)), r.cost, true
}
