package handler

import (
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/spawnrate"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Chefes sozinhos: blocos do NPCGener.txt sem período de minuto
// (MinuteGenerate <= 0), com até três monstros, que valem pelo menos um milhão
// de XP. No original eles nem existem fora de evento: o servidor sobe vazio e o
// timer de minuto pula bloco sem período (ProcessSecMinTimer.cpp:2721-2728). O
// rewrite popula tudo no boot e devolve o morto em 15 s (world/api.go,
// divergência deliberada), e com um monstro de 2,99 milhões isso vira uma
// fazenda a cada 15 segundos. Esses voltam em horas
// (world_event_config.boss_respawn_hours, padrão 24); o resto do mundo segue
// nos 15 s, e os blocos grandes das rotas de caça não mudam.
//
// O Kefra e os guardas ficam de fora: são semanais (kefra.go).

const (
	chefeMaxMonstros = 3
	chefeMinExp      = 1_000_000
	msPorHora        = 3_600_000
	// tamanhoStructMob is the canonical template blob ParseMobBasics reads.
	tamanhoStructMob = 816
)

// ehChefeSozinho classifies one generator from its templates as they spawn —
// the blobs already carry the moderator stat overlay (spawnNPCs applies it).
func ehChefeSozinho(g *world.Generator, idx int) bool {
	if g == nil || g.MinuteGenerate > 0 || world.IsWaterDungeonGenerator(idx) ||
		world.IsEventOwnedGenerator(idx) || world.IsKefraGenerator(idx) {
		return false
	}
	monstros := 0
	var maiorExp int64
	conta := func(tmpl []byte, n int) {
		if n <= 0 || len(tmpl) < tamanhoStructMob {
			return
		}
		b := protocol.ParseMobBasics(tmpl)
		if b.Merchant != 0 || b.Exp <= 0 {
			return
		}
		monstros += n
		maiorExp = max(maiorExp, b.Exp)
	}
	conta(g.LeaderTmpl, 1)
	conta(g.FollowerTmpl, g.MaxNumMob-1)
	return monstros > 0 && monstros <= chefeMaxMonstros && maiorExp >= chefeMinExp
}

// resolverChefes builds the lone-boss table once and logs it, so the list the
// server is running is on record at boot. A call before the content load (no
// generators yet) builds nothing and is tried again on the next death.
func (d *Dispatcher) resolverChefes(w *world.World) {
	n := w.GeneratorCount()
	if d.genChefe != nil || n == 0 {
		return
	}
	d.genChefe = make([]bool, n)
	var lista []string
	for i := 0; i < n; i++ {
		if g := w.GeneratorAt(i); ehChefeSozinho(g, i) {
			d.genChefe[i] = true
			lista = append(lista, fmt.Sprintf("%d:%s", i, g.LeaderName))
		}
	}
	d.log.Info("chefes sozinhos voltam em horas", "geradores", len(lista), "horas", d.horasDosChefes(), "lista", lista)
}

// chefeSozinho reports whether generator idx is a lone boss.
func (d *Dispatcher) chefeSozinho(w *world.World, idx int) bool {
	d.resolverChefes(w)
	return idx >= 0 && idx < len(d.genChefe) && d.genChefe[idx]
}

// horasDosChefes is the configured wait: the decided default until the first
// config arrives, or when no dbServer is wired.
func (d *Dispatcher) horasDosChefes() int32 {
	if d.chefeHoras == 0 {
		return domain.DefaultBossRespawnHours
	}
	return d.chefeHoras
}

// setChefeHoras applies the panel's value. One outside 1..168 is ignored,
// keeping the one in force — the column's CHECK already refuses it, so only a
// test fake or a broken dbServer can send it.
func (d *Dispatcher) setChefeHoras(h int32) {
	if h < domain.MinBossRespawnHours || h > domain.MaxBossRespawnHours {
		if h != 0 {
			d.log.Warn("renascimento dos chefes fora da faixa, ignorado", "horas", h, "em_vigor", d.horasDosChefes())
		}
		return
	}
	d.chefeHoras = h
}

// esperaDoRenascimento is the individual queue's wait for one generator's dead
// monster: hours for a lone boss, the desert dial over 15 s for the rest.
func (d *Dispatcher) esperaDoRenascimento(w *world.World, idx int) uint32 {
	if d.chefeSozinho(w, idx) {
		return uint32(d.horasDosChefes()) * msPorHora
	}
	return spawnrate.ScaleMillis(world.DefaultRespawnDelay, d.spawnPercentFor(w, idx))
}
