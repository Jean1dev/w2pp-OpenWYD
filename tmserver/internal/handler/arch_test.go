package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestArchLevelGate is the Arch counterpart of TestCelestialLevelGate
// (CMob.cpp:1110). It matters more than pacing: combineItemLindy demands the
// character be at exactly level 354 or 369, so an Arch that levels past a wall
// loses access to its unlock quest permanently. Before this gate existed,
// characters reached the 360s with the 355 unlock still undone and no way back.
func TestArchLevelGate(t *testing.T) {
	d, w, e := mobKilledWorld(t)
	e.ClassMaster = classMasterArch
	e.Level = 353
	e.Exp = level.MaxExp

	d.applyLevelUps(w, nil, e)
	if e.Level != level.ArchGateLv355 {
		t.Fatalf("gated level = %d, want %d (blocked without ArchLv355)", e.Level, level.ArchGateLv355)
	}

	e.ArchLv355 = 1
	d.applyLevelUps(w, nil, e)
	if e.Level != level.ArchGateLv370 {
		t.Fatalf("after 355 unlock level = %d, want %d (blocked without ArchLv370)", e.Level, level.ArchGateLv370)
	}

	e.ArchLv370 = 1
	d.applyLevelUps(w, nil, e)
	if e.Level != level.MaxLevel {
		t.Fatalf("after 370 unlock level = %d, want MaxLevel %d", e.Level, level.MaxLevel)
	}
	// The Arch rides the Mortal curve and keeps the per-level point grants.
	if e.SkillBonus == 0 || e.SpecialBonus == 0 {
		t.Errorf("arch level-up granted no skill/special (%d/%d)", e.SkillBonus, e.SpecialBonus)
	}
}

// A Mortal shares the Arch curve but none of its walls: it must run straight
// through 354/369 with the quest flags unset.
func TestMortalIgnoresArchGate(t *testing.T) {
	d, w, e := mobKilledWorld(t)
	e.ClassMaster = classMasterMortal
	e.Level = 353
	e.Exp = level.MaxExp

	d.applyLevelUps(w, nil, e)
	if e.Level != level.MaxLevel {
		t.Fatalf("mortal stalled at %d, want MaxLevel %d — arch gate leaked into the mortal path", e.Level, level.MaxLevel)
	}
}

// archUnlockLevel picks the pending unlock and accepts the character from that
// level upward — the divergence that lets an Arch stranded above a wall (by the
// missing gate above) still run its quest.
func TestArchUnlockLevel(t *testing.T) {
	tests := []struct {
		name        string
		classMaster uint8
		lvl         int32
		l355, l370  uint8
		wantLevel   int32
		wantOK      bool
	}{
		{"mortal is never eligible", classMasterMortal, 364, 0, 0, 0, false},
		{"arch below 355", classMasterArch, 353, 0, 0, level.ArchGateLv355, false},
		{"arch exactly at 355", classMasterArch, level.ArchGateLv355, 0, 0, level.ArchGateLv355, true},
		{"arch stranded above 355", classMasterArch, 364, 0, 0, level.ArchGateLv355, true},
		{"355 done, below 370", classMasterArch, 360, 1, 0, level.ArchGateLv370, false},
		{"355 done, at 370", classMasterArch, level.ArchGateLv370, 1, 0, level.ArchGateLv370, true},
		{"355 done, stranded above 370", classMasterArch, 390, 1, 0, level.ArchGateLv370, true},
		// Past both walls with neither flag: 355 comes first, never 370.
		{"neither done, past both walls", classMasterArch, 390, 0, 0, level.ArchGateLv355, true},
		{"both done", classMasterArch, 390, 1, 1, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &world.Entity{ClassMaster: tt.classMaster, Level: tt.lvl, ArchLv355: tt.l355, ArchLv370: tt.l370}
			gotLevel, gotOK := archUnlockLevel(e)
			if gotLevel != tt.wantLevel || gotOK != tt.wantOK {
				t.Errorf("archUnlockLevel() = (%d, %v), want (%d, %v)", gotLevel, gotOK, tt.wantLevel, tt.wantOK)
			}
		})
	}
}

// downlevelArch must take back exactly what the climb granted, and must leave
// the character unable to instantly re-climb — Exp comes down with the level.
func TestDownlevelArch(t *testing.T) {
	d, w, e := mobKilledWorld(t)
	e.ClassMaster = classMasterArch
	e.Level = 353
	e.Exp = level.MaxExp
	e.ArchLv355 = 1 // let it climb past 354 for the test setup
	d.applyLevelUps(w, nil, e)
	if e.Level != level.ArchGateLv370 {
		t.Fatalf("setup: level = %d, want %d", e.Level, level.ArchGateLv370)
	}
	hpAt369, mpAt369 := e.BaseMaxHP, e.BaseMaxMP
	skillAt369, specialAt369 := e.SkillBonus, e.SpecialBonus

	lost := e.Level - level.ArchGateLv355 // 369 − 354 = 15 levels
	d.downlevelArch(e, level.ArchGateLv355)

	if e.Level != level.ArchGateLv355 {
		t.Errorf("level = %d, want %d", e.Level, level.ArchGateLv355)
	}
	// Exp must sit exactly on the floor of the quest level, so the next
	// applyLevelUps does not immediately replay the levels just taken back.
	if want := level.NextLevelExp(level.ArchGateLv355 - 1); e.Exp != want {
		t.Errorf("exp = %d, want the level floor %d", e.Exp, want)
	}
	if d.applyLevelUps(w, nil, e) {
		t.Error("applyLevelUps levelled again right after the downlevel — exp was not brought down")
	}
	if want := hpAt369 - level.IncHP(e.Class)*lost; e.BaseMaxHP != want {
		t.Errorf("BaseMaxHP = %d, want %d", e.BaseMaxHP, want)
	}
	if want := mpAt369 - level.IncMP(e.Class)*lost; e.BaseMaxMP != want {
		t.Errorf("BaseMaxMP = %d, want %d", e.BaseMaxMP, want)
	}
	if want := playerBaseAC(e); e.BaseAC != want {
		t.Errorf("BaseAC = %d, want %d (re-derived from the level)", e.BaseAC, want)
	}
	if want := skillAt369 - archSkillBonusPerLevel*uint16(lost); e.SkillBonus != want {
		t.Errorf("SkillBonus = %d, want %d", e.SkillBonus, want)
	}
	if want := specialAt369 - archSpecialBonusPerLevel*uint16(lost); e.SpecialBonus != want {
		t.Errorf("SpecialBonus = %d, want %d", e.SpecialBonus, want)
	}
}

// Points already spent cannot be clawed back: the pool floors at zero instead of
// wrapping around uint16.
func TestDownlevelArchFloorsSpentPoints(t *testing.T) {
	d, _, e := mobKilledWorld(t)
	e.ClassMaster = classMasterArch
	e.Level = 364
	e.SkillBonus, e.SpecialBonus = 3, 0 // the player spent almost everything
	d.downlevelArch(e, level.ArchGateLv355)
	if e.SkillBonus != 0 || e.SpecialBonus != 0 {
		t.Errorf("bonuses = (%d, %d), want (0, 0) — spent points must floor, not wrap",
			e.SkillBonus, e.SpecialBonus)
	}
}

// A character already at its quest level loses nothing.
func TestDownlevelArchNoopAtQuestLevel(t *testing.T) {
	d, _, e := mobKilledWorld(t)
	e.ClassMaster = classMasterArch
	e.Level = level.ArchGateLv355
	e.Exp = 12345
	before := *e
	d.downlevelArch(e, level.ArchGateLv355)
	if e.Level != before.Level || e.Exp != before.Exp || e.BaseMaxHP != before.BaseMaxHP {
		t.Errorf("downlevelArch mutated a character already at the quest level")
	}
}

// applyQuestReset is the selection half of /gm questreset, split out so the
// decision can be pinned without standing up a server.
func TestApplyQuestReset(t *testing.T) {
	tests := []struct {
		arg                    string
		want355, want370, wcri uint8
		wantOK                 bool
	}{
		{"355", 0, 1, 4, true},
		{"370", 1, 0, 4, true},
		{"cristal", 1, 1, 0, true},
		{"arch", 0, 0, 0, true},
		{"", 1, 1, 4, false},
		{"lixo", 1, 1, 4, false},
	}
	for _, tt := range tests {
		name := tt.arg
		if name == "" {
			name = "(vazio)"
		}
		t.Run(name, func(t *testing.T) {
			e := &world.Entity{ArchLv355: 1, ArchLv370: 1, ArchCristal: 4}
			_, ok := applyQuestReset(e, tt.arg)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if e.ArchLv355 != tt.want355 || e.ArchLv370 != tt.want370 || e.ArchCristal != tt.wcri {
				t.Errorf("355=%d 370=%d cristal=%d, want %d/%d/%d",
					e.ArchLv355, e.ArchLv370, e.ArchCristal, tt.want355, tt.want370, tt.wcri)
			}
		})
	}
}

// The level-370 reward goes onto the CAPE as instance effects (EF_HP 120,
// EF_RESISTALL 8), not onto the character — so the client's tooltip shows it and
// the server scores it like any other equipment effect.
func TestApplyArchCapeBonus(t *testing.T) {
	t.Run("empty cape takes both effects", func(t *testing.T) {
		cape := &world.Item{Index: 3193}
		if !applyArchCapeBonus(cape) {
			t.Fatal("applyArchCapeBonus() = false, want true")
		}
		if !hasEffect(cape, efHp, archCape370HP) {
			t.Errorf("EF_HP %d missing: %+v", archCape370HP, cape.Effects)
		}
		if !hasEffect(cape, efResistAll, archCape370Resist) {
			t.Errorf("EF_RESISTALL %d missing: %+v", archCape370Resist, cape.Effects)
		}
	})

	t.Run("a refined cape keeps its sanc", func(t *testing.T) {
		cape := &world.Item{Index: 3193}
		cape.Effects[0] = world.Effect{Effect: efSanc, Value: 9}
		if !applyArchCapeBonus(cape) {
			t.Fatal("a +9 cape has two free slots, want true")
		}
		if !hasEffect(cape, efSanc, 9) {
			t.Errorf("the refine level was overwritten: %+v", cape.Effects)
		}
		if !hasEffect(cape, efHp, archCape370HP) || !hasEffect(cape, efResistAll, archCape370Resist) {
			t.Errorf("bonus not written alongside the sanc: %+v", cape.Effects)
		}
	})

	t.Run("re-running does not duplicate", func(t *testing.T) {
		cape := &world.Item{Index: 3193}
		applyArchCapeBonus(cape)
		applyArchCapeBonus(cape)
		var hp int
		for _, ef := range cape.Effects {
			if ef.Effect == efHp {
				hp++
			}
		}
		if hp != 1 {
			t.Errorf("EF_HP appears %d times, want 1: %+v", hp, cape.Effects)
		}
	})

	t.Run("no free slot is reported, not swallowed", func(t *testing.T) {
		cape := &world.Item{Index: 3193}
		cape.Effects[0] = world.Effect{Effect: efSanc, Value: 9}
		cape.Effects[1] = world.Effect{Effect: 70, Value: 20}
		cape.Effects[2] = world.Effect{Effect: 71, Value: 20}
		if applyArchCapeBonus(cape) {
			t.Error("applyArchCapeBonus() = true with all three slots taken, want false")
		}
	})

	t.Run("no cape equipped", func(t *testing.T) {
		if applyArchCapeBonus(&world.Item{}) {
			t.Error("applyArchCapeBonus() = true on an empty slot, want false")
		}
	})
}

func hasEffect(it *world.Item, eff, val uint8) bool {
	for _, ef := range it.Effects {
		if ef.Effect == eff && ef.Value == val {
			return true
		}
	}
	return false
}

// Every Pedra Ideal refusal must reach the player as text. They were silent
// before — notify() carries a numeric code the client does not render yet — so a
// declined transformation looked exactly like a broken one.
func TestIdealStoneRefusalsAreExplained(t *testing.T) {
	for _, msg := range []string{
		msgCantWithArmor, msgIdealStoneArchOnly,
		msgIdealStoneLevel, msgIdealStoneMortalLevel,
	} {
		if msg == "" {
			t.Fatal("a refusal message is empty")
		}
		// The client reads single-byte Windows-1252; a UTF-8 accent would arrive
		// as mojibake, so these literals stay accent-free.
		if got := protocol.ClientText(msg); len(got) != len(msg) {
			t.Errorf("%q changes length when encoded (%d → %d): it carries characters outside the client's codepage",
				msg, len(msg), len(got))
		}
		if len(msg)+1 > protocol.MessageLength {
			t.Errorf("%q is %d bytes, over the %d the client reads", msg, len(msg)+1, protocol.MessageLength)
		}
	}
}

// The Arch→Celestial rebirth is dispatched by EF_VOLATILE 211, which belongs to
// the Pedra Ideal (5338). It used to hang off item 1742 — the Pedra da
// Imortalidade — so clicking the real stone did nothing at all.
func TestPedraIdealIsDispatchedByVolatile(t *testing.T) {
	if volPedraIdeal != 211 {
		t.Errorf("volPedraIdeal = %d, want 211 (_MSG_UseItem.cpp:3002)", volPedraIdeal)
	}
	// 1742 is a different item and must not be the trigger: it carries no
	// EF_VOLATILE at all, and its role is being worn for the King's flows.
	if idealStoneItem == 5338 {
		t.Error("idealStoneItem now points at the Pedra Ideal; the comment explaining the two stones is stale")
	}
}
