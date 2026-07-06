package mapzones

import "testing"

// TestAllMatchesCityOrder locks the order/names against
// tmserver/internal/world/city.go's cities array (0 Armia .. 4 Noatum) — a
// drift here would silently mislabel the moderator UI's map picker.
func TestAllMatchesCityOrder(t *testing.T) {
	want := []Zone{
		{0, "Armia"},
		{1, "Azran"},
		{2, "Erion"},
		{3, "Nippleheim"},
		{4, "Noatum"},
	}
	if len(All) != len(want) {
		t.Fatalf("len(All) = %d, want %d", len(All), len(want))
	}
	for i, z := range want {
		if All[i] != z {
			t.Errorf("All[%d] = %+v, want %+v", i, All[i], z)
		}
	}
}
