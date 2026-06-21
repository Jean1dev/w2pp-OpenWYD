package protocol

import (
	"encoding/binary"
	"fmt"
)

// This file holds explicit, offset-by-offset codecs for the critical message
// bodies from protocol-spec.md §3.5. Bodies are the bytes AFTER the 12-byte
// header (Frame.Payload), so the offsets below are doc-offset minus HeaderSize.
//
// Network messages are pack(1) (no padding) — unlike the save structs of Phase 2
// (natural alignment). Every field is read/written by explicit offset,
// little-endian (general-rule 4). Variable-length bodies (Attack Dam[13], Trade)
// and the billing _AUTH_GAME (196 B, UNVERIFIED) are deferred — see the bottom
// of this file.

// le is the wire byte order for all CPSock messages (little-endian, x86).
var le = binary.LittleEndian

// MsgAccountLoginBody is the body of MSG_AccountLogin (C→S, Type 0x020D),
// protocol-spec.md §3.5 / Basedef.h:1545-1561, pack(1). Total packet 116 bytes.
//
// AccountPassword is plaintext on the wire — a security debt hashed at import
// (Phase 2/7). ClientVersion must equal AppVersion (7640); that check is handler
// logic (Phase 4), not codec.
type MsgAccountLoginBody struct {
	AccountPassword [12]byte
	AccountName     [16]byte
	Zero            [52]byte
	ClientVersion   int32
	DBNeedSave      int32
	AdapterName     [4]int32
}

// MsgAccountLoginBodySize is the body length; the full packet is HeaderSize+this.
const MsgAccountLoginBodySize = 104

// Decode parses an MSG_AccountLogin body from b (length MsgAccountLoginBodySize).
func (m *MsgAccountLoginBody) Decode(b []byte) error {
	if len(b) < MsgAccountLoginBodySize {
		return fmt.Errorf("protocol: MsgAccountLoginBody.Decode: have %d, need %d", len(b), MsgAccountLoginBodySize)
	}
	copy(m.AccountPassword[:], b[0:12])
	copy(m.AccountName[:], b[12:28])
	copy(m.Zero[:], b[28:80])
	m.ClientVersion = int32(le.Uint32(b[80:84]))
	m.DBNeedSave = int32(le.Uint32(b[84:88]))
	for i := 0; i < 4; i++ {
		m.AdapterName[i] = int32(le.Uint32(b[88+i*4 : 92+i*4]))
	}
	return nil
}

// Encode writes the MSG_AccountLogin body into a new MsgAccountLoginBodySize slice.
func (m *MsgAccountLoginBody) Encode() []byte {
	b := make([]byte, MsgAccountLoginBodySize)
	copy(b[0:12], m.AccountPassword[:])
	copy(b[12:28], m.AccountName[:])
	copy(b[28:80], m.Zero[:])
	le.PutUint32(b[80:84], uint32(m.ClientVersion))
	le.PutUint32(b[84:88], uint32(m.DBNeedSave))
	for i := 0; i < 4; i++ {
		le.PutUint32(b[88+i*4:92+i*4], uint32(m.AdapterName[i]))
	}
	return b
}

// MsgCreateCharacterBody — MSG_CreateCharacter (C→S, 0x020F), Basedef.h:1598-1605.
// Total packet 36 bytes.
type MsgCreateCharacterBody struct {
	Slot     int32
	MobName  [16]byte
	MobClass int32
}

// MsgCreateCharacterBodySize is the body length.
const MsgCreateCharacterBodySize = 24

// Decode parses an MSG_CreateCharacter body from b.
func (m *MsgCreateCharacterBody) Decode(b []byte) error {
	if len(b) < MsgCreateCharacterBodySize {
		return fmt.Errorf("protocol: MsgCreateCharacterBody.Decode: have %d, need %d", len(b), MsgCreateCharacterBodySize)
	}
	m.Slot = int32(le.Uint32(b[0:4]))
	copy(m.MobName[:], b[4:20])
	m.MobClass = int32(le.Uint32(b[20:24]))
	return nil
}

// Encode writes the MSG_CreateCharacter body into a new slice.
func (m *MsgCreateCharacterBody) Encode() []byte {
	b := make([]byte, MsgCreateCharacterBodySize)
	le.PutUint32(b[0:4], uint32(m.Slot))
	copy(b[4:20], m.MobName[:])
	le.PutUint32(b[20:24], uint32(m.MobClass))
	return b
}

// MsgDeleteCharacterBody — MSG_DeleteCharacter (C→S, 0x0211), Basedef.h:1608-1616.
// Total packet 44 bytes. Password confirms the deletion.
type MsgDeleteCharacterBody struct {
	Slot     int32
	MobName  [16]byte
	Password [12]byte
}

// MsgDeleteCharacterBodySize is the body length.
const MsgDeleteCharacterBodySize = 32

// Decode parses an MSG_DeleteCharacter body from b.
func (m *MsgDeleteCharacterBody) Decode(b []byte) error {
	if len(b) < MsgDeleteCharacterBodySize {
		return fmt.Errorf("protocol: MsgDeleteCharacterBody.Decode: have %d, need %d", len(b), MsgDeleteCharacterBodySize)
	}
	m.Slot = int32(le.Uint32(b[0:4]))
	copy(m.MobName[:], b[4:20])
	copy(m.Password[:], b[20:32])
	return nil
}

// Encode writes the MSG_DeleteCharacter body into a new slice.
func (m *MsgDeleteCharacterBody) Encode() []byte {
	b := make([]byte, MsgDeleteCharacterBodySize)
	le.PutUint32(b[0:4], uint32(m.Slot))
	copy(b[4:20], m.MobName[:])
	copy(b[20:32], m.Password[:])
	return b
}

// MsgCharacterLoginBody — MSG_CharacterLogin (C→S, 0x0213), Basedef.h:1674-1680.
// Total packet 20 bytes.
type MsgCharacterLoginBody struct {
	Slot  int32
	Force int32
}

// MsgCharacterLoginBodySize is the body length.
const MsgCharacterLoginBodySize = 8

// Decode parses an MSG_CharacterLogin body from b.
func (m *MsgCharacterLoginBody) Decode(b []byte) error {
	if len(b) < MsgCharacterLoginBodySize {
		return fmt.Errorf("protocol: MsgCharacterLoginBody.Decode: have %d, need %d", len(b), MsgCharacterLoginBodySize)
	}
	m.Slot = int32(le.Uint32(b[0:4]))
	m.Force = int32(le.Uint32(b[4:8]))
	return nil
}

// Encode writes the MSG_CharacterLogin body into a new slice.
func (m *MsgCharacterLoginBody) Encode() []byte {
	b := make([]byte, MsgCharacterLoginBodySize)
	le.PutUint32(b[0:4], uint32(m.Slot))
	le.PutUint32(b[4:8], uint32(m.Force))
	return b
}

// MsgActionBody — MSG_Action (movement, C↔S, 0x036C/0x0366/0x0368),
// Basedef.h:2070-2082. Total packet 52 bytes. Effect: 0=walking, 1=teleporting.
type MsgActionBody struct {
	PosX    int16
	PosY    int16
	Effect  int32
	Speed   int32
	Route   [24]byte
	TargetX int16
	TargetY int16
}

// MsgActionBodySize is the body length.
const MsgActionBodySize = 40

// Decode parses an MSG_Action body from b.
func (m *MsgActionBody) Decode(b []byte) error {
	if len(b) < MsgActionBodySize {
		return fmt.Errorf("protocol: MsgActionBody.Decode: have %d, need %d", len(b), MsgActionBodySize)
	}
	m.PosX = int16(le.Uint16(b[0:2]))
	m.PosY = int16(le.Uint16(b[2:4]))
	m.Effect = int32(le.Uint32(b[4:8]))
	m.Speed = int32(le.Uint32(b[8:12]))
	copy(m.Route[:], b[12:36])
	m.TargetX = int16(le.Uint16(b[36:38]))
	m.TargetY = int16(le.Uint16(b[38:40]))
	return nil
}

// Encode writes the MSG_Action body into a new slice.
func (m *MsgActionBody) Encode() []byte {
	b := make([]byte, MsgActionBodySize)
	le.PutUint16(b[0:2], uint16(m.PosX))
	le.PutUint16(b[2:4], uint16(m.PosY))
	le.PutUint32(b[4:8], uint32(m.Effect))
	le.PutUint32(b[8:12], uint32(m.Speed))
	copy(b[12:36], m.Route[:])
	le.PutUint16(b[36:38], uint16(m.TargetX))
	le.PutUint16(b[38:40], uint16(m.TargetY))
	return b
}

// AuthGameSize is sizeof(_AUTH_GAME), the fixed 196-byte TMSrv↔BISrv billing
// packet (CPSock.h:132, protocol-spec.md §1.6).
//
// UNVERIFIED: the internal field layout of _AUTH_GAME is unconfirmed — the live
// struct is char Unk[196] and the "FROM TANTRA" reference layout has not been
// matched to this server (protocol-spec.md §4.3, README pendência). It must be
// captured from real TMSrv↔BISrv traffic before being decoded; implemented in
// Phase 6. See the skipped test in messages_test.go.
const AuthGameSize = 196

// cTrimNUL returns the prefix of b up to the first NUL — a helper for turning
// fixed-size, NUL-padded char arrays (names, passwords) into Go strings at the
// handler boundary (Phase 4).
func cTrimNUL(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
