package world

// Effect is one of an item's three effect/value pairs (STRUCT_ITEM, data-formats
// §1.5). World-local mirror, kept separate from internal/savefmt on purpose: this
// type backs the hot wire/world path, while internal/savefmt models the on-disk
// save-file layout (different concerns, same bytes).
type Effect struct {
	Effect uint8
	Value  uint8
}

// Item is an in-memory STRUCT_ITEM. Index==0 means an empty slot.
type Item struct {
	Index   int16
	Effects [3]Effect
	// ExpiresAt is the Unix-seconds expiry for timed items (0 = permanent). It is
	// carried through persistence; the world drops the item once it passes.
	ExpiresAt int64
}

// Empty reports whether the slot holds no item.
func (it Item) Empty() bool { return it.Index == 0 }

// Item container kinds (ITEM_PLACE_*), confirmed by Basedef.h:146-148.
const (
	ItemPlaceEquip = 0
	ItemPlaceCarry = 1
	ItemPlaceCargo = 2
)

// Gate states for a Static world object (GroundItem.State), Basedef.h:2209/2211. A
// seeded gate starts StateOpen (parity with CreateItem, Server.cpp:8075). Untyped so
// they assign to both the int16 GroundItem.State and the int32 wire State field.
const (
	StateOpen   = 1
	StateLocked = 3
)

// GroundItem is an item lying on the floor (CItem / pItem[], domain-model.md
// §2.3): contents, position and a presence flag. Mode==0 means a free slot.
//
// The same id space also holds static world objects (gates/doors from
// InitItem.csv): State carries their gate state (see gate.go) and Static marks
// them so the pickup/decay paths leave them alone. For a dropped item State is 0
// and Static is false.
type GroundItem struct {
	ID     int
	Item   Item
	X      int16
	Y      int16
	Mode   int
	State  int16 // gate state for Static world objects (STATE_OPEN/STATE_LOCKED); 0 for dropped items
	Static bool  // true for boot-seeded gates/doors: never picked up or decayed
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

// SeedWorldItem places a static world object (a gate/door from InitItem.csv) into
// the ground id space at (x,y) with an initial gate state, returning its id or -1
// when the table is full. Unlike CreateGroundItem it does not index the spatial
// grid — gates are looked up by id, not position, and must not shadow a droppable
// cell. It must run at boot (before Serve, single-threaded) so ids are assigned in
// file order and line up with the legacy CreateItem sequence the client expects.
func (w *World) SeedWorldItem(item Item, x, y, state int16) int {
	for id := 1; id < MaxItem; id++ {
		if w.ground[id] == nil {
			w.ground[id] = &GroundItem{ID: id, Item: item, X: x, Y: y, Mode: 1, State: state, Static: true}
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

// GroundItemAt returns the floor item indexed at (x,y), or nil when the cell is
// empty/out of bounds.
func (w *World) GroundItemAt(x, y int16) *GroundItem {
	if int(x) < 0 || int(y) < 0 || int(x) >= w.grid.dim || int(y) >= w.grid.dim {
		return nil
	}
	id, ok := w.grid.ItemAt(int(x), int(y))
	if !ok {
		return nil
	}
	return w.GroundItem(int(id))
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
	return addFirstEmpty(e.Carry[:], item)
}

// AddToCargo puts item in the account warehouse's first empty slot, returning the
// slot index, or -1 if the cargo is full. It is the delivery target for the
// donate web shop (issue #34): a purchase lands in the next free cargo slot, and
// a full cargo (-1) means the item is lost. Loop-only.
func (w *World) AddToCargo(st *CargoState, item Item) int {
	if st == nil {
		return -1
	}
	return addFirstEmpty(st.Items[:], item)
}

// addFirstEmpty places item in the first empty slot of items, returning its
// index, or -1 if every slot is occupied.
func addFirstEmpty(items []Item, item Item) int {
	for i := range items {
		if items[i].Empty() {
			items[i] = item
			return i
		}
	}
	return -1
}
