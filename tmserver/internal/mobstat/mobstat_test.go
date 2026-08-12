package mobstat

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/savefmt"
)

func blankTemplate(name string) []byte {
	b := make([]byte, savefmt.MobSize)
	copy(b[0:16], name)
	return b
}

// TestApplyMirrorsBaseAndCurrentScore checks the edited score lands at both
// STRUCT_SCORE offsets (44 and 92), matching EDITAPPMOB's own
// `CurrentScore = BaseScore` on save, and that a decode of the result reads
// back exactly what was set.
func TestApplyMirrorsBaseAndCurrentScore(t *testing.T) {
	tmpl := blankTemplate("Karkarian")
	ov := Override{
		Clan: 3, Merchant: 0, Class: 1,
		Coin: 5000, Exp: 123456, SPX: 10, SPY: 20,
		Level: 80, AC: 40, Damage: 300, ChaosRate: 1,
		AttackRun: 5, Direction: 2,
		Str: 100, Int: 10, Dex: 50, Con: 90,
		Special: [4]int16{1, 2, 3, 4},
		MaxHp:   50000, Hp: 50000, MaxMp: 1000, Mp: 1000,
		LearnedSkill: 7, ScoreBonus: 3,
		SkillBar: [4]uint8{1, 2, 3, 4},
		RegenHP:  10, RegenMP: 5,
		Resist: [4]int8{1, -1, 2, -2},
		Equip: []EquipItem{
			{Slot: 0, Index: 1100, Eff: [3][2]uint8{{1, 9}, {0, 0}, {0, 0}}},
			{Slot: 15, Index: 1200},
		},
	}

	out := Apply(tmpl, ov)
	if len(out) != savefmt.MobSize {
		t.Fatalf("Apply output length = %d, want %d", len(out), savefmt.MobSize)
	}

	mob, err := savefmt.DecodeMob(out)
	if err != nil {
		t.Fatalf("DecodeMob: %v", err)
	}

	if mob.Clan != 3 || mob.Class != 1 || mob.Coin != 5000 || mob.Exp != 123456 || mob.SPX != 10 || mob.SPY != 20 {
		t.Fatalf("top-level fields = %+v, want Clan=3 Class=1 Coin=5000 Exp=123456 SPX=10 SPY=20", mob)
	}

	for _, sc := range []struct {
		name string
		got  savefmt.Score
	}{{"BaseScore", mob.BaseScore}, {"CurrentScore", mob.CurrentScore}} {
		if sc.got.Level != 80 || sc.got.AC != 40 || sc.got.Damage != 300 || sc.got.ChaosRate != 1 ||
			sc.got.AttackRun != 5 || sc.got.Direction != 2 ||
			sc.got.Str != 100 || sc.got.Int != 10 || sc.got.Dex != 50 || sc.got.Con != 90 ||
			sc.got.Special != [4]int16{1, 2, 3, 4} ||
			sc.got.MaxHp != 50000 || sc.got.Hp != 50000 || sc.got.MaxMp != 1000 || sc.got.Mp != 1000 {
			t.Errorf("%s = %+v, does not match override", sc.name, sc.got)
		}
	}

	if !mob.Equip[0].Empty() {
		if mob.Equip[0].Index != 1100 || mob.Equip[0].Effects[0].Effect != 1 || mob.Equip[0].Effects[0].Value != 9 {
			t.Errorf("Equip[0] = %+v, want Index=1100 Eff1={1,9}", mob.Equip[0])
		}
	} else {
		t.Error("Equip[0] is empty, want Index=1100")
	}
	if mob.Equip[15].Index != 1200 {
		t.Errorf("Equip[15].Index = %d, want 1200", mob.Equip[15].Index)
	}
	for i := 1; i < 15; i++ {
		if !mob.Equip[i].Empty() {
			t.Errorf("Equip[%d] = %+v, want empty (authoritative replace)", i, mob.Equip[i])
		}
	}

	if mob.LearnedSkill != 7 || mob.ScoreBonus != 3 || mob.SkillBar != [4]uint8{1, 2, 3, 4} ||
		mob.RegenHP != 10 || mob.RegenMP != 5 || mob.Resist != [4]int8{1, -1, 2, -2} {
		t.Errorf("tail fields = %+v, does not match override", mob)
	}
}

// TestApplyPreservesNameWhenDisplayNameEmpty checks that an empty
// DisplayName leaves the template's own name untouched.
func TestApplyPreservesNameWhenDisplayNameEmpty(t *testing.T) {
	tmpl := blankTemplate("Karkarian")
	out := Apply(tmpl, Override{})
	mob, err := savefmt.DecodeMob(out)
	if err != nil {
		t.Fatalf("DecodeMob: %v", err)
	}
	got := string(mob.Name[:9]) // "Karkarian" is 9 bytes
	if got != "Karkarian" {
		t.Errorf("Name = %q, want Karkarian", got)
	}
}

// TestApplyOverwritesDisplayName checks a non-empty DisplayName replaces the
// template's own name and clears any trailing bytes from a longer old name.
func TestApplyOverwritesDisplayName(t *testing.T) {
	tmpl := blankTemplate("VeryLongOldName") // 15 bytes, fills most of the 16-byte field
	out := Apply(tmpl, Override{DisplayName: "Short"})
	mob, err := savefmt.DecodeMob(out)
	if err != nil {
		t.Fatalf("DecodeMob: %v", err)
	}
	// Name is a fixed [16]byte; bytes after "Short" must be zeroed, not leftover
	// from "VeryLongOldName".
	for i := 5; i < 16; i++ {
		if mob.Name[i] != 0 {
			t.Fatalf("Name[%d] = %d, want 0 (leftover from old name not cleared): %+v", i, mob.Name[i], mob.Name)
		}
	}
	if got := string(mob.Name[:5]); got != "Short" {
		t.Errorf("Name = %q, want Short", got)
	}
}

// TestApplyReturnsInputUnchangedForCorruptTemplate checks a template that
// fails to decode (not a STRUCT_MOB in any layout) is returned as-is rather
// than panicking.
func TestApplyReturnsInputUnchangedForCorruptTemplate(t *testing.T) {
	bad := []byte{1, 2, 3}
	out := Apply(bad, Override{Level: 99})
	if len(out) != 3 {
		t.Fatalf("Apply(corrupt) length = %d, want 3 (unchanged)", len(out))
	}
}

// TestApplyUpgradesLegacyTemplates covers issue #244's silent-no-op: a legacy
// 756/920-byte template used to fail DecodeMob and be returned untouched, so a
// moderator's override simply did nothing. It must now come back as a
// canonical MobSize blob with the override applied.
func TestApplyUpgradesLegacyTemplates(t *testing.T) {
	for _, size := range []int{savefmt.MobSizeLegacy756, savefmt.MobSizeLegacy756Padded} {
		tmpl := make([]byte, size)
		copy(tmpl[0:16], "Legacy")
		// CurrentScore.Level in the legacy layout: score at 64, Level at +0.
		tmpl[64] = 50

		out := Apply(tmpl, Override{Level: 99, MaxHp: 1234, Hp: 1234, Merchant: 1})
		if len(out) != savefmt.MobSize {
			t.Fatalf("size %d: Apply length = %d, want %d", size, len(out), savefmt.MobSize)
		}
		mob, err := savefmt.DecodeMob(out)
		if err != nil {
			t.Fatalf("size %d: DecodeMob: %v", size, err)
		}
		if mob.CurrentScore.Level != 99 || mob.BaseScore.Level != 99 {
			t.Errorf("size %d: Level = %d/%d, want 99 in both scores",
				size, mob.BaseScore.Level, mob.CurrentScore.Level)
		}
		if mob.CurrentScore.MaxHp != 1234 || mob.Merchant != 1 {
			t.Errorf("size %d: MaxHp/Merchant = %d/%d, want 1234/1",
				size, mob.CurrentScore.MaxHp, mob.Merchant)
		}
		// The template's own name survives when the override does not set one.
		if got := string(mob.Name[:6]); got != "Legacy" {
			t.Errorf("size %d: Name = %q, want Legacy", size, got)
		}
	}
}
