package handler

import (
	"encoding/binary"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestSetBattleIgnoraJogador: SetBattle só age sobre mob (Server.cpp:8027,
// `mob >= MAX_USER`). O jogador que passa por ele — o BM líder do grupo das
// próprias evocações — continua em MobUser, sem inimigo e sem alvo.
func TestSetBattleIgnoraJogador(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 16}, log, nil, d.Handle)
	mobID := w.SpawnMobAt(world.MobSpawn{Template: plainMobTemplate("Ogro"), X: 6, Y: 5, GenIndex: -1})
	mob := w.Entity(mobID)
	dono := &world.Entity{ID: 1, Mode: world.MobUser, HP: 1000, X: 5, Y: 5}

	setBattle(w, dono.ID, dono, mob)

	if dono.Mode != world.MobUser {
		t.Errorf("Mode do jogador = %v; SetBattle não pode tirar o jogador de MobUser", dono.Mode)
	}
	if hasEnemyList(dono) || dono.Target != 0 {
		t.Errorf("jogador ganhou inimigo/alvo de mob: EnemyList %v, Target %d", dono.EnemyList, dono.Target)
	}
}

// TestGolpeDasEvocacoesContinuaChegandoAoDono é o sintoma, pelo socket: um BM
// com tigres de verdade contra um monstro que revida. Assim que o monstro
// acertava um tigre, setGroupBattle arrastava o líder do grupo do tigre — o
// próprio BM — e o Mode dele virava MobCombat. O mundo só entrega o que acontece
// em volta a quem está em MobUser, e o BM ficava cego: cada tigre dava UM golpe
// na tela e depois matava sem animação, enquanto dano e XP continuavam.
func TestGolpeDasEvocacoesContinuaChegandoAoDono(t *testing.T) {
	raiz := filepath.Join("..", "..", "..", "Release")
	templates, _, err := content.LoadBaseSummons(raiz)
	if err != nil {
		t.Skipf("árvore BaseSummon indisponível: %v", err)
	}
	// InstanceValue 5 → Tigre (BaseSummon é 1-based: 6 é o Gorila).
	spells := content.NewSkillData([]content.Spell{
		{Index: 60, ManaSpent: 10, InstanceType: 11, InstanceValue: 5, MaxTarget: 1, Name: "Evocar Tigre"},
	})
	db := newDB()
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Beast", Class: 2, X: 5, Y: 5,
		HP: 100000, MaxHP: 100000, MP: 50000, MaxMP: 50000, Damage: 200, AC: 40,
		Level: 322, Int: 100,
		LearnedSkill: 1 << 12, // 60 % 24
		BaseSpecial:  [4]int16{0, 0, 320, 0},
	}
	// Vida que não acaba no teste: o monstro precisa continuar vivo e revidando.
	alvo := plainMobTemplate("Ogro")
	const cs = 92
	binary.LittleEndian.PutUint32(alvo[cs+16:], 1_000_000_000)
	binary.LittleEndian.PutUint32(alvo[cs+24:], 1_000_000_000)
	addr, stop, clock := startServerSummonTick(t, db, spells, templates, alvo, 9, 5, 60*time.Millisecond)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	skillAttackFrame(t, c, serverTime, 1, 60, damSkill)
	pets := map[int]bool{}
	golpes := 0
	ler := func(d time.Duration) {
		deadline := time.Now().Add(d)
		for time.Now().Before(deadline) {
			h, p, ok := readMaybeHeaderRaw(t, c)
			if !ok {
				continue
			}
			switch h.Type {
			case protocol.MsgCreateMob:
				if id, _, _, _, _ := petFromCreateMob(p); id > world.MaxUser {
					pets[id] = true
				}
			case protocol.MsgAttackOne:
				if pets[int(binary.LittleEndian.Uint16(p[30:32]))] {
					golpes++
				}
			}
		}
	}
	ler(700 * time.Millisecond)
	if len(pets) == 0 {
		t.Fatal("nenhum tigre chegou ao cliente")
	}

	// O dono bate no monstro (id MaxUser, o primeiro nascido) e os tigres entram.
	clock.Add(2 * attackCadence)
	attackFrame(t, c, clock.Load(), world.MaxUser, noSkill)
	for i := 0; i < 12; i++ {
		clock.Add(mobAttackCadence)
		ler(60 * time.Millisecond)
	}
	// Com o dono cego chegavam 3 golpes (um por tigre que já estava colado) e
	// mais nada. Sem cegueira, cada tigre ao alcance bate a cada segundo.
	if golpes < 2*len(pets) {
		t.Fatalf("o dono viu %d golpes de %d tigres em 12s de luta; os golpes pararam de chegar", golpes, len(pets))
	}
}
