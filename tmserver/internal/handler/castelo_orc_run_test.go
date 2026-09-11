package handler

import (
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// xamaTemplate is a quest NPC the way the Xamã ships: Merchant 100 and grade 40
// on the face.
func xamaTemplate() []byte {
	b := expMobTemplate(80, 0, 6)
	b[17] = 100    // MOB.Merchant
	b[92+12] = 100 // CurrentScore.Merchant, the byte the world reads
	b[140] = 232   // Equip[0].sIndex (Shamã_Orc), low byte
	// Equip[0].Effects[0] = EF_GRADE0 40
	b[142], b[143] = 100, gradeCasteloOrc
	return b
}

func casteloOrcBlock(name string, x, y int16, maxNum, group int) *world.Generator {
	tmpl := expMobTemplate(300, 0, 1)
	g := &world.Generator{
		Name: name, MinuteGenerate: -1, MaxNumMob: maxNum, MinGroup: group, MaxGroup: group,
		LeaderTmpl: tmpl, LeaderName: name,
	}
	if group > 0 {
		g.FollowerTmpl, g.FollowerName = tmpl, name
	}
	for i := range g.SegX {
		g.SegX[i], g.SegY[i], g.SegRange[i] = x, y, 2
	}
	return g
}

// casteloOrcFixture is the castle as production lays it out: one open-world
// castle block (a Meio_Orc group, populated) and the quest's eight blocks.
func casteloOrcFixture(t *testing.T) (*Dispatcher, *world.World, *world.Session, *world.Entity) {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	d := New(Config{Log: log, CasteloOrcNPC: xamaTemplate()})
	w := world.New(world.Config{}, log, nil, d.Handle) // default 4096 grid: the castle sits near (2500,2100)
	gens := make([]*world.Generator, world.CasteloOrcGenLast+1)
	meio := casteloOrcBlock("Meio_Orc", 2458, 2096, 2, 1)
	meio.MinuteGenerate = 1
	gens[402] = meio
	gens[casteloOrcBossGen] = casteloOrcBlock("COrc_GraoLorde", 2533, 2120, 1, 0)
	gens[casteloOrcFollowerGen] = casteloOrcBlock("COrc_Guarda", 2528, 2124, 4, 3)
	gens[casteloOrcBossGen+2] = casteloOrcBlock("COrc_Sentinela", 2470, 2133, 1, 0)
	gens[casteloOrcBossGen+3] = casteloOrcBlock("COrc_Capitao", 2499, 2118, 1, 0)
	gens[casteloOrcBossGen+4] = casteloOrcBlock("COrc_Chefe", 2533, 2157, 1, 0)
	for i, x := range []int16{2470, 2495, 2515} {
		gens[casteloOrcBossGen+5+i] = casteloOrcBlock("COrc_Tropa", x, 2105, 20, 3)
	}
	w.RegisterGenerators(gens)
	w.GenerateMob(402)
	e := &world.Entity{
		ID: 0, Mode: world.MobUser, Name: "Lider",
		Level: 330, MaxHP: 1000, HP: 1000, X: casteloOrcExit[0], Y: casteloOrcExit[1],
	}
	return d, w, &world.Session{Conn: 0, Mode: world.UserPlay}, e
}

func raiseXama(t *testing.T, d *Dispatcher, w *world.World) *world.Entity {
	t.Helper()
	d.ensureCasteloOrcNPC(w)
	npc := w.Entity(d.casteloOrcNPCID)
	if npc == nil {
		t.Fatal("o Xamã não nasceu")
	}
	return npc
}

func live(w *world.World, gen int) int {
	return w.GeneratorAt(gen).CurrentNumMob
}

// The Xamã stands at the /erion landing as a protected quest NPC — one of it,
// however often the keeper looks.
func TestCasteloOrcXamaNasceUmaVez(t *testing.T) {
	d, w, _, _ := casteloOrcFixture(t)
	npc := raiseXama(t, d, w)
	if npc.Merchant != 100 || npc.Grade != gradeCasteloOrc || !npc.NonCombatNPC {
		t.Errorf("Xamã: merchant %d grade %d não-combate %v, want 100/40/true", npc.Merchant, npc.Grade, npc.NonCombatNPC)
	}
	first := d.casteloOrcNPCID
	d.ensureCasteloOrcNPC(w)
	if d.casteloOrcNPCID != first {
		t.Errorf("o Xamã nasceu de novo (%d → %d) sem ter sumido", first, d.casteloOrcNPCID)
	}
	w.DespawnMob(first, 0)
	d.ensureCasteloOrcNPC(w)
	if e := w.Entity(d.casteloOrcNPCID); e == nil || e.Mode == world.MobEmpty {
		t.Error("o Xamã sumiu e não voltou")
	}
}

func TestCasteloOrcSemChaveNaoAbre(t *testing.T) {
	d, w, s, e := casteloOrcFixture(t)
	d.casteloOrcQuestNPC(w, s, e, raiseXama(t, d, w))
	if d.casteloOrc.active {
		t.Fatal("abriu sem a chave do castelo")
	}
	if live(w, casteloOrcBossGen) != 0 {
		t.Error("o boss nasceu sem a corrida")
	}
}

// The key opens the castle for fifteen minutes: the open-world orcs go, and the
// quest's monsters rise — the boss, the four followers, the guardians and every
// troop block filled to its cap.
func TestCasteloOrcChaveAbreOCastelo(t *testing.T) {
	d, w, s, e := casteloOrcFixture(t)
	if live(w, 402) == 0 {
		t.Fatal("fixture: o bloco do castelo aberto devia estar povoado")
	}
	e.Carry[3] = world.Item{Index: itemChaveCasteloOrc}
	d.casteloOrcQuestNPC(w, s, e, raiseXama(t, d, w))

	r := d.casteloOrc
	if !r.active || r.secondsLeft != casteloOrcRunSeconds || r.leaderName != "Lider" {
		t.Fatalf("corrida = %+v, want ativa com %d s", r, casteloOrcRunSeconds)
	}
	if e.Carry[3].Index != 0 {
		t.Error("a chave do castelo não foi consumida")
	}
	if live(w, 402) != 0 {
		t.Error("os orcs do castelo aberto ficaram lá dentro")
	}
	if live(w, casteloOrcBossGen) != 1 || live(w, casteloOrcFollowerGen) != 4 {
		t.Errorf("boss %d, seguidores %d; want 1 e 4", live(w, casteloOrcBossGen), live(w, casteloOrcFollowerGen))
	}
	for gen := casteloOrcBossGen + 5; gen <= world.CasteloOrcGenLast; gen++ {
		if live(w, gen) != 20 {
			t.Errorf("bloco de tropa %d com %d mobs, want 20", gen, live(w, gen))
		}
	}
}

// One party at a time: a second leader is told to wait and keeps the key; a
// member cannot open it for the party.
func TestCasteloOrcUmGrupoPorVez(t *testing.T) {
	d, w, s, e := casteloOrcFixture(t)
	npc := raiseXama(t, d, w)
	e.Carry[0] = world.Item{Index: itemChaveCasteloOrc}
	d.casteloOrcQuestNPC(w, s, e, npc)

	outro := &world.Entity{ID: 7, Mode: world.MobUser, Name: "Outro", HP: 1000}
	outro.Carry[0] = world.Item{Index: itemChaveCasteloOrc}
	d.casteloOrcQuestNPC(w, &world.Session{Conn: 7, Mode: world.UserPlay}, outro, npc)
	if outro.Carry[0].Index != itemChaveCasteloOrc || d.casteloOrc.leaderName != "Lider" {
		t.Error("um segundo grupo entrou ou perdeu a chave com o castelo ocupado")
	}

	d2, w2, s2, membro := casteloOrcFixture(t)
	membro.Leader = 5
	membro.Carry[0] = world.Item{Index: itemChaveCasteloOrc}
	d2.casteloOrcQuestNPC(w2, s2, membro, raiseXama(t, d2, w2))
	if d2.casteloOrc.active || membro.Carry[0].Index != itemChaveCasteloOrc {
		t.Error("um membro de grupo abriu o castelo")
	}
}

// During a run only the party walks in; outside it, and outside the castle,
// nothing is gated.
func TestCasteloOrcSoOGrupoEntra(t *testing.T) {
	d, _, _, _ := casteloOrcFixture(t)
	dentro, fora := casteloOrcEntry, casteloOrcExit
	if !d.casteloOrcMoveAllowed(7, dentro[0], dentro[1]) {
		t.Error("sem corrida, o castelo recusou alguém")
	}
	d.casteloOrc = casteloOrcRun{active: true, secondsLeft: 100, party: []int{0, 3}}
	if !d.casteloOrcMoveAllowed(3, dentro[0], dentro[1]) {
		t.Error("um membro do grupo foi barrado")
	}
	if d.casteloOrcMoveAllowed(7, dentro[0], dentro[1]) {
		t.Error("um estranho entrou no castelo durante a corrida")
	}
	if !d.casteloOrcMoveAllowed(7, fora[0], fora[1]) {
		t.Error("um estranho foi barrado fora do castelo")
	}
	if !d.casteloOrcSuppresses(402) || d.casteloOrcSuppresses(1000) {
		t.Error("a corrida devia segurar só os blocos do castelo aberto")
	}
}

// Warps 2 and 3 of the Armia hunting scroll land inside the castle: during a run
// a stranger keeps the scroll and stays put.
func TestCasteloOrcPedidoDeCacaNaoFuraACorrida(t *testing.T) {
	d, w, _, _ := casteloOrcFixture(t)
	d.casteloOrc = casteloOrcRun{active: true, secondsLeft: 100, party: []int{0}}
	estranho := &world.Entity{ID: 7, Mode: world.MobUser, HP: 1000}
	estranho.Carry[0] = world.Item{Index: itemHuntingScrollBase}
	d.useHuntingScroll(w, &world.Session{Conn: 7, Mode: world.UserPlay}, estranho, 0, 2)
	if estranho.Carry[0].Index != itemHuntingScrollBase {
		t.Error("o Pedido de Caça foi gasto para entrar no castelo ocupado")
	}
}

// The boss down cuts the clock to the loot window; the clock running out ends
// the run and takes the quest's monsters with it.
func TestCasteloOrcBossETempo(t *testing.T) {
	d, w, s, e := casteloOrcFixture(t)
	e.Carry[0] = world.Item{Index: itemChaveCasteloOrc}
	d.casteloOrcQuestNPC(w, s, e, raiseXama(t, d, w))
	var boss *world.Entity
	w.ForEachMob(func(_ int, m *world.Entity) {
		if int(m.GenIndex) == casteloOrcBossGen {
			boss = m
		}
	})
	if boss == nil {
		t.Fatal("o boss não está no mundo")
	}
	d.casteloOrcBossKilled(w, boss)
	if !d.casteloOrc.bossDown || d.casteloOrc.secondsLeft != casteloOrcLootSeconds {
		t.Fatalf("depois do boss: %+v, want %d s de saque", d.casteloOrc, casteloOrcLootSeconds)
	}
	d.casteloOrc.secondsLeft = 2
	d.tickCasteloOrc(w)
	d.tickCasteloOrc(w)
	if d.casteloOrc.active {
		t.Fatal("o relógio zerou e a corrida continuou")
	}
	for gen := world.CasteloOrcGenFirst; gen <= world.CasteloOrcGenLast; gen++ {
		if live(w, gen) != 0 {
			t.Errorf("bloco %d ficou com %d mobs depois do fim", gen, live(w, gen))
		}
	}
	if d.casteloOrcSuppresses(402) {
		t.Error("o castelo aberto continuou segurado depois do fim")
	}
}

// The followers are topped back up every half minute while the run is on.
func TestCasteloOrcSeguidoresRenascem(t *testing.T) {
	d, w, s, e := casteloOrcFixture(t)
	e.Carry[0] = world.Item{Index: itemChaveCasteloOrc}
	d.casteloOrcQuestNPC(w, s, e, raiseXama(t, d, w))
	w.ClearGenerator(casteloOrcFollowerGen) // the party killed them all
	d.casteloOrc.sinceFollower = casteloOrcFollowerEvery - 1
	d.tickCasteloOrc(w)
	if live(w, casteloOrcFollowerGen) == 0 {
		t.Error("os seguidores não renasceram")
	}
}

// A party that left the castle does not hold it: a minute with nobody inside
// ends the run.
func TestCasteloOrcAbandonoLiberaOCastelo(t *testing.T) {
	d, w, s, e := casteloOrcFixture(t)
	e.Carry[0] = world.Item{Index: itemChaveCasteloOrc}
	d.casteloOrcQuestNPC(w, s, e, raiseXama(t, d, w))
	for range casteloOrcAbandonSeconds {
		d.tickCasteloOrc(w)
	}
	if d.casteloOrc.active {
		t.Error("o castelo ficou preso com o grupo fora")
	}
}

// sorteioFixo answers every roll with the same draw: 0 wins a one-in-n roll,
// anything else loses it.
type sorteioFixo int

func (r sorteioFixo) Intn(n int) int { return int(r) % n }

// The Hydra and Elf arenas hand out the key on a paid entry — one in four, one
// in three — and no other Quest 256 step does.
func TestCasteloOrcChaveNaEntradaDaQuest(t *testing.T) {
	for _, tc := range []struct {
		name  string
		step  quest256Step
		roll  sorteioFixo
		chave bool
	}{
		{"hidras, sorteio ganho", quest256Steps[3], 0, true},
		{"hidras, sorteio perdido", quest256Steps[3], 1, false},
		{"elfos, sorteio ganho", quest256Steps[4], 0, true},
		{"elfos, sorteio perdido", quest256Steps[4], 2, false},
		{"coveiro nunca dá", quest256Steps[0], 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, w, killer := mobKilledWorld(t)
			d.eventRNG = tc.roll
			d.casteloOrcKeyOnEntry(w, &world.Session{Conn: killer.ID, Mode: world.UserPlay}, killer, tc.step)
			if _, ok := carryHas(killer, itemChaveCasteloOrc); ok != tc.chave {
				t.Errorf("chave na bolsa = %v, want %v", ok, tc.chave)
			}
		})
	}
}

// Spending the Mana do Batedor on the Hydra arena rolls the key; the Mestre
// Grifo's free ride into the same arena never does.
func TestCasteloOrcChaveSoNaEntradaPaga(t *testing.T) {
	d, w, e := mobKilledWorld(t)
	d.eventRNG = sorteioFixo(0)
	e.Level = 300
	e.Carry[0] = world.Item{Index: itemManaDoBatedor}
	if !d.useQuest256Ticket(w, &world.Session{Conn: e.ID, Mode: world.UserPlay}, e, 0) {
		t.Fatal("o ticket das Hidras não foi tratado")
	}
	if _, ok := carryHas(e, itemChaveCasteloOrc); !ok {
		t.Error("a entrada paga nas Hidras não sorteou a chave")
	}

	d2, w2, e2 := mobKilledWorld(t)
	d2.eventRNG = sorteioFixo(0)
	e2.Level = 300
	d2.mestreGrifo(w2, &world.Session{Conn: e2.ID, Mode: world.UserPlay}, e2, e2)
	if _, ok := carryHas(e2, itemChaveCasteloOrc); ok {
		t.Error("o Mestre Grifo, que leva de graça, deu a chave")
	}
}
