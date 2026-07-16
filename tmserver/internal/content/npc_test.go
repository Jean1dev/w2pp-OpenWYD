package content

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNPCGenerators(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "NPCGener.txt")
	// gen[0] mirrors a real Start/Dest patrol block (Coliseu): waypoints land in
	// SegX[0] (Start) and SegX[4] (Dest) exactly as ParseString maps them.
	const sample = `// header comment
#	[   0]
	MinuteGenerate:	-1
	MaxNumMob:	100
	MinGroup:	4
	MaxGroup:	7
	Leader:		Ciclope_Forte
	Follower:	Arq_Ciclope
	RouteType:	2
	Formation:	0
	FightAction:	Lute
	FightAction:	Venha
	FightAction4:	Ultima
	DieAction2:	Morri
	StartX:		2635
	StartY:		1726
	StartRange:	5
	StartWait:	10
	DestX:		2625
	DestY:		1726
	DestRange:	10
	DestWait:	10

#	[   1]
	Leader:		Ciclop_Selvagem
	Follower:	0
	MinGroup:	3
	StartX:		2700
	StartY:		1800
	StartRange:	8
	Segment1X:	2710
	Segment1Y:	1810
	Segment1Wait:	4
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
	if g.Leader != "Ciclope_Forte" || g.SegX[0] != 2635 || g.SegY[0] != 1726 ||
		g.SegRange[0] != 5 || g.MinGroup != 4 {
		t.Errorf("gen[0] = %+v", g)
	}
	if g.Follower != "Arq_Ciclope" || g.MinuteGenerate != -1 || g.MaxGroup != 7 || g.RouteType != 2 {
		t.Errorf("gen[0] extras = %+v", g)
	}
	if g.FightAction != [4]string{"Lute", "Venha", "", "Ultima"} {
		t.Errorf("gen[0] FightAction = %#v", g.FightAction)
	}
	if g.DieAction != [4]string{"", "Morri", "", ""} {
		t.Errorf("gen[0] DieAction = %#v", g.DieAction)
	}
	if g.SegX[4] != 2625 || g.SegY[4] != 1726 || g.SegRange[4] != 10 || g.SegWait[4] != 10 || g.SegWait[0] != 10 {
		t.Errorf("gen[0] Dest→Seg[4] mapping = %+v", g)
	}
	if g.SegX[1] != 0 || g.SegX[2] != 0 || g.SegX[3] != 0 {
		t.Errorf("gen[0] middle waypoints should stay 0, got %+v", g.SegX)
	}
	g1 := gens[1]
	if g1.Leader != "Ciclop_Selvagem" || g1.MinGroup != 3 || g1.SegX[0] != 2700 {
		t.Errorf("gen[1] = %+v", g1)
	}
	if g1.Follower != "" { // "Follower: 0" means none
		t.Errorf("gen[1] Follower = %q, want empty", g1.Follower)
	}
	if g1.SegX[1] != 2710 || g1.SegY[1] != 1810 || g1.SegWait[1] != 4 {
		t.Errorf("gen[1] Segment1 = %+v", g1)
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
