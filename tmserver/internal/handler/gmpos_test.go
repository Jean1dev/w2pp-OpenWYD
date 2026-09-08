package handler

import (
	"net"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// The test world is a 16×16 grid (startServerClock), so 0..15 is on the map and
// anything from 16 up is off it.
const testGridDim = 16

// gmPosDest runs "/gm pos <args>" and returns the tile the jump frame names. The
// destination is read off the wire rather than out of world state because the
// frame is what the client actually acts on.
func gmPosDest(t *testing.T, c net.Conn, args string) (int16, int16, bool) {
	t.Helper()
	gmFrame(t, c, "pos "+args)
	for {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			return 0, 0, false
		}
		if ty != protocol.MsgAction {
			continue // the panel line and any view churn are not the answer
		}
		var body protocol.MsgActionBody
		if err := body.Decode(payload); err != nil {
			t.Fatalf("MsgAction body did not decode: %v", err)
		}
		return body.TargetX, body.TargetY, true
	}
}

// The whole point of the command: a tile nobody is standing on and no route
// reaches. /gm ir needs a player at the destination; this one needs only the
// coordinates.
func TestGMPosTeleportsToBareCoordinates(t *testing.T) {
	addr, stop, _ := startServerClock(t, gmDB())
	defer stop()
	mod := enterWorldAs(t, addr, "mod")
	defer mod.Close()

	x, y, ok := gmPosDest(t, mod, "12 13")
	if !ok {
		t.Fatal("no jump frame: the command did nothing")
	}
	if x != 12 || y != 13 {
		t.Errorf("landed on (%d,%d), want (12,13)", x, y)
	}
}

// DELIBERATE: the command must not second-guess the destination. A GM asks for a
// tile precisely because they cannot walk there — a shut dungeon, a war zone, a
// room with no route. Here the destination already holds another player, the
// cheapest "occupied tile" a test can build, and the teleport still has to happen.
func TestGMPosDoesNotRefuseAnOccupiedTile(t *testing.T) {
	addr, stop, _ := startServerClock(t, gmDB())
	defer stop()
	mod := enterWorldAs(t, addr, "mod")
	defer mod.Close()
	victim := enterWorldAs(t, addr, "victim")
	defer victim.Close()

	// gmDB seeds everyone at (5,5), so step off it first — otherwise the teleport
	// back would be a no-op and the test would pass without proving anything.
	if _, _, ok := gmPosDest(t, mod, "12 13"); !ok {
		t.Fatal("could not step off the shared tile")
	}
	x, y, ok := gmPosDest(t, mod, "5 5") // straight onto the victim
	if !ok {
		t.Fatal("an occupied tile was refused; the command exists to reach places like this")
	}
	if x != 5 || y != 5 {
		t.Errorf("landed on (%d,%d), want (5,5)", x, y)
	}
}

// The one bound that IS enforced. Grid.SetMob drops an out-of-bounds write
// silently (grid.go:44), so letting this through would leave the character on
// coordinates no cell answers for.
func TestGMPosRefusesOffMapTiles(t *testing.T) {
	addr, stop, _ := startServerClock(t, gmDB())
	defer stop()
	mod := enterWorldAs(t, addr, "mod")
	defer mod.Close()

	for _, args := range []string{"16 5", "5 16", "-1 5", "5 -1", "9999 9999"} {
		t.Run(args, func(t *testing.T) {
			if x, y, ok := gmPosDest(t, mod, args); ok {
				t.Errorf("%q teleported to (%d,%d); the grid is 0..%d", args, x, y, testGridDim-1)
			}
		})
	}
}

// The last tile on the map is ON the map: an off-by-one here would make the
// bound check refuse a legal destination.
func TestGMPosAcceptsTheLastTile(t *testing.T) {
	addr, stop, _ := startServerClock(t, gmDB())
	defer stop()
	mod := enterWorldAs(t, addr, "mod")
	defer mod.Close()

	last := int16(testGridDim - 1)
	x, y, ok := gmPosDest(t, mod, "15 15")
	if !ok {
		t.Fatal("the last tile of the grid was refused")
	}
	if x != last || y != last {
		t.Errorf("landed on (%d,%d), want (%d,%d)", x, y, last, last)
	}
}

// Malformed input answers with a usage line instead of a dead command — the same
// rule the refusal work applied everywhere else this session.
func TestGMPosAnswersMalformedInput(t *testing.T) {
	addr, stop, _ := startServerClock(t, gmDB())
	defer stop()
	mod := enterWorldAs(t, addr, "mod")
	defer mod.Close()

	for _, args := range []string{"", "12", "abc def", "12 xyz"} {
		t.Run("args="+args, func(t *testing.T) {
			gmFrame(t, mod, "pos "+args)
			var sawPanel, sawJump bool
			for {
				ty, _, ok := readMaybe(t, mod)
				if !ok {
					break
				}
				switch ty {
				case protocol.MsgMessagePanel:
					sawPanel = true
				case protocol.MsgAction:
					sawJump = true
				}
			}
			if sawJump {
				t.Errorf("%q moved the character", args)
			}
			if !sawPanel {
				t.Errorf("%q was refused in silence, which is what makes a command look broken", args)
			}
		})
	}
}

// The gate is the same one every other GM command sits behind.
func TestGMPosDeniedToPlayer(t *testing.T) {
	addr, stop, _ := startServerClock(t, gmDB())
	defer stop()
	player := enterWorldAs(t, addr, "player")
	defer player.Close()

	if x, y, ok := gmPosDest(t, player, "12 13"); ok {
		t.Errorf("a plain player teleported to (%d,%d)", x, y)
	}
}
