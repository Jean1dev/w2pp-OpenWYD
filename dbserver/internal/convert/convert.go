package convert

import (
	"fmt"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/savefmt"
)

// cstr trims a fixed-size C char array to a Go string: it cuts at the first NUL
// and strips trailing spaces (legacy files space-pad — see the antonio sample).
func cstr(b []byte) string {
	if i := indexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return strings.TrimRight(string(b), " ")
}

func indexByte(b []byte, c byte) int {
	for i := range b {
		if b[i] == c {
			return i
		}
	}
	return -1
}

// Account converts a current-format (7952-byte) AccountFile into the relational
// domain model, hashing the password, PIN and block password. The account name
// is canonicalized to lowercase (data-formats.md §1.1 case-sensitivity note).
func Account(af savefmt.AccountFile) (domain.Account, error) {
	name := strings.ToLower(cstr(af.Info.AccountName[:]))
	if name == "" {
		return domain.Account{}, fmt.Errorf("convert: account has empty name")
	}

	passHash, err := HashSecret(cstr(af.Info.AccountPass[:]))
	if err != nil {
		return domain.Account{}, fmt.Errorf("convert: hash password for %q: %w", name, err)
	}
	pinHash, err := HashSecret(cstr(af.Info.NumericToken[:]))
	if err != nil {
		return domain.Account{}, fmt.Errorf("convert: hash PIN for %q: %w", name, err)
	}
	blockHash, err := HashSecret(cstr(af.BlockPass[:]))
	if err != nil {
		return domain.Account{}, fmt.Errorf("convert: hash block password for %q: %w", name, err)
	}

	acc := domain.Account{
		Name:          name,
		PassHash:      passHash,
		PinHash:       pinHash,
		BlockPassHash: blockHash,
		RealName:      cstr(af.Info.RealName[:]),
		Email:         cstr(af.Info.Email[:]),
		Telephone:     cstr(af.Info.Telephone[:]),
		Address:       cstr(af.Info.Address[:]),
		SSN1:          af.Info.SSN1,
		SSN2:          af.Info.SSN2,
		DonateBalance: af.Donate,
		CargoCoin:     af.Coin,
		IsBlocked:     af.IsBlocked,
		Year:          af.Info.Year,
		YearDay:       af.Info.YearDay,
		Cargo:         items(af.Cargo[:]),
	}

	for slot := range af.Char {
		mob := af.Char[slot]
		if cstr(mob.Name[:]) == "" {
			continue // empty character slot
		}
		acc.Characters = append(acc.Characters, character(slot, mob, af.MobExtra[slot], af.ShortSkill[slot], af.Affect[slot]))
	}
	return acc, nil
}

func character(slot int, m savefmt.Mob, ex savefmt.MobExtra, shortSkill [16]uint8, affects [32]savefmt.Affect) domain.Character {
	s := m.CurrentScore
	c := domain.Character{
		Slot:          slot,
		Name:          cstr(m.Name[:]),
		Class:         m.Class,
		Clan:          m.Clan,
		GuildID:       m.Guild,
		GuildLevel:    m.GuildLevel,
		Level:         s.Level,
		Exp:           m.Exp,
		Coin:          m.Coin,
		Str:           s.Str,
		Int:           s.Int,
		Dex:           s.Dex,
		Con:           s.Con,
		ScoreBonus:    m.ScoreBonus,
		SpecialBonus:  m.SpecialBonus,
		SkillBonus:    m.SkillBonus,
		MaxHp:         s.MaxHp,
		MaxMp:         s.MaxMp,
		Hp:            s.Hp,
		Mp:            s.Mp,
		Critical:      m.Critical,
		RegenHP:       m.RegenHP,
		RegenMP:       m.RegenMP,
		ResistFire:    m.Resist[0],
		ResistIce:     m.Resist[1],
		ResistThunder: m.Resist[2],
		ResistMagic:   m.Resist[3],
		LearnedSkill:  m.LearnedSkill,
		Magic:         m.Magic,
		SaveX:         m.SPX,
		SaveY:         m.SPY,
		Citizen:       ex.Citizen(),
		ClassMaster:   ex.ClassMaster(),
		SkillBar:      m.SkillBar,
		ShortSkill:    shortSkill,
		Equip:         items(m.Equip[:]),
		Carry:         items(m.Carry[:]),
	}
	for i := range affects {
		a := affects[i]
		if a.Type == 0 && a.Time == 0 {
			continue // empty affect slot
		}
		c.Affects = append(c.Affects, domain.Affect{Type: a.Type, Value: a.Value, Level: a.Level, Time: a.Time})
	}
	return c
}

// items normalizes a fixed array of save items into domain items, dropping empty
// slots (sIndex==0) while preserving the slot index (positional meaning).
func items(src []savefmt.Item) []domain.Item {
	var out []domain.Item
	for slot := range src {
		it := src[slot]
		if it.Empty() {
			continue
		}
		out = append(out, domain.Item{
			Slot:  slot,
			Index: it.Index,
			Eff1:  it.Effects[0].Effect, EffV1: it.Effects[0].Value,
			Eff2: it.Effects[1].Effect, EffV2: it.Effects[1].Value,
			Eff3: it.Effects[2].Effect, EffV3: it.Effects[2].Value,
		})
	}
	return out
}
