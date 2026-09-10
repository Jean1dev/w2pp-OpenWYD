package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/mountbonus"
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
	ListBonus(ctx context.Context) ([]mountgrowth.Bonus, error)
	SetBonus(ctx context.Context, moderatorID int64, moderator string, mountIndex int16, b mountbonus.Bonus) error
	ClearBonus(ctx context.Context, moderatorID int64, mountIndex int16) error
	ConfigVersion(ctx context.Context) (int64, error)
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

// MountConfigVersion answers when the mount overlay last changed, as unix
// seconds.
func (s *MountGrowthAdminServer) MountConfigVersion(ctx context.Context, _ *webv1.MountConfigVersionRequest) (*webv1.MountConfigVersionResponse, error) {
	v, err := s.admin.ConfigVersion(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mount config version: %v", err)
	}
	return &webv1.MountConfigVersionResponse{Version: v}, nil
}

// ListMountBonus returns every lineage's attributes, configured or not, each
// beside its compiled default.
func (s *MountGrowthAdminServer) ListMountBonus(ctx context.Context, _ *webv1.ListMountBonusRequest) (*webv1.ListMountBonusResponse, error) {
	rows, err := s.admin.ListBonus(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list mount bonus: %v", err)
	}
	out := make([]*webv1.AdminMountBonus, 0, len(rows))
	for _, b := range rows {
		out = append(out, &webv1.AdminMountBonus{
			MountIndex:     int32(b.MountIndex),
			DisplayName:    b.DisplayName,
			Configured:     b.Configured,
			Attack:         int32(b.Current.Attack),
			Magic:          int32(b.Current.Magic),
			Evasion:        int32(b.Current.Evasion),
			Resist:         int32(b.Current.Resist),
			DefaultAttack:  int32(b.Default.Attack),
			DefaultMagic:   int32(b.Default.Magic),
			DefaultEvasion: int32(b.Default.Evasion),
			DefaultResist:  int32(b.Default.Resist),
		})
	}
	return &webv1.ListMountBonusResponse{Bonus: out}, nil
}

// SetMountBonus writes one lineage's four numbers.
func (s *MountGrowthAdminServer) SetMountBonus(ctx context.Context, req *webv1.SetMountBonusRequest) (*webv1.AdminAck, error) {
	// Range-checked here and not only in the store, like the absorption: a value
	// outside the model is a caller that disagrees, and InvalidArgument says so
	// where a storage error would read as a database problem. The int32 → int16
	// round trip is part of the check: a number that does not survive it is out
	// of range by definition.
	b := mountbonus.Bonus{
		Attack: int16(req.GetAttack()), Magic: int16(req.GetMagic()),
		Evasion: int16(req.GetEvasion()), Resist: int16(req.GetResist()),
	}
	if !b.Valid() || int32(b.Attack) != req.GetAttack() || int32(b.Magic) != req.GetMagic() ||
		int32(b.Evasion) != req.GetEvasion() || int32(b.Resist) != req.GetResist() {
		return nil, status.Errorf(codes.InvalidArgument, "mount bonus %d/%d/%d/%d is out of range",
			req.GetAttack(), req.GetMagic(), req.GetEvasion(), req.GetResist())
	}
	if !mountbonus.IsAdult(int16(req.GetMountIndex())) {
		return nil, status.Errorf(codes.InvalidArgument, "%d is not an adult mount", req.GetMountIndex())
	}
	if err := s.admin.SetBonus(ctx, req.GetModeratorId(), req.GetModerator(), int16(req.GetMountIndex()), b); err != nil {
		return nil, status.Errorf(codes.Internal, "set mount bonus: %v", err)
	}
	return &webv1.AdminAck{Result: webv1.AdminResult_ADMIN_RESULT_OK}, nil
}

// ClearMountBonus drops the lineage's row so the compiled table applies again.
func (s *MountGrowthAdminServer) ClearMountBonus(ctx context.Context, req *webv1.ClearMountBonusRequest) (*webv1.AdminAck, error) {
	if err := s.admin.ClearBonus(ctx, req.GetModeratorId(), int16(req.GetMountIndex())); err != nil {
		return nil, status.Errorf(codes.Internal, "clear mount bonus: %v", err)
	}
	return &webv1.AdminAck{Result: webv1.AdminResult_ADMIN_RESULT_OK}, nil
}
