package savefmt

import (
	"fmt"
	"math"
)

// Fixed-width little-endian field helpers. Offsets are relative to the start of
// the slice passed in (callers pass the struct's base slice).
func getU16(b []byte, off int) uint16    { return le.Uint16(b[off:]) }
func getU32(b []byte, off int) uint32    { return le.Uint32(b[off:]) }
func getI16(b []byte, off int) int16     { return int16(le.Uint16(b[off:])) }
func getI32(b []byte, off int) int32     { return int32(le.Uint32(b[off:])) }
func getI64(b []byte, off int) int64     { return int64(le.Uint64(b[off:])) }
func putU16(b []byte, off int, v uint16) { le.PutUint16(b[off:], v) }
func putU32(b []byte, off int, v uint32) { le.PutUint32(b[off:], v) }
func putI16(b []byte, off int, v int16)  { le.PutUint16(b[off:], uint16(v)) }
func putI32(b []byte, off int, v int32)  { le.PutUint32(b[off:], uint32(v)) }
func putI64(b []byte, off int, v int64)  { le.PutUint64(b[off:], uint64(v)) }

func boolByte(v bool) byte {
	if v {
		return 1
	}
	return 0
}

// --- STRUCT_ITEM (8) ---

func decodeItem(b []byte) Item {
	var it Item
	it.Index = getI16(b, 0)
	for i := 0; i < 3; i++ {
		it.Effects[i] = Effect{Effect: b[2+i*2], Value: b[3+i*2]}
	}
	return it
}

func encodeItem(b []byte, it Item) {
	putI16(b, 0, it.Index)
	for i := 0; i < 3; i++ {
		b[2+i*2] = it.Effects[i].Effect
		b[3+i*2] = it.Effects[i].Value
	}
}

// --- STRUCT_SCORE (48) ---

func decodeScore(b []byte) Score {
	var s Score
	s.Level = getI32(b, 0)
	s.AC = getI32(b, 4)
	s.Damage = getI32(b, 8)
	s.Merchant = b[12]
	s.AttackRun = b[13]
	s.Direction = b[14]
	s.ChaosRate = b[15]
	s.MaxHp = getI32(b, 16)
	s.MaxMp = getI32(b, 20)
	s.Hp = getI32(b, 24)
	s.Mp = getI32(b, 28)
	s.Str = getI16(b, 32)
	s.Int = getI16(b, 34)
	s.Dex = getI16(b, 36)
	s.Con = getI16(b, 38)
	for i := 0; i < 4; i++ {
		s.Special[i] = getI16(b, 40+i*2)
	}
	return s
}

func encodeScore(b []byte, s Score) {
	putI32(b, 0, s.Level)
	putI32(b, 4, s.AC)
	putI32(b, 8, s.Damage)
	b[12], b[13], b[14], b[15] = s.Merchant, s.AttackRun, s.Direction, s.ChaosRate
	putI32(b, 16, s.MaxHp)
	putI32(b, 20, s.MaxMp)
	putI32(b, 24, s.Hp)
	putI32(b, 28, s.Mp)
	putI16(b, 32, s.Str)
	putI16(b, 34, s.Int)
	putI16(b, 36, s.Dex)
	putI16(b, 38, s.Con)
	for i := 0; i < 4; i++ {
		putI16(b, 40+i*2, s.Special[i])
	}
}

// --- STRUCT_SCORE, legacy compact form (28) ---

// decodeScoreCompact reads the 28-byte STRUCT_SCORE of the 756-byte legacy mob
// layout (data-formats.md §1.4.1). Relative to the 48-byte form it narrows
// Level/AC/Damage to 16 bits and Special[] to 8, and drops Direction,
// ChaosRate, MaxMp and Mp entirely — all four are zero in the current-format
// npc templates too, so widening loses nothing.
func decodeScoreCompact(b []byte) Score {
	var s Score
	s.Level = int32(getU16(b, 0))
	s.AC = int32(getU16(b, 2))
	s.Damage = int32(getU16(b, 4))
	s.Merchant = b[6]
	s.AttackRun = b[7]
	s.MaxHp = getI32(b, 8)
	s.Hp = getI32(b, 12)
	s.Str = getI16(b, 16)
	s.Int = getI16(b, 18)
	s.Dex = getI16(b, 20)
	s.Con = getI16(b, 22)
	for i := 0; i < 4; i++ {
		s.Special[i] = int16(b[24+i])
	}
	return s
}

// --- STRUCT_AFFECT (8) ---

func decodeAffect(b []byte) Affect {
	return Affect{Type: b[0], Value: b[1], Level: getU16(b, 2), Time: getU32(b, 4)}
}

func encodeAffect(b []byte, a Affect) {
	b[0], b[1] = a.Type, a.Value
	putU16(b, 2, a.Level)
	putU32(b, 4, a.Time)
}

// --- STRUCT_MOB (816), offsets from data-formats.md §0.1 ---

func decodeMob(b []byte) Mob {
	var m Mob
	copy(m.Name[:], b[0:16])
	m.Clan = b[16]
	m.Merchant = b[17]
	m.Guild = getU16(b, 18)
	m.Class = b[20]
	m.Rsv = getU16(b, 22)
	m.Quest = b[24]
	m.Coin = getI32(b, 28)
	m.Exp = getI64(b, 32)
	m.SPX = getI16(b, 40)
	m.SPY = getI16(b, 42)
	m.BaseScore = decodeScore(b[44:])
	m.CurrentScore = decodeScore(b[92:])
	for i := 0; i < MaxEquip; i++ {
		m.Equip[i] = decodeItem(b[140+i*ItemSize:])
	}
	for i := 0; i < MaxCarry; i++ {
		m.Carry[i] = decodeItem(b[268+i*ItemSize:])
	}
	m.LearnedSkill = getI32(b, 780)
	m.Magic = getU32(b, 784)
	m.ScoreBonus = getU16(b, 788)
	m.SpecialBonus = getU16(b, 790)
	m.SkillBonus = getU16(b, 792)
	m.Critical = b[794]
	m.SaveMana = b[795]
	copy(m.SkillBar[:], b[796:800])
	m.GuildLevel = b[800]
	m.RegenHP = getU16(b, 802)
	m.RegenMP = getU16(b, 804)
	for i := 0; i < 4; i++ {
		m.Resist[i] = int8(b[806+i])
	}
	return m
}

func encodeMob(b []byte, m Mob) {
	copy(b[0:16], m.Name[:])
	b[16] = m.Clan
	b[17] = m.Merchant
	putU16(b, 18, m.Guild)
	b[20] = m.Class
	putU16(b, 22, m.Rsv)
	b[24] = m.Quest
	putI32(b, 28, m.Coin)
	putI64(b, 32, m.Exp)
	putI16(b, 40, m.SPX)
	putI16(b, 42, m.SPY)
	encodeScore(b[44:], m.BaseScore)
	encodeScore(b[92:], m.CurrentScore)
	for i := 0; i < MaxEquip; i++ {
		encodeItem(b[140+i*ItemSize:], m.Equip[i])
	}
	for i := 0; i < MaxCarry; i++ {
		encodeItem(b[268+i*ItemSize:], m.Carry[i])
	}
	putI32(b, 780, m.LearnedSkill)
	putU32(b, 784, m.Magic)
	putU16(b, 788, m.ScoreBonus)
	putU16(b, 790, m.SpecialBonus)
	putU16(b, 792, m.SkillBonus)
	b[794] = m.Critical
	b[795] = m.SaveMana
	copy(b[796:800], m.SkillBar[:])
	b[800] = m.GuildLevel
	putU16(b, 802, m.RegenHP)
	putU16(b, 804, m.RegenMP)
	for i := 0; i < 4; i++ {
		b[806+i] = byte(m.Resist[i])
	}
}

// decodeMobLegacy756 parses the 756-byte legacy STRUCT_MOB layout into the
// current Mob shape (data-formats.md §1.4.1). Fields absent from that layout
// (Quest, Magic, and the score's Direction/ChaosRate/MaxMp/Mp) stay zero, which
// is what the 816-byte templates carry anyway. b must be at least
// MobSizeLegacy756 bytes; the caller routes by DetectMobVersion.
func decodeMobLegacy756(b []byte) Mob {
	var m Mob
	copy(m.Name[:], b[0:16])
	m.Clan = b[offL756Clan]
	m.Merchant = b[offL756Merchant]
	m.Guild = getU16(b, offL756Guild)
	m.Class = b[offL756Class]
	m.Rsv = getU16(b, offL756Rsv)
	m.Coin = getI32(b, offL756Coin)
	m.Exp = int64(getI32(b, offL756Exp))
	m.SPX = getI16(b, offL756SPX)
	m.SPY = getI16(b, offL756SPY)
	m.BaseScore = decodeScoreCompact(b[offL756BaseScore:])
	m.CurrentScore = decodeScoreCompact(b[offL756CurrentScore:])
	for i := 0; i < MaxEquip; i++ {
		m.Equip[i] = decodeItem(b[offL756Equip+i*ItemSize:])
	}
	for i := 0; i < MaxCarry; i++ {
		m.Carry[i] = decodeItem(b[offL756Carry+i*ItemSize:])
	}
	m.LearnedSkill = getI32(b, offL756LearnedSkill)
	m.ScoreBonus = getU16(b, offL756ScoreBonus)
	m.SpecialBonus = getU16(b, offL756SpecialBonus)
	m.SkillBonus = getU16(b, offL756SkillBonus)
	m.Critical = b[offL756Critical]
	m.SaveMana = b[offL756SaveMana]
	copy(m.SkillBar[:], b[offL756SkillBar:offL756SkillBar+4])
	m.GuildLevel = b[offL756GuildLevel]
	m.RegenHP = uint16(b[offL756RegenHP])
	m.RegenMP = uint16(b[offL756RegenMP])
	for i := 0; i < 4; i++ {
		m.Resist[i] = int8(b[offL756Resist+i])
	}
	return m
}

// --- STRUCT_ACCOUNTINFO (216) ---

func decodeAccountInfo(b []byte) AccountInfo {
	var a AccountInfo
	copy(a.AccountName[:], b[0:16])
	copy(a.AccountPass[:], b[16:28])
	copy(a.RealName[:], b[28:52])
	a.SSN1 = getI32(b, 52)
	a.SSN2 = getI32(b, 56)
	copy(a.Email[:], b[60:108])
	copy(a.Telephone[:], b[108:124])
	copy(a.Address[:], b[124:202])
	copy(a.NumericToken[:], b[202:208])
	a.Year = getI32(b, 208)
	a.YearDay = getI32(b, 212)
	return a
}

func encodeAccountInfo(b []byte, a AccountInfo) {
	copy(b[0:16], a.AccountName[:])
	copy(b[16:28], a.AccountPass[:])
	copy(b[28:52], a.RealName[:])
	putI32(b, 52, a.SSN1)
	putI32(b, 56, a.SSN2)
	copy(b[60:108], a.Email[:])
	copy(b[108:124], a.Telephone[:])
	copy(b[124:202], a.Address[:])
	copy(b[202:208], a.NumericToken[:])
	putI32(b, 208, a.Year)
	putI32(b, 212, a.YearDay)
}

// --- STRUCT_QUEST (56) ---

func decodeQuest(b []byte) Quest {
	var q Quest
	q.IndexQuest = getI16(b, 0)
	q.Nivel = getI16(b, 2)
	q.IDMob1 = getI16(b, 4)
	q.QtdMob1 = getI16(b, 6)
	q.IDMob2 = getI16(b, 8)
	q.QtdMob2 = getI16(b, 10)
	q.IDMob3 = getI16(b, 12)
	q.QtdMob3 = getI16(b, 14)
	q.ExpReward = getI32(b, 16)
	q.GoldReward = getI32(b, 20)
	q.Item[0] = decodeItem(b[24:])
	q.Item[1] = decodeItem(b[32:])
	q.LastTime = getI64(b, 40)
	for i := 0; i < 3; i++ {
		q.MobCount[i] = getI16(b, 48+i*2)
	}
	return q
}

func encodeQuest(b []byte, q Quest) {
	putI16(b, 0, q.IndexQuest)
	putI16(b, 2, q.Nivel)
	putI16(b, 4, q.IDMob1)
	putI16(b, 6, q.QtdMob1)
	putI16(b, 8, q.IDMob2)
	putI16(b, 10, q.QtdMob2)
	putI16(b, 12, q.IDMob3)
	putI16(b, 14, q.QtdMob3)
	putI32(b, 16, q.ExpReward)
	putI32(b, 20, q.GoldReward)
	encodeItem(b[24:], q.Item[0])
	encodeItem(b[32:], q.Item[1])
	putI64(b, 40, q.LastTime)
	for i := 0; i < 3; i++ {
		putI16(b, 48+i*2, q.MobCount[i])
	}
}

// DecodeMob parses a standalone 816-byte STRUCT_MOB blob (e.g. an NPC template in
// Release/TMsrv/run/npc/). It returns an error unless b is exactly MobSize bytes.
func DecodeMob(b []byte) (Mob, error) {
	if len(b) != MobSize {
		return Mob{}, fmt.Errorf("savefmt: DecodeMob: length %d != %d", len(b), MobSize)
	}
	return decodeMob(b), nil
}

// DecodeMobAny parses a standalone STRUCT_MOB template in any layout known to
// DetectMobVersion (mobversion.go) and reports which one it was. Use it for
// files read straight out of Release/TMsrv/run/npc/, where 756- and 920-byte
// legacy records sit alongside the canonical 816-byte ones; use DecodeMob when
// the blob is already canonical.
func DecodeMobAny(b []byte) (Mob, MobVersion, error) {
	v := DetectMobVersion(len(b))
	switch v {
	case MobVersionCurrent:
		return decodeMob(b), v, nil
	case MobVersionLegacy756, MobVersionLegacy756Padded:
		// The padded form's trailing bytes are uninitialized heap junk, not a
		// struct tail — decode the record and drop the rest.
		return decodeMobLegacy756(b[:MobSizeLegacy756]), v, nil
	default:
		return Mob{}, v, fmt.Errorf("savefmt: DecodeMobAny: length %d is not a known STRUCT_MOB layout (%d, %d or %d)",
			len(b), MobSize, MobSizeLegacy756, MobSizeLegacy756Padded)
	}
}

// NormalizeMob converts a template of any known layout into a canonical
// MobSize blob, reporting the layout it came from. Loaders call this at the
// content boundary so no variant-sized slice ever reaches the runtime — several
// consumers (protocol.ParseMobBasics, EncodeCNFCharacterLoginRaw) index the
// 816-byte layout directly. The 756→816 conversion only widens fields, so it is
// lossless; the reverse is never needed because nothing rewrites templates on
// the read path.
func NormalizeMob(b []byte) ([]byte, MobVersion, error) {
	v := DetectMobVersion(len(b))
	if v == MobVersionCurrent {
		out := make([]byte, MobSize)
		copy(out, b)
		return out, v, nil
	}
	m, v, err := DecodeMobAny(b)
	if err != nil {
		return nil, v, err
	}
	return EncodeMob(m), v, nil
}

// PutMobExp overwrites the kill-reward Exp of a standalone STRUCT_MOB template
// in place, in whichever layout b already is, leaving every other byte — and the
// slice length — untouched. cmd/exptool restamps the balanced curve (issue #43)
// through it, so the mixed-layout content tree gets one consistent pass without
// any template changing size on disk.
//
// The legacy layouts keep Exp as a 32-bit field at a different offset
// (data-formats.md §1.4.1), so a value that does not fit there is an error
// rather than a silent truncation — level.MobExpForLevel peaks around 5.5M, far
// inside the range, so this only fires on a caller passing something bogus.
func PutMobExp(b []byte, exp int64) error {
	switch v := DetectMobVersion(len(b)); v {
	case MobVersionCurrent:
		putI64(b, offMobExp, exp)
		return nil
	case MobVersionLegacy756, MobVersionLegacy756Padded:
		if exp < math.MinInt32 || exp > math.MaxInt32 {
			return fmt.Errorf("savefmt: PutMobExp: %d does not fit the %s layout's 32-bit Exp", exp, v)
		}
		putI32(b, offL756Exp, int32(exp))
		return nil
	default:
		return fmt.Errorf("savefmt: PutMobExp: length %d is not a known STRUCT_MOB layout (%d, %d or %d)",
			len(b), MobSize, MobSizeLegacy756, MobSizeLegacy756Padded)
	}
}

// EncodeMob serializes m into a standalone 816-byte STRUCT_MOB blob — the
// counterpart to DecodeMob, used to write a moderator-edited NPC/mob template
// back to raw bytes (e.g. tmserver's template-stat overlay).
func EncodeMob(m Mob) []byte {
	b := make([]byte, MobSize)
	encodeMob(b, m)
	return b
}

// --- STRUCT_ACCOUNTFILE (7952) ---

// Decode parses a current-format (7952-byte) account blob. It returns an error
// if b is not exactly AccountFileSize bytes; legacy formats must be routed by
// DetectVersion first (version.go).
func Decode(b []byte) (AccountFile, error) {
	if len(b) != AccountFileSize {
		return AccountFile{}, fmt.Errorf("savefmt: Decode: length %d != %d (current format)", len(b), AccountFileSize)
	}
	var af AccountFile
	af.Info = decodeAccountInfo(b[offInfo:])
	for i := 0; i < MobPerAccount; i++ {
		af.Char[i] = decodeMob(b[offChar+i*MobSize:])
	}
	for i := 0; i < MaxCargo; i++ {
		af.Cargo[i] = decodeItem(b[offCargo+i*ItemSize:])
	}
	af.Coin = getI32(b, offCoin)
	for i := 0; i < ShortSkillBars; i++ {
		copy(af.ShortSkill[i][:], b[offShortSkill+i*ShortSkillSlot:offShortSkill+(i+1)*ShortSkillSlot])
	}
	for c := 0; c < MobPerAccount; c++ {
		for a := 0; a < MaxAffect; a++ {
			off := offAffect + (c*MaxAffect+a)*AffectSize
			af.Affect[c][a] = decodeAffect(b[off:])
		}
	}
	for i := 0; i < MobPerAccount; i++ {
		copy(af.MobExtra[i].Raw[:], b[offMobExtra+i*MobExtraSize:offMobExtra+(i+1)*MobExtraSize])
	}
	af.Donate = getI32(b, offDonate)
	copy(af.TempKey[:], b[offTempKey:offTempKey+52])
	af.ReceivedItem = b[offReceived] != 0
	af.QuestDaily = decodeQuest(b[offQuestDaily:])
	copy(af.BlockPass[:], b[offBlockPass:offBlockPass+16])
	af.IsBlocked = b[offIsBlocked] != 0
	return af, nil
}

// Encode serializes af back to a 7952-byte current-format blob. Unmodeled
// padding bytes are written as zero, so Decode∘Encode is the identity for blobs
// produced by Encode (round-trip parity — DoD "dump round-trip confere").
func Encode(af AccountFile) []byte {
	b := make([]byte, AccountFileSize)
	encodeAccountInfo(b[offInfo:], af.Info)
	for i := 0; i < MobPerAccount; i++ {
		encodeMob(b[offChar+i*MobSize:], af.Char[i])
	}
	for i := 0; i < MaxCargo; i++ {
		encodeItem(b[offCargo+i*ItemSize:], af.Cargo[i])
	}
	putI32(b, offCoin, af.Coin)
	for i := 0; i < ShortSkillBars; i++ {
		copy(b[offShortSkill+i*ShortSkillSlot:offShortSkill+(i+1)*ShortSkillSlot], af.ShortSkill[i][:])
	}
	for c := 0; c < MobPerAccount; c++ {
		for a := 0; a < MaxAffect; a++ {
			off := offAffect + (c*MaxAffect+a)*AffectSize
			encodeAffect(b[off:], af.Affect[c][a])
		}
	}
	for i := 0; i < MobPerAccount; i++ {
		copy(b[offMobExtra+i*MobExtraSize:offMobExtra+(i+1)*MobExtraSize], af.MobExtra[i].Raw[:])
	}
	putI32(b, offDonate, af.Donate)
	copy(b[offTempKey:offTempKey+52], af.TempKey[:])
	b[offReceived] = boolByte(af.ReceivedItem)
	encodeQuest(b[offQuestDaily:], af.QuestDaily)
	copy(b[offBlockPass:offBlockPass+16], af.BlockPass[:])
	b[offIsBlocked] = boolByte(af.IsBlocked)
	return b
}
