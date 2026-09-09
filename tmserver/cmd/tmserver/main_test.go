package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
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
	spawnNPCs(w, dir, false, nil, logger)

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
	spawnNPCs(w, dir, false, nil, logger)

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

func testMobTemplate(name string) []byte { return testMobTemplateNivel(name, 1) }

func testMobTemplateNivel(name string, nivel int32) []byte {
	b := make([]byte, content.BaseMobSize)
	copy(b[0:16], name)
	b[16] = 2
	// Exp dentro da faixa sã (STRUCT_MOB.Exp @32): sem isto todo template de
	// teste dispara o aviso de Exp desbalanceada, que enche o log de linhas
	// citando nomes e faz qualquer asserção sobre o log virar falso positivo.
	binary.LittleEndian.PutUint64(b[32:], 1000)
	const cs = 92
	binary.LittleEndian.PutUint32(b[cs+0:], uint32(nivel))
	binary.LittleEndian.PutUint32(b[cs+16:], 100)
	binary.LittleEndian.PutUint32(b[cs+24:], 100)
	return b
}

// TestSpawnNPCsAvisaNivelForaDaFaixa is the guard the level fix needs to survive
// the next content import.
//
// The 599s are not a typo somebody made once: they read like a convention
// ("stronger than 400"), which means a future import brings them back. Nothing
// in the game refuses the value and nothing in the log mentions it, so the only
// symptom is that a level-200 player earns more from a 599 than from a 399 —
// which nobody attributes to a template field.
func TestSpawnNPCsAvisaNivelForaDaFaixa(t *testing.T) {
	dir := t.TempDir()
	runDir := filepath.Join(dir, "TMsrv", "run")
	npcDir := filepath.Join(runDir, "npc")
	if err := os.MkdirAll(npcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Um bloco por template, porque o aviso conta TEMPLATES e não blocos.
	niveis := []struct {
		nome  string
		nivel int32
	}{
		{"NoLimite", 399},          // o teto do jogador: certo, não pode aparecer
		{"NoTeto", 400},            // acima do teto, mas a escala ainda vale
		{"PrimeiroSemEscala", 401}, // aqui a escala desliga
		{"Absurdo", 599},
	}
	var gener strings.Builder
	for i, n := range niveis {
		fmt.Fprintf(&gener, "# [%d]\n\tLeader: %s\n\tMinGroup: 0\n\tMaxGroup: 0\n\tMaxNumMob: 1\n\tStartX: %d\n\tStartY: 10\n\n",
			i, n.nome, 10+i)
	}
	if err := os.WriteFile(filepath.Join(runDir, "NPCGener.txt"), []byte(gener.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, n := range niveis {
		if err := os.WriteFile(filepath.Join(npcDir, n.nome), testMobTemplateNivel(n.nome, n.nivel), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	w := world.New(world.Config{GridDim: 64}, logger, nil, nil)
	spawnNPCs(w, dir, false, nil, logger)

	got := logs.String()
	for _, quer := range []string{
		"monster template level outside 1..399",
		"templates=3",          // 400, 401 e 599
		"unscaled_above_400=2", // só 401 e 599
		"highest_level=599",
		"Absurdo",
	} {
		if !strings.Contains(got, quer) {
			t.Fatalf("faltou %q no log:\n%s", quer, got)
		}
	}
	// O que está dentro da faixa não pode ser citado: um aviso que nomeia quem
	// está certo é um aviso que ninguém lê até o fim.
	if strings.Contains(got, "NoLimite") {
		t.Errorf("o aviso citou um template de nível 399, que está certo:\n%s", got)
	}
}

// TestSpawnNPCsCaladoComTudoNaFaixa: o estado normal, depois que os 55 forem
// corrigidos, é log limpo. Um aviso que aparece sempre não avisa nada.
func TestSpawnNPCsCaladoComTudoNaFaixa(t *testing.T) {
	dir := t.TempDir()
	runDir := filepath.Join(dir, "TMsrv", "run")
	npcDir := filepath.Join(runDir, "npc")
	if err := os.MkdirAll(npcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const gener = "# [0]\n\tLeader: Normal\n\tMinGroup: 0\n\tMaxGroup: 0\n\tMaxNumMob: 1\n\tStartX: 10\n\tStartY: 10\n"
	if err := os.WriteFile(filepath.Join(runDir, "NPCGener.txt"), []byte(gener), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(npcDir, "Normal"), testMobTemplateNivel("Normal", 250), 0o644); err != nil {
		t.Fatal(err)
	}

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	w := world.New(world.Config{GridDim: 64}, logger, nil, nil)
	spawnNPCs(w, dir, false, nil, logger)

	if strings.Contains(logs.String(), "level outside") {
		t.Errorf("avisou com todo mundo na faixa:\n%s", logs.String())
	}
}
