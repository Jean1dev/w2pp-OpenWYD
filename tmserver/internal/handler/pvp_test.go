package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/combatrule"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestPerfuracao(t *testing.T) {
	jogador := &world.Entity{ID: 2}
	mob := &world.Entity{ID: world.MaxUser + 1}
	evocacao := &world.Entity{ID: world.MaxUser + 2, Clan: 4}
	tests := []struct {
		name     string
		alvo     *world.Entity
		dmg, lam int
		want     int
	}{
		{"jogador toma um quarto", jogador, 1000, 0, 250},
		{"monstro toma inteiro", mob, 1000, 0, 1000},
		{"evocação toma um quarto", evocacao, 1000, 0, 250},
		// dam = Dam[1] + (dam >> 2): o proc já está dentro de dam e ainda volta inteiro.
		{"lâmina aérea fica inteira", jogador, 1098, 98, 98 + 1098/4},
		{"golpe miúdo não vira zero", jogador, 3, 0, 1},
		{"erro passa como veio", jogador, -3, 0, -3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := perfuracao(tt.alvo, tt.alvo.ID, tt.dmg, tt.lam); got != tt.want {
				t.Errorf("perfuracao = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRegraPvPDoPainel(t *testing.T) {
	d := New(Config{})
	if got := d.applyPvPRule(1000, true); got != 1000 {
		t.Errorf("com a regra padrão (100%%) a skill em jogador = %d, want 1000", got)
	}
	d.combatRules.PvPSkillPct, d.combatRules.PvPMeleePct = 40, 60
	if got := d.applyPvPRule(1000, true); got != 400 {
		t.Errorf("skill a 40%% = %d, want 400", got)
	}
	if got := d.applyPvPRule(1000, false); got != 600 {
		t.Errorf("golpe físico a 60%% = %d, want 600", got)
	}
	if got := d.applyPvPRule(1, true); got != 1 {
		t.Errorf("golpe de 1 = %d, want 1 (acertou, não pode virar erro)", got)
	}
}

// TestAtaqueEDefesaPvP: os efeitos do catálogo contam, refinados; a marca de
// guilda gravada no próprio item (os mesmos dois códigos de efeito) não conta.
func TestAtaqueEDefesaPvP(t *testing.T) {
	const lanca, set, capa = 1008, 3862, 4000
	d := New(Config{
		ItemEffects: map[int][]content.BaseEffect{
			lanca: {{Eff: efHWordGuild, Val: 60}},
			set:   {{Eff: efLWordGuild, Val: 12}},
		},
	})
	atacante := &world.Entity{ID: 1}
	atacante.Equip[weaponSlotR] = world.Item{Index: lanca}
	// Capa com a guilda 0x3F21 carimbada: vira 0x3F e 0x21 nos dois efeitos.
	atacante.Equip[1] = world.Item{Index: capa, Effects: [3]world.Effect{{Effect: efHWordGuild, Value: 0x3F}, {Effect: efLWordGuild, Value: 0x21}}}
	alvo := &world.Entity{ID: 2}
	alvo.Equip[4] = world.Item{Index: set}

	if got := d.pvpAttackPct(atacante); got != 6 {
		t.Errorf("Ataque PvP = %d%%, want 6%% ((60+1)/10, sem a capa)", got)
	}
	if got := d.pvpDefensePct(alvo); got != 1 {
		t.Errorf("Defesa PvP = %d%%, want 1%% ((12+1)/10)", got)
	}
	if got := d.pvpDefensePct(atacante); got != 0 {
		t.Errorf("a capa carimbada deu %d%% de Defesa PvP, want 0", got)
	}
	// 1000 + 1000/100×6 = 1060; − 1060/100×1 = 1050.
	if got := d.applyPvPStats(atacante, alvo, 1000); got != 1050 {
		t.Errorf("golpe com Ataque 6%% e Defesa 1%% = %d, want 1050", got)
	}
}

func TestReducaoFixa(t *testing.T) {
	const grau8 = 2000
	d := New(Config{ItemGrades: map[int]int{grau8: 8}})
	bm := &world.Entity{ID: 2, Class: 2, LearnedSkill: 1 << 17}
	bm.Special[3] = 179
	bm.Equip[2] = world.Item{Index: grau8}
	// (179+1)/6 = 30 do Coração de Lobo + 20 do item de grau 8.
	if got := d.reflectDamage(bm); got != 50 {
		t.Errorf("redução fixa = %d, want 50", got)
	}
	if got := d.applyPvPStats(&world.Entity{ID: 1}, bm, 40); got != 1 {
		t.Errorf("golpe menor que a redução = %d, want 1", got)
	}
}

// TestGolpePvPTomaUmQuarto passa pelo servidor inteiro: um golpe físico de
// jogador em jogador sai com um quarto do que sairia. Ataque 200 contra defesa
// 40 (×3 em jogador) dá pelo menos 138 sem o quarto — com ele, no máximo 77,
// mesmo com o crítico dobrando.
func TestGolpePvPTomaUmQuarto(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Hero", X: 5, Y: 5,
		HP: 1000, MaxHP: 1000, Damage: 200, AC: 40,
	}
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	atacante := enterWorld(t, addr)
	defer atacante.Close()
	alvo := enterWorld(t, addr)
	defer alvo.Close()

	send(t, atacante, protocol.MsgPKMode, protocol.EncodeStandardParm(1))
	attackFrame(t, atacante, serverTime, 2, 0)

	ty, payload, ok := readMaybe(t, alvo)
	if !ok || ty != protocol.MsgAttack {
		t.Fatalf("alvo recebeu %#x ok=%v, want o MsgAttack", ty, ok)
	}
	var got protocol.MsgAttackBody
	if err := got.Decode(payload); err != nil {
		t.Fatal(err)
	}
	if len(got.Dam) != 1 || got.Dam[0].TargetID != 2 {
		t.Fatalf("Dam = %+v", got.Dam)
	}
	if d := got.Dam[0].Damage; d <= 0 || d > 77 {
		t.Errorf("golpe PvP = %d, want entre 1 e 77 (um quarto de ≥138)", d)
	}
}

// TestRegraPadraoMantemOLegadoNoPvP: sem ninguém mexer no painel, o PvP é o
// quarto do legado e nada mais.
func TestRegraPadraoMantemOLegadoNoPvP(t *testing.T) {
	if r := combatrule.Default(); r.PvPSkillPct != 100 || r.PvPMeleePct != 100 {
		t.Errorf("padrão PvP = %d/%d, want 100/100 (só o quarto do legado)", r.PvPSkillPct, r.PvPMeleePct)
	}
}
