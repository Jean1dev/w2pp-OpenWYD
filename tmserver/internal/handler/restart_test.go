package handler

import (
	"encoding/binary"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// TestRestartRevives covers the death-respawn / recall flow: on _MSG_Restart the
// server revives the character at 2 HP and refreshes the client (the HP rides on
// _MSG_UpdateScore, whose Score.Hp@24 must read 2).
func TestRestartRevives(t *testing.T) {
	addr, stop, _ := startServerClock(t, combatDB()) // logs in with HP 1000
	defer stop()

	c := enterWorld(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgRestart, nil)

	for i := 0; i < 10; i++ {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			t.Fatal("no response to _MSG_Restart")
		}
		if ty != protocol.MsgUpdateScore {
			continue // skip the recall jump / etc noise
		}
		if hp := binary.LittleEndian.Uint32(payload[24:28]); hp != 2 {
			t.Fatalf("revived HP = %d, want 2", hp)
		}
		return
	}
	t.Fatal("never received UpdateScore after restart")
}
