package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/dailyreward"
)

// DailyRewardAdmin is the moderator daily-reward surface (satisfied by
// *dailyreward.Service). Kept as an interface so the server is unit-testable.
type DailyRewardAdmin interface {
	List(ctx context.Context, moderatorID int64) (dailyreward.Result, []domain.DailyRewardItem, error)
	Upsert(ctx context.Context, moderatorID int64, d domain.DailyRewardItem) (dailyreward.Result, int64, error)
	SetEnabled(ctx context.Context, moderatorID, itemID int64, enabled bool) (dailyreward.Result, error)
	Delete(ctx context.Context, moderatorID, itemID int64) (dailyreward.Result, error)
}

// DailyReward is the player daily-reward surface (satisfied by *dailyreward.Service).
type DailyReward interface {
	Vitrine(ctx context.Context) ([]domain.DailyRewardItem, error)
	ClaimStatus(ctx context.Context, accountID int64) (claimed bool, itemID int64, title string, err error)
	Claim(ctx context.Context, accountID, rewardItemID int64) (dailyreward.ClaimOutcome, error)
}

// DailyRewardAdminServer implements webv1.DailyRewardAdminServiceServer.
type DailyRewardAdminServer struct {
	webv1.UnimplementedDailyRewardAdminServiceServer
	admin DailyRewardAdmin
}

// NewDailyRewardAdmin builds the DailyRewardAdminService over the given admin logic.
func NewDailyRewardAdmin(a DailyRewardAdmin) *DailyRewardAdminServer {
	return &DailyRewardAdminServer{admin: a}
}

// ListRewardItems returns every offer (after authorization).
func (s *DailyRewardAdminServer) ListRewardItems(ctx context.Context, req *webv1.ListRewardItemsRequest) (*webv1.ListRewardItemsResponse, error) {
	res, items, err := s.admin.List(ctx, req.GetModeratorId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list reward items: %v", err)
	}
	out := make([]*webv1.DailyRewardItem, 0, len(items))
	for _, it := range items {
		out = append(out, dailyRewardItemToProto(it))
	}
	return &webv1.ListRewardItemsResponse{Result: dailyRewardAdminResultToProto(res), Items: out}, nil
}

// UpsertRewardItem creates or updates an offer.
func (s *DailyRewardAdminServer) UpsertRewardItem(ctx context.Context, req *webv1.UpsertRewardItemRequest) (*webv1.UpsertRewardItemResponse, error) {
	res, id, err := s.admin.Upsert(ctx, req.GetModeratorId(), protoToDailyRewardItem(req.GetItem()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "upsert reward item: %v", err)
	}
	return &webv1.UpsertRewardItemResponse{Result: dailyRewardAdminResultToProto(res), ItemId: id}, nil
}

// SetRewardItemEnabled toggles whether an offer is claimable.
func (s *DailyRewardAdminServer) SetRewardItemEnabled(ctx context.Context, req *webv1.SetRewardItemEnabledRequest) (*webv1.AdminAck, error) {
	res, err := s.admin.SetEnabled(ctx, req.GetModeratorId(), req.GetItemId(), req.GetEnabled())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "set reward item enabled: %v", err)
	}
	return &webv1.AdminAck{Result: dailyRewardAdminResultToProto(res)}, nil
}

// DeleteRewardItem removes an offer.
func (s *DailyRewardAdminServer) DeleteRewardItem(ctx context.Context, req *webv1.DeleteRewardItemRequest) (*webv1.AdminAck, error) {
	res, err := s.admin.Delete(ctx, req.GetModeratorId(), req.GetItemId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "delete reward item: %v", err)
	}
	return &webv1.AdminAck{Result: dailyRewardAdminResultToProto(res)}, nil
}

// DailyRewardServer implements webv1.DailyRewardServiceServer.
type DailyRewardServer struct {
	webv1.UnimplementedDailyRewardServiceServer
	reward DailyReward
}

// NewDailyReward builds the DailyRewardService over the given reward logic.
func NewDailyReward(r DailyReward) *DailyRewardServer { return &DailyRewardServer{reward: r} }

// ListRewards returns the enabled offers (the vitrine).
func (s *DailyRewardServer) ListRewards(ctx context.Context, _ *webv1.ListRewardsRequest) (*webv1.ListRewardsResponse, error) {
	items, err := s.reward.Vitrine(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list rewards: %v", err)
	}
	out := make([]*webv1.DailyRewardItem, 0, len(items))
	for _, it := range items {
		out = append(out, dailyRewardItemToProto(it))
	}
	return &webv1.ListRewardsResponse{Items: out}, nil
}

// GetClaimStatus reports whether the account already claimed today.
func (s *DailyRewardServer) GetClaimStatus(ctx context.Context, req *webv1.GetClaimStatusRequest) (*webv1.GetClaimStatusResponse, error) {
	claimed, itemID, title, err := s.reward.ClaimStatus(ctx, req.GetAccountId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get claim status: %v", err)
	}
	return &webv1.GetClaimStatusResponse{ClaimedToday: claimed, ClaimedItemId: itemID, ClaimedItemTitle: title}, nil
}

// Claim redeems a reward offer for the account.
func (s *DailyRewardServer) Claim(ctx context.Context, req *webv1.ClaimRequest) (*webv1.ClaimResponse, error) {
	outcome, err := s.reward.Claim(ctx, req.GetAccountId(), req.GetRewardItemId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "claim: %v", err)
	}
	return &webv1.ClaimResponse{Result: claimOutcomeToProto(outcome)}, nil
}

func dailyRewardItemToProto(d domain.DailyRewardItem) *webv1.DailyRewardItem {
	return &webv1.DailyRewardItem{
		Id: d.ID, ItemIndex: d.ItemIndex,
		Eff1: int32(d.Eff1), Effv1: int32(d.EffV1),
		Eff2: int32(d.Eff2), Effv2: int32(d.EffV2),
		Eff3: int32(d.Eff3), Effv3: int32(d.EffV3),
		Title: d.Title, Description: d.Description,
		Enabled: d.Enabled, ExpiresDays: d.ExpiresDays,
	}
}

func protoToDailyRewardItem(d *webv1.DailyRewardItem) domain.DailyRewardItem {
	return domain.DailyRewardItem{
		ID: d.GetId(), ItemIndex: d.GetItemIndex(),
		Eff1: uint8(d.GetEff1()), EffV1: uint8(d.GetEffv1()),
		Eff2: uint8(d.GetEff2()), EffV2: uint8(d.GetEffv2()),
		Eff3: uint8(d.GetEff3()), EffV3: uint8(d.GetEffv3()),
		Title: d.GetTitle(), Description: d.GetDescription(),
		Enabled: d.GetEnabled(), ExpiresDays: d.GetExpiresDays(),
	}
}

func dailyRewardAdminResultToProto(r dailyreward.Result) webv1.AdminResult {
	switch r {
	case dailyreward.OK:
		return webv1.AdminResult_ADMIN_RESULT_OK
	case dailyreward.Forbidden:
		return webv1.AdminResult_ADMIN_RESULT_FORBIDDEN
	case dailyreward.NotFound:
		return webv1.AdminResult_ADMIN_RESULT_NOT_FOUND
	default:
		return webv1.AdminResult_ADMIN_RESULT_INVALID
	}
}

func claimOutcomeToProto(o dailyreward.ClaimOutcome) webv1.ClaimResult {
	switch o {
	case dailyreward.ClaimOK:
		return webv1.ClaimResult_CLAIM_RESULT_OK
	case dailyreward.ClaimAlreadyClaimed:
		return webv1.ClaimResult_CLAIM_RESULT_ALREADY_CLAIMED
	case dailyreward.ClaimDisabled:
		return webv1.ClaimResult_CLAIM_RESULT_DISABLED
	default:
		return webv1.ClaimResult_CLAIM_RESULT_NOT_FOUND
	}
}
