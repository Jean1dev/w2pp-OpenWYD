// Package route ports the original tile pathfinding: BASE_GetRoute's greedy
// 8-direction line-walk over the height grid (Basedef.cpp:6194-6482) and
// BASE_ApplyAttribute's blocked-tile bake (Basedef.cpp:2624-2647). It is pure
// (no world state) so the walk can be golden-tested against synthetic maps.
package route

import "github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"

const (
	// MaxRoute is MAX_ROUTE (Basedef.h:184): the longest route the original
	// fills per call, so a walk advances at most MaxRoute-1 steps.
	MaxRoute = 24
	// MH is the max height step a walker can climb/descend between adjacent
	// tiles (Basedef.h:194): a neighbour is passable iff its height is within
	// (cur-MH, cur+MH) exclusive.
	MH = 8
	// Blocked is the height Bake writes into cells whose attribute has bit 2
	// set — outside every (cur±MH) window, so nothing can step in or out.
	Blocked = 127
)

// Bake burns the AttributeMap into the height grid, exactly as the original does
// once at boot (Server.cpp:7954 → BASE_ApplyAttribute, Basedef.cpp:2624-2647):
// each attribute byte covers a 4×4 block of height cells ((x>>2)&0x3FF), and
// att&2 marks the block impassable (height ← 127). Mutates h.Data; call once
// before wiring the grid into the AI.
func Bake(h, attr *content.Grid) {
	for y := 0; y < h.Dim; y++ {
		row := h.Data[y*h.Dim : (y+1)*h.Dim]
		arow := attr.Data[((y>>2)&0x3FF)*attr.Dim:]
		for x := range row {
			if arow[(x>>2)&0x3FF]&2 != 0 {
				row[x] = Blocked
			}
		}
	}
}

// Next ports BASE_GetRoute (Basedef.cpp:6194-6482): from (x,y) it walks up to
// min(distance, MaxRoute-1) greedy steps toward (tx,ty) and returns where it
// stopped. Each step tries the exact branch order of the original — the primary
// direction from the sign of (tx-x, ty-y), then its lateral fallbacks — taking
// the first passable neighbour (height within ±MH of the current cell, signed
// bytes); if none passes, or the walker reaches the target or the map border
// margin, it stops early. moved=false means it could not take even one step
// (the caller's "stuck" signal, Route[0]==0 in the original).
//
// Quirk kept as-is: after the primary branches fail, the original gives up when
// the target is within 1 on EITHER axis (Basedef.cpp:6379) — so the lateral
// fallbacks never fire right next to the target.
func Next(h *content.Grid, x, y, tx, ty, distance int) (nx, ny int, moved bool) {
	sx, sy := x, y
	dim := h.Dim
	// ht reads a signed height: the original's pHeight is char (signed on MSVC
	// x86) and real map cells go negative; Blocked=127 stays the max int8.
	ht := func(x, y int) int { return int(int8(h.Data[y*dim+x])) }

	for i := 0; i < distance && i < MaxRoute-1; i++ {
		if x < 1 || y < 1 || x > dim-2 || y > dim-2 {
			break // border margin (Basedef.cpp:6205): neighbours must exist
		}
		cul := ht(x, y)
		pass := func(dx, dy int) bool {
			v := ht(x+dx, y+dy)
			return v < cul+MH && v > cul-MH
		}

		if tx == x && ty == y {
			break // arrived
		}
		// Primary direction (branch order = Basedef.cpp:6232-6305). Note the
		// digit convention: y-- is '2' and y++ is '8' (screen-space numpad).
		if tx == x && ty < y && pass(0, -1) {
			y--
			continue
		}
		if tx > x && ty < y && pass(1, -1) {
			x++
			y--
			continue
		}
		if tx > x && ty == y && pass(1, 0) {
			x++
			continue
		}
		if tx > x && ty > y && pass(1, 1) {
			x++
			y++
			continue
		}
		if tx == x && ty > y && pass(0, 1) {
			y++
			continue
		}
		if tx < x && ty > y && pass(-1, 1) {
			x--
			y++
			continue
		}
		if tx < x && ty == y && pass(-1, 0) {
			x--
			continue
		}
		if tx < x && ty < y && pass(-1, -1) {
			x--
			y--
			continue
		}
		// Diagonal fallbacks: try each axis of the blocked diagonal separately
		// (Basedef.cpp:6307-6377).
		if tx > x && ty < y && pass(1, 0) {
			x++
			continue
		}
		if tx > x && ty < y && pass(0, -1) {
			y--
			continue
		}
		if tx > x && ty > y && pass(1, 0) {
			x++
			continue
		}
		if tx > x && ty > y && pass(0, 1) {
			y++
			continue
		}
		if tx < x && ty > y && pass(-1, 0) {
			x--
			continue
		}
		if tx < x && ty > y && pass(0, 1) {
			y++
			continue
		}
		if tx < x && ty < y && pass(-1, 0) {
			x--
			continue
		}
		if tx < x && ty < y && pass(0, -1) {
			y--
			continue
		}
		// Adjacent-to-target bailout (Basedef.cpp:6379) — before the straight-
		// line deviations below.
		if tx == x+1 || ty == y+1 || tx == x-1 || ty == y-1 {
			break
		}
		// Straight-line deviations: the direct cardinal is blocked, sidestep
		// diagonally (Basedef.cpp:6386-6464).
		if tx == x && ty > y && pass(1, 1) {
			x++
			y++
			continue
		}
		if tx == x && ty > y && pass(-1, 1) {
			x--
			y++
			continue
		}
		if tx == x && ty < y && pass(1, -1) {
			x++
			y--
			continue
		}
		if tx == x && ty < y && pass(-1, -1) {
			x--
			y--
			continue
		}
		if tx < x && ty == y && pass(-1, 1) {
			x--
			y++
			continue
		}
		if tx < x && ty == y && pass(-1, -1) {
			x--
			y--
			continue
		}
		if tx > x && ty == y && pass(1, 1) {
			x++
			y++
			continue
		}
		if tx > x && ty == y && pass(1, -1) {
			x++
			y--
			continue
		}
		break // fully stuck (Basedef.cpp:6466)
	}
	return x, y, x != sx || y != sy
}
