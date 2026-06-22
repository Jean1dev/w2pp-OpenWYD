package world

// Effect is one of an item's three effect/value pairs (STRUCT_ITEM, data-formats
// §1.5). World-local mirror (tmserver cannot import dbserver/internal/savefmt).
type Effect struct {
	Effect uint8
	Value  uint8
}

// Item is an in-memory STRUCT_ITEM. Index==0 means an empty slot.
type Item struct {
	Index   int16
	Effects [3]Effect
}

// Empty reports whether the slot holds no item.
func (it Item) Empty() bool { return it.Index == 0 }

// Item container kinds (ITEM_PLACE_*).
//
// UNVERIFIED: the original numeric values are not documented; these are
// placeholders used consistently by the handlers and tests, to be pinned by
// capture (handlers/_MSG_DropItem.md / _MSG_GetItem.md).
const (
	ItemPlaceEquip = 0
	ItemPlaceCarry = 1
	ItemPlaceCargo = 2
)

// GroundItem is an item lying on the floor (CItem / pItem[], domain-model.md
// §2.3): contents, position and a presence flag. Mode==0 means a free slot.
type GroundItem struct {
	ID   int
	Item Item
	X    int16
	Y    int16
	Mode int
}

// CreateGroundItem places item on the floor at (x,y) and indexes it in the
// spatial grid. It returns the new ground id (∈ [1, MaxItem)) or -1 if the floor
// is full. Loop-only.
func (w *World) CreateGroundItem(item Item, x, y int16) int {
	for id := 1; id < MaxItem; id++ {
		if w.ground[id] == nil {
			w.ground[id] = &GroundItem{ID: id, Item: item, X: x, Y: y, Mode: 1}
			w.grid.SetItem(int(x), int(y), uint16(id))
			return id
		}
	}
	return -1
}

// GroundItem returns the floor item with the given id, or nil.
func (w *World) GroundItem(id int) *GroundItem {
	if id <= 0 || id >= MaxItem {
		return nil
	}
	return w.ground[id]
}

// RemoveGroundItem clears a floor item and its grid cell. This is the atomic
// claim point: because it runs in the single loop goroutine, two GetItem events
// for the same id are serialized and only the first succeeds (no dup). Loop-only.
func (w *World) RemoveGroundItem(id int) {
	gi := w.GroundItem(id)
	if gi == nil {
		return
	}
	if cur, ok := w.grid.ItemAt(int(gi.X), int(gi.Y)); ok && int(cur) == id {
		w.grid.ClearItem(int(gi.X), int(gi.Y))
	}
	w.ground[id] = nil
}

// AddToCarry puts item in the entity's first empty inventory slot, returning the
// slot index, or -1 if the inventory is full. Loop-only.
func (w *World) AddToCarry(e *Entity, item Item) int {
	for i := range e.Carry {
		if e.Carry[i].Empty() {
			e.Carry[i] = item
			return i
		}
	}
	return -1
}
