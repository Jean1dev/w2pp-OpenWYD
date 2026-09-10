// Package mountbonus is the one place a mount's attribute bonus is decided:
// the attack, magic, evasion and immunity it lends its rider.
//
// It is shared on purpose. The game applies these numbers, the staff panel shows
// and edits them, and the client-file generator writes them into the client so
// its tooltip says the same thing. Three copies of one table is exactly how the
// server ended up applying the Dragão Vermelho's numbers to every mount while
// the client showed each mount's own — so there is one copy, here.
//
// A mount's stats do NOT come from ItemList: for 2330-2389 it carries only the
// mesh, the level requirement and the price. The legacy reads them from
// g_pMountBonus / g_pMountTempBonus inside BASE_GetItemAbility
// (Basedef.cpp:1599-1654).
package mountbonus

// Bonus is one mount's contribution. Attack and Magic are COEFFICIENTS for an
// adult mount — the legacy scales them with the mount's level,
// (level+20)*Attack/100 and (level+15)*Magic/100 — and flat values for a
// temporary one. Evasion is in tenths of a percent (60 is the "6.0%" on the
// tooltip) and Resist is applied to all four resistances alike.
type Bonus struct {
	Attack  int16
	Magic   int16
	Evasion int16
	Resist  int16
}

// The ranges a configured bonus may take. The ceilings are what the engine can
// actually use, so a value above them would be a number on a screen that
// changes nothing in a fight: equipment evasion is capped at 100 (10%) where the
// dodge rate is computed (GetParryRate), and every resistance at 100
// (CMob.cpp:640-643). Attack and Magic have no engine ceiling; these are wide
// enough for the strongest mount in the tables (the Dragão Akelo's 950/145)
// several times over, and narrow enough to catch a typo.
const (
	MaxAttack  = 2000
	MaxMagic   = 500
	MaxEvasion = 100
	MaxResist  = 100
)

// Valid reports whether every field is inside its range.
func (b Bonus) Valid() bool {
	return b.Attack >= 0 && b.Attack <= MaxAttack &&
		b.Magic >= 0 && b.Magic <= MaxMagic &&
		b.Evasion >= 0 && b.Evasion <= MaxEvasion &&
		b.Resist >= 0 && b.Resist <= MaxResist
}

// The item ranges the tables cover.
const (
	AdultLo = 2360 // Porco
	AdultHi = 2389 // Pantera Negra
	TempLo  = 3980 // Shire (3 dias)
	TempHi  = 3994 // Dragão Hekalo
)

// IsAdult reports whether index is an adult mount — the only kind the panel
// can configure, like the growth curve and the absorption.
func IsAdult(index int16) bool { return index >= AdultLo && index <= AdultHi }

// IsTemp reports whether index is a temporary/premium mount.
func IsTemp(index int16) bool { return index >= TempLo && index <= TempHi }

// Default is the compiled bonus for a mount, and whether index is one.
func Default(index int16) (Bonus, bool) {
	switch {
	case IsAdult(index):
		return adult[index-AdultLo], true
	case IsTemp(index):
		return temp[index-TempLo], true
	}
	return Bonus{}, false
}

// Table is the configured overlay: adult lineages someone set in the panel.
// A lineage with no entry uses Default — absence means "not configured", never
// "no bonus", the same rule as every other overlay in this project.
type Table map[int16]Bonus

// For resolves what a mount lends: the configured bonus when there is one, the
// compiled default otherwise. ok is false for anything that is not a mount.
func (t Table) For(index int16) (Bonus, bool) {
	if b, ok := t[index]; ok && IsAdult(index) {
		return b, true
	}
	return Default(index)
}

// The compiled tables are the CLIENT's, not the legacy server's.
//
// The shipped legacy Source has every row flattened to the Dragão Vermelho's
// {750,110,80,32}, with each mount's own numbers left behind in comments
// (Basedef.cpp:242-292). Ported verbatim, that made thirty mounts identical,
// while the client kept a per-mount table and drew its tooltip from it — so what
// the player read was never what the server applied.
//
// These rows were read out of WYD.exe (client 7662) at file offset 0x21DCE0
// (VA 0x61DCE0, in .data): 30 adult rows followed by the temporary ones, six
// int32 each — Attack, Magic, Evasion, Resist, then a movement tier and a sixth
// column this server has no use for. They reproduce the tooltip exactly: a
// level-120 Svadilfari {600,40,60,28} shows Aumento de Dano (120+20)*600/100 =
// 840, Ataque Mágico (120+15)*40/100 = 54, Evasão 6.0%, Imunidades 28. Where the
// client and the legacy comments differ (Dragão Vermelho 700 vs 750, the Grifo
// line, Svadilfari, Sleipnir, Pantera Negra) the client wins, because it is the
// number on the player's screen.

// adult is g_pMountBonus: items 2360-2389, indexed by sIndex-2360.
var adult = [AdultHi - AdultLo + 1]Bonus{
	{10, 1, 0, 0},      // Porco
	{10, 1, 0, 0},      // Javali
	{50, 10, 0, 0},     // Lobo
	{80, 15, 0, 0},     // Dragão Menor
	{100, 20, 0, 0},    // Urso
	{150, 25, 0, 0},    // Dente de Sabre
	{250, 50, 40, 0},   // Cavalo s/ Sela N
	{300, 60, 50, 0},   // Cavalo Fantasma N
	{350, 65, 60, 0},   // Cavalo Leve N
	{400, 70, 70, 0},   // Cavalo Equipado N
	{500, 85, 80, 0},   // Andaluz N
	{250, 50, 0, 16},   // Cavalo s/ Sela B
	{300, 60, 0, 20},   // Cavalo Fantasma B
	{350, 65, 0, 24},   // Cavalo Leve B
	{400, 70, 0, 28},   // Cavalo Equipado B
	{500, 85, 0, 32},   // Andaluz B
	{550, 90, 0, 0},    // Fenrir
	{600, 90, 0, 0},    // Dragão
	{550, 90, 0, 20},   // Fenrir das Sombras
	{650, 100, 60, 28}, // Tigre de Fogo
	{700, 110, 80, 32}, // Dragão Vermelho
	{570, 90, 20, 16},  // Unicórnio
	{570, 90, 30, 8},   // Pegasus
	{570, 90, 40, 12},  // Unisus
	{590, 95, 30, 20},  // Grifo
	{600, 95, 40, 16},  // Hipogrifo
	{600, 95, 50, 16},  // Grifo Sangrento
	{600, 40, 60, 28},  // Svadilfari
	{300, 95, 60, 28},  // Sleipnir
	{150, 25, 0, 20},   // Pantera Negra
}

// temp is g_pMountTempBonus: items 3980-3994, indexed by sIndex-3980. The client
// carries four more rows past these, for indices 3995-3998 that ItemList.csv
// does not define, so they are left out.
var temp = [TempHi - TempLo + 1]Bonus{
	{35, 7, 0, 0},      // Shire 3D
	{350, 55, 10, 28},  // Thoroughbred 3D
	{450, 55, 0, 0},    // Klazedale 3D
	{35, 7, 0, 0},      // Shire 15D
	{450, 72, 10, 28},  // Thoroughbred 15D
	{450, 72, 0, 0},    // Klazedale 15D
	{120, 45, 0, 0},    // Shire 30D
	{450, 72, 10, 28},  // Thoroughbred 30D
	{450, 72, 0, 0},    // Klazedale 30D
	{325, 35, 16, 28},  // Gullfaxi 30D
	{350, 45, 10, 4},   // Tigre de Fogo
	{250, 25, 0, 31},   // Dragão Vermelho
	{80, 15, 0, 31},    // Dragão Menor
	{950, 145, 60, 20}, // Dragão Akelo
	{950, 145, 60, 20}, // Dragão Hekalo
}

// AtLevel is what an adult mount with bonus b actually adds at the given mount
// level — the numbers on its tooltip. Evasion and Resist do not scale.
func (b Bonus) AtLevel(level int) (attack, magic int) {
	return (level + 20) * int(b.Attack) / 100, (level + 15) * int(b.Magic) / 100
}
