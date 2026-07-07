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
		anvil       string
		level, want int
	}{
		{"PO", 0, 100},
		{"PO", 3, 85},
		{"PO", 5, 40},
		{"PL", 6, 80},
		{"PL", 9, 10},
	}
	for _, tt := range cases {
		got, ok := s.Rate(tt.anvil, tt.level)
		if !ok || got != tt.want {
			t.Errorf("Rate(%s,%d) = %d,%v, want %d", tt.anvil, tt.level, got, ok, tt.want)
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
	// Score-relevant effects plus EF_ITEMLEVEL (used by combine matching).
	if len(got) != 3 || got[3] != 96 || got[2] != 24 || got[87] != 5 {
		t.Errorf("BaseEffects = %v, want AC(3):96, DAMAGE(2):24, ITEMLEVEL(87):5", got)
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
