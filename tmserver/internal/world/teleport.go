package world

import "math/rand"

type teleRoute struct {
	dx, dy int16
	cost   int32
}

// teleportTable maps a rounded origin tile (x&0xFFFC, y&0xFFFC) to its
// destination + gold cost. It is the complete GetTeleportPosition
// (GetFunc.cpp:782-1026) — all 37 routes, in the order the original tests them.
// The client sends an empty _MSG_ReqTeleport when it steps on a teleport tile;
// the server resolves the route from the player's position.
//
// This used to carry 18 of the 37, and the missing 19 were not a random subset:
// they were EVERY stair between dungeon floors. A player standing on the tile at
// (744,3820) that leads to the Dungeon 3rd floor got silence, because the tile
// was not in the map — the floors past the first were reachable only by
// coordinates. Noatum is the hub: the three cities pay 700 to reach it; leaving
// Noatum is free.
var teleportTable = map[[2]int16]teleRoute{
	// Cities ↔ Noatum, the paid hub.
	{2116, 2100}: {1044, 1724, 700}, // Armia → Noatum
	{2480, 1716}: {1044, 1716, 700}, // Azran → Noatum
	{2456, 2016}: {1044, 1708, 700}, // Erion → Noatum
	{1044, 1724}: {2116, 2100, 0},   // Noatum → Armia
	{1044, 1716}: {2480, 1716, 0},   // Noatum → Azran
	{1044, 1708}: {2456, 2016, 0},   // Noatum → Erion
	{1048, 1764}: {1100, 1712, 0},   // Noatum guilda → área de cerco de Noatum
	{1052, 1708}: {3650, 3110, 0},   // Noatum → Karden
	{3648, 3108}: {1054, 1710, 0},   // Karden → Noatum
	{1056, 1724}: {3250, 1703, 0},   // Noatum → RvR, Deserto

	// Cities ↔ their fields.
	{2140, 2068}: {2588, 2096, 0}, // Armia → Campo de Armia
	{2468, 1716}: {2248, 1556, 0}, // Azran → Campo de Azran, junto da quest da capa
	{2452, 1716}: {1969, 1711, 0}, // Azran, 2º teleporte → Campo de Azran
	{2452, 1988}: {1989, 1755, 0}, // Erion → Campo de Azran

	// Dungeon, 1º andar (as duas bocas, no Campo de Armia).
	{2364, 2284}: {144, 3788, 0},  // Campo de Armia → Dungeon 1º andar
	{144, 3788}:  {2364, 2284, 0}, // Dungeon 1º andar → Campo de Armia
	{2668, 2156}: {148, 3774, 0},  // Campo de Armia → Dungeon 1º andar (outra boca)
	{144, 3772}:  {2668, 2156, 0}, // Dungeon 1º andar → Campo de Armia (outra boca)

	// Dungeon, escadas entre andares. Nenhuma destas existia neste port.
	// Os dois tiles 148/144 x 3780 descem para o mesmo ponto: é assim no original,
	// uma escada larga o bastante para ocupar duas casas arredondadas.
	{148, 3780}:  {1004, 4028, 0}, // Dungeon 1º → 2º andar
	{144, 3780}:  {1004, 4028, 0}, // Dungeon 1º → 2º andar (casa vizinha da mesma escada)
	{1004, 4028}: {148, 3780, 0},  // Dungeon 2º → 1º andar
	{408, 4072}:  {1004, 4064, 0}, // Dungeon 1º → 2º andar
	{1004, 4064}: {408, 4072, 0},  // Dungeon 2º → 1º andar
	{744, 3820}:  {1004, 3992, 0}, // Dungeon 1º → 3º andar
	{1004, 3992}: {744, 3820, 0},  // Dungeon 3º → 1º andar
	{680, 4076}:  {916, 3820, 0},  // Dungeon 2º → 3º andar
	{916, 3820}:  {680, 4076, 0},  // Dungeon 3º → 2º andar
	{876, 3872}:  {932, 3820, 0},  // Dungeon 2º → 3º andar
	{932, 3820}:  {876, 3872, 0},  // Dungeon 3º → 2º andar

	// Submundo.
	{1824, 1772}: {1172, 4080, 0}, // Campo de Azran → Submundo
	{1172, 4080}: {1824, 1772, 0}, // Submundo → Campo de Azran
	{1516, 3996}: {1304, 3816, 0}, // Submundo 1º → 2º andar
	{1304, 3816}: {1516, 3996, 0}, // Submundo 2º → 1º andar

	// Guerra e vale.
	{188, 188}:   {2548, 1740, 0}, // Área de guerra → Azran
	{2548, 1740}: {2281, 3688, 0}, // Azran → vale

	// As duas últimas rotas de GetFunc.cpp:1010-1024 não trazem comentário no
	// original e o par não é simétrico: a ida sai de (1312,1900) e a volta chega em
	// (1314,1900). Portadas como estão, incluindo a assimetria.
	{1312, 1900}: {2366, 4073, 0},
	{2364, 4072}: {1314, 1900, 0},
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
