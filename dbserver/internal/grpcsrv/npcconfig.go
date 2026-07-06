package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// NpcConfigStore is the read surface the NPC-config service needs (satisfied by
// *store.Store). tmServer only reads NPC config through here; all writes go
// through the web-api.
type NpcConfigStore interface {
	NPCConfigVersion(ctx context.Context) (int64, error)
	ListNPCDefinitions(ctx context.Context) ([]domain.NPCDefinition, error)
	ItemPriceOverrides(ctx context.Context) ([]domain.ItemPriceOverride, error)
}

// NpcConfigServer implements dbv1.NpcConfigServiceServer. It is the read-only
// bridge tmServer polls to reload moderator-edited NPC config (npc-editing-plan.md).
type NpcConfigServer struct {
	dbv1.UnimplementedNpcConfigServiceServer
	store NpcConfigStore
}

// NewNpcConfig builds the NPC-config service over the given store.
func NewNpcConfig(s NpcConfigStore) *NpcConfigServer { return &NpcConfigServer{store: s} }

// NpcConfigVersion returns the monotonic config version for the tmServer poll.
func (s *NpcConfigServer) NpcConfigVersion(ctx context.Context, _ *dbv1.NpcConfigVersionRequest) (*dbv1.NpcConfigVersionResponse, error) {
	v, err := s.store.NPCConfigVersion(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "npc config version: %v", err)
	}
	return &dbv1.NpcConfigVersionResponse{Version: v}, nil
}

// ListNpcDefinitions returns the full definition snapshot plus global price
// overrides, tagged with the version they were read at.
func (s *NpcConfigServer) ListNpcDefinitions(ctx context.Context, _ *dbv1.ListNpcDefinitionsRequest) (*dbv1.ListNpcDefinitionsResponse, error) {
	// Read the version FIRST so a concurrent write can only make the snapshot look
	// older than it is (the tmServer re-polls and reloads), never newer — which
	// would let a stale snapshot masquerade as current and skip the next reload.
	version, err := s.store.NPCConfigVersion(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "npc config version: %v", err)
	}
	defs, err := s.store.ListNPCDefinitions(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list npc definitions: %v", err)
	}
	prices, err := s.store.ItemPriceOverrides(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "item price overrides: %v", err)
	}
	return &dbv1.ListNpcDefinitionsResponse{
		Version:        version,
		Definitions:    npcDefinitionsToProto(defs),
		PriceOverrides: itemPricesToProto(prices),
	}, nil
}

func npcDefinitionsToProto(defs []domain.NPCDefinition) []*dbv1.NpcDefinition {
	out := make([]*dbv1.NpcDefinition, 0, len(defs))
	for _, d := range defs {
		shop := make([]*dbv1.NpcShopItem, 0, len(d.Shop))
		for _, it := range d.Shop {
			shop = append(shop, &dbv1.NpcShopItem{
				Slot: int32(it.Slot), ItemIndex: it.ItemIndex,
				Eff1: int32(it.Eff1), Effv1: int32(it.EffV1),
				Eff2: int32(it.Eff2), Effv2: int32(it.EffV2),
				Eff3: int32(it.Eff3), Effv3: int32(it.EffV3),
			})
		}
		out = append(out, &dbv1.NpcDefinition{
			Id: d.ID, Slug: d.Slug, TemplateName: d.TemplateName, DisplayName: d.DisplayName,
			Enabled: d.Enabled, MapId: d.MapID, PosX: d.PosX, PosY: d.PosY,
			RouteType: int32(d.RouteType), Merchant: int32(d.Merchant), Shop: shop,
		})
	}
	return out
}

func itemPricesToProto(prices []domain.ItemPriceOverride) []*dbv1.ItemPrice {
	out := make([]*dbv1.ItemPrice, 0, len(prices))
	for _, p := range prices {
		out = append(out, &dbv1.ItemPrice{ItemIndex: p.ItemIndex, Price: p.Price})
	}
	return out
}
