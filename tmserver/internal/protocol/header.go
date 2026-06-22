// Package protocol implements the CPSock wire codec used between the
// unmodified WYD.exe 7662 client and the game server (tmServer).
//
// Everything here follows protocol-spec.md (Phase 1). All wire layout is read
// and written by explicit offset, little-endian, via encoding/binary — never by
// relying on Go struct layout (NF2/NF8, migration-plan.md §5). The four-stage
// pipeline is: INITCODE gate → framing by Size → keyword transform → checksum.
package protocol

import (
	"encoding/binary"
	"fmt"
)

// Transport constants (protocol-spec.md §4.5; Source/Code/CPSock.h, Basedef.h).
const (
	// HeaderSize is sizeof(HEADER) — a fixed 12-byte prefix on every packet
	// except the handshake and the billing link (protocol-spec.md §1.1).
	HeaderSize = 12

	// InitCode is the 4-byte handshake every new TCP connection must send first
	// (CPSock.h:40, §1.2). Little-endian on the wire: 11 F3 11 1F.
	InitCode uint32 = 0x1F11F311

	// MaxMessageSize is the maximum accepted packet Size (CPSock.h:38, §1.3).
	MaxMessageSize = 8192

	// AppVersion is the protocol version the client trafficks in
	// MSG_AccountLogin.ClientVersion (Basedef.h:102, §4.1). It is 7640, NOT 7662
	// (7662 is the executable/patch build name).
	AppVersion = 7640

	// SkipCheckTick is an internal tick value the client must never send; the
	// dispatcher drops client packets carrying it (Basedef.h:232, §2).
	SkipCheckTick uint32 = 235543242

	// MaxUser is the per-channel connection/player slot count; HEADER.ID must be
	// in [0, MaxUser) (Basedef.h:116, §2).
	MaxUser = 1000

	// transformStart is the offset at which obfuscation and checksum begin; the
	// first four bytes (Size, KeyWord, CheckSum) travel in clear (§1.1, §1.4).
	transformStart = 4
)

// Header is the 12-byte CPSock packet header (protocol-spec.md §1.1,
// CPSock.h:42-50). Little-endian.
//
//	Offset  Field        Type     Bytes
//	0       Size         uint16   2   total packet size incl. header
//	2       KeyWord      uint8    1   random index (0..255) into keyWord seeds
//	3       CheckSum     uint8    1   additive checksum of the payload (§1.5)
//	4       Type         uint16   2   message id incl. direction flags (§2)
//	6       ID           uint16   2   connection/player slot in pUser[]
//	8       ClientTick   uint32   4   timestamp/tick
type Header struct {
	Size       uint16
	KeyWord    uint8
	CheckSum   uint8
	Type       Type
	ID         uint16
	ClientTick uint32
}

// EncodeHeader writes h into the first HeaderSize bytes of dst by explicit
// offset. dst must be at least HeaderSize bytes long.
func EncodeHeader(dst []byte, h Header) error {
	if len(dst) < HeaderSize {
		return fmt.Errorf("protocol: EncodeHeader: dst too small: have %d, need %d", len(dst), HeaderSize)
	}
	binary.LittleEndian.PutUint16(dst[0:2], h.Size)
	dst[2] = h.KeyWord
	dst[3] = h.CheckSum
	binary.LittleEndian.PutUint16(dst[4:6], uint16(h.Type))
	binary.LittleEndian.PutUint16(dst[6:8], h.ID)
	binary.LittleEndian.PutUint32(dst[8:12], h.ClientTick)
	return nil
}

// DecodeHeader reads a Header from the first HeaderSize bytes of src by explicit
// offset. src must be at least HeaderSize bytes long.
func DecodeHeader(src []byte) (Header, error) {
	if len(src) < HeaderSize {
		return Header{}, fmt.Errorf("protocol: DecodeHeader: src too small: have %d, need %d", len(src), HeaderSize)
	}
	return Header{
		Size:       binary.LittleEndian.Uint16(src[0:2]),
		KeyWord:    src[2],
		CheckSum:   src[3],
		Type:       Type(binary.LittleEndian.Uint16(src[4:6])),
		ID:         binary.LittleEndian.Uint16(src[6:8]),
		ClientTick: binary.LittleEndian.Uint32(src[8:12]),
	}, nil
}
