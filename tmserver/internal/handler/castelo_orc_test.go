package handler

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/droprule"
	"github.com/jeanluca/w2pp-openwyd/internal/migrations"
	"github.com/jeanluca/w2pp-openwyd/internal/npcgener"
	"github.com/jeanluca/w2pp-openwyd/internal/npctemplate"
	"github.com/jeanluca/w2pp-openwyd/internal/savefmt"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const (
	itemAmuletoDePrata = 551
)

// The quest pays in loot, never in experience — and the same template under
// any other file name still pays as it always did.
func TestCasteloOrcNaoDaXP(t *testing.T) {
	tmpl := expMobTemplate(1, 1000, 0)

	d, w, killer := mobKilledWorld(t)
	d.mobKilled(w, killer, spawnNamed(t, w, tmpl, "COrc_Cavaleiro"))
	if killer.Exp != 0 {
		t.Errorf("matar COrc_Cavaleiro deu %d de XP, want 0", killer.Exp)
	}

	d, w, killer = mobKilledWorld(t)
	d.mobKilled(w, killer, spawnNamed(t, w, tmpl, "Orc_Cavaleiro"))
	if killer.Exp == 0 {
		t.Error("o Orc_Cavaleiro de Erion parou de dar XP")
	}
}

// Every amulet the Grão-Lorde drops leaves refinable (+0) and with exactly one
// add from the design's table, inside its range.
func TestCasteloOrcAmuletoSaiComUmAdd(t *testing.T) {
	d, w, killer := mobKilledWorld(t)
	d.dropRules = droprule.NewTable([]droprule.Rule{{Mob: "COrc_GraoLorde", Item: itemAmuletoDePrata, Chance: droprule.MaxChance}})
	faixa := map[uint8][3]int{}
	for _, a := range casteloOrcAmuletAdds {
		faixa[a.effect] = [3]int{a.min, a.max, a.step}
	}
	visto := map[uint8]bool{}
	for kill := 0; kill < 40; kill++ {
		for i := range killer.Carry {
			killer.Carry[i] = world.Item{}
		}
		d.mobKilled(w, killer, spawnNamed(t, w, expMobTemplate(350, 0, 0), "COrc_GraoLorde"))
		it, ok := carryHas(killer, itemAmuletoDePrata)
		if !ok {
			t.Fatal("o Grão-Lorde não derrubou o amuleto a 100%")
		}
		if it.Effects[0] != (world.Effect{Effect: efSanc, Value: 0}) {
			t.Errorf("slot 0 do amuleto = %+v, want EF_SANC +0", it.Effects[0])
		}
		add := it.Effects[1]
		f, ok := faixa[add.Effect]
		if !ok || int(add.Value) < f[0] || int(add.Value) > f[1] || (int(add.Value)-f[0])%f[2] != 0 {
			t.Errorf("add %+v fora da tabela do design", add)
		}
		if it.Effects[2] != (world.Effect{}) {
			t.Errorf("slot 2 do amuleto = %+v, want vazio (um add só)", it.Effects[2])
		}
		visto[add.Effect] = true
	}
	if len(visto) < 2 {
		t.Errorf("40 amuletos e só %d tipo(s) de add: o sorteio não está escolhendo a linha", len(visto))
	}
}

// The amulet's critical is 1% or 2% on the tooltip — the byte 10 or 20, never
// the 1-2 that read "0.2%" in game — and both come up.
func TestCasteloOrcAmuletoCriticoUmOuDoisPorCento(t *testing.T) {
	_, w, _ := mobKilledWorld(t)
	var crit []addRoll
	for _, a := range casteloOrcAmuletAdds {
		if a.effect == efCritical {
			crit = append(crit, a)
		}
	}
	if len(crit) != 1 {
		t.Fatalf("%d linhas de crítico na tabela do amuleto, want 1", len(crit))
	}
	visto := map[uint8]bool{}
	for range 40 {
		it := world.Item{Index: itemAmuletoDePrata}
		stampAccessoryAdd(w, &it, crit)
		if v := it.Effects[1].Value; it.Effects[1].Effect != efCritical || (v != 10 && v != 20) {
			t.Fatalf("add de crítico = %+v, want 10 ou 20 (1%% ou 2%%)", it.Effects[1])
		}
		visto[it.Effects[1].Value] = true
	}
	if !visto[10] || !visto[20] {
		t.Errorf("40 sorteios e só saiu %v", visto)
	}
}

// Outside the quest nothing changes: the same amulet from another monster
// keeps coming out bare.
func TestCasteloOrcNaoMexeNoDropDeOutroMonstro(t *testing.T) {
	d, w, killer := mobKilledWorld(t)
	d.dropRules = droprule.NewTable([]droprule.Rule{{Mob: "Chefe_Teste", Item: itemAmuletoDePrata, Chance: droprule.MaxChance}})
	d.mobKilled(w, killer, spawnNamed(t, w, expMobTemplate(350, 0, 0), "Chefe_Teste"))
	it, ok := carryHas(killer, itemAmuletoDePrata)
	if !ok {
		t.Fatal("o amuleto não caiu")
	}
	if it.Effects != ([3]world.Effect{}) {
		t.Errorf("amuleto de fora da quest ganhou efeitos %+v", it.Effects)
	}
}

// casteloOrcDesign is the quest as designed (Atlas de Quests, 2026-09-11): the
// numbers a Mortal party of 320-400 at +6..+9 was calibrated against. A change
// here is a balance change, and should be one on purpose.
//
// The Dano column assumes the monster swing reads the player's armour as it is
// (dano − AC/2) against the legacy's doubled player HP (65346fe8). It was 2300-2700
// while the port tripled that armour; a change to either rule moves all of it.
var casteloOrcDesign = map[string]struct {
	name             string
	lvl, hp, ac, dmg int32
	res              int8
	key              int16
}{
	"COrc_GraoLorde": {"Grão-Lorde Orc", 350, 3000000, 3000, 2020, 25, 0},
	"COrc_Guarda":    {"Guarda do Lorde", 320, 150000, 2200, 1520, 15, 0},
	"COrc_Sentinela": {"Sentinela Orc", 330, 450000, 2400, 1620, 20, 466},
	"COrc_Capitao":   {"Capitão Orc", 330, 450000, 2400, 1620, 20, 467},
	"COrc_Chefe":     {"Chefe Orc", 330, 450000, 2400, 1620, 20, 469},
	"COrc_Cavaleiro": {"Cavaleiro Orc", 300, 18000, 1800, 1220, 10, 0},
	"COrc_Arqueiro":  {"Arqueiro Orc", 300, 18000, 1800, 1220, 10, 0},
	"COrc_MeioOrc":   {"Meio Orc", 300, 18000, 1800, 1220, 10, 0},
}

func releaseDir(t *testing.T) string {
	t.Helper()
	root := filepath.Join("..", "..", "..", "Release")
	if _, _, err := npctemplate.Load(root, "COrc_GraoLorde"); err != nil {
		t.Skipf("Release content tree not available: %v", err)
	}
	return root
}

// The shipped templates are the design, and nothing else rides along: no gold,
// no experience in the file, no template loot besides the guardians' keys, and
// no negative resistance (read unsigned, -20 becomes 100 and halves every
// spell — mob-bate-com-defesa-x3).
func TestCasteloOrcTemplatesBatemComODesign(t *testing.T) {
	root := releaseDir(t)
	if len(casteloOrcDesign) != len(casteloOrcTemplates) {
		t.Fatalf("%d templates no design, %d na regra do handler", len(casteloOrcDesign), len(casteloOrcTemplates))
	}
	for file, want := range casteloOrcDesign {
		if !casteloOrcTemplates[droprule.Canonical(file)] {
			t.Errorf("%s não está em casteloOrcTemplates", file)
		}
		b, res, err := npctemplate.Load(root, file)
		if err != nil {
			t.Errorf("%s: %v", file, err)
			continue
		}
		if res.Version != savefmt.MobVersionCurrent {
			t.Errorf("%s: layout %v, want o de 816 bytes", file, res.Version)
		}
		m, err := savefmt.DecodeMob(b)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		// MobName is CP1252 on the wire; every character the design uses is Latin-1.
		var got []rune
		for _, c := range m.Name {
			if c == 0 {
				break
			}
			got = append(got, rune(c))
		}
		if string(got) != want.name {
			t.Errorf("%s: nome %q, want %q", file, string(got), want.name)
		}
		c := m.CurrentScore
		if c.Level != want.lvl || c.MaxHp != want.hp || c.Hp != want.hp || c.AC != want.ac || c.Damage != want.dmg {
			t.Errorf("%s: nv %d hp %d/%d def %d dano %d, want nv %d hp %d def %d dano %d",
				file, c.Level, c.Hp, c.MaxHp, c.AC, c.Damage, want.lvl, want.hp, want.ac, want.dmg)
		}
		if m.BaseScore.MaxHp != c.MaxHp || m.BaseScore.AC != c.AC || m.BaseScore.Damage != c.Damage {
			t.Errorf("%s: BaseScore difere do CurrentScore", file)
		}
		if m.Resist != [4]int8{want.res, want.res, want.res, want.res} {
			t.Errorf("%s: resistência %v, want %d em tudo", file, m.Resist, want.res)
		}
		// Both Merchant bytes: the world reads CurrentScore's, the NPC overlay the other.
		if m.Exp != 0 || m.Coin != 0 || m.Merchant != 0 || c.Merchant != 0 || m.Clan == 4 {
			t.Errorf("%s: exp %d coin %d merchant %d/%d clan %d", file, m.Exp, m.Coin, m.Merchant, c.Merchant, m.Clan)
		}
		for slot, it := range m.Carry {
			if slot == 56 && want.key != 0 {
				if it.Index != want.key {
					t.Errorf("%s: chave %d no slot 56, want %d", file, it.Index, want.key)
				}
				continue
			}
			if it.Index != 0 {
				t.Errorf("%s: drop de template %d no slot %d; o saque é da Mesa de Drops", file, it.Index, slot)
			}
		}
	}
}

// The Grão-Lorde carries a Espada Bastarda +11 in each hand. +10 and up are not
// the plain number in EF_SANC: the client reads anything under 230 modulo 10, so
// a raw 11 showed as +1; +11 is 234..237 (protocol/visual.go).
func TestCasteloOrcBossComDuasEspadasMais11(t *testing.T) {
	root := releaseDir(t)
	b, _, err := npctemplate.Load(root, "COrc_GraoLorde")
	if err != nil {
		t.Fatal(err)
	}
	m, err := savefmt.DecodeMob(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, slot := range []int{6, 7} {
		it := m.Equip[slot]
		if it.Index != 907 || it.Effects[0].Effect != efSanc || it.Effects[0].Value < 234 || it.Effects[0].Value > 237 {
			t.Errorf("mão %d: %d %+v, want Espada Bastarda (907) com EF_SANC 234..237 (+11)", slot, it.Index, it.Effects[0])
		}
	}
}

// The Xamã is the only quest NPC on grade 40: another Merchant-100 template on
// that grade would open the castle too.
func TestCasteloOrcXamaNoConteudo(t *testing.T) {
	root := releaseDir(t)
	// Read directly rather than through npctemplate.ScanDir, whose per-file name
	// resolution lists the directory each time: 50 s on Windows for this check.
	dir := filepath.Join(root, "TMsrv", "run", "npc")
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var comGrau []string
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			t.Fatal(err)
		}
		norm, _, err := savefmt.NormalizeMob(b)
		if err != nil {
			continue // not a template; the inventory test owns that census
		}
		m, err := savefmt.DecodeMob(norm)
		if err != nil || m.CurrentScore.Merchant != 100 {
			continue
		}
		for _, ef := range m.Equip[0].Effects {
			if ef.Effect == 100 && ef.Value == gradeCasteloOrc {
				comGrau = append(comGrau, f.Name())
			}
		}
	}
	if len(comGrau) != 1 || droprule.Canonical(comGrau[0]) != droprule.Canonical(casteloOrcNPCTemplate) {
		t.Errorf("templates de quest no grau %d: %v, want só %s", gradeCasteloOrc, comGrau, casteloOrcNPCTemplate)
	}
}

// Each quest monster leads one of the quest's own blocks — which is what lets
// a GM raise it with "criar" — and none of those blocks regenerates on its own.
func TestCasteloOrcBlocosDoNPCGener(t *testing.T) {
	root := releaseDir(t)
	gens, err := npcgener.Load(filepath.Join(root, "TMsrv", "run", "NPCGener.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(gens) <= world.CasteloOrcGenLast {
		t.Fatalf("NPCGener tem %d blocos, want pelo menos %d", len(gens), world.CasteloOrcGenLast+1)
	}
	lidera := map[string]bool{}
	for idx := world.CasteloOrcGenFirst; idx <= world.CasteloOrcGenLast; idx++ {
		g := gens[idx]
		if !casteloOrcTemplates[droprule.Canonical(g.Leader)] {
			t.Errorf("bloco %d lidera %q, que não é da quest", idx, g.Leader)
		}
		if g.MinuteGenerate > 0 {
			t.Errorf("bloco %d renasce a cada %d min; os da quest não renascem sozinhos", idx, g.MinuteGenerate)
		}
		lidera[droprule.Canonical(g.Leader)] = true
	}
	for file := range casteloOrcDesign {
		if !lidera[droprule.Canonical(file)] {
			t.Errorf("%s não lidera nenhum bloco da quest: o /gm criar não o acha", file)
		}
	}
	for idx := range gens {
		if !world.IsCasteloOrcGenerator(idx) && casteloOrcTemplates[droprule.Canonical(gens[idx].Leader)] {
			t.Errorf("bloco %d, fora da faixa da quest, usa %s: nasceria no mundo aberto", idx, gens[idx].Leader)
		}
	}
}

// Every row of the quest's loot migration names one of the quest's monsters and
// an item the catalog has. A misspelled monster would be a rule that never
// fires, and the panel would still list it as in force.
func TestCasteloOrcMigracaoDeDrops(t *testing.T) {
	root := releaseDir(t)
	items, err := content.LoadItemList(filepath.Join(root, "Common", "ItemList.csv"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := migrations.FS.ReadFile("0053_castelo_orc_drops.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	rows := regexp.MustCompile(`\('([^']+)',\s*(\d+),\s*(\d+)\)`).FindAllStringSubmatch(string(b), -1)
	if len(rows) == 0 {
		t.Fatal("nenhuma linha na migração")
	}
	porMonstro := map[string]int{}
	chaveTodos, chaveEmAlgum := false, false
	for _, r := range rows {
		item, _ := strconv.Atoi(r[2])
		chance, _ := strconv.Atoi(r[3])
		rule := droprule.Rule{Mob: r[1], Item: int16(item), Chance: int32(chance)}
		if !rule.Valid() {
			t.Errorf("%v: a Mesa de Drops recusaria esta linha", r[0])
		}
		if _, ok := items.Get(item); !ok {
			t.Errorf("item %d não existe no ItemList", item)
		}
		if item == itemChaveCasteloOrc {
			// The key's rows name monsters outside the quest: each must be a real
			// template file, or the rule never fires while the panel lists it.
			switch {
			case r[1] == droprule.AllMobs:
				chaveTodos = true
			case casteloOrcTemplates[droprule.Canonical(r[1])]:
				t.Errorf("%s dá a chave da própria quest: cada corrida pagaria a seguinte", r[1])
			default:
				if _, _, err := npctemplate.Load(root, r[1]); err != nil {
					t.Errorf("a chave cai de %q, que não existe: %v", r[1], err)
				}
				chaveEmAlgum = chaveEmAlgum || chance > 0
			}
			continue
		}
		if !casteloOrcTemplates[droprule.Canonical(r[1])] {
			t.Errorf("%s não é monstro da quest", r[1])
		}
		porMonstro[r[1]]++
	}
	if !chaveTodos || !chaveEmAlgum {
		t.Errorf("a chave precisa sair de todos (%v) e voltar em algum lugar (%v)", chaveTodos, chaveEmAlgum)
	}
	for file := range casteloOrcDesign {
		if porMonstro[file] == 0 {
			t.Errorf("%s não tem saque nenhum na migração", file)
		}
	}
}
