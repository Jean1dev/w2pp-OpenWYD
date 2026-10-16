package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestSpawnNPCsLeavesWaterGeneratorsForRoomLifecycle(t *testing.T) {
	dir := t.TempDir()
	runDir := filepath.Join(dir, "TMsrv", "run")
	npcDir := filepath.Join(runDir, "npc")
	if err := os.MkdirAll(npcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	for index := 0; index <= 10; index++ {
		fmt.Fprintf(&text, "# [%d]\nLeader: Grunt\nMinGroup: 0\nMaxGroup: 0\nMaxNumMob: 1\nStartX: %d\nStartY: 20\n\n", index, 20+index*5)
	}
	if err := os.WriteFile(filepath.Join(runDir, "NPCGener.txt"), []byte(text.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(npcDir, "Grunt"), testMobTemplate("Grunt"), 0o644); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	w := world.New(world.Config{GridDim: 128}, log, nil, nil)
	spawnNPCs(w, dir, nil, false, nil, log)
	if w.GeneratorAt(10) == nil || w.GeneratorAt(10).CurrentNumMob != 0 {
		t.Fatal("water generator spawned during world startup")
	}
	if w.GeneratorAt(9) == nil || w.GeneratorAt(9).CurrentNumMob != 1 {
		t.Fatal("ordinary generator failed to spawn")
	}
	for index := 0; index <= 10; index++ {
		w.ClearGenerator(index)
	}
}

func TestSpawnNPCsWarnsMissingTemplates(t *testing.T) {
	dir := t.TempDir()
	runDir := filepath.Join(dir, "TMsrv", "run")
	npcDir := filepath.Join(runDir, "npc")
	if err := os.MkdirAll(npcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const npcGener = `# [0]
	Leader: MissingLeader
	MinGroup: 0
	MaxGroup: 0
	MaxNumMob: 1
	StartX: 10
	StartY: 10

# [1]
	Leader: ExistingLeader
	Follower: MissingFollower
	MinGroup: 1
	MaxGroup: 1
	MaxNumMob: 2
	StartX: 20
	StartY: 20
`
	if err := os.WriteFile(filepath.Join(runDir, "NPCGener.txt"), []byte(npcGener), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(npcDir, "ExistingLeader"), testMobTemplate("ExistingLeader"), 0o644); err != nil {
		t.Fatal(err)
	}

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	w := world.New(world.Config{GridDim: 64}, logger, nil, nil)
	spawnNPCs(w, dir, nil, false, nil, logger)

	if g := w.GeneratorAt(0); g != nil {
		t.Fatalf("missing leader generator = %#v, want skipped nil slot", g)
	}
	g := w.GeneratorAt(1)
	if g == nil {
		t.Fatal("existing leader generator was not registered")
	}
	if g.FollowerTmpl != nil {
		t.Fatal("missing follower template should degrade to leader-only group")
	}
	if g.CurrentNumMob != 1 {
		t.Fatalf("CurrentNumMob = %d, want one spawned leader", g.CurrentNumMob)
	}

	gotLog := logs.String()
	for _, want := range []string{
		"NPC templates missing",
		"leader_blocks_skipped=1",
		"missing_leader_templates=1",
		"MissingLeader",
		"follower_blocks_degraded=1",
		"missing_follower_templates=1",
		"MissingFollower",
	} {
		if !strings.Contains(gotLog, want) {
			t.Fatalf("log missing %q:\n%s", want, gotLog)
		}
	}
}

func TestSpawnNPCsResolvesLegacyTemplateNames(t *testing.T) {
	dir := t.TempDir()
	runDir := filepath.Join(dir, "TMsrv", "run")
	npcDir := filepath.Join(runDir, "npc")
	if err := os.MkdirAll(npcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const npcGener = `# [0]
	Leader: Chefe_Treina.
	MinGroup: 0
	MaxGroup: 0
	MaxNumMob: 1
	StartX: 10
	StartY: 10

# [1]
	Leader: Reiners
	MinGroup: 0
	MaxGroup: 0
	MaxNumMob: 1
	StartX: 12
	StartY: 12
`
	if err := os.WriteFile(filepath.Join(runDir, "NPCGener.txt"), []byte(npcGener), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(npcDir, "Chefe_Treina"), testMobTemplate("Chefe_Treina."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(npcDir, "reiners"), testMobTemplate("Reiners"), 0o644); err != nil {
		t.Fatal(err)
	}

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	w := world.New(world.Config{GridDim: 64}, logger, nil, nil)
	spawnNPCs(w, dir, nil, false, nil, logger)

	for i := 0; i < 2; i++ {
		g := w.GeneratorAt(i)
		if g == nil {
			t.Fatalf("generator %d was not registered", i)
		}
		if g.CurrentNumMob != 1 {
			t.Fatalf("generator %d CurrentNumMob = %d, want one spawned leader", i, g.CurrentNumMob)
		}
	}
	if strings.Contains(logs.String(), "NPC templates missing") {
		t.Fatalf("legacy aliases should not log missing templates:\n%s", logs.String())
	}
}

func testMobTemplate(name string) []byte {
	b := make([]byte, content.BaseMobSize)
	copy(b[0:16], name)
	b[16] = 2
	const cs = 92
	binary.LittleEndian.PutUint32(b[cs+0:], 1)
	binary.LittleEndian.PutUint32(b[cs+16:], 100)
	binary.LittleEndian.PutUint32(b[cs+24:], 100)
	return b
}

// TestTrainingFieldAt reads the AttributeMap the way GetAttribute does: one
// byte per 4×4 tiles, bit 0x80 marking the training grounds (issue #383).
func TestTrainingFieldAt(t *testing.T) {
	attrs := &content.Grid{Dim: 8, Data: make([]byte, 64)}
	attrs.Data[2*8+3] = 0x80 | 0x02 // tiles x 12..15, y 8..11
	attrs.Data[0] = 0x40
	tests := []struct {
		name  string
		attrs *content.Grid
		x, y  int16
		want  bool
	}{
		{name: "field cell", attrs: attrs, x: 12, y: 8, want: true},
		{name: "same block far corner", attrs: attrs, x: 15, y: 11, want: true},
		{name: "neighbour block", attrs: attrs, x: 16, y: 8, want: false},
		{name: "other attribute bits", attrs: attrs, x: 0, y: 0, want: false},
		{name: "negative", attrs: attrs, x: -1, y: 8, want: false},
		{name: "out of range", attrs: attrs, x: 400, y: 8, want: false},
		{name: "no map", attrs: nil, x: 12, y: 8, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := trainingFieldAt(tt.attrs, tt.x, tt.y); got != tt.want {
				t.Errorf("trainingFieldAt(%d,%d) = %v, want %v", tt.x, tt.y, got, tt.want)
			}
		})
	}
}

func TestSpawnNPCsMarksTrainingFieldGenerators(t *testing.T) {
	dir := t.TempDir()
	runDir := filepath.Join(dir, "TMsrv", "run")
	npcDir := filepath.Join(runDir, "npc")
	if err := os.MkdirAll(npcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const npcGener = "# [0]\nLeader: Grunt\nMinGroup: 0\nMaxGroup: 0\nMaxNumMob: 1\nStartX: 13\nStartY: 9\n\n" +
		"# [1]\nLeader: Grunt\nMinGroup: 0\nMaxGroup: 0\nMaxNumMob: 1\nStartX: 40\nStartY: 40\n\n"
	if err := os.WriteFile(filepath.Join(runDir, "NPCGener.txt"), []byte(npcGener), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(npcDir, "Grunt"), testMobTemplate("Grunt"), 0o644); err != nil {
		t.Fatal(err)
	}
	attrs := &content.Grid{Dim: 32, Data: make([]byte, 32*32)}
	attrs.Data[2*32+3] = 0x80
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	w := world.New(world.Config{GridDim: 128}, log, nil, nil)
	spawnNPCs(w, dir, attrs, false, nil, log)
	if g := w.GeneratorAt(0); g == nil || !g.TrainingField {
		t.Error("generator on a 0x80 cell not marked as training field")
	}
	if g := w.GeneratorAt(1); g == nil || g.TrainingField {
		t.Error("generator off the field marked as training field")
	}
	for index := 0; index <= 1; index++ {
		w.ClearGenerator(index)
	}
}
