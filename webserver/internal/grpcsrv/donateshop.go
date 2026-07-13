package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/donateshop"
)

// DonateAdmin is the moderator donate-shop surface (satisfied by
// *donateshop.Service). Kept as an interface so the server is unit-testable.
type DonateAdmin interface {
	List(ctx context.Context, moderatorID int64) (donateshop.Result, []domain.DonateShopItem, error)
	Upsert(ctx context.Context, moderatorID int64, d domain.DonateShopItem) (donateshop.Result, int64, error)
	SetEnabled(ctx context.Context, moderatorID, itemID int64, enabled bool) (donateshop.Result, error)
	Delete(ctx context.Context, moderatorID, itemID int64) (donateshop.Result, error)
	CreditBalance(ctx context.Context, moderatorID, accountID int64, amount int32, reason string) (donateshop.Result, int32, error)
}

// DonateShop is the player donate-shop surface (satisfied by *donateshop.Service).
type DonateShop interface {
	Vitrine(ctx context.Context) ([]domain.DonateShopItem, error)
	Balance(ctx context.Context, accountID int64) (int32, error)
	Buy(ctx context.Context, accountID, shopItemID int64) (donateshop.BuyOutcome, int32, error)
}

// DonateAdminServer implements webv1.DonateAdminServiceServer.
type DonateAdminServer struct {
	webv1.UnimplementedDonateAdminServiceServer
	admin DonateAdmin
}

// NewDonateAdmin builds the DonateAdminService over the given admin logic.
func NewDonateAdmin(a DonateAdmin) *DonateAdminServer { return &DonateAdminServer{admin: a} }

// ListShopItems returns every offer (after authorization).
func (s *DonateAdminServer) ListShopItems(ctx context.Context, req *webv1.ListShopItemsRequest) (*webv1.ListShopItemsResponse, error) {
	res, items, err := s.admin.List(ctx, req.GetModeratorId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list shop items: %v", err)
	}
	out := make([]*webv1.DonateShopItem, 0, len(items))
	for _, it := range items {
		out = append(out, donateItemToProto(it))
	}
	return &webv1.ListShopItemsResponse{Result: adminResultToProto(res), Items: out}, nil
}

// UpsertShopItem creates or updates an offer.
func (s *DonateAdminServer) UpsertShopItem(ctx context.Context, req *webv1.UpsertShopItemRequest) (*webv1.UpsertShopItemResponse, error) {
	res, id, err := s.admin.Upsert(ctx, req.GetModeratorId(), protoToDonateItem(req.GetItem()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "upsert shop item: %v", err)
	}
	return &webv1.UpsertShopItemResponse{Result: adminResultToProto(res), ItemId: id}, nil
}

// SetShopItemEnabled toggles whether an offer is on sale.
func (s *DonateAdminServer) SetShopItemEnabled(ctx context.Context, req *webv1.SetShopItemEnabledRequest) (*webv1.AdminAck, error) {
	res, err := s.admin.SetEnabled(ctx, req.GetModeratorId(), req.GetItemId(), req.GetEnabled())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "set shop item enabled: %v", err)
	}
	return &webv1.AdminAck{Result: adminResultToProto(res)}, nil
}

// DeleteShopItem removes an offer.
func (s *DonateAdminServer) DeleteShopItem(ctx context.Context, req *webv1.DeleteShopItemRequest) (*webv1.AdminAck, error) {
	res, err := s.admin.Delete(ctx, req.GetModeratorId(), req.GetItemId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "delete shop item: %v", err)
	}
	return &webv1.AdminAck{Result: adminResultToProto(res)}, nil
}

// CreditDonateBalance adds donate currency to an account's wallet.
func (s *DonateAdminServer) CreditDonateBalance(ctx context.Context, req *webv1.CreditDonateBalanceRequest) (*webv1.CreditDonateBalanceResponse, error) {
	res, newBal, err := s.admin.CreditBalance(ctx, req.GetModeratorId(), req.GetAccountId(), req.GetAmount(), req.GetReason())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "credit donate balance: %v", err)
	}
	return &webv1.CreditDonateBalanceResponse{Result: adminResultToProto(res), NewBalance: newBal}, nil
}

// DonateShopServer implements webv1.DonateShopServiceServer.
type DonateShopServer struct {
	webv1.UnimplementedDonateShopServiceServer
	shop DonateShop
}

// NewDonateShop builds the DonateShopService over the given shop logic.
func NewDonateShop(sh DonateShop) *DonateShopServer { return &DonateShopServer{shop: sh} }

// ListShopItems returns the enabled offers (the vitrine).
func (s *DonateShopServer) ListShopItems(ctx context.Context, _ *webv1.ListStoreItemsRequest) (*webv1.ListStoreItemsResponse, error) {
	items, err := s.shop.Vitrine(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list store items: %v", err)
	}
	out := make([]*webv1.DonateShopItem, 0, len(items))
	for _, it := range items {
		out = append(out, donateItemToProto(it))
	}
	return &webv1.ListStoreItemsResponse{Items: out}, nil
}

// GetBalance returns the account's donate balance.
func (s *DonateShopServer) GetBalance(ctx context.Context, req *webv1.GetBalanceRequest) (*webv1.GetBalanceResponse, error) {
	bal, err := s.shop.Balance(ctx, req.GetAccountId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get balance: %v", err)
	}
	return &webv1.GetBalanceResponse{Balance: bal}, nil
}

// Buy purchases an offer for the account.
func (s *DonateShopServer) Buy(ctx context.Context, req *webv1.BuyRequest) (*webv1.BuyResponse, error) {
	outcome, newBal, err := s.shop.Buy(ctx, req.GetAccountId(), req.GetShopItemId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "buy: %v", err)
	}
	return &webv1.BuyResponse{Result: buyOutcomeToProto(outcome), NewBalance: newBal}, nil
}

func donateItemToProto(d domain.DonateShopItem) *webv1.DonateShopItem {
	return &webv1.DonateShopItem{
		Id: d.ID, ItemIndex: d.ItemIndex,
		Eff1: int32(d.Eff1), Effv1: int32(d.EffV1),
		Eff2: int32(d.Eff2), Effv2: int32(d.EffV2),
		Eff3: int32(d.Eff3), Effv3: int32(d.EffV3),
		Price: d.Price, Title: d.Title, Description: d.Description,
		Enabled: d.Enabled, ExpiresDays: d.ExpiresDays,
	}
}

func protoToDonateItem(d *webv1.DonateShopItem) domain.DonateShopItem {
	return domain.DonateShopItem{
		ID: d.GetId(), ItemIndex: d.GetItemIndex(),
		Eff1: uint8(d.GetEff1()), EffV1: uint8(d.GetEffv1()),
		Eff2: uint8(d.GetEff2()), EffV2: uint8(d.GetEffv2()),
		Eff3: uint8(d.GetEff3()), EffV3: uint8(d.GetEffv3()),
		Price: d.GetPrice(), Title: d.GetTitle(), Description: d.GetDescription(),
		Enabled: d.GetEnabled(), ExpiresDays: d.GetExpiresDays(),
	}
}

func adminResultToProto(r donateshop.Result) webv1.AdminResult {
	switch r {
	case donateshop.OK:
		return webv1.AdminResult_ADMIN_RESULT_OK
	case donateshop.Forbidden:
		return webv1.AdminResult_ADMIN_RESULT_FORBIDDEN
	case donateshop.NotFound:
		return webv1.AdminResult_ADMIN_RESULT_NOT_FOUND
	default:
		return webv1.AdminResult_ADMIN_RESULT_INVALID
	}
}

func buyOutcomeToProto(o donateshop.BuyOutcome) webv1.BuyResult {
	switch o {
	case donateshop.BuyOK:
		return webv1.BuyResult_BUY_RESULT_OK
	case donateshop.BuyInsufficient:
		return webv1.BuyResult_BUY_RESULT_INSUFFICIENT_FUNDS
	case donateshop.BuyDisabled:
		return webv1.BuyResult_BUY_RESULT_DISABLED
	default:
		return webv1.BuyResult_BUY_RESULT_NOT_FOUND
	}
}
