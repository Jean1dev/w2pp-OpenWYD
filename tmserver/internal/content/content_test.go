package content

import (
	"path/filepath"
	"strings"
	"testing"
)

// release returns a path under the repo's Release/ tree, or skips if absent.
func release(t *testing.T, parts ...string) string {
	t.Helper()
	return filepath.Join(append([]string{"..", "..", "..", "Release"}, parts...)...)
}

func TestCombineVariantRatesFromRelease(t *testing.T) {
	path := filepath.Join("..", "..", "..", "Release", "Common", "Settings", "CompRate.txt")
	c, err := LoadCompRate(path)
	if err != nil {
		t.Skipf("Release content unavailable: %v", err)
	}
	if got := c.ChanceBase("Tiny"); got != 15 {
		t.Errorf("Tiny ChanceBase=%d, want 15", got)
	}
	if got := c.ChanceBase("Ailyn"); got != 10 {
		t.Errorf("Ailyn ChanceBase=%d, want 10", got)
	}
	if got := c.ChanceBase("Agatha"); got != 15 {
		t.Errorf("Agatha ChanceBase=%d, want 15", got)
	}
	if got := c.EhreRates()[3]; got != 40 {
		t.Errorf("Ehre Espiritual=%d, want 40", got)
	}
}

func TestLoadCompRate(t *testing.T) {
	c, err := LoadCompRate(release(t, "Common", "Settings", "CompRate.txt"))
	if err != nil {
		t.Skipf("CompRate.txt unavailable: %v", err)
	}
	cases := []struct {
		family, key string
		want        int
	}{
		{"Tiny", "ChanceBase", 15},
		{"Shany", "ChanceBase", 30},
		{"Ehre", "Espiritual", 40},
		{"Ehre", "Amunra", 10},
		{"Odin", "Item_12_Ref_8", 12},
		{"Odin", "Item_12_Secreta", 1},
	}
	for _, tt := range cases {
		got, ok := c.Rate(tt.family, tt.key)
		if !ok || got != tt.want {
			t.Errorf("Rate(%s,%s) = %d,%v, want %d", tt.family, tt.key, got, ok, tt.want)
		}
	}
	if c.Families() < 4 {
		t.Errorf("families = %d, want >= 4", c.Families())
	}
	ch := c.AnctChance()
	if ch != [3]int{2, 4, 10} {
		t.Errorf("AnctChance = %v, want [2 4 10]", ch)
	}
}

func TestLoadSancRate(t *testing.T) {
	s, err := LoadSancRate(release(t, "Common", "Settings", "SancRate.txt"))
	if err != nil {
		t.Skipf("SancRate.txt unavailable: %v", err)
	}
	cases := []struct {
		name      string
		anvil     int
		idx, want int
	}{
		{"PO 0 from file", sancAnvilOri, 0, 100},
		{"PO 3 from file", sancAnvilOri, 3, 85},
		{"PO 5 from file", sancAnvilOri, 5, 40},
		{"PL 6 from file", sancAnvilLac, 6, 80},
		{"PL 9 from file", sancAnvilLac, 9, 10},

		// The file lists only PO 0..5, so the tail keeps the Basedef.cpp defaults.
		// Index 6 = 100 makes the +5→+6 Ori refine free; an operator who wants it
		// gated adds a "PO 6 <rate>" line (issue #103).
		{"PO 6 falls back to the compiled default", sancAnvilOri, 6, 100},
		{"PO 7 falls back to the compiled default", sancAnvilOri, 7, 0},
		{"PL 10 falls back to the compiled default", sancAnvilLac, 10, 0},

		// The Âmago lines land on row 2. In the legacy they aliased onto row 0 and
		// clobbered every PO rate above (CReadFiles.cpp:157); issue #103 fixed that,
		// so PO 1 must still be the file's 100 and NOT Âmago's 80.
		{"Amago 1 on its own row", sancAnvilAma, 1, 80},
		{"Amago 11 on its own row", sancAnvilAma, 11, 5},
		{"PO 1 not clobbered by Amago", sancAnvilOri, 1, 100},
		{"PO 4 not clobbered by Amago", sancAnvilOri, 4, 70},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.Rate(tt.anvil, tt.idx); got != tt.want {
				t.Errorf("Rate(%d,%d) = %d, want %d", tt.anvil, tt.idx, got, tt.want)
			}
		})
	}
}

func TestQuestRateDefaults(t *testing.T) {
	q := DefaultQuestRates()
	want := []QuestTierRate{
		{1000, 500, 2000, 39, 115, 39, 115},
		{2000, 1000, 4000, 115, 190, 115, 190},
		{3000, 1500, 6000, 190, 265, 190, 265},
		{4000, 2000, 8000, 265, 320, 265, 320},
		{5000, 2500, 10000, 320, 350, 320, 350},
	}
	for i := range want {
		if got, ok := q.Tier(i); !ok || got != want[i] {
			t.Errorf("Tier(%d) = %+v,%v, want %+v,true", i, got, ok, want[i])
		}
	}
	if _, ok := q.Tier(-1); ok {
		t.Error("Tier(-1) succeeded")
	}
}

func TestLoadQuestRates(t *testing.T) {
	q, err := LoadQuestRates(release(t, "Common", "Settings", "QuestsRate.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := q.Tier(2); got != (QuestTierRate{200000, 200000, 100000, 190, 265, 190, 265}) {
		t.Errorf("Tier(2) = %+v", got)
	}
}

func TestParseQuestRatesOverlayAndValidation(t *testing.T) {
	q, err := parseQuestRates(strings.NewReader("exp 0 30 20\nCoin 0 40\nLEVEL 0 1 2 3 4\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := q.Tier(0); got != (QuestTierRate{30, 20, 40, 1, 2, 3, 4}) {
		t.Errorf("Tier(0) = %+v", got)
	}
	if got, _ := q.Tier(1); got != questRateDefaults[1] {
		t.Errorf("missing Tier(1) override changed default: %+v", got)
	}
	bad := []string{"EXP 5 1 1", "EXP 0 nope 1", "COIN 0 -1", "LEVEL 0 4 4 1 2", "LEVEL 0 0 400 1 2", "wat 0 1"}
	for _, input := range bad {
		if _, err := parseQuestRates(strings.NewReader(input)); err == nil {
			t.Errorf("parseQuestRates(%q) succeeded", input)
		}
	}
}

func TestSancRateOutOfRange(t *testing.T) {
	s := DefaultSancRate()
	for _, tt := range []struct{ anvil, idx int }{{-1, 0}, {3, 0}, {0, -1}, {0, 12}} {
		if got := s.Rate(tt.anvil, tt.idx); got != 0 {
			t.Errorf("Rate(%d,%d) = %d, want 0", tt.anvil, tt.idx, got)
		}
	}
	var nilTable *SancRate
	if got := nilTable.Rate(0, 1); got != 0 {
		t.Errorf("nil Rate = %d, want 0", got)
	}
}

// The anvil token is matched on raw cp1252 bytes, the way _strupr+strcmp does.
func TestSancAnvilRow(t *testing.T) {
	cases := []struct {
		token string
		want  int
		ok    bool
	}{
		{"PO", sancAnvilOri, true},
		{"po", sancAnvilOri, true},
		{"PL", sancAnvilLac, true},
		{"\xC2mago", sancAnvilAma, true}, // cp1252, as the real file spells it
		{"\xC2MAGO", sancAnvilAma, true},
		{"\xC3\x82mago", sancAnvilAma, true}, // UTF-8, if the file is ever re-saved
		{"Ori", 0, false},
		{"", 0, false},
	}
	for _, tt := range cases {
		got, ok := sancAnvilRow(tt.token)
		if ok != tt.ok || (ok && got != tt.want) {
			t.Errorf("sancAnvilRow(%q) = %d,%v, want %d,%v", tt.token, got, ok, tt.want, tt.ok)
		}
	}
}

func TestLoadItemList(t *testing.T) {
	l, err := LoadItemList(release(t, "Common", "ItemList.csv"))
	if err != nil {
		t.Skipf("ItemList.csv unavailable: %v", err)
	}
	if e, ok := l.Get(1); !ok || e.Name != "TransKnight" {
		t.Errorf("Get(1) = %+v, want name TransKnight", e)
	}
	if l.Len() < 1000 {
		t.Errorf("item count = %d, want >= 1000", l.Len())
	}
}

func TestBaseEffects(t *testing.T) {
	// A boot row (real format) carrying EF_AC and EF_DAMAGE among ignored effects.
	const row = "168,Botas_de_Guarda(Az),17.0,0.0.0.0.0,0,0,32,0,0," +
		"EF_CLASS,1,EF_GRID,0,EF_AC,96,EF_RUNSPEED,2,EF_REGENMP,40,EF_DAMAGE,24,EF_ITEMLEVEL,5"
	l, err := parseItemList(strings.NewReader(row))
	if err != nil {
		t.Fatal(err)
	}
	eff := l.BaseEffects()[168]
	got := map[uint8]int16{}
	for _, e := range eff {
		got[e.Eff] = e.Val
	}
	// Score-relevant effects plus EF_ITEMLEVEL (used by combine matching) and
	// EF_RUNSPEED (issue #64: boots' move-speed bonus).
	if len(got) != 4 || got[3] != 96 || got[2] != 24 || got[87] != 5 || got[29] != 2 {
		t.Errorf("BaseEffects = %v, want AC(3):96, DAMAGE(2):24, ITEMLEVEL(87):5, RUNSPEED(29):2", got)
	}
}

func TestBaseEffectsSpecialAll(t *testing.T) {
	const row = "700,Anel_Aprendizagem,0.0,0.0.0.0.0,0,0,1024,0,0,EF_CLASS,255,EF_SPECIALALL,18"
	l, err := parseItemList(strings.NewReader(row))
	if err != nil {
		t.Fatal(err)
	}
	var got int16
	var found bool
	for _, e := range l.BaseEffects()[700] {
		if e.Eff == 74 {
			got, found = e.Val, true
		}
	}
	if !found || got != 18 {
		t.Errorf("BaseEffects()[700] EF_SPECIALALL = %d (found=%v), want 18", got, found)
	}
}

// TestBaseEffectsCritical: EF_CRITICAL/EF_CRITICAL2 are score stats (Basedef.cpp:3209
// derives MOB.Critical from them), not the visual effects they were once filed as —
// dropping them here is what left crit gear worth nothing (issue #102). Most crit in the
// catalog rides on set pieces and the class body items.
func TestBaseEffectsCritical(t *testing.T) {
	// Real rows: an armor set piece (ItemList.csv:225) and a class body item (:1).
	const rows = "165,Armadura_de_Guarda(Az),17.0,0.0.0.0.0,0,0,4,0,0," +
		"EF_CLASS,1,EF_GRID,0,EF_AC,138,EF_CRITICAL,50,EF_HPADD,9,EF_ITEMLEVEL,5\n" +
		"1,TransKnight,0.0,0.0.0.0.0,0,0,1,0,0," +
		"EF_CLASS,1,EF_RANGE,1,EF_REGENHP,2,EF_REGENMP,2,EF_CRITICAL,10\n"
	l, err := parseItemList(strings.NewReader(rows))
	if err != nil {
		t.Fatal(err)
	}
	eff := l.BaseEffects()
	for _, tc := range []struct {
		idx  int
		want int16
	}{{165, 50}, {1, 10}} {
		var got int16
		for _, e := range eff[tc.idx] {
			if e.Eff == 42 { // EF_CRITICAL
				got = e.Val
			}
		}
		if got != tc.want {
			t.Errorf("BaseEffects()[%d] EF_CRITICAL = %d, want %d", tc.idx, got, tc.want)
		}
	}

	// The real catalog must actually carry crit — the parser silently dropping it is
	// what issue #102 was. Guards against the effect disappearing from efName again.
	full, err := LoadItemList(release(t, "Common", "ItemList.csv"))
	if err != nil {
		t.Skipf("ItemList.csv unavailable: %v", err)
	}
	var withCrit int
	for _, effs := range full.BaseEffects() {
		for _, e := range effs {
			if e.Eff == 42 || e.Eff == 71 {
				withCrit++
				break
			}
		}
	}
	if withCrit < 100 {
		t.Errorf("real catalog items carrying EF_CRITICAL = %d, want >= 100", withCrit)
	}
	// Pedra_Amunra (the issue's amulet) equips into the four accessory slots: its nPos
	// 0xF00 is what triggers the +9 x2 refine promotion in handler.itemCritical.
	if pos := full.Positions()[3464]; pos&0xF00 == 0 {
		t.Errorf("Pedra_Amunra nPos = %d, want the accessory mask 0xF00 set", pos)
	}
}

// TestBaseEffectsResist: EF_RESIST1..4/EF_RESISTALL are score stats too (CMob.cpp:640-643
// derives MOB.Resist[0..3] from them), not ignored ones — missing from efName is what left
// every immunity/resist item granting nothing (issue #211).
func TestBaseEffectsResist(t *testing.T) {
	// Real rows: Bolsa_Perfumada (ItemList.csv:1009, EF_RESISTALL) and Orb_de_Fogo
	// (ItemList.csv:1013, EF_RESIST1).
	const rows = "599,Bolsa_Perfumada,319.0,0.0.0.0.0,0,50000,1024,0,0,EF_CLASS,255,EF_RESISTALL,5\n" +
		"601,Orb_de_Fogo,63.0,37.0.0.0.0,0,30000,1024,0,0,EF_CLASS,255,EF_RESIST1,3,EF_RESIST2,0,EF_RESIST3,0,EF_RESIST4,0,EF_ITEMLEVEL,1\n"
	l, err := parseItemList(strings.NewReader(rows))
	if err != nil {
		t.Fatal(err)
	}
	eff := l.BaseEffects()
	for _, tc := range []struct {
		idx  int
		eff  uint8
		want int16
	}{{599, 54, 5}, {601, 49, 3}} {
		var got int16
		var found bool
		for _, e := range eff[tc.idx] {
			if e.Eff == tc.eff {
				got, found = e.Val, true
			}
		}
		if !found || got != tc.want {
			t.Errorf("BaseEffects()[%d] EF(%d) = %d (found=%v), want %d", tc.idx, tc.eff, got, found, tc.want)
		}
	}

	// The real catalog must actually carry resist — the parser silently dropping it is
	// what issue #211 was. Guards against the effects disappearing from efName again.
	full, err := LoadItemList(release(t, "Common", "ItemList.csv"))
	if err != nil {
		t.Skipf("ItemList.csv unavailable: %v", err)
	}
	var withResist int
	for _, effs := range full.BaseEffects() {
		for _, e := range effs {
			if e.Eff >= 49 && e.Eff <= 52 || e.Eff == 54 {
				withResist++
				break
			}
		}
	}
	if withResist < 20 {
		t.Errorf("real catalog items carrying EF_RESIST1..4/EF_RESISTALL = %d, want >= 20", withResist)
	}
}

// TestBaseEffectsMagic: EF_MAGIC/EF_MAGICADD are score stats too (Basedef.cpp:3194-3195
// derives the caster's Magic from BASE_GetMobAbility(EF_MAGIC)+BASE_GetMobAbility(EF_MAGICADD)),
// not ignored ones — missing from efName is what left every "Ataque Mágico" item granting
// nothing (issue #223).
func TestBaseEffectsMagic(t *testing.T) {
	// Real rows: Bracelete_de_Hecate (ItemList.csv:871, EF_MAGIC,6) and Colar_Mágico
	// (ItemList.csv:1075, EF_MAGIC,22).
	const rows = "514,Bracelete_de_Hecate,96.0,132.0.0.0.0,0,73000,256,0,0,EF_CLASS,255,EF_MAGIC,6,EF_ITEMLEVEL,2\n" +
		"642,Colar_Magico,2.6,306.0.0.0.0,0,125000,256,0,0,EF_CLASS,255,EF_MAGIC,22,EF_ITEMLEVEL,5\n"
	l, err := parseItemList(strings.NewReader(rows))
	if err != nil {
		t.Fatal(err)
	}
	eff := l.BaseEffects()
	for _, tc := range []struct {
		idx  int
		eff  uint8
		want int16
	}{{514, 60, 6}, {642, 60, 22}} {
		var got int16
		var found bool
		for _, e := range eff[tc.idx] {
			if e.Eff == tc.eff {
				got, found = e.Val, true
			}
		}
		if !found || got != tc.want {
			t.Errorf("BaseEffects()[%d] EF(%d) = %d (found=%v), want %d", tc.idx, tc.eff, got, found, tc.want)
		}
	}

	// The real catalog must actually carry magic — the parser silently dropping it is
	// what issue #223 was. Guards against the effect disappearing from efName again.
	full, err := LoadItemList(release(t, "Common", "ItemList.csv"))
	if err != nil {
		t.Skipf("ItemList.csv unavailable: %v", err)
	}
	realEffects := full.BaseEffects()
	for _, tc := range []struct {
		idx  int
		want int16
	}{
		{3582, 55},                                     // The common Cajado Caotico keeps its legacy value.
		{3725, 70}, {3726, 70}, {3727, 70}, {3728, 70}, // Issue #281: rebalanced Anct variants.
	} {
		var got int16
		for _, e := range realEffects[tc.idx] {
			if e.Eff == 60 {
				got = e.Val
				break
			}
		}
		if got != tc.want {
			t.Errorf("real catalog item %d EF_MAGIC = %d, want %d", tc.idx, got, tc.want)
		}
	}
	var withMagic int
	for _, effs := range realEffects {
		for _, e := range effs {
			if e.Eff == 60 || e.Eff == 68 {
				withMagic++
				break
			}
		}
	}
	if withMagic < 20 {
		t.Errorf("real catalog items carrying EF_MAGIC/EF_MAGICADD = %d, want >= 20", withMagic)
	}
}

// TestBaseEffectsNoSanc: quest/trophy stones (Almas, Pedras, Sephirot) are
// equippable but carry no EF_VOLATILE, so nothing stopped the dust-refine
// handler from planting an EF_SANC pair into them (issue #133). ItemList.csv
// now tags them EF_NOSANC so handler.refineItem's
// itemAbility(dst, efNoSanc) != 0 gate rejects them.
func TestBaseEffectsNoSanc(t *testing.T) {
	full, err := LoadItemList(release(t, "Common", "ItemList.csv"))
	if err != nil {
		t.Skipf("ItemList.csv unavailable: %v", err)
	}
	for _, idx := range []int{1740, 1742, 1748, 1755, 1760, 1763} {
		var got int16
		for _, e := range full.BaseEffects()[idx] {
			if e.Eff == 126 { // EF_NOSANC
				got = e.Val
			}
		}
		if got == 0 {
			t.Errorf("BaseEffects()[%d] EF_NOSANC = %d, want nonzero", idx, got)
		}
	}
}

func TestRanges(t *testing.T) {
	// Mob-model rows (real format): the archer's body item carries EF_RANGE,4 —
	// the source of a mob's ranged reach (BASE_GetMobAbility). A row without
	// EF_RANGE must be absent from the map.
	const rows = "242,Ciclope_Arqueiro,6.0,0.0.0.0.0,0,0,1,0,0,EF_CLASS,21,EF_RANGE,4\n" +
		"168,Botas_de_Guarda(Az),17.0,0.0.0.0.0,0,0,32,0,0,EF_CLASS,1,EF_AC,96\n"
	l, err := parseItemList(strings.NewReader(rows))
	if err != nil {
		t.Fatal(err)
	}
	r := l.Ranges()
	if r[242] != 4 {
		t.Errorf("Ranges()[242] = %d, want 4", r[242])
	}
	if v, ok := r[168]; ok {
		t.Errorf("Ranges()[168] = %d, want absent (no EF_RANGE)", v)
	}

	// The real catalog (when mounted) must agree on the archer model.
	full, err := LoadItemList(release(t, "Common", "ItemList.csv"))
	if err != nil {
		t.Skipf("ItemList.csv unavailable: %v", err)
	}
	if got := full.Ranges()[242]; got != 4 {
		t.Errorf("real ItemList Ranges()[242] = %d, want 4 (Ciclope_Arqueiro)", got)
	}
}

func TestRequirements(t *testing.T) {
	// A sword needing level 100 + STR 50 (col 4 = ReqLvl.Str.Int.Dex.Con); plus a
	// no-requirement item that must be omitted.
	const rows = "900,Espada,0.0,100.50.0.0.0,0,0,0,0,0,EF_DAMAGE,80\n" +
		"901,Adaga,0.0,0.0.0.0.0,0,0,0,0,0,EF_DAMAGE,20"
	l, err := parseItemList(strings.NewReader(rows))
	if err != nil {
		t.Fatal(err)
	}
	reqs := l.Requirements()
	if r := reqs[900]; r.Lvl != 100 || r.Str != 50 || r.Int != 0 {
		t.Errorf("reqs[900] = %+v, want Lvl 100 Str 50", r)
	}
	if _, ok := reqs[901]; ok {
		t.Errorf("no-requirement item 901 should be omitted, got %+v", reqs[901])
	}
}

// TestRequirementsClasseD is the issue #278 regression. Class D gear
// (EF_ITEMLEVEL 4) demanded up to level 254 while its own class D siblings
// (Conjuradora/Aeon/Natureza) sat at 128-174, so every row in 185..260 had its
// level cut by 80 and its attributes scaled by the same ratio. Anchored on the
// real catalog: a synthetic fixture would not catch the .csv being regenerated
// from a stale source.
func TestRequirementsClasseD(t *testing.T) {
	full, err := LoadItemList(release(t, "Common", "ItemList.csv"))
	if err != nil {
		t.Skipf("ItemList.csv unavailable: %v", err)
	}
	reqs := full.Requirements()
	for _, tc := range []struct {
		idx  int
		name string
		want ItemReq
	}{
		{1346, "Tunica_de_Mytril(A)", ItemReq{Lvl: 174, Str: 117, Int: 205}},
		{911, "Solaris", ItemReq{Lvl: 174, Str: 464, Dex: 140}},
		{1191, "Elmo_Anao(N)", ItemReq{Lvl: 115, Str: 146, Con: 98}},
		{1892, "Tunica_Conjuradora(A), the EF_BONUS variant", ItemReq{Lvl: 132, Str: 90, Int: 157}},
		// Issue #308: the client displays ReqLvl one-based, so raw 160 is level 161.
		{661, "Ankh_da_Justica", ItemReq{Lvl: 160}},
		{662, "Ankh_da_Eternidade", ItemReq{Lvl: 160}},
		{663, "Ankh_da_Gloria", ItemReq{Lvl: 160}},
		// Untouched: below the rescaled band, and above it (the special capes).
		{1331, "Tunica_Conjuradora(A), base set", ItemReq{Lvl: 174, Str: 119, Int: 208}},
		{1720, "Capa_da_Escuridao", ItemReq{Lvl: 399}},
	} {
		if got := reqs[tc.idx]; got != tc.want {
			t.Errorf("Requirements()[%d] (%s) = %+v, want %+v", tc.idx, tc.name, got, tc.want)
		}
	}

	// The band itself must stay empty: no class D item may demand 181..260
	// again. Above 260 only the preserved specials survive (capes, Pedra Amunra).
	effects := full.BaseEffects()
	for idx, req := range reqs {
		classeD := false
		for _, e := range effects[idx] {
			if e.Eff == 87 && e.Val == 4 { // EF_ITEMLEVEL 4 = classe D
				classeD = true
				break
			}
		}
		if classeD && req.Lvl >= 181 && req.Lvl <= 260 {
			t.Errorf("class D item %d requires level %d, want <= 180 (or a preserved special > 260)", idx, req.Lvl)
		}
	}
}

func TestLoadSkillData(t *testing.T) {
	s, err := LoadSkillData(release(t, "Common", "SkillData.csv"))
	if err != nil {
		t.Skipf("SkillData.csv unavailable: %v", err)
	}
	// Golden rows straight from the CSV (sscanf order, AffectTime already ÷4).
	tests := []struct {
		index int
		want  Spell
	}{
		{0, Spell{Index: 0, SkillPoint: 24, TargetType: 3, ManaSpent: 15, Delay: 3,
			Range: 5, InstanceType: 4, InstanceValue: 5, InstanceAttribute: 4,
			Aggressive: 1, MaxTarget: 13, AffectResist: 3, Name: "Giro_da_F\xfaria"}},
		{5, Spell{Index: 5, SkillPoint: 81, ManaSpent: 53, TickType: 17, TickValue: 75,
			AffectTime: 150, MaxTarget: 1, Name: "Aura_da_Vida"}},
		// Escudo Dourado (affect 31): the short tail of the table, raw AffectTime
		// 30. Pinned here because the issue #92 ÷8 truncated it to 3 (issue #236);
		// the loader is legacy-faithful again and the tuning moved to
		// world.AffectDuration, which floors short friendly buffs instead.
		{85, Spell{Index: 85, SkillPoint: 90, ManaSpent: 120, Delay: 5, AffectType: 31,
			AffectValue: 150, AffectTime: 7, MaxTarget: 1, Name: "Explos\xe3o_Et\xe9rea"}},
	}
	for _, tt := range tests {
		got, ok := s.Get(tt.index)
		if !ok {
			t.Errorf("Get(%d): missing", tt.index)
			continue
		}
		if got != tt.want {
			t.Errorf("Get(%d) = %+v, want %+v", tt.index, got, tt.want)
		}
	}
	// Indexes are sparse past the row count: the legacy loader keys on column 0.
	// Names keep the file's Latin-1 bytes (the client charset) — no re-encode.
	if e, ok := s.Get(200); !ok || e.Name != "Prote\xe7\xe3o_Divina" {
		t.Errorf("Get(200) = %+v, %v, want Proteção_Divina (Latin-1)", e, ok)
	}
	if s.Len() < 100 {
		t.Errorf("skill count = %d, want >= 100", s.Len())
	}
}

// TestParseSkillDataDividesAffectTime pins the loader's AffectTime ÷4 rule on a
// synthetic row (no dependency on the Release CSV): 13 ints, the 2 dotted Act
// strings, 7 ints, then the name. Column 12 is the raw AffectTime.
func TestParseSkillDataDividesAffectTime(t *testing.T) {
	const row = "2,3,0,10,4,0,1,10,0,0,11,5,600,anim.a,anim.b,0,0,1,2,0,0,0,MySkill"
	s, err := parseSkillData(strings.NewReader(row))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s.Get(2)
	if !ok {
		t.Fatal("skill 2 missing after parse")
	}
	if got.AffectTime != 150 {
		t.Errorf("AffectTime = %d, want 150 (600÷4)", got.AffectTime)
	}
	if got.AffectType != 11 || got.AffectValue != 5 {
		t.Errorf("parsed = %+v, want AffectType 11 / AffectValue 5", got)
	}
	if got.Name != "MySkill" {
		t.Errorf("Name = %q, want MySkill", got.Name)
	}
}

// TestParseSkillDataUsesLegacyAffectTimeDivisor pins the uniform legacy loader
// behavior. Duration tuning belongs to world.AffectDuration at cast time, so
// both the short shield row and an otherwise identical row load with ÷4.
func TestParseSkillDataUsesLegacyAffectTimeDivisor(t *testing.T) {
	const (
		shield = "85,90,0,120,5,0,0,0,0,0,31,150,30,anim.a,anim.b,0,0,0,1,0,0,0,Explosao_Eterea"
		other  = "84,90,0,120,5,0,0,0,0,0,31,150,30,anim.a,anim.b,0,0,0,1,0,0,0,Outra"
	)
	s, err := parseSkillData(strings.NewReader(shield + "\n" + other))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s.Get(85)
	if !ok {
		t.Fatal("skill 85 missing after parse")
	}
	if got.AffectTime != 7 {
		t.Errorf("skill 85 AffectTime = %d, want 7 (30÷4, legacy divisor)", got.AffectTime)
	}
	if got, ok := s.Get(84); !ok || got.AffectTime != 7 {
		t.Errorf("skill 84 AffectTime = %d (ok=%v), want 7 (30÷4)", got.AffectTime, ok)
	}
}

func TestSkillKindAndClass(t *testing.T) {
	tests := []struct{ index, kind, class int }{
		{0, 1, 0},   // TK tree 1
		{8, 2, 0},   // TK tree 2
		{16, 3, 0},  // TK tree 3
		{25, 1, 1},  // Foema tree 1
		{95, 3, 3},  // Huntress last
		{100, 1, 4}, // Sephira range
	}
	for _, tt := range tests {
		if got := SkillKind(tt.index); got != tt.kind {
			t.Errorf("SkillKind(%d) = %d, want %d", tt.index, got, tt.kind)
		}
		if got := SkillClass(tt.index); got != tt.class {
			t.Errorf("SkillClass(%d) = %d, want %d", tt.index, got, tt.class)
		}
	}
}

func TestLoadMaps(t *testing.T) {
	attr, err := LoadGrid(release(t, "TMsrv", "run", "AttributeMap.dat"), AttributeMapDim)
	if err != nil {
		t.Skipf("AttributeMap.dat unavailable: %v", err)
	}
	if len(attr.Data) != AttributeMapDim*AttributeMapDim {
		t.Errorf("attribute map size %d", len(attr.Data))
	}

	hm, err := LoadHeightMap(release(t, "TMsrv", "run", "HeightMap.dat"))
	if err != nil {
		t.Skipf("HeightMap.dat unavailable: %v", err)
	}
	if hm.Dim != HeightMapDim || len(hm.Data) != HeightMapDim*HeightMapDim {
		t.Errorf("height map dim %d size %d", hm.Dim, len(hm.Data))
	}
}
