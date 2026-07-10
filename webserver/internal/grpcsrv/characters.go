package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// Characters is the player-facing character query surface the server depends on.
type Characters interface {
	ListMyCharacters(ctx context.Context, accountID int64) ([]domain.Character, error)
}

// CharacterServer implements webv1.CharacterWebServiceServer.
type CharacterServer struct {
	webv1.UnimplementedCharacterWebServiceServer
	characters Characters
}

// NewCharacters builds the CharacterWebService over the given query logic.
func NewCharacters(c Characters) *CharacterServer { return &CharacterServer{characters: c} }

// ListMyCharacters returns read-only character summaries for the logged account.
func (s *CharacterServer) ListMyCharacters(ctx context.Context, req *webv1.ListMyCharactersRequest) (*webv1.ListMyCharactersResponse, error) {
	chars, err := s.characters.ListMyCharacters(ctx, req.GetAccountId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list my characters: %v", err)
	}
	out := make([]*webv1.WebCharacterSummary, 0, len(chars))
	for _, ch := range chars {
		out = append(out, characterSummaryToProto(ch))
	}
	return &webv1.ListMyCharactersResponse{Characters: out}, nil
}

func characterSummaryToProto(ch domain.Character) *webv1.WebCharacterSummary {
	return &webv1.WebCharacterSummary{
		Slot:  int32(ch.Slot),
		Name:  ch.Name,
		Class: int32(ch.Class),
		Level: ch.Level,
		Exp:   ch.Exp,
		Coin:  ch.Coin,
		MaxHp: ch.MaxHp,
		Hp:    ch.Hp,
		MaxMp: ch.MaxMp,
		Mp:    ch.Mp,
		Str:   int32(ch.Str),
		Int:   int32(ch.Int),
		Dex:   int32(ch.Dex),
		Con:   int32(ch.Con),
	}
}
