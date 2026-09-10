package world

import "testing"

func TestCharacterSaveExcludesEquipmentAttributeResources(t *testing.T) {
	w := New(Config{GridDim: 16}, slogDiscard(), nil, nil)
	e := &Entity{
		MaxHP: 1700, MaxMP: 2650, HP: 1600, MP: 2500,
		EquipmentAttributeHP: 500, EquipmentAttributeMP: 1000,
		Str: 350, Int: 600, Dex: 350, Con: 350,
	}
	e.Equip[12] = Item{Index: 4185}
	s := &Session{Conn: 1, AccountID: 7, Slot: 0}
	w.entities[1] = e
	for range 3 {
		// Both staged operations and the ordinary logout/shutdown path must use
		// the same representation. Current resources can exceed stored maxima,
		// just as with the existing read-time percentage/buff bonuses.
		for _, save := range []CharacterSave{w.CharacterSaveFor(s, e), w.characterSave(s)} {
			if save.MaxHP != 1200 || save.MaxMP != 1650 || save.HP != 1600 || save.MP != 2500 {
				t.Fatalf("saved resources = %d/%d %d/%d", save.HP, save.MaxHP, save.MP, save.MaxMP)
			}
			if save.Int != 600 || save.Con != 350 || len(save.Equip) != 1 || save.Equip[0].Index != 4185 {
				t.Fatal("snapshot altered attributes or equipment")
			}
		}
		if e.MaxHP != 1700 || e.MaxMP != 2650 {
			t.Fatal("snapshot mutated live maxima")
		}
	}
}
