package route

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
)

// flat returns a dim×dim height grid of zeros with the given cells raised to h.
func flat(dim int, h byte, cells ...[2]int) *content.Grid {
	g := &content.Grid{Dim: dim, Data: make([]byte, dim*dim)}
	for _, c := range cells {
		g.Data[c[1]*dim+c[0]] = h
	}
	return g
}

func TestNextWalksAndArrives(t *testing.T) {
	g := flat(16, 0)
	tests := []struct {
		name         string
		x, y, tx, ty int
		dist         int
		wantX, wantY int
		wantMoved    bool
	}{
		{"straight east", 2, 2, 10, 2, 5, 7, 2, true},
		{"diagonal SE arrives", 2, 2, 6, 6, 8, 6, 6, true},
		{"diagonal NW", 8, 8, 5, 5, 8, 5, 5, true},
		{"already there", 5, 5, 5, 5, 8, 5, 5, false},
		{"capped by distance", 1, 1, 14, 1, 3, 4, 1, true},
		{"border margin stops", 0, 5, 8, 5, 4, 0, 5, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nx, ny, moved := Next(g, tt.x, tt.y, tt.tx, tt.ty, tt.dist)
			if nx != tt.wantX || ny != tt.wantY || moved != tt.wantMoved {
				t.Errorf("Next = (%d,%d,%v), want (%d,%d,%v)", nx, ny, moved, tt.wantX, tt.wantY, tt.wantMoved)
			}
		})
	}
}

// TestNextDeviatesAroundWall: the direct east cell is Blocked, so the walker
// takes the straight-line deviation (SE, Basedef.cpp:6446) and then converges
// back via the NE diagonal-fallback — sliding around the wall like the original.
func TestNextDeviatesAroundWall(t *testing.T) {
	g := flat(16, Blocked, [2]int{4, 3})
	nx, ny, moved := Next(g, 3, 3, 8, 3, 6)
	if !moved || nx != 8 || ny != 3 {
		t.Fatalf("Next around wall = (%d,%d,%v), want (8,3,true)", nx, ny, moved)
	}
	// And the wall cell itself was never entered: re-walk one step at a time.
	x, y := 3, 3
	for i := 0; i < 6; i++ {
		px, py := x, y
		x, y, _ = Next(g, x, y, 8, 3, 1)
		if x == 4 && y == 3 {
			t.Fatalf("walker stepped into the blocked cell from (%d,%d)", px, py)
		}
		if x == 8 && y == 3 {
			return
		}
	}
	t.Fatalf("single-stepping never reached the target, stopped at (%d,%d)", x, y)
}

// TestNextHeightStep: a neighbour is passable iff |Δheight| < MH (exclusive on
// both sides, Basedef.cpp:6232) — a 7 step passes, an 8 step blocks.
func TestNextHeightStep(t *testing.T) {
	pass := flat(16, 7, [2]int{6, 5})
	if _, _, moved := Next(pass, 5, 5, 6, 5, 1); !moved {
		t.Error("Δh=7 (<MH) should be passable")
	}
	block := flat(16, 8, [2]int{6, 5})
	if _, _, moved := Next(block, 5, 5, 6, 5, 1); moved {
		t.Error("Δh=8 (=MH) must block")
	}
	// Signed heights: from a -6 cell to a 0 cell is Δ6 → passable.
	neg := flat(16, 0)
	neg.Data[5*16+5] = uint8(0xFA) // int8(-6) two's-complement
	if _, _, moved := Next(neg, 5, 5, 6, 5, 1); !moved {
		t.Error("Δh=6 from a negative cell should be passable")
	}
}

// TestNextAdjacentQuirk pins the original's bailout (Basedef.cpp:6379): when the
// target is within 1 on either axis and the primary step is blocked, the walker
// gives up rather than sidestepping.
func TestNextAdjacentQuirk(t *testing.T) {
	g := flat(16, Blocked, [2]int{5, 6})
	nx, ny, moved := Next(g, 5, 5, 5, 6, 4)
	if moved {
		t.Errorf("Next = (%d,%d,%v), want no move (adjacent bailout)", nx, ny, moved)
	}
}

// TestBake: each attribute byte covers a 4×4 height block; att&2 → Blocked
// (BASE_ApplyAttribute, Basedef.cpp:2624-2647). Other bits leave heights alone.
func TestBake(t *testing.T) {
	h := &content.Grid{Dim: 8, Data: make([]byte, 64)}
	attr := &content.Grid{Dim: 2, Data: []byte{
		0, 2, // (1,0): block height cells x4-7, y0-3
		1, 0, // (0,1): bit 2 clear → untouched
	}}
	Bake(h, attr)
	if h.At(4, 0) != Blocked || h.At(7, 3) != Blocked {
		t.Error("att&2 block not baked to Blocked")
	}
	if h.At(3, 0) != 0 || h.At(4, 4) != 0 || h.At(0, 4) != 0 {
		t.Error("cells outside the att&2 block were modified")
	}
}
