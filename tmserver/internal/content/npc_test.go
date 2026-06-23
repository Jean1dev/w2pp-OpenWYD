package content

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNPCGenerators(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "NPCGener.txt")
	const sample = `// header comment
#	[   0]
	MinuteGenerate:	-1
	MaxNumMob:	100
	MinGroup:	4
	Leader:		Ciclope_Forte
	StartX:		2635
	StartY:		1726
	StartRange:	5

#	[   1]
	Leader:		Ciclop_Selvagem
	MinGroup:	3
	StartX:		2700
	StartY:		1800
	StartRange:	8
`
	if err := os.WriteFile(path, []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	gens, err := LoadNPCGenerators(path)
	if err != nil {
		t.Fatalf("LoadNPCGenerators: %v", err)
	}
	if len(gens) != 2 {
		t.Fatalf("got %d generators, want 2", len(gens))
	}
	g := gens[0]
	if g.Leader != "Ciclope_Forte" || g.StartX != 2635 || g.StartY != 1726 ||
		g.StartRange != 5 || g.MinGroup != 4 {
		t.Errorf("gen[0] = %+v", g)
	}
	if gens[1].Leader != "Ciclop_Selvagem" || gens[1].MinGroup != 3 || gens[1].StartX != 2700 {
		t.Errorf("gen[1] = %+v", gens[1])
	}
}

// TestLoadNPCGeneratorsReal parses the shipped NPCGener.txt if present.
func TestLoadNPCGeneratorsReal(t *testing.T) {
	path := filepath.Join("..", "..", "..", "Release", "TMsrv", "run", "NPCGener.txt")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("NPCGener.txt unavailable: %v", err)
	}
	gens, err := LoadNPCGenerators(path)
	if err != nil {
		t.Fatalf("LoadNPCGenerators(real): %v", err)
	}
	if len(gens) < 1000 {
		t.Errorf("real NPCGener has %d generators, want many", len(gens))
	}
}
