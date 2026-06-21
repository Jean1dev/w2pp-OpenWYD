package content

import (
	"path/filepath"
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

func TestLoadSkillData(t *testing.T) {
	s, err := LoadSkillData(release(t, "Common", "SkillData.csv"))
	if err != nil {
		t.Skipf("SkillData.csv unavailable: %v", err)
	}
	if e, ok := s.Get(1); !ok || e.Name != "Toque_Sagrado" {
		t.Errorf("Get(1) = %+v, want name Toque_Sagrado", e)
	}
	if s.Len() < 1 {
		t.Errorf("skill count = %d, want >= 1", s.Len())
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
