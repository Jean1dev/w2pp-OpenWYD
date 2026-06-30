// Package grpcsrv implements the web-api's gRPC AccountWebService (api/web/v1)
// over the account service. It is the edge the Next.js BFF calls server-side
// over gRPC+mTLS (web-platform-plan.md); the browser never reaches here.
package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/account"
)

// Accounts is the account-logic surface the server depends on (satisfied by
// *account.Service). Kept as an interface so the server is unit-testable.
type Accounts interface {
	Create(ctx context.Context, name, password, email string) (account.CreateResult, int64, error)
	Verify(ctx context.Context, name, password string) (ok bool, accountID int64, blocked bool, err error)
}

// Server implements webv1.AccountWebServiceServer.
type Server struct {
	webv1.UnimplementedAccountWebServiceServer
	accounts Accounts
}

// New builds the AccountWebService over the given account logic.
func New(a Accounts) *Server { return &Server{accounts: a} }

// CreateAccount registers a new account. Business outcomes (name taken, invalid
// input) ride in the response enum; only infra failures become gRPC errors.
func (s *Server) CreateAccount(ctx context.Context, req *webv1.CreateAccountRequest) (*webv1.CreateAccountResponse, error) {
	res, id, err := s.accounts.Create(ctx, req.GetName(), req.GetPassword(), req.GetEmail())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create account: %v", err)
	}
	return &webv1.CreateAccountResponse{Result: createResultToProto(res), AccountId: id}, nil
}

// VerifyCredentials validates name + password for the BFF session cookie.
func (s *Server) VerifyCredentials(ctx context.Context, req *webv1.VerifyCredentialsRequest) (*webv1.VerifyCredentialsResponse, error) {
	ok, id, blocked, err := s.accounts.Verify(ctx, req.GetName(), req.GetPassword())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "verify credentials: %v", err)
	}
	return &webv1.VerifyCredentialsResponse{Ok: ok, AccountId: id, Blocked: blocked}, nil
}

// createResultToProto maps the domain outcome to the wire enum.
func createResultToProto(r account.CreateResult) webv1.CreateResult {
	switch r {
	case account.CreateOK:
		return webv1.CreateResult_CREATE_RESULT_OK
	case account.CreateNameTaken:
		return webv1.CreateResult_CREATE_RESULT_NAME_TAKEN
	default:
		return webv1.CreateResult_CREATE_RESULT_INVALID
	}
}
