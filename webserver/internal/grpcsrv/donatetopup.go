package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/donatetopup"
)

// DonateTopup is the donate top-up surface (satisfied by *donatetopup.Service).
// Kept as an interface so the server is unit-testable.
type DonateTopup interface {
	SavePayerProfile(ctx context.Context, accountID int64, name, cpf string) (donatetopup.Result, error)
	GetPayerProfile(ctx context.Context, accountID int64) (found bool, name, cpf string, err error)
	CreateTopupOrder(ctx context.Context, o domain.TopupOrder) (donatetopup.Result, int64, error)
	ConfirmTopupOrder(ctx context.Context, externalRef string) (donatetopup.ConfirmOutcome, int64, error)
	GetTopupOrder(ctx context.Context, externalRef string, accountID int64) (status int16, credits int32, newBalance int64, err error)
}

// DonateTopupServer implements webv1.DonateTopupServiceServer.
type DonateTopupServer struct {
	webv1.UnimplementedDonateTopupServiceServer
	topup DonateTopup
}

// NewDonateTopup builds the DonateTopupService over the given top-up logic.
func NewDonateTopup(t DonateTopup) *DonateTopupServer { return &DonateTopupServer{topup: t} }

// GetPayerProfile returns the payer's stored name + CPF (found=false when none).
func (s *DonateTopupServer) GetPayerProfile(ctx context.Context, req *webv1.GetPayerProfileRequest) (*webv1.GetPayerProfileResponse, error) {
	found, name, cpf, err := s.topup.GetPayerProfile(ctx, req.GetAccountId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get payer profile: %v", err)
	}
	return &webv1.GetPayerProfileResponse{Found: found, Name: name, Cpf: cpf}, nil
}

// SavePayerProfile upserts the payer's name + CPF.
func (s *DonateTopupServer) SavePayerProfile(ctx context.Context, req *webv1.SavePayerProfileRequest) (*webv1.SavePayerProfileResponse, error) {
	res, err := s.topup.SavePayerProfile(ctx, req.GetAccountId(), req.GetName(), req.GetCpf())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "save payer profile: %v", err)
	}
	return &webv1.SavePayerProfileResponse{Result: topupResultToProto(res)}, nil
}

// CreateTopupOrder persists a PENDING order.
func (s *DonateTopupServer) CreateTopupOrder(ctx context.Context, req *webv1.CreateTopupOrderRequest) (*webv1.CreateTopupOrderResponse, error) {
	res, id, err := s.topup.CreateTopupOrder(ctx, domain.TopupOrder{
		AccountID:         req.GetAccountId(),
		ExternalReference: req.GetExternalReference(),
		Credits:           req.GetCredits(),
		AmountCents:       req.GetAmountCents(),
		PaymentMethod:     int16(req.GetPaymentMethod()),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create topup order: %v", err)
	}
	return &webv1.CreateTopupOrderResponse{Result: topupResultToProto(res), OrderId: id}, nil
}

// ConfirmTopupOrder settles a paid order idempotently.
func (s *DonateTopupServer) ConfirmTopupOrder(ctx context.Context, req *webv1.ConfirmTopupOrderRequest) (*webv1.ConfirmTopupOrderResponse, error) {
	outcome, newBal, err := s.topup.ConfirmTopupOrder(ctx, req.GetExternalReference())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "confirm topup order: %v", err)
	}
	return &webv1.ConfirmTopupOrderResponse{Result: confirmOutcomeToProto(outcome), NewBalance: newBal}, nil
}

// GetTopupOrder returns the current status/credits for the polling portal.
func (s *DonateTopupServer) GetTopupOrder(ctx context.Context, req *webv1.GetTopupOrderRequest) (*webv1.GetTopupOrderResponse, error) {
	st, credits, newBal, err := s.topup.GetTopupOrder(ctx, req.GetExternalReference(), req.GetAccountId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get topup order: %v", err)
	}
	return &webv1.GetTopupOrderResponse{
		Status:     topupStatusToProto(st),
		Credits:    credits,
		NewBalance: newBal,
	}, nil
}

// topupResultToProto maps the profile/create outcome to the shared AdminResult.
func topupResultToProto(r donatetopup.Result) webv1.AdminResult {
	switch r {
	case donatetopup.OK:
		return webv1.AdminResult_ADMIN_RESULT_OK
	case donatetopup.NotFound:
		return webv1.AdminResult_ADMIN_RESULT_NOT_FOUND
	default:
		return webv1.AdminResult_ADMIN_RESULT_INVALID
	}
}

func confirmOutcomeToProto(o donatetopup.ConfirmOutcome) webv1.TopupResult {
	switch o {
	case donatetopup.Confirmed:
		return webv1.TopupResult_TOPUP_RESULT_CONFIRMED
	case donatetopup.AlreadyConfirmed:
		return webv1.TopupResult_TOPUP_RESULT_ALREADY_CONFIRMED
	default:
		return webv1.TopupResult_TOPUP_RESULT_NOT_FOUND
	}
}

// topupStatusToProto maps the stored status int (0=none, 1=PENDING, 2=PAID) to
// the wire enum; an unknown/absent order (0) is TOPUP_STATUS_UNSPECIFIED.
func topupStatusToProto(st int16) webv1.TopupStatus {
	switch st {
	case 1:
		return webv1.TopupStatus_TOPUP_STATUS_PENDING
	case 2:
		return webv1.TopupStatus_TOPUP_STATUS_PAID
	default:
		return webv1.TopupStatus_TOPUP_STATUS_UNSPECIFIED
	}
}
