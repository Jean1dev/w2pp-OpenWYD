package handler

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestEvocacaoDoMesmoGrupo: protegida é só a evocação do grupo do atacante — a
// dele e a do companheiro. O pet de um estranho e o monstro comum continuam
// alvos.
func TestEvocacaoDoMesmoGrupo(t *testing.T) {
	bm := &world.Entity{ID: 7}
	companheiro := &world.Entity{ID: 8, Leader: 7}
	estranho := &world.Entity{ID: 9}
	casos := []struct {
		nome      string
		atacante  *world.Entity
		alvo      *world.Entity
		protegido bool
	}{
		{"pet do próprio BM", bm, &world.Entity{ID: world.MaxUser + 1, Summoner: 7, Leader: 7}, true},
		{"BM acerta o pet do companheiro", bm, &world.Entity{ID: world.MaxUser + 2, Summoner: 8, Leader: 7}, true},
		{"companheiro acerta o pet do BM", companheiro, &world.Entity{ID: world.MaxUser + 1, Summoner: 7, Leader: 7}, true},
		{"estranho acerta o pet do BM", estranho, &world.Entity{ID: world.MaxUser + 1, Summoner: 7, Leader: 7}, false},
		{"monstro comum", bm, &world.Entity{ID: world.MaxUser + 3}, false},
		{"jogador do grupo não é evocação", bm, companheiro, false},
	}
	for _, c := range casos {
		if got := evocacaoDoMesmoGrupo(c.atacante, c.alvo); got != c.protegido {
			t.Errorf("%s: evocacaoDoMesmoGrupo = %v, esperado %v", c.nome, got, c.protegido)
		}
	}
}

// TestBMEmModoPKNaoFereAsPropriasEvocacoes: em modo PK o cliente põe os pets do
// BM entre os alvos, no golpe físico e na magia de área. O servidor zera o dano
// em evocação do mesmo grupo (_MSG_Attack.cpp:1334, `leader == mobleader`).
func TestBMEmModoPKNaoFereAsPropriasEvocacoes(t *testing.T) {
	raiz := filepath.Join("..", "..", "..", "Release")
	templates, _, err := content.LoadBaseSummons(raiz)
	if err != nil {
		t.Skipf("árvore BaseSummon indisponível: %v", err)
	}
	const evocar, area = 60, 61
	spells := content.NewSkillData([]content.Spell{
		{Index: evocar, ManaSpent: 10, InstanceType: 11, InstanceValue: 5, MaxTarget: 1, Name: "Evocar Tigre"},
		{Index: area, ManaSpent: 10, Aggressive: 1, InstanceType: 1, InstanceValue: 500, MaxTarget: 12, TargetType: 1, Name: "Magia de area"},
	})
	db := newDB()
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Beast", Class: 2, X: 5, Y: 5,
		HP: 100000, MaxHP: 100000, MP: 50000, MaxMP: 50000, Damage: 5000, AC: 40,
		Level: 322, Int: 100,
		LearnedSkill: 1<<(evocar%24) | 1<<(area%24),
		BaseSpecial:  [4]int16{0, 0, 320, 0},
	}
	addr, stop, clock := startServerSummonTick(t, db, spells, templates, nil, 0, 0, time.Hour)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	skillAttackFrame(t, c, serverTime, 1, evocar, damSkill)
	var pets []int
	deadline := time.Now().Add(700 * time.Millisecond)
	for time.Now().Before(deadline) {
		ty, p, ok := readMaybeRaw(t, c)
		if ok && ty == protocol.MsgCreateMob {
			if id, _, _, _, _ := petFromCreateMob(p); id >= world.MaxUser {
				pets = append(pets, id)
			}
		}
	}
	if len(pets) < 2 {
		t.Fatalf("esperava tigres em campo, vieram %d", len(pets))
	}
	send(t, c, protocol.MsgPKMode, protocol.EncodeStandardParm(1))

	// O eco do próprio golpe diz o dano que o servidor aplicou em cada alvo.
	danoNoEco := func() []protocol.DamEntry {
		t.Helper()
		limite := time.Now().Add(time.Second)
		for time.Now().Before(limite) {
			ty, p, ok := readMaybeRaw(t, c)
			if !ok || (ty != protocol.MsgAttack && ty != protocol.MsgAttackOne && ty != protocol.MsgAttackTwo) {
				continue
			}
			var b protocol.MsgAttackBody
			if err := b.Decode(p); err != nil {
				t.Fatal(err)
			}
			// O teste só manda golpes do BM contra os tigres; um tigre não bate em
			// ninguém aqui (não há monstro), então todo golpe com alvo é o eco.
			if len(b.Dam) > 0 && b.Dam[0].TargetID >= world.MaxUser {
				return b.Dam
			}
		}
		t.Fatal("o eco do golpe do BM não chegou")
		return nil
	}

	tick := serverTime + 2*attackCadence
	clock.Store(tick)
	attackFrame(t, c, tick, pets[0], noSkill)
	for _, dm := range danoNoEco() {
		if int(dm.TargetID) == pets[0] && dm.Damage != 0 {
			t.Errorf("golpe físico em modo PK tirou %d do próprio tigre %d", dm.Damage, pets[0])
		}
	}

	tick += 2 * attackCadence
	clock.Store(tick)
	body := protocol.MsgAttackBody{SkillIndex: area}
	for _, id := range pets[:2] {
		body.Dam = append(body.Dam, protocol.DamEntry{TargetID: int32(id), Damage: damSkill})
	}
	wire, err := protocol.Encode(protocol.Header{Type: protocol.MsgAttack, ClientTick: tick}, body.Encode(), 9)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write(wire); err != nil {
		t.Fatal(err)
	}
	for _, dm := range danoNoEco() {
		if dm.TargetID != 0 && dm.Damage != 0 {
			t.Errorf("magia de área em modo PK tirou %d do próprio tigre %d", dm.Damage, dm.TargetID)
		}
	}
}
