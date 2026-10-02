// Package grpcsrv implements the web-api's gRPC AccountWebService (api/web/v1)
// over the account service. It is the edge the Next.js BFF calls server-side
// over gRPC+mTLS (web-platform-plan.md); the browser never reaches here.
package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/playcode"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/account"
)

// Accounts is the account-logic surface the server depends on (satisfied by
// *account.Service). Kept as an interface so the server is unit-testable.
type Accounts interface {
	Create(ctx context.Context, name, password, email string) (account.CreateResult, int64, error)
	Verify(ctx context.Context, name, password string) (ok bool, accountID int64, blocked bool, role string, err error)
	IssuePlayCode(ctx context.Context, accountID int64) (res account.PlayCodeResult, name, code string, err error)
}

// Server implements webv1.AccountWebServiceServer.
type Server struct {
	webv1.UnimplementedAccountWebServiceServer
	accounts  Accounts
	playCodes *playcode.Verifier // nil: IssuePlayCode answers DISABLED
}

// New builds the AccountWebService over the given account logic.
func New(a Accounts) *Server { return &Server{accounts: a} }

// WithPlayCodes enables IssuePlayCode with the portal's assertion verifier
// (nil keeps it disabled).
func (s *Server) WithPlayCodes(v *playcode.Verifier) *Server {
	s.playCodes = v
	return s
}

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
	ok, id, blocked, role, err := s.accounts.Verify(ctx, req.GetName(), req.GetPassword())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "verify credentials: %v", err)
	}
	return &webv1.VerifyCredentialsResponse{Ok: ok, AccountId: id, Blocked: blocked, Role: role}, nil
}

// IssuePlayCode returns a one-time web client login code. This service is
// reachable without a client certificate behind the platform's HTTPS edge, so
// the account id is only taken from a valid portal assertion, never from the
// request alone (web client ADR 017).
func (s *Server) IssuePlayCode(ctx context.Context, req *webv1.IssuePlayCodeRequest) (*webv1.IssuePlayCodeResponse, error) {
	if s.playCodes == nil {
		return &webv1.IssuePlayCodeResponse{Result: webv1.PlayCodeResult_PLAY_CODE_RESULT_DISABLED}, nil
	}
	accountID, err := s.playCodes.Verify(req.GetAssertion())
	if err != nil {
		return &webv1.IssuePlayCodeResponse{Result: webv1.PlayCodeResult_PLAY_CODE_RESULT_INVALID_ASSERTION}, nil
	}
	res, name, code, err := s.accounts.IssuePlayCode(ctx, accountID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "issue play code: %v", err)
	}
	switch res {
	case account.PlayCodeOK:
		return &webv1.IssuePlayCodeResponse{
			Result:           webv1.PlayCodeResult_PLAY_CODE_RESULT_OK,
			AccountName:      name,
			Code:             code,
			ExpiresInSeconds: int32(playcode.TTL.Seconds()),
		}, nil
	case account.PlayCodeBlocked:
		return &webv1.IssuePlayCodeResponse{Result: webv1.PlayCodeResult_PLAY_CODE_RESULT_BLOCKED}, nil
	default:
		return &webv1.IssuePlayCodeResponse{Result: webv1.PlayCodeResult_PLAY_CODE_RESULT_NO_ACCOUNT}, nil
	}
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
