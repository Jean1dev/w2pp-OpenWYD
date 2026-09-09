package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Owning a city buys the guild its own respawn point, from anywhere on the map.
func TestSpawnDaGuildDona(t *testing.T) {
	d := &Dispatcher{}
	d.guildZones[1] = world.GuildZone{Zone: 1, ChargeGuild: 77, GuildSpawnX: 2500, GuildSpawnY: 1700}

	x, y, ok := d.guildSpawnFor(77)
	if !ok {
		t.Fatal("a guild dona da zona não recebeu ponto próprio")
	}
	if x != 2500 || y != 1700 {
		t.Errorf("ponto = (%d,%d), want (2500,1700)", x, y)
	}
}

// Quem não domina nada renasce como todo mundo.
func TestQuemNaoDominaNaoTemPonto(t *testing.T) {
	d := &Dispatcher{}
	d.guildZones[1] = world.GuildZone{Zone: 1, ChargeGuild: 77, GuildSpawnX: 2500, GuildSpawnY: 1700}

	if _, _, ok := d.guildSpawnFor(99); ok {
		t.Error("uma guild que não domina zona nenhuma ganhou ponto próprio")
	}
	// Sem guild alguma: o caso mais comum do servidor.
	if _, _, ok := d.guildSpawnFor(0); ok {
		t.Error("personagem sem guild ganhou ponto de guild")
	}
}

// Zero é "não configurado", não uma coordenada. Honrá-lo jogaria a guild
// inteira no canto do mapa — exatamente o defeito do legado, que lê os campos
// e nunca os escreve.
func TestZeroNaoEhCoordenada(t *testing.T) {
	casos := []struct {
		nome string
		x, y int32
	}{
		{"os dois zerados", 0, 0},
		{"só X zerado", 0, 1700},
		{"só Y zerado", 2500, 0},
		{"negativo", -1, 1700},
	}
	for _, c := range casos {
		d := &Dispatcher{}
		d.guildZones[0] = world.GuildZone{Zone: 0, ChargeGuild: 77, GuildSpawnX: c.x, GuildSpawnY: c.y}
		if _, _, ok := d.guildSpawnFor(77); ok {
			t.Errorf("%s: (%d,%d) foi aceito como ponto", c.nome, c.x, c.y)
		}
	}
}

// A primeira zona configurada vence, como o legado faz (break no primeiro
// ChargeGuild que bate) — uma guild que domine duas cidades tem um ponto só.
func TestDuasCidadesUsamAPrimeira(t *testing.T) {
	d := &Dispatcher{}
	d.guildZones[1] = world.GuildZone{Zone: 1, ChargeGuild: 77, GuildSpawnX: 2500, GuildSpawnY: 1700}
	d.guildZones[3] = world.GuildZone{Zone: 3, ChargeGuild: 77, GuildSpawnX: 3652, GuildSpawnY: 3122}

	x, _, ok := d.guildSpawnFor(77)
	if !ok || x != 2500 {
		t.Errorf("ponto = (%d,...) ok=%v, want a primeira zona (2500)", x, ok)
	}
}
