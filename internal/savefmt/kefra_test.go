package savefmt

import (
	"encoding/binary"
	"testing"
)

func TestKefraTicketOffset(t *testing.T) {
	for _, value := range []int32{0, 137, 2147483647, -1} {
		var extra MobExtra
		// Distinct adjacent fields catch alignment/width regressions.
		binary.LittleEndian.PutUint32(extra.Raw[448:452], 0x11223344)
		binary.LittleEndian.PutUint32(extra.Raw[452:456], uint32(value))
		binary.LittleEndian.PutUint64(extra.Raw[456:464], 0x5566778899aabbcc)
		if got := extra.KefraTicket(); got != value {
			t.Errorf("KefraTicket=%d, want %d", got, value)
		}
	}
}
