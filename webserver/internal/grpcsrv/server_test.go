package grpcsrv

import (
	"context"
	"errors"
	"testing"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/account"
)

// fakeAccounts is a scripted Accounts for testing the gRPC mapping in isolation.
type fakeAccounts struct {
	createRes account.CreateResult
	createID  int64
	createErr error

	verifyOK  bool
	verifyID  int64
	verifyBlk bool
	verifyErr error
}

func (f *fakeAccounts) Create(context.Context, string, string, string) (account.CreateResult, int64, error) {
	return f.createRes, f.createID, f.createErr
}

func (f *fakeAccounts) Verify(context.Context, string, string) (bool, int64, bool, error) {
	return f.verifyOK, f.verifyID, f.verifyBlk, f.verifyErr
}

func TestCreateAccountMapping(t *testing.T) {
	cases := []struct {
		name string
		res  account.CreateResult
		id   int64
		want webv1.CreateResult
	}{
		{"ok", account.CreateOK, 7, webv1.CreateResult_CREATE_RESULT_OK},
		{"taken", account.CreateNameTaken, 0, webv1.CreateResult_CREATE_RESULT_NAME_TAKEN},
		{"invalid", account.CreateInvalid, 0, webv1.CreateResult_CREATE_RESULT_INVALID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := New(&fakeAccounts{createRes: tc.res, createID: tc.id})
			resp, err := s.CreateAccount(context.Background(), &webv1.CreateAccountRequest{Name: "x", Password: "y"})
			if err != nil {
				t.Fatalf("CreateAccount: %v", err)
			}
			if resp.GetResult() != tc.want || resp.GetAccountId() != tc.id {
				t.Fatalf("got result=%v id=%d; want result=%v id=%d", resp.GetResult(), resp.GetAccountId(), tc.want, tc.id)
			}
		})
	}
}

func TestCreateAccountInfraError(t *testing.T) {
	s := New(&fakeAccounts{createErr: errors.New("db down")})
	if _, err := s.CreateAccount(context.Background(), &webv1.CreateAccountRequest{}); err == nil {
		t.Fatal("expected gRPC error on infra failure")
	}
}

func TestVerifyCredentialsMapping(t *testing.T) {
	s := New(&fakeAccounts{verifyOK: true, verifyID: 3, verifyBlk: true})
	resp, err := s.VerifyCredentials(context.Background(), &webv1.VerifyCredentialsRequest{Name: "a", Password: "b"})
	if err != nil {
		t.Fatalf("VerifyCredentials: %v", err)
	}
	if !resp.GetOk() || resp.GetAccountId() != 3 || !resp.GetBlocked() {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
