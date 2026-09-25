package savefmt

import (
	"encoding/binary"
	"testing"
)

func TestNightmareOffsets(t *testing.T) {
	var extra MobExtra
	for _, entries := range []int32{0, 13, 2147483647, -1} {
		binary.LittleEndian.PutUint64(extra.Raw[432:440], 0x1122334455667788)
		binary.LittleEndian.PutUint64(extra.Raw[440:448], 5000000000)
		binary.LittleEndian.PutUint32(extra.Raw[448:452], uint32(entries))
		binary.LittleEndian.PutUint32(extra.Raw[452:456], 137)
		if extra.NightmareEntries() != entries || extra.LastNightmareUse() != 5000000000 || extra.KefraTicket() != 137 {
			t.Fatal("Nightmare fields overlap or truncate time_t")
		}
	}
}
