package savefmt

import (
	"bytes"
	"strings"
	"testing"
)

func name16(m Mob) string { return strings.TrimRight(string(m.Name[:]), "\x00") }

// TestDecodeMobAnyLegacy756 pins the reverse-engineered 756-byte layout against
// two real templates: a monster (@Gargula) and a weapon shop (Armas[D], whose
// populated Carry[] is what proves the Equip/Carry offsets).
func TestDecodeMobAnyLegacy756(t *testing.T) {
	tests := []struct {
		file string
		want Mob
		// carry lists the leading Carry[] item indices that must be present.
		carry []int16
		equip []int16
	}{
		{
			file: fxLegacy756Gargula,
			want: Mob{
				Clan: 1, Merchant: 0, Class: 1,
				Coin: 500, Exp: 20000, SPX: 2113, SPY: 2096,
				ScoreBonus: 27, Critical: 0xff, SaveMana: 0xff,
				SkillBar:   [4]uint8{0xff, 0xff, 0xff, 0xff},
				GuildLevel: 0, RegenHP: 0, RegenMP: 0,
				Resist: [4]int8{0, 0, 0, 10},
				CurrentScore: Score{
					Level: 399, AC: 1820, Damage: 645, Merchant: 0, AttackRun: 2,
					MaxHp: 14000, Hp: 14000,
					Str: 4, Int: 70, Dex: 90, Con: 800,
					Special: [4]int16{104, 1, 104, 1},
				},
			},
			equip: []int16{295},
		},
		{
			file: fxLegacy756Shop,
			want: Mob{
				Clan: 3, Merchant: 1, Class: 1,
				Coin: 9999, Exp: 1, SPX: 2144, SPY: 2088,
				ScoreBonus: 0, Critical: 0xff, SaveMana: 0xff,
				SkillBar: [4]uint8{0xff, 0xff, 0xff, 0xff},
				CurrentScore: Score{
					Level: 2, AC: 3000, Damage: 500, Merchant: 1, AttackRun: 1,
					MaxHp: 9000, Hp: 9000,
					Str: 5, Int: 100, Dex: 0, Con: 0,
					Special: [4]int16{255, 7, 255, 7},
				},
			},
			carry: []int16{900, 825, 855, 870, 810, 840, 936, 911, 1710},
			equip: []int16{59, 120, 121, 122, 123, 124},
		},
		{
			// @Tauron is the file that settles the tail: RegenHP/RegenMP are
			// 8-bit here (30/9), matching the canonical Tauron's 16-bit fields.
			file: fxLegacy756Tauron,
			want: Mob{
				Clan: 1, Merchant: 0, Class: 1,
				Coin: 500, Exp: 8500, SPX: 2113, SPY: 2096,
				LearnedSkill: 50050, ScoreBonus: 33,
				Critical: 0xff, SaveMana: 0xff,
				SkillBar:   [4]uint8{0xff, 0xff, 0xff, 0xff},
				GuildLevel: 0, RegenHP: 30, RegenMP: 9,
				CurrentScore: Score{
					Level: 399, AC: 2900, Damage: 1150, Merchant: 0, AttackRun: 3,
					MaxHp: 30000, Hp: 30000,
					Str: 5, Int: 70, Dex: 100, Con: 1200,
					Special: [4]int16{255, 7, 255, 7},
				},
			},
			equip: []int16{312},
		},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			raw := fixture(t, tt.file)
			got, version, err := DecodeMobAny(raw)
			if err != nil {
				t.Fatalf("DecodeMobAny: %v", err)
			}
			if version != MobVersionLegacy756 {
				t.Fatalf("version = %v, want %v", version, MobVersionLegacy756)
			}
			// BaseScore mirrors CurrentScore on a freshly authored template.
			if got.BaseScore != got.CurrentScore {
				t.Errorf("BaseScore != CurrentScore:\n base %+v\n cur  %+v", got.BaseScore, got.CurrentScore)
			}
			if got.CurrentScore != tt.want.CurrentScore {
				t.Errorf("CurrentScore =\n %+v\nwant\n %+v", got.CurrentScore, tt.want.CurrentScore)
			}
			checkScalars(t, got, tt.want)
			for i, want := range tt.equip {
				if got.Equip[i].Index != want {
					t.Errorf("Equip[%d].Index = %d, want %d", i, got.Equip[i].Index, want)
				}
			}
			for i, want := range tt.carry {
				if got.Carry[i].Index != want {
					t.Errorf("Carry[%d].Index = %d, want %d", i, got.Carry[i].Index, want)
				}
			}
			// Fields the legacy layout does not carry must stay zero.
			if got.Quest != 0 || got.Magic != 0 {
				t.Errorf("Quest/Magic = %d/%d, want 0/0 (absent from the 756 layout)", got.Quest, got.Magic)
			}
			if got.CurrentScore.MaxMp != 0 || got.CurrentScore.Mp != 0 {
				t.Errorf("MaxMp/Mp = %d/%d, want 0/0", got.CurrentScore.MaxMp, got.CurrentScore.Mp)
			}
		})
	}
}

func checkScalars(t *testing.T, got, want Mob) {
	t.Helper()
	type field struct {
		name     string
		got, wnt any
	}
	for _, f := range []field{
		{"Clan", got.Clan, want.Clan},
		{"Merchant", got.Merchant, want.Merchant},
		{"Class", got.Class, want.Class},
		{"Coin", got.Coin, want.Coin},
		{"Exp", got.Exp, want.Exp},
		{"SPX", got.SPX, want.SPX},
		{"SPY", got.SPY, want.SPY},
		{"LearnedSkill", got.LearnedSkill, want.LearnedSkill},
		{"ScoreBonus", got.ScoreBonus, want.ScoreBonus},
		{"Critical", got.Critical, want.Critical},
		{"SaveMana", got.SaveMana, want.SaveMana},
		{"SkillBar", got.SkillBar, want.SkillBar},
		{"GuildLevel", got.GuildLevel, want.GuildLevel},
		{"RegenHP", got.RegenHP, want.RegenHP},
		{"RegenMP", got.RegenMP, want.RegenMP},
		{"Resist", got.Resist, want.Resist},
	} {
		if f.got != f.wnt {
			t.Errorf("%s = %v, want %v", f.name, f.got, f.wnt)
		}
	}
}

// TestDecodeMobAnyLegacy920 proves the 920-byte form is the same 756-byte
// record with trailing junk: decoding it must equal decoding its first 756
// bytes, and the junk must differ from zero (otherwise the fixture would not be
// exercising the case it claims to).
func TestDecodeMobAnyLegacy920(t *testing.T) {
	raw := fixture(t, fxLegacy920NPC1)
	got, version, err := DecodeMobAny(raw)
	if err != nil {
		t.Fatalf("DecodeMobAny: %v", err)
	}
	if version != MobVersionLegacy756Padded {
		t.Fatalf("version = %v, want %v", version, MobVersionLegacy756Padded)
	}
	if name16(got) != "NPC_1" {
		t.Errorf("Name = %q, want %q", name16(got), "NPC_1")
	}
	if got.CurrentScore.Merchant != 1 {
		t.Errorf("CurrentScore.Merchant = %d, want 1 (NPC_1 is a shop)", got.CurrentScore.Merchant)
	}
	if got.Coin != 500 || got.SPX != 2113 || got.SPY != 2096 {
		t.Errorf("Coin/SPX/SPY = %d/%d/%d, want 500/2113/2096", got.Coin, got.SPX, got.SPY)
	}

	truncated, tv, err := DecodeMobAny(raw[:MobSizeLegacy756])
	if err != nil {
		t.Fatalf("DecodeMobAny(truncated): %v", err)
	}
	if tv != MobVersionLegacy756 {
		t.Fatalf("truncated version = %v, want %v", tv, MobVersionLegacy756)
	}
	if got != truncated {
		t.Error("920-byte decode differs from its own first 756 bytes; the tail is not junk")
	}
	if bytes.Equal(raw[MobSizeLegacy756:], make([]byte, len(raw)-MobSizeLegacy756)) {
		t.Error("fixture tail is all zeros; it no longer exercises the uninitialized-junk case")
	}
}

// TestLegacy756MatchesCanonicalTwin is the evidence for the layout: @Gargula
// and Gargula (and the Tauron pair) are the same mob shipped in the two
// formats. The fields a rebalance would not touch have to agree exactly across
// the two completely different offset maps.
func TestLegacy756MatchesCanonicalTwin(t *testing.T) {
	pairs := []struct{ legacy, canonical string }{
		{fxLegacy756Gargula, fxCurrent816Gargul},
		{fxLegacy756Tauron, fxCurrent816Tauron},
	}
	for _, p := range pairs {
		t.Run(p.legacy, func(t *testing.T) {
			legacy, _, err := DecodeMobAny(fixture(t, p.legacy))
			if err != nil {
				t.Fatalf("decode legacy: %v", err)
			}
			canonical, err := DecodeMob(fixture(t, p.canonical))
			if err != nil {
				t.Fatalf("decode canonical: %v", err)
			}
			if legacy.Clan != canonical.Clan {
				t.Errorf("Clan: legacy %d, canonical %d", legacy.Clan, canonical.Clan)
			}
			if legacy.Class != canonical.Class {
				t.Errorf("Class: legacy %d, canonical %d", legacy.Class, canonical.Class)
			}
			if legacy.SPX != canonical.SPX || legacy.SPY != canonical.SPY {
				t.Errorf("spawn point: legacy %d,%d canonical %d,%d",
					legacy.SPX, legacy.SPY, canonical.SPX, canonical.SPY)
			}
			if legacy.ScoreBonus != canonical.ScoreBonus {
				t.Errorf("ScoreBonus: legacy %d, canonical %d", legacy.ScoreBonus, canonical.ScoreBonus)
			}
			if legacy.Critical != canonical.Critical || legacy.SaveMana != canonical.SaveMana {
				t.Errorf("Critical/SaveMana: legacy %d/%d canonical %d/%d",
					legacy.Critical, legacy.SaveMana, canonical.Critical, canonical.SaveMana)
			}
			if legacy.SkillBar != canonical.SkillBar {
				t.Errorf("SkillBar: legacy %v, canonical %v", legacy.SkillBar, canonical.SkillBar)
			}
			ls, cs := legacy.CurrentScore, canonical.CurrentScore
			if ls.AC != cs.AC || ls.Damage != cs.Damage {
				t.Errorf("AC/Damage: legacy %d/%d canonical %d/%d", ls.AC, ls.Damage, cs.AC, cs.Damage)
			}
			if ls.MaxHp != cs.MaxHp || ls.Hp != cs.Hp {
				t.Errorf("MaxHp/Hp: legacy %d/%d canonical %d/%d", ls.MaxHp, ls.Hp, cs.MaxHp, cs.Hp)
			}
			if ls.Str != cs.Str || ls.Int != cs.Int || ls.Dex != cs.Dex {
				t.Errorf("Str/Int/Dex: legacy %d/%d/%d canonical %d/%d/%d",
					ls.Str, ls.Int, ls.Dex, cs.Str, cs.Int, cs.Dex)
			}
			if ls.AttackRun != cs.AttackRun {
				t.Errorf("AttackRun: legacy %d, canonical %d", ls.AttackRun, cs.AttackRun)
			}
			if ls.Special != cs.Special {
				t.Errorf("Special: legacy %v, canonical %v", ls.Special, cs.Special)
			}
		})
	}
}

// TestLegacy756RegenIsByteWide guards the one field width the offset map got
// wrong at first: reading RegenHP/RegenMP as uint16 turned Tauron's 30/9 into
// 0/2334.
func TestLegacy756RegenIsByteWide(t *testing.T) {
	legacy, _, err := DecodeMobAny(fixture(t, fxLegacy756Tauron))
	if err != nil {
		t.Fatalf("decode legacy: %v", err)
	}
	canonical, err := DecodeMob(fixture(t, fxCurrent816Tauron))
	if err != nil {
		t.Fatalf("decode canonical: %v", err)
	}
	if legacy.RegenHP != canonical.RegenHP || legacy.RegenMP != canonical.RegenMP {
		t.Errorf("Regen HP/MP: legacy %d/%d, canonical %d/%d",
			legacy.RegenHP, legacy.RegenMP, canonical.RegenHP, canonical.RegenMP)
	}
}

func TestDecodeMobAnyRejectsUnknownLayout(t *testing.T) {
	for _, size := range []int{0, 3, MobSize - 1, MobSize + 1, MobSizeLegacy756Padded + 1} {
		if _, v, err := DecodeMobAny(make([]byte, size)); err == nil {
			t.Errorf("DecodeMobAny(%d bytes) succeeded as %v, want error", size, v)
		}
	}
}

// TestNormalizeMob is the loader contract: every accepted layout comes out as
// one canonical MobSize blob that decodes to the same Mob.
func TestNormalizeMob(t *testing.T) {
	for _, f := range []string{fxCurrent816Gargul, fxLegacy756Gargula, fxLegacy756Shop, fxLegacy920NPC1} {
		t.Run(f, func(t *testing.T) {
			raw := fixture(t, f)
			canonical, version, err := NormalizeMob(raw)
			if err != nil {
				t.Fatalf("NormalizeMob: %v", err)
			}
			if len(canonical) != MobSize {
				t.Fatalf("normalized length = %d, want %d", len(canonical), MobSize)
			}
			if version != DetectMobVersion(len(raw)) {
				t.Errorf("version = %v, want %v", version, DetectMobVersion(len(raw)))
			}
			want, _, err := DecodeMobAny(raw)
			if err != nil {
				t.Fatalf("DecodeMobAny: %v", err)
			}
			got, err := DecodeMob(canonical)
			if err != nil {
				t.Fatalf("DecodeMob(normalized): %v", err)
			}
			if got != want {
				t.Error("DecodeMob(NormalizeMob(x)) != DecodeMobAny(x)")
			}
			// An already-canonical blob must survive byte for byte, and the
			// result must never alias the caller's slice.
			if version == MobVersionCurrent && !bytes.Equal(canonical, raw) {
				t.Error("canonical input was modified by NormalizeMob")
			}
			canonical[0] ^= 0xff
			if bytes.Equal(canonical[:1], raw[:1]) {
				t.Error("NormalizeMob aliased the input slice")
			}
		})
	}
}

func TestNormalizeMobRejectsUnknownLayout(t *testing.T) {
	if _, _, err := NormalizeMob([]byte{1, 2, 3}); err == nil {
		t.Error("NormalizeMob({1,2,3}) succeeded, want error")
	}
}
