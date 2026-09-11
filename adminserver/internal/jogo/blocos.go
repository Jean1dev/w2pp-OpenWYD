package jogo

import (
	"context"

	gamev1 "github.com/jeanluca/w2pp-openwyd/api/game/v1"
)

// Bloco is one NPCGener block as the running game holds it.
type Bloco struct {
	Numero    int32 // position in NPCGener.txt — what every block command asks for
	Nome      string
	X, Y      int32
	Vivos     int32
	Max       int32
	Minuto    int32 // MinuteGenerate; <= 0 = only events raise it in the legacy
	Desligado bool
	DoPainel  bool // a merchant of the NPC panel
	DeEvento  bool // an event or a dungeon run raises it, not the world
	SemMolde  bool // its templates failed to load: it never spawns
}

// BuscaBlocos narrows the list. Numero < 0 means any block.
type BuscaBlocos struct {
	Numero int32
	Nome   string
	X, Y   int32
	Raio   int32
}

// Blocos finds blocks in the running game, and how many matched in total.
func (c *Client) Blocos(parent context.Context, b BuscaBlocos) ([]Bloco, int32, error) {
	ctx, cancel := c.ctx(parent)
	defer cancel()
	req := &gamev1.ListBlocksRequest{Name: b.Nome, X: b.X, Y: b.Y, Radius: b.Raio}
	if b.Numero >= 0 {
		n := b.Numero
		req.Index = &n
	}
	resp, err := c.api.ListBlocks(ctx, req)
	if err != nil {
		return nil, 0, traduz(err, "listar os blocos")
	}
	out := make([]Bloco, 0, len(resp.GetBlocks()))
	for _, g := range resp.GetBlocks() {
		out = append(out, Bloco{
			Numero: g.GetIndex(), Nome: g.GetName(), X: g.GetX(), Y: g.GetY(),
			Vivos: g.GetAlive(), Max: g.GetMax(), Minuto: g.GetMinute(),
			Desligado: g.GetOff(), DoPainel: g.GetDbManaged(), DeEvento: g.GetEventOwned(),
			SemMolde: !g.GetHasTemplates(),
		})
	}
	return out, resp.GetTotal(), nil
}

// ComandoBloco runs one block command ("npc off 23", "gerar 396 aqui", …) as a
// GM standing at (x, y), and returns what the game answered.
func (c *Client) ComandoBloco(parent context.Context, linha string, x, y int32, quem string) ([]string, error) {
	ctx, cancel := c.ctx(parent)
	defer cancel()
	resp, err := c.api.BlockCommand(ctx, &gamev1.BlockCommandRequest{Line: linha, X: x, Y: y, By: quem})
	if err != nil {
		return nil, traduz(err, "rodar o comando de bloco")
	}
	return resp.GetLines(), nil
}
