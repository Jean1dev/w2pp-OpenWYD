package level

import "math"

// Zone is one of the seven EXP distribution branches of the legacy kill reward
// (MobKilled.cpp:443-1425). The branch is chosen by the 128-tile block both the
// killer and the corpse stand on — `(tx/128, ty/128)` — so each instanced
// dungeon, which lives on a tile block of its own, pays on its own divisor
// table. Only ZoneField was ported before this file, which is why every dungeon
// silently paid open-field rates.
type Zone uint8

const (
	ZoneField           Zone = iota // MobKilled.cpp:1272 — o campo geral, o fallback
	ZonePesadeloArcano              // MobKilled.cpp:443  — bloco (9,1)
	ZonePesadeloMistico             // MobKilled.cpp:592  — bloco (8,2)
	ZonePesadeloNormal              // MobKilled.cpp:737  — bloco (10,2)
	ZoneAguaArcano                  // MobKilled.cpp:851  — bloco (10,27)
	ZoneAguaMistico                 // MobKilled.cpp:1001 — bloco (9,28)
	ZoneAguaNormal                  // MobKilled.cpp:1150 — bloco (8,27)

	// The Deserto belt. NOT legacy branches — the legacy pays open-field rates
	// across the whole desert, and these start out doing exactly the same
	// (deserto* below reuse the field's own tables), so adding them changes no
	// reward until somebody edits one. They exist because the desert is the
	// busiest levelling ground in the content tree — 412 generator blocks — and
	// the field table is the one lever that also moves everywhere else.
	//
	// They are matched by RECTANGLE, not by 128-tile block, which is why they
	// could not simply be added to the switch above: Pilar, Manticora, Lugefer
	// and Baixo all share block (10,13), so a block rule cannot tell them apart.
	ZoneDesertoPilar
	ZoneDesertoManticora
	ZoneDesertoLugefer
	ZoneDesertoBaixo
	ZoneDesertoReino
)

// zoneRect is one named rectangle on the global grid, for zones the legacy
// chose by block and we choose more finely.
type zoneRect struct {
	x1, y1, x2, y2 int32
	zone           Zone
}

// zoneRects are consulted only after every legacy block rule has missed, so no
// legacy branch can be stolen by one of them. Coordinates are the
// Release/TMsrv/run/Regions.txt rows of the same name, transcribed rather than
// read at runtime: internal/level does no I/O, and a reward table that changed
// with the contents of a file on disk would be untestable.
var zoneRects = []zoneRect{
	{1137, 1669, 1282, 1786, ZoneDesertoPilar},
	{1282, 1664, 1396, 1785, ZoneDesertoManticora},
	{1283, 1788, 1397, 1910, ZoneDesertoLugefer},
	{1397, 1671, 1521, 1785, ZoneDesertoBaixo},
	{1522, 1675, 1669, 1787, ZoneDesertoReino},
}

// Name is the zone's name in the language the panel and the design docs use.
func (z Zone) Name() string { return z.rule().name }

// CountsFairyContent reports whether the zone adds the Fada Suprema's flat +30
// to the item bonus. Exported because the /xp screen has to say WHERE the fairy
// is worth 46% and where it is worth 16%, and stating the rule a second time
// there would give the player an answer that can drift from the one ExpReward
// actually pays.
func (z Zone) CountsFairyContent() bool { return z.rule().fairyContent }

// IsPesadelo reports whether the zone is one of the three Pesadelo instances.
func (z Zone) IsPesadelo() bool {
	switch z {
	case ZonePesadeloArcano, ZonePesadeloMistico, ZonePesadeloNormal:
		return true
	default:
		return false
	}
}

// ZoneForTile is the legacy branch selector: the chain of else-if guards at
// MobKilled.cpp:443-1272 compares (tx/128, ty/128) against a fixed block per
// dungeon and falls through to the general field when none matches. The legacy
// requires killer and corpse to share the block; callers pass the corpse's
// position, since a kill from outside the killer's own block is already refused
// upstream.
func ZoneForTile(x, y int32) Zone {
	bx, by := x/128, y/128
	switch {
	case bx == 9 && by == 1:
		return ZonePesadeloArcano
	case bx == 8 && by == 2:
		return ZonePesadeloMistico
	case bx == 10 && by == 2:
		return ZonePesadeloNormal
	case bx == 10 && by == 27:
		return ZoneAguaArcano
	case bx == 9 && by == 28:
		return ZoneAguaMistico
	case bx == 8 && by == 27:
		return ZoneAguaNormal
	}
	// Only after every legacy block has missed. These are ours, not the
	// legacy's, and they must never shadow a branch MobKilled.cpp defines.
	for _, r := range zoneRects {
		if x >= r.x1 && x <= r.x2 && y >= r.y1 && y <= r.y2 {
			return r.zone
		}
	}
	return ZoneField
}

type divKind uint8

const (
	divNone divKind = iota // `exp /= 1` — the legacy writes the rung but it is a no-op
	divInt                 // integer literal: integer division
	divF32                 // float32 literal (`1.07f`)
	divF64                 // double literal (`1.70`)
)

// ZoneForKill is the branch guard as written: each dungeon's `else if` demands
// that the corpse *and* the killer stand on the same block, so a kill reaching
// across a block boundary falls through to the general field.
func ZoneForKill(mobX, mobY, killerX, killerY int32) Zone {
	z := ZoneForTile(mobX, mobY)
	if z == ZoneField || ZoneForTile(killerX, killerY) != z {
		return ZoneField
	}
	return z
}

// expBand is one `else if (myLevel <= N) exp /= D` rung of a divisor table. The
// C code divides an int by literals of three different kinds and truncates back
// to int, and the kind moves the truncation edge, so it is preserved per rung:
// `exp /= 1.07f` is float32, `exp /= 1.70` is float64, `exp /= 4` is integer.
//
// The celestial tables are written `myLevel < N` with a trailing `else`; they
// are normalised here to the same inclusive upTo (`< 120` becomes `upTo 119`)
// plus a final rung at math.MaxInt64, which is the identical partition.
type expBand struct {
	upTo int64
	div  float64
	kind divKind
}

func (b expBand) apply(exp int64) int64 {
	switch b.kind {
	case divInt:
		return exp / int64(b.div)
	case divF32:
		return int64(float32(exp) / float32(b.div))
	case divF64:
		return int64(float64(exp) / b.div)
	default:
		return exp
	}
}

// applyBands walks a table top to bottom and stops at the first rung that
// matches, exactly as the else-if chain does. An empty table is a branch that
// has no table for that tier — the legacy then simply does not divide, which is
// a real (and very generous) property of Pesadelo Normal and Água Normal.
func applyBands(exp, myLevel int64, bands []expBand) int64 {
	for _, b := range bands {
		if myLevel <= b.upTo {
			return b.apply(exp)
		}
	}
	return exp
}

// expRule is one branch of MobKilled.cpp reduced to the parts that differ
// between branches. Everything else — the (0,10M] gate, ×0.6, the newbie and
// double-exp events, the ±15% swing — is identical in all seven and lives in
// SoloExpRewardZone.
type expRule struct {
	name string
	line int // MobKilled.cpp line the branch starts at, for the tests to cite

	// identityBase: the three Pesadelo branches write the base scaling as
	// `(UNK_1 + myLevel) * isExp / (UNK_1 + myLevel)` — algebraically the
	// identity. The four others write `450 * isExp / (30 + myLevel)`. This is
	// the single largest reason Pesadelo pays more than the field for the same
	// mob, and it was the piece missing from the port.
	identityBase bool

	// capToEMob: `if (exp > eMob) exp = eMob`. Commented out in Pesadelo
	// Arcano (:531) and absent from the other two Pesadelo branches.
	capToEMob bool

	// fairyContent: the bonus line reads `ExpBonus + g_pFairyContent[0]` in the
	// Água and field branches and plain `ExpBonus` in the Pesadelo ones, so the
	// Fada Suprema's +30% does not pay inside Pesadelo.
	fairyContent bool

	// celestialTwice: Pesadelo Normal repeats the celestial block verbatim
	// (:752 and :773), dividing celestial exp twice. Kept as the legacy has it.
	celestialTwice bool

	mortal, arch, celestial []expBand
}

// celestialBands is the same table in all seven branches (10/20/40/80/160/320).
var celestialBands = []expBand{
	{upTo: 119, div: 10, kind: divInt},
	{upTo: 149, div: 20, kind: divInt},
	{upTo: 169, div: 40, kind: divInt},
	{upTo: 179, div: 80, kind: divInt},
	{upTo: 189, div: 160, kind: divInt},
	{upTo: math.MaxInt64, div: 320, kind: divInt},
}

// pesadeloArchBands is shared verbatim by Pesadelo Arcano (:484) and Pesadelo
// Místico (:632).
var pesadeloArchBands = []expBand{
	{upTo: 200, div: 0.84, kind: divF32},
	{upTo: 300, div: 0.72, kind: divF32},
	{upTo: 356, div: 1.40, kind: divF32},
	{upTo: 360, div: 4.75, kind: divF32},
	{upTo: 370, div: 6.60, kind: divF32},
	{upTo: 380, div: 15, kind: divInt},
	{upTo: 390, div: 21, kind: divInt},
	{upTo: 400, div: 35, kind: divInt},
}

// zoneRules is indexed by Zone. Every divisor cites the MobKilled.cpp line it
// was read from, and expzone_test.go re-reads those lines out of the legacy
// source, so an edit here that drifts from the legacy fails the test run.
var zoneRules = [...]expRule{
	ZoneField: campoRule(),

	ZonePesadeloArcano: {
		name: "Pesadelo Arcano", line: 443,
		identityBase: true,
		mortal: []expBand{ // :460
			{upTo: 200, kind: divNone},
			{upTo: 300, div: 0.84, kind: divF32},
			{upTo: 356, div: 1.05, kind: divF32},
			{upTo: 370, div: 1.63, kind: divF32},
			{upTo: 380, div: 1.95, kind: divF32},
			{upTo: 390, div: 2.55, kind: divF32},
			{upTo: 399, div: 3.70, kind: divF32},
		},
		arch:      pesadeloArchBands,
		celestial: celestialBands,
	},

	ZonePesadeloMistico: {
		name: "Pesadelo Místico", line: 592,
		identityBase: true,
		mortal: []expBand{ // :608
			{upTo: 200, kind: divNone},
			{upTo: 300, div: 1.03, kind: divF32},
			{upTo: 356, div: 1.50, kind: divF32},
			{upTo: 370, div: 2.15, kind: divF32},
			{upTo: 380, div: 2.78, kind: divF32},
			{upTo: 390, div: 3.70, kind: divF32},
			{upTo: 399, div: 5.20, kind: divF32},
		},
		arch:      pesadeloArchBands,
		celestial: celestialBands,
	},

	ZonePesadeloNormal: {
		name: "Pesadelo Normal", line: 737,
		identityBase: true,
		// No Mortal or Arch table at all (:747-790): both keep the undivided
		// identity-scaled reward, which makes Pesadelo Normal the richest map
		// in the legacy for those two tiers.
		celestial: celestialBands, celestialTwice: true,
	},

	ZoneAguaArcano: {
		name: "Água Arcano", line: 851,
		capToEMob: true, fairyContent: true,
		mortal: []expBand{ // :868
			{upTo: 200, kind: divNone},
			{upTo: 300, div: 0.10, kind: divF32},
			{upTo: 356, kind: divNone},
			{upTo: 370, div: 1.55, kind: divF32},
			{upTo: 380, div: 2.20, kind: divF32},
			{upTo: 390, div: 2.60, kind: divF32},
			{upTo: 399, div: 3.85, kind: divF32},
		},
		arch: []expBand{ // :892
			{upTo: 200, kind: divNone},
			{upTo: 300, div: 0.05, kind: divF32},
			{upTo: 356, div: 0.85, kind: divF32},
			{upTo: 360, div: 4.0, kind: divF32},
			{upTo: 370, div: 6.60, kind: divF32},
			{upTo: 380, div: 9, kind: divInt},
			{upTo: 390, div: 15, kind: divInt},
			{upTo: 400, div: 22, kind: divInt},
		},
		celestial: celestialBands,
	},

	ZoneAguaMistico: {
		name: "Água Místico", line: 1001,
		capToEMob: true, fairyContent: true,
		mortal: []expBand{ // :1018
			{upTo: 200, kind: divNone},
			{upTo: 300, div: 1.08, kind: divF32},
			{upTo: 356, div: 1.30, kind: divF32},
			{upTo: 370, div: 1.80, kind: divF32},
			{upTo: 380, div: 2.20, kind: divF32},
			{upTo: 390, div: 2.60, kind: divF32},
			{upTo: 399, div: 4.70, kind: divF32},
		},
		arch: []expBand{ // :1042
			{upTo: 200, kind: divNone},
			{upTo: 300, div: 0.90, kind: divF32},
			{upTo: 356, div: 0.95, kind: divF32},
			{upTo: 360, div: 4.50, kind: divF32},
			{upTo: 370, div: 6.60, kind: divF32},
			{upTo: 380, div: 12, kind: divInt},
			{upTo: 390, div: 19, kind: divInt},
			{upTo: 400, div: 32, kind: divInt},
		},
		celestial: celestialBands,
	},

	ZoneAguaNormal: {
		name: "Água Normal", line: 1150,
		capToEMob: true, fairyContent: true,
		mortal: []expBand{ // :1167
			{upTo: 200, div: 1.28, kind: divF64},
			{upTo: 300, div: 1.03, kind: divF32},
			{upTo: 356, div: 1.45, kind: divF32},
			{upTo: 370, div: 1.80, kind: divF32},
			{upTo: 380, div: 2.45, kind: divF32},
			{upTo: 390, div: 3.70, kind: divF32},
			{upTo: 399, div: 6.80, kind: divF32},
		},
		// No Arch table (:1188): Arch keeps the undivided reward here.
		celestial: celestialBands,
	},

	// The Deserto belt. Every one of these is the field's own branch with a
	// different name — same flags, same three tables — because the legacy pays
	// field rates across the whole desert and this must keep doing that until a
	// moderator changes one. Splitting them apart is the point: the desert is
	// where people level, and until now the only way to tune it was the field
	// table, which also moves every other open-world mob in the game.
	ZoneDesertoPilar:     desertoRule("Deserto Pilar"),
	ZoneDesertoManticora: desertoRule("Deserto Manticora"),
	ZoneDesertoLugefer:   desertoRule("Deserto Lugefer (Tauron)"),
	ZoneDesertoBaixo:     desertoRule("Deserto Baixo"),
	ZoneDesertoReino:     desertoRule("Deserto Reino"),
}

// desertoRule copies the field branch under a new name.
//
// A copy rather than a reference to zoneRules[ZoneField], because that entry is
// still being built when this runs — Go initializes the composite literal as a
// whole — and reading it here would take a zero value. The bands are shared
// slices, which is safe: nothing mutates them, and a moderator's edit replaces
// the table through Config.Overrides instead of writing into these.
func desertoRule(name string) expRule {
	f := campoRule()
	f.name = name
	return f
}

// campoRule is the field branch's own definition, kept as a function so both
// ZoneField and the Deserto zones read from one source. If MobKilled.cpp's
// field branch is ever corrected, the desert follows automatically rather than
// drifting into a stale copy.
func campoRule() expRule {
	return expRule{
		name: "Campo", line: 1272,
		capToEMob: true, fairyContent: true,
		mortal: []expBand{ // :1289
			{upTo: 200, kind: divNone},
			{upTo: 300, div: 1.07, kind: divF32},
			{upTo: 356, div: 1.25, kind: divF32},
			{upTo: 370, div: 1.70, kind: divF64},
			{upTo: 380, div: 2.10, kind: divF32},
			{upTo: 390, div: 2.60, kind: divF64},
			{upTo: 399, div: 4, kind: divInt},
		},
		arch: []expBand{ // :1313
			{upTo: 200, kind: divNone},
			{upTo: 300, div: 0.85, kind: divF32},
			{upTo: 356, div: 0.90, kind: divF32},
			{upTo: 360, div: 4.50, kind: divF32},
			{upTo: 370, div: 5.90, kind: divF32},
			{upTo: 380, div: 11, kind: divInt},
			{upTo: 390, div: 17, kind: divInt},
			{upTo: 400, div: 35, kind: divInt},
		},
		celestial: celestialBands,
	}
}

// rule returns the branch for a zone, falling back to the field for an
// out-of-range value so a bad tile can never panic the game loop.
func (z Zone) rule() expRule {
	if int(z) >= len(zoneRules) {
		return zoneRules[ZoneField]
	}
	return zoneRules[z]
}
