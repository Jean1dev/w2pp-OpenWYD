package world

import "testing"

// campoTemplate is a mob template with both merchant bytes set: mobMerchant at
// 17 (STRUCT_MOB.Merchant, the legacy's) and scoreMerchant at 104
// (CurrentScore.Merchant, the one this port reads).
func campoTemplate(clan, mobMerchant, scoreMerchant uint8) []byte {
	b := genMobTemplate(clan)
	b[17] = mobMerchant
	b[92+12] = scoreMerchant
	return b
}

// Dentro do campo de treino decide o byte do legado (internal/campotreino). Os
// bytes são os dos templates reais: Orc_Sniper e Aguia têm 0 no 17 e 16 no 104;
// Treinador1 36/100; os Ajudantes 120/120.
func TestCampoDeTreinoClassificaPeloByteDoLegado(t *testing.T) {
	w := New(Config{GridDim: 4096}, slogDiscard(), nil, nil)
	casos := []struct {
		nome string
		tmpl []byte
		x, y int16
		npc  bool
	}{
		{"Orc_Sniper no campo é monstro", campoTemplate(1, 0, 16), 2079, 1976, false},
		{"Águia no campo é monstro", campoTemplate(2, 0, 16), 2147, 2028, false},
		{"Treinador1 no campo continua NPC", campoTemplate(0, 36, 100), 2080, 2018, true},
		{"Ajudante no campo continua NPC", campoTemplate(0, 120, 120), 2130, 2035, true},
		{"o mesmo Orc fora do campo segue como está", campoTemplate(1, 0, 16), 2600, 1700, true},
	}
	for _, c := range casos {
		id := w.SpawnMob(c.tmpl, c.x, c.y)
		if id < 0 {
			t.Fatalf("%s: não nasceu", c.nome)
		}
		if got := w.Entity(id).NonCombatNPC; got != c.npc {
			t.Errorf("%s: NonCombatNPC = %v, want %v", c.nome, got, c.npc)
		}
	}
}

// Libertar o golpe sem a contagem é o meio conserto que o Imp_ da Água teve: o
// gerador segue contando o morto, e ele nunca volta. O Orc do campo tem de sair
// da contagem ao morrer — é o que deixa o gerador de minuto refazê-lo — e, sem
// gerador de minuto, entrar na fila de respawn como qualquer monstro.
func TestMonstroDoCampoSaiDaContagemEVolta(t *testing.T) {
	w := New(Config{GridDim: 4096}, slogDiscard(), nil, nil)
	g := &Generator{
		MinuteGenerate: 2, MaxNumMob: 1, // o bloco 1684 do NPCGener
		SegX: [5]int16{2079}, SegY: [5]int16{1976},
		LeaderTmpl: campoTemplate(1, 0, 16),
	}
	w.RegisterGenerators([]*Generator{g})
	ids := w.GenerateMob(0)
	if len(ids) != 1 || g.CurrentNumMob != 1 {
		t.Fatalf("GenerateMob = %v, CurrentNumMob = %d; want 1 e 1", ids, g.CurrentNumMob)
	}
	w.DespawnMob(ids[0], 1)
	if g.CurrentNumMob != 0 {
		t.Errorf("CurrentNumMob depois da morte = %d, want 0: o gerador ainda conta o morto", g.CurrentNumMob)
	}

	// Mundo sem gerador: a volta é a fila de respawn.
	w2 := New(Config{GridDim: 4096}, slogDiscard(), nil, nil)
	id := w2.SpawnMob(campoTemplate(1, 0, 16), 2079, 1976)
	antes := len(w2.respawnQueue)
	w2.DespawnMob(id, 1)
	if len(w2.respawnQueue) != antes+1 {
		t.Errorf("fila de respawn = %d, want %d: o Orc do campo morreu e não vai voltar", len(w2.respawnQueue), antes+1)
	}
}
