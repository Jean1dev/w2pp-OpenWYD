package main

import (
	"bytes"
	"encoding/binary"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

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
	spawnNPCs(w, dir, false, logger)

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
	spawnNPCs(w, dir, false, logger)

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
