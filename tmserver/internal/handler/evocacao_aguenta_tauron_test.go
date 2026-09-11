package handler

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combat"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestEvocacaoAguentaDozeGolpesDeTauron trava o pedido de 2026-09-11: os Taurons
// matavam as evocações de uma vez (o Tigre caía em 2 golpes), e cada criatura
// tinha de aguentar ~12 golpes do Tauron mais forte.
//
// Mede com o template real do Tauron_Agmo (1.900 de dano), os BaseSummon reais e
// a mesma fórmula de golpe do servidor (combat.ResolveHit), em Evocação 320, onde
// a tabela é medida. A faixa 11-14 aceita a variação do sorteio do golpe e o
// arredondamento do multiplicador. O teste pega quem baixar a vida ou a AC de um
// bicho sem querer, e também quem inflar demais.
func TestEvocacaoAguentaDozeGolpesDeTauron(t *testing.T) {
	raiz := filepath.Join("..", "..", "..", "Release")
	templates, _, err := content.LoadBaseSummons(raiz)
	if err != nil {
		t.Skipf("BaseSummon indisponível: %v", err)
	}
	tauron, err := os.ReadFile(filepath.Join(raiz, "TMsrv", "run", "npc", "Tauron_Agmo"))
	if err != nil {
		t.Skipf("template Tauron_Agmo indisponível: %v", err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 256}, log, nil, d.Handle)
	tid := w.SpawnMobAt(world.MobSpawn{Template: tauron, X: 100, Y: 100, GenIndex: -1})
	tau := w.Entity(tid)

	const evocacao = 320
	nomes := []string{"Condor", "Javali", "Lobo", "Urso", "Tigre", "Gorila", "Dragão", "Succubus"}
	for i, nome := range nomes {
		pid := w.SpawnMobAt(world.MobSpawn{Template: templates[i], X: 102, Y: 100, GenIndex: -1})
		pet := w.Entity(pid)
		b := summonBonus[i]
		pet.BaseAC += evocacao * b.acEvo / 100
		pet.BaseMaxHP += evocacao * b.hpEvo / 100
		pet.AC, pet.MaxHP = pet.BaseAC, pet.BaseMaxHP

		const amostras = 2000
		soma := 0
		for k := 0; k < amostras; k++ {
			soma += combat.ResolveHit(w.Rand(), combat.HitInput{
				AttackerDamage: int(d.effectiveDamage(tau)),
				TargetAC:       int(effectiveAC(pet)),
			})
		}
		golpeMedio := float64(soma) / amostras
		golpes := float64(pet.MaxHP) / golpeMedio
		t.Logf("%-8s HP %6d AC %5d: golpe médio do Tauron %4.0f → %.1f golpes", nome, pet.MaxHP, effectiveAC(pet), golpeMedio, golpes)
		if golpes < 11 || golpes > 14 {
			t.Errorf("%s aguenta %.1f golpes do Tauron_Agmo; o pedido é ~12", nome, golpes)
		}
		w.DespawnMob(pid, 0)
	}
}
