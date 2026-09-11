package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/savefmt"
)

// No campo de treino decide o byte do legado (internal/campotreino). O
// Orc_Sniper, com loja só no byte 104, não vira definição de NPC — e é isso que
// faz a poda do seed apagar a linha que ele já tem em produção. O Treinador, com
// os dois bytes ligados, continua definição; e o mesmo Orc fora do campo não
// muda, porque a correção vale só lá dentro.
func TestBuildNPCDefinitionsDeixaDeForaOsMonstrosDoCampo(t *testing.T) {
	dir := t.TempDir()
	runDir := filepath.Join(dir, "TMsrv", "run")
	npcDir := filepath.Join(runDir, "npc")
	if err := os.MkdirAll(npcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const npcGener = `# [0]
	Leader: Orc_Sniper
	StartX: 2079
	StartY: 1976
	RouteType: 2

# [1]
	Leader: Treinador1
	StartX: 2080
	StartY: 2018
	RouteType: 0

# [2]
	Leader: OrcForaDoCampo
	StartX: 2600
	StartY: 1700
	RouteType: 2
`
	if err := os.WriteFile(filepath.Join(runDir, "NPCGener.txt"), []byte(npcGener), 0o644); err != nil {
		t.Fatal(err)
	}
	writeDBNPCMobBytes(t, npcDir, "Orc_Sniper", 0, 16)
	writeDBNPCMobBytes(t, npcDir, "Treinador1", 36, 100)
	writeDBNPCMobBytes(t, npcDir, "OrcForaDoCampo", 0, 16)

	defs, err := buildNPCDefinitions(dir, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("buildNPCDefinitions: %v", err)
	}
	slugs := make(map[string]bool, len(defs))
	for _, d := range defs {
		slugs[d.Slug] = true
	}
	if slugs["Orc_Sniper-0"] {
		t.Error("o Orc_Sniper do campo virou definição de NPC: continua imortal e fora da Mesa de Drops")
	}
	if !slugs["Treinador1-1"] {
		t.Error("o Treinador1 saiu das definições: NPC de serviço do campo tem de continuar lá")
	}
	if !slugs["OrcForaDoCampo-2"] {
		t.Error("um bloco fora do campo mudou: a correção é só do campo de treino")
	}
	if len(defs) != 2 {
		t.Errorf("definições = %d (%v), want 2", len(defs), slugs)
	}
}

// writeDBNPCMobBytes writes a template with both merchant bytes: STRUCT_MOB.Merchant
// at 17 (the legacy's) and CurrentScore.Merchant at 104.
func writeDBNPCMobBytes(t *testing.T, dir, name string, mobMerchant, scoreMerchant byte) {
	t.Helper()
	b := make([]byte, savefmt.MobSize)
	copy(b[0:16], name)
	b[17] = mobMerchant
	b[104] = scoreMerchant
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
		t.Fatal(err)
	}
}
