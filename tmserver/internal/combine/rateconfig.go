package combine

// The Mesa das Máquinas: the moderator's edits on top of CompRate.txt.
//
// Two layers, and the order between them is the whole design. A machine's rate
// is looked up as:
//
//	painel  →  CompRate.txt  →  padrão compilado
//
// so a server with no dbServer, or with an empty table, behaves exactly as it
// did before this existed. Nothing here can make a machine stop working; the
// worst a missing layer does is fall through to the one under it.

// SlotKind says which band table applies to an item. Weapons and armour are kept
// apart because they concentrate at opposite ends of the ReqLvl scale: between
// ReqLvl 200 and 249 the catalog holds 112 weapons and one single piece of
// armour, so one shared ruler would have the moderator tuning bands that are
// nearly empty for one of the two.
type SlotKind int32

const (
	SlotNone   SlotKind = 0
	SlotWeapon SlotKind = 1
	SlotArmour SlotKind = 2
)

// Band is one item band: everything from ReqLvlMin to ReqLvlMax, inclusive, gets
// MultPct hundredths of the machine's own rate.
type Band struct {
	SlotKind  SlotKind
	ReqLvlMin int32
	ReqLvlMax int32
	Label     string
	MultPct   int32
}

// RateConfig is the whole table at one version. The zero value is "nothing
// edited", which every lookup treats as "use the file".
type RateConfig struct {
	Version int64
	// rates is keyed family→key, both as the file spells them. Lookup is
	// case-insensitive, like the port's CompRate reader, because the panel and
	// the file disagree on case in practice ("Ailyn" vs "AILYN").
	rates map[rateKey]int32
	bands map[SlotKind][]Band
}

type rateKey struct{ family, key string }

// NewRateConfig builds the lookup from the rows the panel saved.
func NewRateConfig(version int64, rates []RateRow, bands []Band) RateConfig {
	c := RateConfig{Version: version}
	if len(rates) > 0 {
		c.rates = make(map[rateKey]int32, len(rates))
		for _, r := range rates {
			c.rates[rateKey{lower(r.Family), lower(r.Key)}] = r.Rate
		}
	}
	if len(bands) > 0 {
		c.bands = make(map[SlotKind][]Band, 2)
		for _, b := range bands {
			c.bands[b.SlotKind] = append(c.bands[b.SlotKind], b)
		}
	}
	return c
}

// RateRow is one family/key rate as stored.
type RateRow struct {
	Family string
	Key    string
	Rate   int32
}

// Rate returns the moderator's rate for a family/key, and whether there is one.
// A miss is not an error: it means the machine keeps running on CompRate.txt.
func (c RateConfig) Rate(family, key string) (int32, bool) {
	if c.rates == nil {
		return 0, false
	}
	v, ok := c.rates[rateKey{lower(family), lower(key)}]
	return v, ok
}

// BandFor returns the band an item falls in, by its ReqLvl, and whether any band
// covers it. Bands are walked in order and the FIRST match wins, so an overlap
// resolves the same way every time instead of depending on map iteration.
//
// An item outside every band — or of a slot with no bands at all — has no
// multiplier, which is the same as 1x. That is deliberate: a gap in the curve
// must leave the machine at its plain rate, never at zero, or a forgotten band
// would silently make a whole tier impossible to refine.
func (c RateConfig) BandFor(kind SlotKind, reqLvl int32) (Band, bool) {
	if c.bands == nil {
		return Band{}, false
	}
	for _, b := range c.bands[kind] {
		if reqLvl >= b.ReqLvlMin && reqLvl <= b.ReqLvlMax {
			return b, true
		}
	}
	return Band{}, false
}

// Apply scales a machine's rate by the band an item falls in, and clamps to the
// 1..100 the roll is compared against.
//
// The floor is 1, not 0: a band set to zero would make the machine eat the
// player's gold and materials on a roll it can never win, which reads as a
// broken machine rather than a hard one. A moderator who wants a combine
// disabled should have a control that says so, not a rate that pretends.
func (c RateConfig) Apply(base int32, kind SlotKind, reqLvl int32) int32 {
	b, ok := c.BandFor(kind, reqLvl)
	if !ok {
		return clampRate(base)
	}
	return clampRate(base * b.MultPct / 100)
}

func clampRate(v int32) int32 {
	if v < 1 {
		return 1
	}
	if v > 100 {
		return 100
	}
	return v
}

// SlotKindForPos maps an item's nPos bitmask to the band table it belongs to.
// 64/128/192 are the weapon slots (main hand, off hand, two-handed) and
// 2/4/8/16/32 the five armour pieces; everything else — rings, amulets, mounts,
// the gem slot — has no band and runs on the machine's plain rate.
func SlotKindForPos(nPos int32) SlotKind {
	switch nPos {
	case 64, 128, 192:
		return SlotWeapon
	case 2, 4, 8, 16, 32:
		return SlotArmour
	default:
		return SlotNone
	}
}

// lower is ASCII-only on purpose: these keys are file identifiers, and
// strings.ToLower would drag in Unicode case folding for names that never have
// any.
func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
