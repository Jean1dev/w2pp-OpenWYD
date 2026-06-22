package protocol

// The keyword transform obfuscates the payload (bytes [4:Size)); the header
// bytes 0..3 always travel in clear (protocol-spec.md §1.4,
// Source/Code/CPSock.cpp:428-453 decode / :556-581 encode).
//
// Parity notes (must match byte-for-byte):
//   - All arithmetic is uint8 with wrap mod 256. In C the shifted Trans is
//     promoted to int, then truncated to char on store; computing the whole
//     expression in uint8 reproduces that truncation exactly.
//   - pos seeds from keyWord[iKeyWord*2] and increments by 1 per byte. Using a
//     uint8 pos makes "pos % 256" implicit (rst == pos for a uint8).
//   - The shift case is selected by the byte's absolute index i & 0x3, so the
//     loop must run over absolute packet offsets starting at transformStart.

// decodePayload deobfuscates buf[transformStart:] in place, where iKeyWord is
// HEADER.KeyWord. It returns the two checksum accumulators: sum1 over the plain
// bytes (after transform) and sum2 over the ciphered bytes (before transform).
// See checksum.go for how they combine.
func decodePayload(buf []byte, iKeyWord uint8) (sum1, sum2 uint8) {
	pos := keyWord[uint16(iKeyWord)*2]
	for i := transformStart; i < len(buf); i++ {
		rst := pos
		cipher := buf[i]
		sum2 += cipher
		trans := keyWord[uint16(rst)*2+1]
		switch i & 0x3 {
		case 0:
			buf[i] = cipher - (trans << 1)
		case 1:
			buf[i] = cipher + (trans >> 3)
		case 2:
			buf[i] = cipher - (trans << 2)
		case 3:
			buf[i] = cipher + (trans >> 5)
		}
		sum1 += buf[i]
		pos++
	}
	return sum1, sum2
}

// encodePayload obfuscates buf[transformStart:] in place — the exact inverse of
// decodePayload — using iKeyWord as the seed. It returns sum1 over the plain
// bytes (before transform) and sum2 over the ciphered bytes (after transform).
func encodePayload(buf []byte, iKeyWord uint8) (sum1, sum2 uint8) {
	pos := keyWord[uint16(iKeyWord)*2]
	for i := transformStart; i < len(buf); i++ {
		plain := buf[i]
		sum1 += plain
		rst := pos
		trans := keyWord[uint16(rst)*2+1]
		switch i & 0x3 {
		case 0:
			buf[i] = plain + (trans << 1)
		case 1:
			buf[i] = plain - (trans >> 3)
		case 2:
			buf[i] = plain + (trans << 2)
		case 3:
			buf[i] = plain - (trans >> 5)
		}
		sum2 += buf[i]
		pos++
	}
	return sum1, sum2
}
