package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/mountgrowth"
)

// MountGrowthAdmin is the moderator surface for the mount growth curves
// (satisfied by *mountgrowth.Service). Kept as an interface so the server is
// unit-testable.
type MountGrowthAdmin interface {
	List(ctx context.Context) ([]mountgrowth.Curve, error)
	Set(ctx context.Context, moderatorID int64, moderator string, mountIndex int16, rates []int16) error
	Clear(ctx context.Context, moderatorID int64, mountIndex int16) error
	ListAbsorb(ctx context.Context) ([]mountgrowth.Absorb, error)
	SetAbsorb(ctx context.Context, moderatorID int64, moderator string, mountIndex, pvp, pve int16) error
	ClearAbsorb(ctx context.Context, moderatorID int64, mountIndex int16) error
}

// MountGrowthAdminServer implements webv1.MountGrowthAdminServiceServer.
type MountGrowthAdminServer struct {
	webv1.UnimplementedMountGrowthAdminServiceServer
	admin MountGrowthAdmin
}

// NewMountGrowthAdmin builds the service over the given admin logic.
func NewMountGrowthAdmin(a MountGrowthAdmin) *MountGrowthAdminServer {
	return &MountGrowthAdminServer{admin: a}
}

// ListMountGrowthCurves returns the whole roster, configured or not.
func (s *MountGrowthAdminServer) ListMountGrowthCurves(ctx context.Context, _ *webv1.ListMountGrowthCurvesRequest) (*webv1.ListMountGrowthCurvesResponse, error) {
	curves, err := s.admin.List(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list mount growth curves: %v", err)
	}
	out := make([]*webv1.AdminMountGrowthCurve, 0, len(curves))
	for _, c := range curves {
		rates := make([]int32, 0, len(c.Rates))
		for _, r := range c.Rates {
			rates = append(rates, int32(r)) // Unset (-1) survives the widening
		}
		out = append(out, &webv1.AdminMountGrowthCurve{
			MountIndex:  int32(c.MountIndex),
			DisplayName: c.DisplayName,
			CriaIndex:   int32(c.CriaIndex),
			AmagoIndex:  int32(c.AmagoIndex),
			Configured:  c.Configured,
			Rates:       rates,
		})
	}
	return &webv1.ListMountGrowthCurvesResponse{Curves: out}, nil
}

// SetMountGrowthCurve writes one lineage's six bands.
func (s *MountGrowthAdminServer) SetMountGrowthCurve(ctx context.Context, req *webv1.SetMountGrowthCurveRequest) (*webv1.AdminAck, error) {
	// The band count is checked here rather than only in the store: a short list
	// is a caller that disagrees about the model, and answering InvalidArgument
	// says so where a storage error would read as a database problem.
	if len(req.GetRates()) != domain.MountGrowthBands {
		return nil, status.Errorf(codes.InvalidArgument,
			"a curve has %d bands, got %d", domain.MountGrowthBands, len(req.GetRates()))
	}
	rates := make([]int16, 0, len(req.GetRates()))
	for _, r := range req.GetRates() {
		if r < 0 || r > 100 {
			return nil, status.Errorf(codes.InvalidArgument, "rate %d is outside 0..100", r)
		}
		rates = append(rates, int16(r))
	}
	if err := s.admin.Set(ctx, req.GetModeratorId(), req.GetModerator(), int16(req.GetMountIndex()), rates); err != nil {
		return nil, status.Errorf(codes.Internal, "set mount growth curve: %v", err)
	}
	return &webv1.AdminAck{Result: webv1.AdminResult_ADMIN_RESULT_OK}, nil
}

// ClearMountGrowthCurve drops the lineage's rows so the default applies again.
func (s *MountGrowthAdminServer) ClearMountGrowthCurve(ctx context.Context, req *webv1.ClearMountGrowthCurveRequest) (*webv1.AdminAck, error) {
	if err := s.admin.Clear(ctx, req.GetModeratorId(), int16(req.GetMountIndex())); err != nil {
		return nil, status.Errorf(codes.Internal, "clear mount growth curve: %v", err)
	}
	return &webv1.AdminAck{Result: webv1.AdminResult_ADMIN_RESULT_OK}, nil
}

// ListMountAbsorb returns every lineage's absorption pair, configured or not.
func (s *MountGrowthAdminServer) ListMountAbsorb(ctx context.Context, _ *webv1.ListMountAbsorbRequest) (*webv1.ListMountAbsorbResponse, error) {
	rows, err := s.admin.ListAbsorb(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list mount absorb: %v", err)
	}
	out := make([]*webv1.AdminMountAbsorb, 0, len(rows))
	for _, a := range rows {
		out = append(out, &webv1.AdminMountAbsorb{
			MountIndex:  int32(a.MountIndex),
			DisplayName: a.DisplayName,
			Configured:  a.Configured,
			AbsorbPvp:   int32(a.PvP),
			AbsorbPve:   int32(a.PvE),
		})
	}
	return &webv1.ListMountAbsorbResponse{Absorb: out}, nil
}

// SetMountAbsorb writes one lineage's pair.
func (s *MountGrowthAdminServer) SetMountAbsorb(ctx context.Context, req *webv1.SetMountAbsorbRequest) (*webv1.AdminAck, error) {
	// Checked here and not only in the store: a value outside the range is a
	// caller that disagrees about the model, and InvalidArgument says so where a
	// storage error would read as a database problem.
	for _, v := range [...]int32{req.GetAbsorbPvp(), req.GetAbsorbPve()} {
		if v < 0 || v > 100 {
			return nil, status.Errorf(codes.InvalidArgument, "absorb %d is outside 0..100", v)
		}
	}
	if err := s.admin.SetAbsorb(ctx, req.GetModeratorId(), req.GetModerator(),
		int16(req.GetMountIndex()), int16(req.GetAbsorbPvp()), int16(req.GetAbsorbPve())); err != nil {
		return nil, status.Errorf(codes.Internal, "set mount absorb: %v", err)
	}
	return &webv1.AdminAck{Result: webv1.AdminResult_ADMIN_RESULT_OK}, nil
}

// ClearMountAbsorb drops the lineage's row so the default applies again.
func (s *MountGrowthAdminServer) ClearMountAbsorb(ctx context.Context, req *webv1.ClearMountAbsorbRequest) (*webv1.AdminAck, error) {
	if err := s.admin.ClearAbsorb(ctx, req.GetModeratorId(), int16(req.GetMountIndex())); err != nil {
		return nil, status.Errorf(codes.Internal, "clear mount absorb: %v", err)
	}
	return &webv1.AdminAck{Result: webv1.AdminResult_ADMIN_RESULT_OK}, nil
}
