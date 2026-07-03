package world

import (
	"encoding/binary"
	"testing"
)

// TestSpawnMobRange covers the Entity.Range derivation at spawn — the port of
// BASE_GetMobAbility(EF_RANGE) (Basedef.cpp:2415/2523): per equip, catalog
// EF_RANGE (Config.ItemRanges) plus the instance EF_RANGE effects, and the MAX
// across the 16 equips wins. This is the production path (main wires
// ItemList.Ranges() into the Config): archer mobs get their reach from their
// Equip[0] model item, e.g. Ciclope_Arqueiro 242 → EF_RANGE 4.
func TestSpawnMobRange(t *testing.T) {
	w := New(Config{GridDim: 32, ItemRanges: map[int]int16{242: 4}}, slogDiscard(), nil, nil)

	// Equip[0] = item 242 (catalog reach 4); Equip[1] = item 99 with an instance
	// EF_RANGE=2 effect (a lesser source — the max must win, not the sum across
	// items).
	tmpl := make([]byte, structMobTemplateSize)
	binary.LittleEndian.PutUint16(tmpl[140:], 242)
	binary.LittleEndian.PutUint16(tmpl[148:], 99)
	tmpl[150] = 27 // Eff[0] = EF_RANGE
	tmpl[151] = 2
	id := w.SpawnMob(tmpl, 5, 5)
	if got := w.Entity(id).Range; got != 4 {
		t.Errorf("Range with catalog gear = %d, want 4", got)
	}

	// Catalog and instance effects on the SAME item add up (BASE_GetItemAbility
	// sums the catalog stEffect and the instance stEffect for the queried type).
	tmpl2 := make([]byte, structMobTemplateSize)
	binary.LittleEndian.PutUint16(tmpl2[140:], 242)
	tmpl2[142] = 27 // instance EF_RANGE +3 on the same item
	tmpl2[143] = 3
	id2 := w.SpawnMob(tmpl2, 6, 5)
	if got := w.Entity(id2).Range; got != 7 {
		t.Errorf("Range with catalog+instance on one item = %d, want 7", got)
	}

	// No gear (or no catalog): Range stays 0 → the AI falls back to melee reach.
	id3 := w.SpawnMob(make([]byte, structMobTemplateSize), 7, 5)
	if got := w.Entity(id3).Range; got != 0 {
		t.Errorf("Range with no gear = %d, want 0", got)
	}
}
