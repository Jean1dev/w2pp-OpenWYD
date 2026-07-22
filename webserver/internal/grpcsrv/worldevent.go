package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/worldevent"
)

// WorldEventAdmin is the moderator world-event surface (satisfied by
// *worldevent.Service). Kept as an interface so the server is unit-testable.
type WorldEventAdmin interface {
	Get(ctx context.Context, moderatorID int64) (worldevent.Result, int64, domain.WorldEventConfig, error)
	Set(ctx context.Context, moderatorID int64, cfg domain.WorldEventConfig) (worldevent.Result, error)
}

// WorldEventAdminServer implements webv1.WorldEventAdminServiceServer.
type WorldEventAdminServer struct {
	webv1.UnimplementedWorldEventAdminServiceServer
	admin WorldEventAdmin
}

// NewWorldEventAdmin builds the WorldEventAdminService over the given admin logic.
func NewWorldEventAdmin(a WorldEventAdmin) *WorldEventAdminServer {
	return &WorldEventAdminServer{admin: a}
}

// GetWorldEventConfig returns the current portal-managed event settings.
func (s *WorldEventAdminServer) GetWorldEventConfig(ctx context.Context, req *webv1.GetWorldEventConfigRequest) (*webv1.GetWorldEventConfigResponse, error) {
	res, version, cfg, err := s.admin.Get(ctx, req.GetModeratorId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get world event config: %v", err)
	}
	return &webv1.GetWorldEventConfigResponse{
		Result:  worldEventResultToProto(res),
		Version: version,
		Config:  worldEventConfigToWebProto(cfg),
	}, nil
}

// SetWorldEventConfig replaces the current event settings.
func (s *WorldEventAdminServer) SetWorldEventConfig(ctx context.Context, req *webv1.SetWorldEventConfigRequest) (*webv1.AdminAck, error) {
	res, err := s.admin.Set(ctx, req.GetModeratorId(), webProtoToWorldEventConfig(req.GetConfig()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "set world event config: %v", err)
	}
	return &webv1.AdminAck{Result: worldEventResultToProto(res)}, nil
}

func worldEventConfigToWebProto(cfg domain.WorldEventConfig) *webv1.WorldEventConfig {
	return &webv1.WorldEventConfig{
		Enabled: cfg.Enabled, ItemIndex: cfg.ItemIndex, Rate: cfg.Rate,
		StartIndex: cfg.StartIndex, CurrentIndex: cfg.CurrentIndex, EndIndex: cfg.EndIndex,
		Indexed: cfg.Indexed, NoticeEnabled: cfg.NoticeEnabled,
		DoubleExpEnabled: cfg.DoubleExpEnabled, NewbieEventEnabled: cfg.NewbieEventEnabled,
	}
}

func webProtoToWorldEventConfig(cfg *webv1.WorldEventConfig) domain.WorldEventConfig {
	return domain.WorldEventConfig{
		Enabled: cfg.GetEnabled(), ItemIndex: cfg.GetItemIndex(), Rate: cfg.GetRate(),
		StartIndex: cfg.GetStartIndex(), CurrentIndex: cfg.GetCurrentIndex(), EndIndex: cfg.GetEndIndex(),
		Indexed: cfg.GetIndexed(), NoticeEnabled: cfg.GetNoticeEnabled(),
		DoubleExpEnabled: cfg.GetDoubleExpEnabled(), NewbieEventEnabled: cfg.GetNewbieEventEnabled(),
	}
}

func worldEventResultToProto(r worldevent.Result) webv1.AdminResult {
	switch r {
	case worldevent.OK:
		return webv1.AdminResult_ADMIN_RESULT_OK
	case worldevent.Forbidden:
		return webv1.AdminResult_ADMIN_RESULT_FORBIDDEN
	default:
		return webv1.AdminResult_ADMIN_RESULT_INVALID
	}
}
