package handler

import (
	"encoding/binary"
	"net"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// twoItemDB gives the character two droppable items, so a second drop can be
// aimed at the tile the first one is already sitting on.
func twoItemDB(a, b int16) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: a}
	st.Carry[1] = world.Item{Index: b}
	db.loadResult = st
	return db
}

// dropAt sends a drop carrying a rotation, which the plain dropFrame helper does
// not — the confirmation has to echo it back and it used to be dropped entirely.
func dropAt(t *testing.T, c net.Conn, sourPos int, rotate int32, gx, gy uint16) {
	t.Helper()
	body := protocol.MsgDropItemBody{
		SourType: world.ItemPlaceCarry, SourPos: int32(sourPos),
		Rotate: rotate, GridX: gx, GridY: gy,
	}
	send(t, c, protocol.MsgDropItem, body.Encode())
}

type cnfDrop struct {
	sourType, sourPos, rotate int32
	gridX, gridY              uint16
}

func readCNFDrop(t *testing.T, c net.Conn) (cnfDrop, []byte) {
	t.Helper()
	payload := expect(t, c, protocol.MsgCNFDropItem)
	if len(payload) < protocol.MsgDropItemBodySize {
		t.Fatalf("corpo do CNFDropItem tem %d bytes, o cliente lê %d (Basedef.h:2236) — "+
			"o resto ele lê da memória depois do frame", len(payload), protocol.MsgDropItemBodySize)
	}
	le := binary.LittleEndian
	return cnfDrop{
		sourType: int32(le.Uint32(payload[0:4])),
		sourPos:  int32(le.Uint32(payload[4:8])),
		rotate:   int32(le.Uint32(payload[8:12])),
		gridX:    le.Uint16(payload[12:14]),
		gridY:    le.Uint16(payload[14:16]),
	}, payload
}

// THE CRASH. The confirmation used to be four bytes holding the slot number. The
// client reads five fields from it (SourType, SourPos, Rotate, GridX, GridY) and
// uses the last two to place the object on the floor, so a short frame left it
// reading coordinates off the end of the buffer — whatever memory followed. It
// placed the item at those coordinates, which is why dropping certain items took
// the client down and why it looked intermittent.
func TestDropConfirmationCarriesTheWholeRequest(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	dropAt(t, c, 0, 3, 6, 7)
	got, payload := readCNFDrop(t, c)

	if got.sourType != world.ItemPlaceCarry {
		t.Errorf("SourType = %d, esperado CARRY (%d) — o slot costumava cair aqui",
			got.sourType, world.ItemPlaceCarry)
	}
	if got.sourPos != 0 {
		t.Errorf("SourPos = %d, esperado 0", got.sourPos)
	}
	if got.rotate != 3 {
		t.Errorf("Rotate = %d, esperado 3", got.rotate)
	}
	if got.gridX != 6 || got.gridY != 7 {
		t.Errorf("posição = (%d,%d), esperado (6,7)", got.gridX, got.gridY)
	}
	if len(payload) != protocol.MsgDropItemBodySize {
		t.Errorf("corpo com %d bytes, esperado %d", len(payload), protocol.MsgDropItemBodySize)
	}
}

// The confirmation must name the cell REALLY used, not the one asked for: the
// original moves a drop off an occupied tile (GetEmptyItemGrid) and the client
// draws the object where the server says it landed.
func TestDropConfirmationNamesTheCellActuallyUsed(t *testing.T) {
	addr, stop, _ := startServerClock(t, twoItemDB(1100, 1200))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	dropAt(t, c, 0, 0, 6, 7)
	first, _ := readCNFDrop(t, c)
	if first.gridX != 6 || first.gridY != 7 {
		t.Fatalf("primeiro drop foi para (%d,%d), esperado (6,7)", first.gridX, first.gridY)
	}

	// Same tile again: it is taken, so the drop must move and SAY where it moved.
	dropAt(t, c, 1, 0, 6, 7)
	second, _ := readCNFDrop(t, c)
	if second.gridX == 6 && second.gridY == 7 {
		t.Error("segundo item caiu na mesma casa do primeiro, sobrescrevendo-o no grid")
	}
	if abs16(int16(second.gridX)-6) > 1 || abs16(int16(second.gridY)-7) > 1 {
		t.Errorf("desviou para (%d,%d), fora do 3×3 que o legado varre", second.gridX, second.gridY)
	}
}

// A tile off the map is refused out loud. The low bound was missing entirely:
// only the ceiling was tested, so a negative tile took a ground slot whose grid
// write was silently dropped — an item nobody could pick up.
func TestDropOffMapIsRefused(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgDropItemBody{
		SourType: world.ItemPlaceCarry, SourPos: 0,
		GridX: uint16(9999), GridY: uint16(9999),
	}
	send(t, c, protocol.MsgDropItem, body.Encode())

	var sawConfirm, sawNotice bool
	for {
		ty, _, ok := readMaybe(t, c)
		if !ok {
			break
		}
		switch ty {
		case protocol.MsgCNFDropItem:
			sawConfirm = true
		case protocol.MsgMessageBoxOk:
			sawNotice = true
		}
	}
	if sawConfirm {
		t.Error("drop fora do mapa foi confirmado")
	}
	if !sawNotice {
		t.Error("drop fora do mapa foi recusado calado")
	}
}
