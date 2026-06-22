package grpcsrv

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/domain"
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
		Slot:    int32(ch.Slot),
		Name:    ch.Name,
		Class:   int32(ch.Class),
		Clan:    int32(ch.Clan),
		GuildId: uint32(ch.GuildID),
		Level:   ch.Level,
		Exp:     ch.Exp,
		Coin:    ch.Coin,
		Str:     int32(ch.Str),
		Int:     int32(ch.Int),
		Dex:     int32(ch.Dex),
		Con:     int32(ch.Con),
		MaxHp:   ch.MaxHp,
		MaxMp:   ch.MaxMp,
		Hp:      ch.Hp,
		Mp:      ch.Mp,
		Equip:   itemsToProto(ch.Equip),
		Carry:   itemsToProto(ch.Carry),
		Affects: affectsToProto(ch.Affects),
	}
}

// protoToCharacter maps the wire contract back to the relational character.
func protoToCharacter(c *dbv1.Character) domain.Character {
	if c == nil {
		return domain.Character{}
	}
	return domain.Character{
		Slot:    int(c.GetSlot()),
		Name:    c.GetName(),
		Class:   uint8(c.GetClass()),
		Clan:    uint8(c.GetClan()),
		GuildID: uint16(c.GetGuildId()),
		Level:   c.GetLevel(),
		Exp:     c.GetExp(),
		Coin:    c.GetCoin(),
		Str:     int16(c.GetStr()),
		Int:     int16(c.GetInt()),
		Dex:     int16(c.GetDex()),
		Con:     int16(c.GetCon()),
		MaxHp:   c.GetMaxHp(),
		MaxMp:   c.GetMaxMp(),
		Hp:      c.GetHp(),
		Mp:      c.GetMp(),
		Equip:   protoToItems(c.GetEquip()),
		Carry:   protoToItems(c.GetCarry()),
		Affects: protoToAffects(c.GetAffects()),
	}
}

func itemsToProto(items []domain.Item) []*dbv1.Item {
	if len(items) == 0 {
		return nil
	}
	out := make([]*dbv1.Item, 0, len(items))
	for _, it := range items {
		out = append(out, &dbv1.Item{
			Slot:  int32(it.Slot),
			Index: int32(it.Index),
			Eff1:  int32(it.Eff1),
			Effv1: int32(it.EffV1),
			Eff2:  int32(it.Eff2),
			Effv2: int32(it.EffV2),
			Eff3:  int32(it.Eff3),
			Effv3: int32(it.EffV3),
		})
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
			Slot:  int(it.GetSlot()),
			Index: int16(it.GetIndex()),
			Eff1:  uint8(it.GetEff1()),
			EffV1: uint8(it.GetEffv1()),
			Eff2:  uint8(it.GetEff2()),
			EffV2: uint8(it.GetEffv2()),
			Eff3:  uint8(it.GetEff3()),
			EffV3: uint8(it.GetEffv3()),
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
