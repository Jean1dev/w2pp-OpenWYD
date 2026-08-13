// Package mobstat applies moderator-edited stat overrides onto raw STRUCT_MOB
// mob/NPC template bytes (mob-template-editing-plan.md) — the equivalent-tool
// successor to the legacy Win32 EDITAPPMOB. It is the sibling of npccfg: that
// package carries NPC placement/shop overrides materialized by
// handler/npcconfig.go; this one carries combat/attribute stat overrides
// applied to ANY npc/<template_name> template at load time (boot only — no
// hot-reload for this feature, matching EDITAPPMOB's own restart-to-apply
// behavior). Override is its own type, not domain.MobTemplateStat, so only
// tmserver/internal/dbclient depends on the wire shape — npccfg.Definition
// follows the same decoupling for NPC placement/shop.
package mobstat

import (
	"strings"

	"github.com/jeanluca/w2pp-openwyd/internal/savefmt"
)

// EquipItem is one Equip[] slot override (0..15, MAX_EQUIP).
type EquipItem struct {
	Slot  int
	Index uint16
	Eff   [3][2]uint8
}

// Override is a moderator-edited stat override for a raw STRUCT_MOB template,
// resolved from the dbServer's ListMobTemplateStats snapshot. Mirrors the
// EDITAPPMOB-editable field surface, minus Carry[] (npccfg.ShopItem's job)
// and DB-managed spawn position (npccfg.Definition's job).
type Override struct {
	DisplayName                     string // "" keeps the template file's own name
	Clan, Merchant, Class           uint8
	Coin                            int32
	Exp                             int64
	SPX, SPY                        int16
	Level, AC, Damage               int32
	ChaosRate, AttackRun, Direction uint8
	Str, Int, Dex, Con              int16
	Special                         [4]int16
	MaxHp, Hp, MaxMp, Mp            int32
	LearnedSkill                    int32
	ScoreBonus                      uint16
	SkillBar                        [4]uint8
	RegenHP, RegenMP                uint16
	Resist                          [4]int8
	Equip                           []EquipItem
}

// Apply returns a copy of template with ov's fields patched in. The edited
// score is mirrored into BOTH BaseScore and CurrentScore, replicating
// EDITAPPMOB's own `mob->CurrentScore = mob->BaseScore` on save. Equip is
// authoritative like npccfg's shop overlay: slots not listed in ov.Equip are
// cleared, not left at the template's original values — a moderator emptying
// the equip editor means "no equipment", not "keep whatever was there".
// template is normally already a canonical MobSize blob (npctemplate.Load
// widens the legacy layouts at content-load time), but the legacy 756/920-byte
// forms are accepted here too so an override never silently degrades to a
// no-op; the result is always MobSize. A corrupt template is returned unchanged.
func Apply(template []byte, ov Override) []byte {
	mob, _, err := savefmt.DecodeMobAny(template)
	if err != nil {
		return template
	}

	if strings.TrimSpace(ov.DisplayName) != "" {
		mob.Name = [16]byte{}
		copy(mob.Name[:], ov.DisplayName)
	}
	mob.Clan = ov.Clan
	mob.Merchant = ov.Merchant
	mob.Class = ov.Class
	mob.Coin = ov.Coin
	mob.Exp = ov.Exp
	mob.SPX = ov.SPX
	mob.SPY = ov.SPY

	score := savefmt.Score{
		Level: ov.Level, AC: ov.AC, Damage: ov.Damage,
		Merchant: ov.Merchant, AttackRun: ov.AttackRun, Direction: ov.Direction, ChaosRate: ov.ChaosRate,
		MaxHp: ov.MaxHp, MaxMp: ov.MaxMp, Hp: ov.Hp, Mp: ov.Mp,
		Str: ov.Str, Int: ov.Int, Dex: ov.Dex, Con: ov.Con,
		Special: ov.Special,
	}
	mob.BaseScore = score
	mob.CurrentScore = score

	mob.Equip = [savefmt.MaxEquip]savefmt.Item{}
	for _, it := range ov.Equip {
		if it.Slot < 0 || it.Slot >= savefmt.MaxEquip {
			continue
		}
		mob.Equip[it.Slot] = savefmt.Item{
			Index: int16(it.Index),
			Effects: [3]savefmt.Effect{
				{Effect: it.Eff[0][0], Value: it.Eff[0][1]},
				{Effect: it.Eff[1][0], Value: it.Eff[1][1]},
				{Effect: it.Eff[2][0], Value: it.Eff[2][1]},
			},
		}
	}

	mob.LearnedSkill = ov.LearnedSkill
	mob.ScoreBonus = ov.ScoreBonus
	mob.SkillBar = ov.SkillBar
	mob.RegenHP = ov.RegenHP
	mob.RegenMP = ov.RegenMP
	mob.Resist = ov.Resist

	return savefmt.EncodeMob(mob)
}

// ApplyOverride applies overrides[name] to template if present, returning
// template unchanged otherwise. Shared by the two places that resolve a
// template by name at boot (spawnNPCs and dbclient.NewNpcConfig's
// TemplateLoader) so the lookup-and-patch step isn't repeated at each call site.
func ApplyOverride(template []byte, name string, overrides map[string]Override) []byte {
	if ov, ok := overrides[name]; ok {
		return Apply(template, ov)
	}
	return template
}
