package content

import (
	"fmt"
	"os"
)

// Map dimensions (data-formats.md §2). HeightMap.dat is 4096×4096 (16 MiB, 1
// byte/cell); AttributeMap.dat is 1024×1024 (1 MiB) — one attribute per 4×4
// block of height cells.
const (
	HeightMapDim    = 4096
	AttributeMapDim = 1024
)

// Grid is a dense square byte map (HeightMap/AttributeMap), row-major: index
// y*Dim + x (pHeightGrid[y][x]).
type Grid struct {
	Dim  int
	Data []byte
}

// At returns the cell value at (x,y), or 0 if out of bounds.
func (g *Grid) At(x, y int) byte {
	if x < 0 || y < 0 || x >= g.Dim || y >= g.Dim {
		return 0
	}
	return g.Data[y*g.Dim+x]
}

// LoadGrid reads a dense dim×dim byte map and verifies its exact size (the file
// is an asset, kept binary — data-formats.md §2).
func LoadGrid(path string, dim int) (*Grid, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("content: read map %q: %w", path, err)
	}
	if want := dim * dim; len(data) != want {
		return nil, fmt.Errorf("content: map %q size %d != %d (dim %d)", path, len(data), want, dim)
	}
	return &Grid{Dim: dim, Data: data}, nil
}

// LoadHeightMap reads HeightMap.dat (4096²). AttributeMap.dat (1024²) is loaded
// with LoadGrid(path, AttributeMapDim); its per-byte bit semantics are UNVERIFIED
// (data-formats.md §2).
func LoadHeightMap(path string) (*Grid, error) { return LoadGrid(path, HeightMapDim) }
