package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/droptool"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/itemcatalog"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/mapzones"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/npcadmin"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/npctemplates"
)

// NpcAdmin is the moderator NPC-editing surface the server depends on (satisfied
// by *npcadmin.Service). Kept as an interface so the server is unit-testable.
type NpcAdmin interface {
	List(ctx context.Context, moderatorID int64) (npcadmin.Result, []domain.NPCDefinition, error)
	Get(ctx context.Context, moderatorID, npcID int64) (npcadmin.Result, domain.NPCDefinition, error)
	Upsert(ctx context.Context, moderatorID int64, d domain.NPCDefinition) (npcadmin.Result, int64, error)
	SetVisibility(ctx context.Context, moderatorID, npcID int64, enabled bool) (npcadmin.Result, error)
	SetShop(ctx context.Context, moderatorID, npcID int64, items []domain.NPCShopItem) (npcadmin.Result, error)
	SetItemPrice(ctx context.Context, moderatorID int64, itemIndex int32, price int64) (npcadmin.Result, error)
	Delete(ctx context.Context, moderatorID, npcID int64) (npcadmin.Result, error)
	ListMerchantTemplates(ctx context.Context, moderatorID int64) (npcadmin.Result, []npctemplates.Template, error)
	ListItemCatalog(ctx context.Context, moderatorID int64) (npcadmin.Result, itemcatalog.Catalog, error)
	ListDropItems(ctx context.Context, moderatorID int64, filter droptool.Filter) (npcadmin.Result, []droptool.ItemDropEntry, error)
	ListMobDrops(ctx context.Context, moderatorID int64, filter droptool.Filter) (npcadmin.Result, []droptool.MobDropEntry, error)
	ListItemPrices(ctx context.Context, moderatorID int64) (npcadmin.Result, []domain.ItemPriceOverride, error)
	ListMapZones(ctx context.Context, moderatorID int64) (npcadmin.Result, []mapzones.Zone, error)
}

// NpcAdminServer implements webv1.NpcAdminServiceServer.
type NpcAdminServer struct {
	webv1.UnimplementedNpcAdminServiceServer
	admin NpcAdmin
}

// NewNpcAdmin builds the NpcAdminService over the given admin logic.
func NewNpcAdmin(a NpcAdmin) *NpcAdminServer { return &NpcAdminServer{admin: a} }

// ListNpcs returns every NPC definition (after authorization).
func (s *NpcAdminServer) ListNpcs(ctx context.Context, req *webv1.ListNpcsRequest) (*webv1.ListNpcsResponse, error) {
	res, defs, err := s.admin.List(ctx, req.GetModeratorId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list npcs: %v", err)
	}
	out := make([]*webv1.AdminNpc, 0, len(defs))
	for _, d := range defs {
		out = append(out, adminNpcToProto(d))
	}
	return &webv1.ListNpcsResponse{Result: resultToProto(res), Npcs: out}, nil
}

// GetNpc returns one definition with its shop stock.
func (s *NpcAdminServer) GetNpc(ctx context.Context, req *webv1.GetNpcRequest) (*webv1.GetNpcResponse, error) {
	res, def, err := s.admin.Get(ctx, req.GetModeratorId(), req.GetNpcId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get npc: %v", err)
	}
	resp := &webv1.GetNpcResponse{Result: resultToProto(res)}
	if res == npcadmin.OK {
		resp.Npc = adminNpcToProto(def)
	}
	return resp, nil
}

// UpsertNpc creates or updates a definition.
func (s *NpcAdminServer) UpsertNpc(ctx context.Context, req *webv1.UpsertNpcRequest) (*webv1.UpsertNpcResponse, error) {
	def := domain.NPCDefinition{
		Slug:         req.GetSlug(),
		TemplateName: req.GetTemplateName(),
		DisplayName:  req.GetDisplayName(),
		Enabled:      req.GetEnabled(),
		MapID:        req.GetMapId(),
		PosX:         req.GetPosX(),
		PosY:         req.GetPosY(),
		RouteType:    int16(req.GetRouteType()),
		Merchant:     int16(req.GetMerchant()),
	}
	res, id, err := s.admin.Upsert(ctx, req.GetModeratorId(), def)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "upsert npc: %v", err)
	}
	return &webv1.UpsertNpcResponse{Result: resultToProto(res), NpcId: id}, nil
}

// SetNpcVisibility toggles whether the NPC appears.
func (s *NpcAdminServer) SetNpcVisibility(ctx context.Context, req *webv1.SetNpcVisibilityRequest) (*webv1.AdminAck, error) {
	res, err := s.admin.SetVisibility(ctx, req.GetModeratorId(), req.GetNpcId(), req.GetEnabled())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "set visibility: %v", err)
	}
	return &webv1.AdminAck{Result: resultToProto(res)}, nil
}

// SetNpcShop replaces a merchant NPC's shop stock.
func (s *NpcAdminServer) SetNpcShop(ctx context.Context, req *webv1.SetNpcShopRequest) (*webv1.AdminAck, error) {
	items := make([]domain.NPCShopItem, 0, len(req.GetItems()))
	for _, it := range req.GetItems() {
		items = append(items, protoToShopItem(it))
	}
	res, err := s.admin.SetShop(ctx, req.GetModeratorId(), req.GetNpcId(), items)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "set shop: %v", err)
	}
	return &webv1.AdminAck{Result: resultToProto(res)}, nil
}

// SetItemPrice sets or clears the global price of an item.
func (s *NpcAdminServer) SetItemPrice(ctx context.Context, req *webv1.SetItemPriceRequest) (*webv1.AdminAck, error) {
	res, err := s.admin.SetItemPrice(ctx, req.GetModeratorId(), req.GetItemIndex(), req.GetPrice())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "set item price: %v", err)
	}
	return &webv1.AdminAck{Result: resultToProto(res)}, nil
}

// DeleteNpc removes a definition.
func (s *NpcAdminServer) DeleteNpc(ctx context.Context, req *webv1.DeleteNpcRequest) (*webv1.AdminAck, error) {
	res, err := s.admin.Delete(ctx, req.GetModeratorId(), req.GetNpcId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "delete npc: %v", err)
	}
	return &webv1.AdminAck{Result: resultToProto(res)}, nil
}

// ListMerchantTemplates returns the merchant templates scanned from the
// content tree, for the moderator UI's template_name picker.
func (s *NpcAdminServer) ListMerchantTemplates(ctx context.Context, req *webv1.ListMerchantTemplatesRequest) (*webv1.ListMerchantTemplatesResponse, error) {
	res, tmpls, err := s.admin.ListMerchantTemplates(ctx, req.GetModeratorId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list merchant templates: %v", err)
	}
	out := make([]*webv1.MerchantTemplate, 0, len(tmpls))
	for _, t := range tmpls {
		out = append(out, merchantTemplateToProto(t))
	}
	return &webv1.ListMerchantTemplatesResponse{Result: resultToProto(res), Templates: out}, nil
}

func merchantTemplateToProto(t npctemplates.Template) *webv1.MerchantTemplate {
	return &webv1.MerchantTemplate{
		TemplateName: t.TemplateName,
		DisplayName:  t.DisplayName,
		Merchant:     int32(t.Merchant),
	}
}

// ListItemCatalog returns the item index/name catalog for the shop-item
// picker (SetNpcShop/SetItemPrice take a raw item_index).
func (s *NpcAdminServer) ListItemCatalog(ctx context.Context, req *webv1.ListItemCatalogRequest) (*webv1.ListItemCatalogResponse, error) {
	res, catalog, err := s.admin.ListItemCatalog(ctx, req.GetModeratorId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list item catalog: %v", err)
	}
	out := make([]*webv1.ItemCatalogEntry, 0, len(catalog.Items))
	for _, it := range catalog.Items {
		out = append(out, itemCatalogEntryToProto(it))
	}
	return &webv1.ListItemCatalogResponse{
		Result: resultToProto(res), Items: out,
		CatalogVersion: catalog.Version, IconPackVersion: catalog.IconPackVersion,
	}, nil
}

// ListDropItems returns the item-centric DropTool report for the moderator
// panel.
func (s *NpcAdminServer) ListDropItems(ctx context.Context, req *webv1.ListDropItemsRequest) (*webv1.ListDropItemsResponse, error) {
	res, items, err := s.admin.ListDropItems(ctx, req.GetModeratorId(), droptool.Filter{
		ItemIndex:            req.GetItemIndex(),
		ItemQuery:            req.GetItemQuery(),
		MobQuery:             req.GetMobQuery(),
		IncludeZeroDropItems: req.GetIncludeZeroDropItems(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list drop items: %v", err)
	}
	out := make([]*webv1.DropItemEntry, 0, len(items))
	for _, item := range items {
		out = append(out, dropItemEntryToProto(item))
	}
	return &webv1.ListDropItemsResponse{Result: resultToProto(res), Items: out}, nil
}

func dropItemEntryToProto(item droptool.ItemDropEntry) *webv1.DropItemEntry {
	mobs := make([]*webv1.DropItemMob, 0, len(item.Mobs))
	for _, mob := range item.Mobs {
		mobs = append(mobs, &webv1.DropItemMob{
			TemplateName: mob.TemplateName,
			MobName:      mob.MobName,
			MobLevel:     mob.MobLevel,
			Slot:         mob.Slot,
			RateDivisor:  mob.RateDivisor,
		})
	}
	return &webv1.DropItemEntry{ItemIndex: item.ItemIndex, ItemName: item.ItemName, Mobs: mobs}
}

// ListMobDrops returns the mob-centric DropTool report for the moderator panel.
func (s *NpcAdminServer) ListMobDrops(ctx context.Context, req *webv1.ListMobDropsRequest) (*webv1.ListMobDropsResponse, error) {
	res, mobs, err := s.admin.ListMobDrops(ctx, req.GetModeratorId(), droptool.Filter{
		MobQuery:  req.GetMobQuery(),
		ItemIndex: req.GetItemIndex(),
		ItemQuery: req.GetItemQuery(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list mob drops: %v", err)
	}
	out := make([]*webv1.MobDropEntry, 0, len(mobs))
	for _, mob := range mobs {
		out = append(out, mobDropEntryToProto(mob))
	}
	return &webv1.ListMobDropsResponse{Result: resultToProto(res), Mobs: out}, nil
}

func mobDropEntryToProto(mob droptool.MobDropEntry) *webv1.MobDropEntry {
	items := make([]*webv1.MobDropItem, 0, len(mob.Items))
	for _, item := range mob.Items {
		items = append(items, &webv1.MobDropItem{
			Slot:        item.Slot,
			ItemIndex:   item.ItemIndex,
			ItemName:    item.ItemName,
			RateDivisor: item.RateDivisor,
		})
	}
	return &webv1.MobDropEntry{
		TemplateName: mob.TemplateName,
		MobName:      mob.MobName,
		MobLevel:     mob.MobLevel,
		Items:        items,
	}
}

// ListItemPrices returns every global item price override for the moderator
// panel's price column/editor.
func (s *NpcAdminServer) ListItemPrices(ctx context.Context, req *webv1.ListItemPricesRequest) (*webv1.ListItemPricesResponse, error) {
	res, prices, err := s.admin.ListItemPrices(ctx, req.GetModeratorId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list item prices: %v", err)
	}
	out := make([]*webv1.ItemPrice, 0, len(prices))
	for _, p := range prices {
		out = append(out, &webv1.ItemPrice{ItemIndex: p.ItemIndex, Price: p.Price})
	}
	return &webv1.ListItemPricesResponse{Result: resultToProto(res), Prices: out}, nil
}

// ListMapZones returns the fixed map_id label table for the NPC form's map
// picker.
func (s *NpcAdminServer) ListMapZones(ctx context.Context, req *webv1.ListMapZonesRequest) (*webv1.ListMapZonesResponse, error) {
	res, zones, err := s.admin.ListMapZones(ctx, req.GetModeratorId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list map zones: %v", err)
	}
	out := make([]*webv1.MapZone, 0, len(zones))
	for _, z := range zones {
		out = append(out, &webv1.MapZone{Id: z.ID, Name: z.Name})
	}
	return &webv1.ListMapZonesResponse{Result: resultToProto(res), Zones: out}, nil
}

func adminNpcToProto(d domain.NPCDefinition) *webv1.AdminNpc {
	shop := make([]*webv1.AdminNpcShopItem, 0, len(d.Shop))
	for _, it := range d.Shop {
		shop = append(shop, &webv1.AdminNpcShopItem{
			Slot: int32(it.Slot), ItemIndex: it.ItemIndex, Quantity: int32(it.Quantity),
			Eff1: int32(it.Eff1), Effv1: int32(it.EffV1),
			Eff2: int32(it.Eff2), Effv2: int32(it.EffV2),
			Eff3: int32(it.Eff3), Effv3: int32(it.EffV3),
		})
	}
	return &webv1.AdminNpc{
		Id: d.ID, Slug: d.Slug, TemplateName: d.TemplateName, DisplayName: d.DisplayName,
		Enabled: d.Enabled, MapId: d.MapID, PosX: d.PosX, PosY: d.PosY,
		RouteType: int32(d.RouteType), Merchant: int32(d.Merchant), Shop: shop,
		Origin: d.Origin, GeneratorIndex: d.GeneratorIndex,
	}
}

func protoToShopItem(it *webv1.AdminNpcShopItem) domain.NPCShopItem {
	return domain.NPCShopItem{
		Slot: int16(it.GetSlot()), ItemIndex: it.GetItemIndex(), Quantity: protoQuantity(it.GetQuantity()),
		Eff1: uint8(it.GetEff1()), EffV1: uint8(it.GetEffv1()),
		Eff2: uint8(it.GetEff2()), EffV2: uint8(it.GetEffv2()),
		Eff3: uint8(it.GetEff3()), EffV3: uint8(it.GetEffv3()),
	}
}

func protoQuantity(q int32) int16 {
	if q < 0 || q > 255 {
		return -1
	}
	return int16(q)
}

func resultToProto(r npcadmin.Result) webv1.AdminResult {
	switch r {
	case npcadmin.OK:
		return webv1.AdminResult_ADMIN_RESULT_OK
	case npcadmin.Forbidden:
		return webv1.AdminResult_ADMIN_RESULT_FORBIDDEN
	case npcadmin.NotFound:
		return webv1.AdminResult_ADMIN_RESULT_NOT_FOUND
	case npcadmin.ContentOwned:
		return webv1.AdminResult_ADMIN_RESULT_CONTENT_OWNED
	default:
		return webv1.AdminResult_ADMIN_RESULT_INVALID
	}
}
