package protocol

import "fmt"

// Frame is a fully decoded CPSock packet: its plain header and the plain message
// body (bytes [HeaderSize:Size)).
type Frame struct {
	Header  Header
	Payload []byte
}

// Encode builds the complete obfuscated wire bytes for a packet whose plain body
// is payload. h supplies Type, ID and ClientTick; Size, KeyWord and CheckSum are
// computed here. iKeyWord seeds the keyword transform — it is injectable so
// tests are deterministic; in production it is rand()%256 (CPSock.cpp:535).
//
// The obfuscation/checksum cover bytes [4:Size), i.e. Type, ID, ClientTick AND
// the body; only Size, KeyWord and CheckSum (bytes 0..3) stay in clear
// (protocol-spec.md §1.1, §1.4).
func Encode(h Header, payload []byte, iKeyWord uint8) ([]byte, error) {
	size := HeaderSize + len(payload)
	if size < HeaderSize || size > MaxMessageSize {
		return nil, fmt.Errorf("protocol: Encode: invalid size %d (must be %d..%d)", size, HeaderSize, MaxMessageSize)
	}

	buf := make([]byte, size)
	// Lay down the full plain header (CheckSum filled after the transform).
	h.Size = uint16(size)
	h.KeyWord = iKeyWord
	h.CheckSum = 0
	if err := EncodeHeader(buf, h); err != nil {
		return nil, fmt.Errorf("protocol: Encode: %w", err)
	}
	copy(buf[HeaderSize:], payload)

	sum1, sum2 := encodePayload(buf, iKeyWord)
	buf[3] = checksum(sum1, sum2)
	return buf, nil
}

// Decode parses one complete wire frame (exactly Size bytes, as produced by the
// framer) into its plain header and body. mismatch reports a checksum mismatch;
// per the legacy semantics this layer does NOT reject on mismatch (§1.5) — the
// caller decides. Decode does not modify wire.
func Decode(wire []byte) (h Header, payload []byte, mismatch bool, err error) {
	if len(wire) < HeaderSize {
		return Header{}, nil, false, fmt.Errorf("protocol: Decode: frame too small: %d < %d", len(wire), HeaderSize)
	}
	size := int(uint16(wire[0]) | uint16(wire[1])<<8)
	if size != len(wire) {
		return Header{}, nil, false, fmt.Errorf("protocol: Decode: frame length %d != header Size %d", len(wire), size)
	}

	buf := make([]byte, len(wire))
	copy(buf, wire)

	iKeyWord := buf[2]
	wantSum := buf[3]
	sum1, sum2 := decodePayload(buf, iKeyWord)
	mismatch = checksum(sum1, sum2) != wantSum

	h, err = DecodeHeader(buf)
	if err != nil {
		return Header{}, nil, false, fmt.Errorf("protocol: Decode: %w", err)
	}
	payload = buf[HeaderSize:]
	return h, payload, mismatch, nil
}
