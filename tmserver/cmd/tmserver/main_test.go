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
	spawnNPCs(w, dir, false, nil, nil, nil, logger)

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
	spawnNPCs(w, dir, false, nil, nil, nil, logger)

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
	// Um lojista com atributo de chefe, que e o caso real: entra na conta e e
	// contado a parte, porque baixar o nivel dele mexe no que ele vende.
	const lojista = "LojistaChefe"
	var gener strings.Builder
	for i, n := range niveis {
		fmt.Fprintf(&gener, "# [%d]\n\tLeader: %s\n\tMinGroup: 0\n\tMaxGroup: 0\n\tMaxNumMob: 1\n\tStartX: %d\n\tStartY: 10\n\n",
			i, n.nome, 10+i)
	}
	fmt.Fprintf(&gener, "# [%d]\n\tLeader: %s\n\tMinGroup: 0\n\tMaxGroup: 0\n\tMaxNumMob: 1\n\tStartX: 30\n\tStartY: 10\n\n",
		len(niveis), lojista)
	if err := os.WriteFile(filepath.Join(runDir, "NPCGener.txt"), []byte(gener.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, n := range niveis {
		if err := os.WriteFile(filepath.Join(npcDir, n.nome), testMobTemplateNivel(n.nome, n.nivel), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tmplLojista := testMobTemplateNivel(lojista, 599)
	tmplLojista[92+12] = 16 // CurrentScore.Merchant
	if err := os.WriteFile(filepath.Join(npcDir, lojista), tmplLojista, 0o644); err != nil {
		t.Fatal(err)
	}

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	w := world.New(world.Config{GridDim: 64}, logger, nil, nil)
	spawnNPCs(w, dir, false, nil, nil, nil, logger)

	got := logs.String()
	for _, quer := range []string{
		"monster template level outside 1..399",
		"templates=4",          // 400, 401, 599 e o lojista
		"unscaled_above_400=3", // 401, 599 e o lojista
		"with_merchant=1",      // só o lojista
		"highest_level=599",
		"Absurdo",
		lojista,
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
	spawnNPCs(w, dir, false, nil, nil, nil, logger)

	if strings.Contains(logs.String(), "level outside") {
		t.Errorf("avisou com todo mundo na faixa:\n%s", logs.String())
	}
}

// TestSpawnNPCsAvisaEstoqueDeGraca cobre as duas contagens, e a segunda é a que
// justifica o teste existir: um item de graça guardado fora das três abas da
// vitrine não aparece para ninguém, e mesmo assim handler.buy o alcança, porque
// valida só npcPos < MaxCarry. Contar só a vitrine deixaria passar o caso que
// ninguém tem como ver.
func TestSpawnNPCsAvisaEstoqueDeGraca(t *testing.T) {
	dir := t.TempDir()
	runDir := filepath.Join(dir, "TMsrv", "run")
	npcDir := filepath.Join(runDir, "npc")
	if err := os.MkdirAll(npcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const gener = "# [0]\n\tLeader: Lojista\n\tMinGroup: 0\n\tMaxGroup: 0\n\tMaxNumMob: 1\n\tStartX: 10\n\tStartY: 10\n"
	if err := os.WriteFile(filepath.Join(runDir, "NPCGener.txt"), []byte(gener), 0o644); err != nil {
		t.Fatal(err)
	}
	tmpl := testMobTemplateNivel("Lojista", 50)
	tmpl[92+12] = 1 // CurrentScore.Merchant: sem isto o Carry é tabela de drop
	// Carry[0] é aba 1 da vitrine; Carry[20] não é aba nenhuma; Carry[54] é aba 3.
	const carry = 268
	binary.LittleEndian.PutUint16(tmpl[carry+0*8:], 1000)  // de graça, na vitrine
	binary.LittleEndian.PutUint16(tmpl[carry+20*8:], 1001) // de graça, escondido
	binary.LittleEndian.PutUint16(tmpl[carry+54*8:], 1002) // com preço, na vitrine
	binary.LittleEndian.PutUint16(tmpl[carry+1*8:], 9999)  // fora do catálogo
	if err := os.WriteFile(filepath.Join(npcDir, "Lojista"), tmpl, 0o644); err != nil {
		t.Fatal(err)
	}

	precos := map[int]int32{1000: 0, 1001: 0, 1002: 500}
	nomes := map[int]string{1000: "Presente", 1001: "Escondido", 1002: "Pago"}

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	w := world.New(world.Config{GridDim: 64}, logger, nil, nil)
	spawnNPCs(w, dir, false, nil, precos, nomes, logger)

	got := logs.String()
	for _, quer := range []string{
		"shop stock priced at zero",
		"Presente(1000)",
		"OUTSIDE the shop window",
		"Escondido(1001)",
	} {
		if !strings.Contains(got, quer) {
			t.Fatalf("faltou %q no log:\n%s", quer, got)
		}
	}
	// O que tem preço não pode ser citado, e o que não está no catálogo também
	// não: aquele não é item de graça, é vitrine suja, e tem outro conserto.
	for _, proibido := range []string{"Pago(1002)", "9999"} {
		if strings.Contains(got, proibido) {
			t.Errorf("o aviso citou %q, que não é item de graça:\n%s", proibido, got)
		}
	}
}

// TestSpawnNPCsCaladoComTudoPago: log limpo é o estado normal depois que os
// preços entrarem. Aviso que aparece sempre não avisa.
func TestSpawnNPCsCaladoComTudoPago(t *testing.T) {
	dir := t.TempDir()
	runDir := filepath.Join(dir, "TMsrv", "run")
	npcDir := filepath.Join(runDir, "npc")
	if err := os.MkdirAll(npcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const gener = "# [0]\n\tLeader: Lojista\n\tMinGroup: 0\n\tMaxGroup: 0\n\tMaxNumMob: 1\n\tStartX: 10\n\tStartY: 10\n"
	if err := os.WriteFile(filepath.Join(runDir, "NPCGener.txt"), []byte(gener), 0o644); err != nil {
		t.Fatal(err)
	}
	tmpl := testMobTemplateNivel("Lojista", 50)
	tmpl[92+12] = 1
	binary.LittleEndian.PutUint16(tmpl[268:], 1002)
	if err := os.WriteFile(filepath.Join(npcDir, "Lojista"), tmpl, 0o644); err != nil {
		t.Fatal(err)
	}

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	w := world.New(world.Config{GridDim: 64}, logger, nil, nil)
	spawnNPCs(w, dir, false, nil, map[int]int32{1002: 500}, nil, logger)

	if strings.Contains(logs.String(), "priced at zero") {
		t.Errorf("avisou com tudo pago:\n%s", logs.String())
	}
}
