package control

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	gamev1 "github.com/jeanluca/w2pp-openwyd/api/game/v1"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// BlockRunner runs one block command ("npc off 23", "gerar 396 aqui", …) as a GM
// standing at (x, y) and returns the lines the GM would read. It is the same
// code as "/gm" in game, injected like the Teleporter because the commands
// belong to the gameplay layer. Called INSIDE the game loop.
type BlockRunner func(w *world.World, by string, x, y int16, line string) []string

// blockCommands are the subcommands the panel may send. Anything else is
// refused here rather than handed to the game, so this door only opens onto the
// block commands and not onto the rest of the GM bus.
var blockCommands = map[string]bool{
	"npc": true, "gerar": true, "criar": true, "matar": true, "recarregar": true,
}

// SetBlockRunner installs the block commands. Optional: a server wired without
// it answers the two block calls with FailedPrecondition and keeps the rest.
func (s *Server) SetBlockRunner(r BlockRunner) { s.blocos = r }

// ListBlocks finds NPCGener blocks for the panel.
func (s *Server) ListBlocks(ctx context.Context, req *gamev1.ListBlocksRequest) (*gamev1.ListBlocksResponse, error) {
	q := world.GeneratorQuery{
		Index: -1, Name: req.GetName(),
		X: int16(req.GetX()), Y: int16(req.GetY()), Radius: int(req.GetRadius()),
		Limit: int(req.GetLimit()),
	}
	if req.Index != nil {
		q.Index = int(req.GetIndex())
		if q.Index < 0 {
			return &gamev1.ListBlocksResponse{}, nil
		}
	}
	return noLoop(ctx, s.world, func(w *world.World) *gamev1.ListBlocksResponse {
		found, total := w.FindGenerators(q)
		resp := &gamev1.ListBlocksResponse{Total: int32(total)}
		for _, g := range found {
			resp.Blocks = append(resp.Blocks, &gamev1.Block{
				Index: int32(g.Index), Name: g.Name, X: int32(g.X), Y: int32(g.Y),
				Alive: int32(g.Alive), Max: int32(g.Max), Minute: int32(g.Minute),
				Off: g.Off, DbManaged: g.DBManaged, EventOwned: g.EventOwned,
				HasTemplates: g.HasTemplates,
			})
		}
		return resp
	})
}

// BlockCommand runs one block command for the panel.
func (s *Server) BlockCommand(ctx context.Context, req *gamev1.BlockCommandRequest) (*gamev1.BlockCommandResponse, error) {
	if s.blocos == nil {
		return nil, status.Error(codes.FailedPrecondition, "block commands are not wired on this server")
	}
	line := strings.TrimSpace(req.GetLine())
	fields := strings.Fields(line)
	if len(fields) == 0 || !blockCommands[strings.ToLower(fields[0])] {
		return nil, status.Errorf(codes.InvalidArgument, "not a block command: %q", line)
	}
	x, y := req.GetX(), req.GetY()
	lines, err := noLoop(ctx, s.world, func(w *world.World) []string {
		// Checked in the loop, where the grid size is: a typo in a coordinate
		// box must not raise a mob off the map.
		if dim := int32(w.GridDim()); x < 0 || y < 0 || x >= dim || y >= dim {
			return []string{"Coordenada fora do mapa."}
		}
		return s.blocos(w, req.GetBy(), int16(x), int16(y), line)
	})
	if err != nil {
		return nil, err
	}
	s.log.Info("control: block command", "by", req.GetBy(), "line", line, "x", x, "y", y)
	return &gamev1.BlockCommandResponse{Lines: lines}, nil
}
