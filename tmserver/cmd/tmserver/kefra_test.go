package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestSpawnNPCsDefersKefraUntilStateLoad(t *testing.T) {
	dir := t.TempDir()
	runDir := filepath.Join(dir, "TMsrv", "run")
	npcDir := filepath.Join(runDir, "npc")
	if err := os.MkdirAll(npcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(npcDir, "Boss"), testMobTemplate("Boss"), 0o644); err != nil {
		t.Fatal(err)
	}
	var config strings.Builder
	// NPCGener indices are the block order; the bracketed number is a comment.
	for idx := 0; idx <= 400; idx++ {
		if idx > 0 && idx < 396 {
			fmt.Fprintf(&config, "# [%d]\nLeader: 0\n\n", idx)
			continue
		}
		fmt.Fprintf(&config, "# [%d]\nLeader: Boss\nMinGroup: 0\nMaxGroup: 0\nMaxNumMob: 1\nStartX: 10\nStartY: 10\n\n", idx)
	}
	if err := os.WriteFile(filepath.Join(runDir, "NPCGener.txt"), []byte(config.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.DiscardHandler)
	w := world.New(world.Config{GridDim: 64}, log, nil, nil)
	spawnNPCs(w, dir, false, nil, log)
	if w.GeneratorAt(0) == nil || w.GeneratorAt(0).CurrentNumMob != 1 {
		t.Fatal("ordinary generator did not spawn")
	}
	for idx := 396; idx <= 400; idx++ {
		g := w.GeneratorAt(idx)
		if g == nil || g.LeaderTmpl == nil || g.CurrentNumMob != 0 {
			t.Fatalf("event recipe %d was lost or spawned before load: %+v", idx, g)
		}
	}
}
