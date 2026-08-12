package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/savefmt"
)

func TestBuildNPCDefinitionsRealCatalog(t *testing.T) {
	contentDir := filepath.Join("..", "..", "..", "Release")
	if _, err := os.Stat(contentDir); err != nil {
		t.Skip("Release content unavailable")
	}
	defs, err := buildNPCDefinitions(contentDir, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 548 {
		t.Fatalf("merchant generator definitions = %d, want 548", len(defs))
	}
	want := map[string]struct {
		index    int32
		merchant int16
		x, y     int32
	}{
		"Jeffi-290": {290, 12, 1112, 290}, "Jeffi-987": {987, 12, 2517, 1727},
		"Kibita-3811": {3811, 74, 2135, 2104},
	}
	for _, d := range defs {
		w, ok := want[d.Slug]
		if !ok {
			continue
		}
		if d.GeneratorIndex != w.index || d.Merchant != w.merchant || d.PosX != w.x || d.PosY != w.y {
			t.Errorf("%s = index %d merchant %d (%d,%d), want %+v", d.Slug, d.GeneratorIndex, d.Merchant, d.PosX, d.PosY, w)
		}
		delete(want, d.Slug)
	}
	if len(want) != 0 {
		t.Fatalf("missing expected definitions: %v", want)
	}
}

func TestBuildNPCDefinitionsNormalizesLegacyTemplateNames(t *testing.T) {
	dir := t.TempDir()
	runDir := filepath.Join(dir, "TMsrv", "run")
	npcDir := filepath.Join(runDir, "npc")
	if err := os.MkdirAll(npcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const npcGener = `# [0]
	Leader: Chefe_Treina.
	StartX: 2234
	StartY: 1566
	RouteType: 0

# [1]
	Leader: Reiners
	StartX: 2469
	StartY: 1991
	RouteType: 2
`
	if err := os.WriteFile(filepath.Join(runDir, "NPCGener.txt"), []byte(npcGener), 0o644); err != nil {
		t.Fatal(err)
	}
	writeDBNPCMob(t, npcDir, "Chefe_Treina", "Chefe_Treina.", 8)
	writeDBNPCMob(t, npcDir, "reiners", "Reiners", 1)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	defs, err := buildNPCDefinitions(dir, logger)
	if err != nil {
		t.Fatalf("buildNPCDefinitions: %v", err)
	}
	if len(defs) != 2 {
		t.Fatalf("got %d defs, want 2: %+v", len(defs), defs)
	}
	if defs[0].Slug != "Chefe_Treina.-0" || defs[0].TemplateName != "Chefe_Treina" ||
		defs[0].DisplayName != "Chefe_Treina." || defs[0].Merchant != 8 {
		t.Fatalf("def[0] = %+v, want normalized template with legacy slug/display name", defs[0])
	}
	if defs[1].Slug != "Reiners-1" || defs[1].TemplateName != "reiners" ||
		defs[1].DisplayName != "Reiners" || defs[1].Merchant != 1 {
		t.Fatalf("def[1] = %+v, want normalized lowercase template", defs[1])
	}
}

func writeDBNPCMob(t *testing.T, dir, fileName, mobName string, merchant byte) {
	t.Helper()
	b := make([]byte, savefmt.MobSize)
	copy(b[0:16], mobName)
	b[104] = merchant // CurrentScore.Merchant
	if err := os.WriteFile(filepath.Join(dir, fileName), b, 0o644); err != nil {
		t.Fatal(err)
	}
}
