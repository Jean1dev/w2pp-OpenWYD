package protocol

// checksum returns the CPSock 1-byte additive checksum, CheckSum = Sum2 - Sum1
// (uint8 wrap), where sum1 is the sum of the plain payload bytes and sum2 the
// sum of the ciphered payload bytes (protocol-spec.md §1.5,
// Source/Code/CPSock.cpp:583-584).
func checksum(sum1, sum2 uint8) uint8 {
	return sum2 - sum1
}

// The original server and client both DO NOT reject on checksum mismatch: the
// server returns the packet with ErrorCode=1 anyway (CPSock.cpp:457-466), and
// the ClientPatch NOPs the client-side checks (Hook.cpp:211-214). For wire
// compatibility we MUST emit a correct checksum on send (done in codec.go), and
// on receive we compute and report a mismatch but, at this codec layer, do not
// reject — the reject-on-mismatch hardening is a Phase 7 decision
// (protocol-spec.md §1.5, migration-plan.md §5).
