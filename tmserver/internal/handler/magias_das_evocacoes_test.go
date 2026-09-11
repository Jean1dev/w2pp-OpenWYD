package handler

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func skillDataReal(t *testing.T) *content.SkillData {
	t.Helper()
	spells, err := content.LoadSkillData(filepath.Join("..", "..", "..", "Release", "Common", "SkillData.csv"))
	if err != nil {
		t.Skipf("SkillData.csv indisponível: %v", err)
	}
	return spells
}

// TestLancaDeGeloDeixaOMonstroLento: a lentidão da Lança de Gelo do Dragão
// segura o monstro de verdade — golpe mais espaçado e passo em ticks alternados.
// Antes o afeto entrava e o monstro corria e batia igual, porque a IA de monstro
// não olhava velocidade nenhuma.
func TestLancaDeGeloDeixaOMonstroLento(t *testing.T) {
	spells := skillDataReal(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, Spells: spells})
	w := world.New(world.Config{GridDim: 64}, log, nil, d.Handle)
	mid := w.SpawnMobAt(world.MobSpawn{Template: plainMobTemplate("Ogro"), X: 21, Y: 20, GenIndex: -1})
	pid := w.SpawnMobAt(world.MobSpawn{Template: plainMobTemplate("Dragao"), X: 20, Y: 20, GenIndex: -1})
	mob, pet := w.Entity(mid), w.Entity(pid)
	pet.Summoner = 7

	if got := cadenciaDoGolpe(mob); got != mobAttackCadence {
		t.Fatalf("monstro sem afeto: cadência %d, esperado %d", got, mobAttackCadence)
	}
	d.applyMobSkill(w, pet, mob, mobSkill{index: 34})
	if mob.AffRunSpeed >= 0 {
		t.Fatalf("a Lança de Gelo não tirou corrida do monstro (AffRunSpeed %d)", mob.AffRunSpeed)
	}
	if got := cadenciaDoGolpe(mob); got <= mobAttackCadence {
		t.Errorf("monstro lento: cadência %d, tinha de passar de %d", got, mobAttackCadence)
	}
	presos := 0
	for tick := 0; tick < 10; tick++ {
		d.tickCount = tick
		if d.passoPresoPelaLentidao(mid, mob) {
			presos++
		}
	}
	if presos != 5 {
		t.Errorf("monstro lento perdeu %d de 10 passos; meia velocidade é 5", presos)
	}
	pet.AffRunSpeed = 0
	if d.passoPresoPelaLentidao(pid, pet) {
		t.Error("quem não está lento não pode perder passo")
	}
}

// TestMeteoroDaSuccubusAcertaEmVolta: a Tempestade de Meteoros do pet acerta os
// monstros até 2 casas do alvo, cada um com o próprio golpe. Ficam de fora o
// monstro mais longe, a outra evocação e o próprio alvo (que já leva o golpe
// principal). A Lança de Gelo, de alvo único, não espalha.
func TestMeteoroDaSuccubusAcertaEmVolta(t *testing.T) {
	spells := skillDataReal(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, Spells: spells})
	w := world.New(world.Config{GridDim: 64}, log, nil, d.Handle)
	spawn := func(nome string, x, y int16) *world.Entity {
		id := w.SpawnMobAt(world.MobSpawn{Template: plainMobTemplate(nome), X: x, Y: y, GenIndex: -1})
		if id < 0 {
			t.Fatalf("não nasceu %s em (%d,%d)", nome, x, y)
		}
		e := w.Entity(id)
		e.HP, e.MaxHP = 100000, 100000
		return e
	}
	alvo := spawn("Ogro", 20, 20)
	perto1 := spawn("Ogro", 21, 21)
	perto2 := spawn("Ogro", 22, 18)
	longe := spawn("Ogro", 23, 20)
	succubus := spawn("Succubus", 19, 20)
	succubus.Summoner = 7
	succubus.Clan = summonClan
	outroPet := spawn("Tigre", 20, 21)
	outroPet.Summoner = 7
	outroPet.Clan = summonClan
	succubus.Damage = 500

	area := d.golpesDaArea(w, succubus.ID, succubus, alvo, mobSkill{index: 35})
	atingidos := map[int]bool{}
	for _, a := range area {
		atingidos[a.alvo.ID] = true
		if a.dano > 0 && a.alvo.HP != 100000-int32(a.dano) {
			t.Errorf("%d levou %d no pacote e ficou com %d de vida", a.alvo.ID, a.dano, a.alvo.HP)
		}
	}
	for _, e := range []*world.Entity{perto1, perto2} {
		if !atingidos[e.ID] {
			t.Errorf("o monstro em (%d,%d), a até 2 casas do alvo, não foi atingido", e.X, e.Y)
		}
	}
	for nome, e := range map[string]*world.Entity{"o alvo principal": alvo, "o monstro a 3 casas": longe, "a outra evocação": outroPet, "a própria Succubus": succubus} {
		if atingidos[e.ID] {
			t.Errorf("%s entrou na área", nome)
		}
	}
	if got := d.golpesDaArea(w, succubus.ID, succubus, alvo, mobSkill{index: 34}); len(got) != 0 {
		t.Errorf("Lança de Gelo é de alvo único e espalhou em %d", len(got))
	}
	if got := d.golpesDaArea(w, succubus.ID, succubus, alvo, mobSkill{index: 51}); len(got) != 0 {
		t.Errorf("Enfraquecer é afeto, não dano, e espalhou em %d", len(got))
	}
}

// TestMeteoroChegaAoClienteEmArea é o mesmo meteoro pelo socket: a Succubus de
// verdade, com a barra fixa na Tempestade de Meteoros, cercada de monstros. O
// dono tem de receber o golpe como MSG_Attack (0x367), SkillIndex 35, com as
// treze entradas — o cliente lê treze no 0x367 sempre, e as que sobram vão
// zeradas — e mais de um monstro atingido.
func TestMeteoroChegaAoClienteEmArea(t *testing.T) {
	raiz := filepath.Join("..", "..", "..", "Release")
	templates, _, err := content.LoadBaseSummons(raiz)
	if err != nil {
		t.Skipf("BaseSummon indisponível: %v", err)
	}
	spells := skillDataReal(t)
	// Barra inteira no meteoro: toda banda do sorteio cai nele.
	var succ []byte
	for i, tpl := range templates {
		if bytes.HasPrefix(tpl, []byte("Succubus")) {
			tpl = append([]byte(nil), tpl...)
			copy(tpl[796:800], []byte{35, 35, 35, 255})
			templates[i] = tpl
			succ = tpl
		}
	}
	if succ == nil {
		t.Skip("template Succubus não encontrado")
	}
	const evocarSuccubus = 63
	db := newDB()
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Beast", Class: 2, X: 5, Y: 5,
		HP: 100000, MaxHP: 100000, MP: 50000, MaxMP: 50000, Damage: 200, AC: 40,
		Level: 399, Int: 100,
		LearnedSkill: 1 << (evocarSuccubus % 24),
		BaseSpecial:  [4]int16{0, 0, 320, 0},
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	clock := &atomic.Uint32{}
	clock.Store(serverTime)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, Spells: spells, SummonMobs: templates})
	w := world.New(world.Config{GridDim: 16, Now: clock.Load}, log, db, d.Handle)
	alvo := plainMobTemplate("Ogro")
	const cs = 92
	binary.LittleEndian.PutUint32(alvo[cs+16:], 1_000_000_000)
	binary.LittleEndian.PutUint32(alvo[cs+24:], 1_000_000_000)
	for _, p := range [][2]int16{{9, 5}, {10, 5}, {10, 6}, {9, 4}, {10, 4}} {
		w.SpawnMobAt(world.MobSpawn{Template: alvo, X: p[0], Y: p[1], GenIndex: -1})
	}
	w.SetTickHandler(60*time.Millisecond, d.Tick)
	w.SetSessionEndHandler(d.SessionEnd)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	defer func() { cancel(); <-done }()

	c := enterWorld(t, ln.Addr().String())
	defer c.Close()
	skillAttackFrame(t, c, serverTime, 1, evocarSuccubus, damSkill)
	pets := map[int]bool{}
	meteoros := 0
	ler := func(dur time.Duration) {
		fim := time.Now().Add(dur)
		for time.Now().Before(fim) {
			h, p, ok := readMaybeHeaderRaw(t, c)
			if !ok {
				continue
			}
			switch h.Type {
			case protocol.MsgCreateMob:
				if id, _, _, _, _ := petFromCreateMob(p); id >= world.MaxUser+5 {
					pets[id] = true
				}
			case protocol.MsgAttack:
				atk := int(binary.LittleEndian.Uint16(p[30:32]))
				if !pets[atk] {
					continue
				}
				if want := protocol.MsgAttackDamOffset + protocol.MaxTarget*protocol.MsgAttackDamStride; len(p) != want {
					t.Fatalf("golpe de área com %d bytes; o 0x367 tem de ter as 13 entradas (%d)", len(p), want)
				}
				if sk := int16(binary.LittleEndian.Uint16(p[44:46])); sk != 35 {
					t.Fatalf("golpe de área com SkillIndex %d, esperado 35", sk)
				}
				alvos := 0
				for i := 0; i < protocol.MaxTarget; i++ {
					if binary.LittleEndian.Uint32(p[protocol.MsgAttackDamOffset+i*protocol.MsgAttackDamStride:]) != 0 {
						alvos++
					}
				}
				if alvos >= 2 {
					meteoros++
				}
			}
		}
	}
	ler(700 * time.Millisecond)
	if len(pets) == 0 {
		t.Fatal("nenhuma Succubus chegou ao cliente")
	}
	clock.Add(2 * attackCadence)
	attackFrame(t, c, clock.Load(), world.MaxUser, noSkill)
	for i := 0; i < 12 && meteoros == 0; i++ {
		clock.Add(mobAttackCadence)
		ler(60 * time.Millisecond)
	}
	if meteoros == 0 {
		t.Fatal("nenhum meteoro em área chegou ao dono em 12s de luta")
	}
}
