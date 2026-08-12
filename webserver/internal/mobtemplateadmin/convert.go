package mobtemplateadmin

import (
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/savefmt"
)

// statFromRawTemplate decodes a raw STRUCT_MOB (Get's read-through fallback)
// into the same field shape as a stat override, reading the template's
// CurrentScore (BaseScore mirrors it on a freshly-loaded template, same as
// EDITAPPMOB's own ReadMob). Any layout savefmt knows is accepted, so the
// legacy 756/920-byte templates are editable too (data-formats.md §1.4.1).
func statFromRawTemplate(templateName string, raw []byte) (domain.MobTemplateStat, error) {
	mob, _, err := savefmt.DecodeMobAny(raw)
	if err != nil {
		return domain.MobTemplateStat{}, fmt.Errorf("mobtemplateadmin: decode %q: %w", templateName, err)
	}
	sc := mob.CurrentScore
	st := domain.MobTemplateStat{
		TemplateName: templateName,
		Clan:         mob.Clan,
		Merchant:     mob.Merchant,
		Class:        mob.Class,
		Coin:         mob.Coin,
		Exp:          mob.Exp,
		SPX:          int32(mob.SPX),
		SPY:          int32(mob.SPY),
		Level:        sc.Level,
		AC:           sc.AC,
		Damage:       sc.Damage,
		ChaosRate:    sc.ChaosRate,
		AttackRun:    sc.AttackRun,
		Direction:    sc.Direction,
		Str:          sc.Str,
		Int:          sc.Int,
		Dex:          sc.Dex,
		Con:          sc.Con,
		Special:      sc.Special,
		MaxHp:        sc.MaxHp,
		Hp:           sc.Hp,
		MaxMp:        sc.MaxMp,
		Mp:           sc.Mp,
		LearnedSkill: mob.LearnedSkill,
		ScoreBonus:   mob.ScoreBonus,
		SkillBar:     mob.SkillBar,
		RegenHP:      mob.RegenHP,
		RegenMP:      mob.RegenMP,
		Resist:       mob.Resist,
	}
	for i, it := range mob.Equip {
		if it.Empty() {
			continue
		}
		st.Equip = append(st.Equip, domain.MobTemplateEquipItem{
			Slot: int16(i), ItemIndex: int32(it.Index),
			Eff1: it.Effects[0].Effect, EffV1: it.Effects[0].Value,
			Eff2: it.Effects[1].Effect, EffV2: it.Effects[1].Value,
			Eff3: it.Effects[2].Effect, EffV3: it.Effects[2].Value,
		})
	}
	return st, nil
}
