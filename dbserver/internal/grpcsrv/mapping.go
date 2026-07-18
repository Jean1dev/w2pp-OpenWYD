package grpcsrv

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// uniqueViolation is the PostgreSQL SQLSTATE for a unique-constraint conflict.
const uniqueViolation = "23505"

// isUniqueViolation reports whether err is a PostgreSQL unique-constraint error
// (e.g. a taken character slot or name on create).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}

// characterToProto maps the relational character to the wire contract. Only the
// fields the contract carries are emitted (see store.SaveCharacter for why the
// model is intentionally a subset).
func characterToProto(ch domain.Character) *dbv1.Character {
	return &dbv1.Character{
		Slot:     int32(ch.Slot),
		Name:     ch.Name,
		Class:    int32(ch.Class),
		Clan:     int32(ch.Clan),
		GuildId:  uint32(ch.GuildID),
		Level:    ch.Level,
		Exp:      ch.Exp,
		Coin:     ch.Coin,
		Str:      int32(ch.Str),
		Int:      int32(ch.Int),
		Dex:      int32(ch.Dex),
		Con:      int32(ch.Con),
		MaxHp:    ch.MaxHp,
		MaxMp:    ch.MaxMp,
		Hp:       ch.Hp,
		Mp:       ch.Mp,
		LastCity: int32(ch.LastCity),
		Equip:    itemsToProto(ch.Equip),
		Carry:    itemsToProto(ch.Carry),
		Affects:  affectsToProto(ch.Affects),

		ScoreBonus:      int32(ch.ScoreBonus),
		SpecialBonus:    int32(ch.SpecialBonus),
		LearnedSkill:    ch.LearnedSkill,
		SecLearnedSkill: ch.SecLearnedSkill,
		Magic:           int64(ch.Magic),
		Special:         int16ArrToProto(ch.Special[:]),
		SkillBar:        byteArrToProto(ch.SkillBar[:]),
		ShortSkill:      byteArrToProto(ch.ShortSkill[:]),
		Soul:            int32(ch.Soul),
		ClassMaster:     int32(ch.ClassMaster),
		Citizen:         int32(ch.Citizen),
		Fame:            ch.Fame,
		SaveX:           int32(ch.SaveX),
		SaveY:           int32(ch.SaveY),
		CelestialLv40:   int32(ch.CelLv40),
		CelestialLv90:   int32(ch.CelLv90),
		CelestialCircle: int32(ch.CelCircle),
	}
}

func int16ArrToProto(v []int16) []int32 {
	out := make([]int32, len(v))
	for i, x := range v {
		out[i] = int32(x)
	}
	return out
}

func byteArrToProto(v []uint8) []uint32 {
	out := make([]uint32, len(v))
	for i, x := range v {
		out[i] = uint32(x)
	}
	return out
}

// protoToCharacter maps the wire contract back to the relational character.
func protoToCharacter(c *dbv1.Character) domain.Character {
	if c == nil {
		return domain.Character{}
	}
	return domain.Character{
		Slot:     int(c.GetSlot()),
		Name:     c.GetName(),
		Class:    uint8(c.GetClass()),
		Clan:     uint8(c.GetClan()),
		GuildID:  uint16(c.GetGuildId()),
		Level:    c.GetLevel(),
		Exp:      c.GetExp(),
		Coin:     c.GetCoin(),
		Str:      int16(c.GetStr()),
		Int:      int16(c.GetInt()),
		Dex:      int16(c.GetDex()),
		Con:      int16(c.GetCon()),
		MaxHp:    c.GetMaxHp(),
		MaxMp:    c.GetMaxMp(),
		Hp:       c.GetHp(),
		Mp:       c.GetMp(),
		LastCity: int16(c.GetLastCity()),
		Equip:    protoToItems(c.GetEquip()),
		Carry:    protoToItems(c.GetCarry()),
		Affects:  protoToAffects(c.GetAffects()),

		ScoreBonus:      uint16(c.GetScoreBonus()),
		SpecialBonus:    uint16(c.GetSpecialBonus()),
		LearnedSkill:    c.GetLearnedSkill(),
		SecLearnedSkill: c.GetSecLearnedSkill(),
		Magic:           uint32(c.GetMagic()),
		Special:         protoToInt16Arr4(c.GetSpecial()),
		SkillBar:        [4]uint8(protoToByteArr(c.GetSkillBar(), 4)),
		ShortSkill:      [16]uint8(protoToByteArr(c.GetShortSkill(), 16)),
		Soul:            uint8(c.GetSoul()),
		ClassMaster:     uint8(c.GetClassMaster()),
		Citizen:         uint8(c.GetCitizen()),
		Fame:            c.GetFame(),
		SaveX:           int16(c.GetSaveX()),
		SaveY:           int16(c.GetSaveY()),
		CelLv40:         uint8(c.GetCelestialLv40()),
		CelLv90:         uint8(c.GetCelestialLv90()),
		CelCircle:       uint8(c.GetCelestialCircle()),
	}
}

func protoToInt16Arr4(v []int32) [4]int16 {
	var out [4]int16
	for i := 0; i < len(v) && i < 4; i++ {
		out[i] = int16(v[i])
	}
	return out
}

func protoToByteArr(v []uint32, n int) []uint8 {
	out := make([]uint8, n)
	for i := 0; i < len(v) && i < n; i++ {
		out[i] = uint8(v[i])
	}
	return out
}

func itemToProto(it domain.Item) *dbv1.Item {
	return &dbv1.Item{
		Slot:      int32(it.Slot),
		Index:     int32(it.Index),
		Eff1:      int32(it.Eff1),
		Effv1:     int32(it.EffV1),
		Eff2:      int32(it.Eff2),
		Effv2:     int32(it.EffV2),
		Eff3:      int32(it.Eff3),
		Effv3:     int32(it.EffV3),
		ExpiresAt: it.ExpiresAt,
	}
}

func itemsToProto(items []domain.Item) []*dbv1.Item {
	if len(items) == 0 {
		return nil
	}
	out := make([]*dbv1.Item, 0, len(items))
	for _, it := range items {
		out = append(out, itemToProto(it))
	}
	return out
}

func protoToItems(items []*dbv1.Item) []domain.Item {
	if len(items) == 0 {
		return nil
	}
	out := make([]domain.Item, 0, len(items))
	for _, it := range items {
		out = append(out, domain.Item{
			Slot:      int(it.GetSlot()),
			Index:     int16(it.GetIndex()),
			Eff1:      uint8(it.GetEff1()),
			EffV1:     uint8(it.GetEffv1()),
			Eff2:      uint8(it.GetEff2()),
			EffV2:     uint8(it.GetEffv2()),
			Eff3:      uint8(it.GetEff3()),
			EffV3:     uint8(it.GetEffv3()),
			ExpiresAt: it.GetExpiresAt(),
		})
	}
	return out
}

func affectsToProto(affects []domain.Affect) []*dbv1.Affect {
	if len(affects) == 0 {
		return nil
	}
	out := make([]*dbv1.Affect, 0, len(affects))
	for _, a := range affects {
		out = append(out, &dbv1.Affect{
			Type:  int32(a.Type),
			Value: int32(a.Value),
			Level: int32(a.Level),
			Time:  a.Time,
		})
	}
	return out
}

func protoToAffects(affects []*dbv1.Affect) []domain.Affect {
	if len(affects) == 0 {
		return nil
	}
	out := make([]domain.Affect, 0, len(affects))
	for _, a := range affects {
		out = append(out, domain.Affect{
			Type:  uint8(a.GetType()),
			Value: uint8(a.GetValue()),
			Level: uint16(a.GetLevel()),
			Time:  a.GetTime(),
		})
	}
	return out
}
