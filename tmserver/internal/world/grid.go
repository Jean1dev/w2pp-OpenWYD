package world

// gridEmpty is the sentinel for an empty cell. The original pMobGrid/pItemGrid
// store a ushort index per cell (domain-model.md §4); since index 0 is a valid
// player conn, an empty cell needs a non-zero sentinel. 0xFFFF is used here.
//
// UNVERIFIED: the original's exact empty sentinel is not documented; confirm
// against a capture/build before relying on it for visibility/collision parity.
const gridEmpty uint16 = 0xFFFF

// Grid is the dense spatial index of mobs and ground items by cell
// (pMobGrid/pItemGrid, domain-model.md §4). It is derived state kept in sync
// with entity positions by the loop; it is not safe for concurrent use.
type Grid struct {
	dim  int
	mob  []uint16
	item []uint16
}

func newGrid(dim int) *Grid {
	g := &Grid{dim: dim, mob: make([]uint16, dim*dim), item: make([]uint16, dim*dim)}
	for i := range g.mob {
		g.mob[i] = gridEmpty
		g.item[i] = gridEmpty
	}
	return g
}

// Dim returns the grid's side length.
func (g *Grid) Dim() int { return g.dim }

func (g *Grid) inBounds(x, y int) bool { return x >= 0 && y >= 0 && x < g.dim && y < g.dim }

// MobAt returns the mob index at (x,y) and whether the cell is occupied.
func (g *Grid) MobAt(x, y int) (uint16, bool) {
	if !g.inBounds(x, y) {
		return 0, false
	}
	v := g.mob[y*g.dim+x]
	return v, v != gridEmpty
}

// SetMob records mob id at (x,y). Out-of-bounds writes are ignored.
func (g *Grid) SetMob(x, y int, id uint16) {
	if g.inBounds(x, y) {
		g.mob[y*g.dim+x] = id
	}
}

// ClearMob empties the cell at (x,y).
func (g *Grid) ClearMob(x, y int) {
	if g.inBounds(x, y) {
		g.mob[y*g.dim+x] = gridEmpty
	}
}
