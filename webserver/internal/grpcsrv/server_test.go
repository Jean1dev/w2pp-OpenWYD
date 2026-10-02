package grpcsrv

import (
	"context"
	"errors"
	"testing"
	"time"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/playcode"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/account"
)

// fakeAccounts is a scripted Accounts for testing the gRPC mapping in isolation.
type fakeAccounts struct {
	createRes account.CreateResult
	createID  int64
	createErr error

	verifyOK   bool
	verifyID   int64
	verifyBlk  bool
	verifyRole string
	verifyErr  error

	issueRes  account.PlayCodeResult
	issueName string
	issueCode string
	issueErr  error
	issuedFor int64
}

func (f *fakeAccounts) IssuePlayCode(_ context.Context, accountID int64) (account.PlayCodeResult, string, string, error) {
	f.issuedFor = accountID
	return f.issueRes, f.issueName, f.issueCode, f.issueErr
}

func (f *fakeAccounts) Create(context.Context, string, string, string) (account.CreateResult, int64, error) {
	return f.createRes, f.createID, f.createErr
}

func (f *fakeAccounts) Verify(context.Context, string, string) (bool, int64, bool, string, error) {
	return f.verifyOK, f.verifyID, f.verifyBlk, f.verifyRole, f.verifyErr
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

const playCodeSecret = "wyd-play-code-test-secret-0123456789abcdef"

func assertion(sub string) string {
	now := time.Now().Unix()
	return playcode.Sign([]byte(playCodeSecret), playcode.Claims{
		Sub: sub, Iat: now, Exp: now + 60, Jti: sub + "-0123456789abcdef-" + time.Now().Format("150405.000000000"),
	})
}

func TestIssuePlayCode(t *testing.T) {
	ctx := context.Background()

	// Without a secret the RPC is off and never reaches the accounts.
	off := &fakeAccounts{}
	resp, err := New(off).IssuePlayCode(ctx, &webv1.IssuePlayCodeRequest{Assertion: assertion("7")})
	if err != nil || resp.GetResult() != webv1.PlayCodeResult_PLAY_CODE_RESULT_DISABLED || off.issuedFor != 0 {
		t.Fatalf("disabled: %v %v issuedFor=%d", resp.GetResult(), err, off.issuedFor)
	}

	fa := &fakeAccounts{issueRes: account.PlayCodeOK, issueName: "alice", issueCode: "abcdefgh29"}
	s := New(fa).WithPlayCodes(playcode.NewVerifier(playCodeSecret))
	resp, err = s.IssuePlayCode(ctx, &webv1.IssuePlayCodeRequest{Assertion: assertion("7")})
	if err != nil || resp.GetResult() != webv1.PlayCodeResult_PLAY_CODE_RESULT_OK || fa.issuedFor != 7 ||
		resp.GetAccountName() != "alice" || resp.GetCode() != "abcdefgh29" || resp.GetExpiresInSeconds() != 120 {
		t.Fatalf("ok: %+v err=%v issuedFor=%d", resp, err, fa.issuedFor)
	}

	// A forged or unsigned request never picks the account.
	fa.issuedFor = 0
	forged := playcode.Sign([]byte("another-secret-0123456789abcdef-xyz"), playcode.Claims{Sub: "9", Iat: time.Now().Unix(), Exp: time.Now().Unix() + 60, Jti: "0123456789abcdef0123"})
	for _, a := range []string{"", "garbage", forged} {
		resp, err = s.IssuePlayCode(ctx, &webv1.IssuePlayCodeRequest{Assertion: a})
		if err != nil || resp.GetResult() != webv1.PlayCodeResult_PLAY_CODE_RESULT_INVALID_ASSERTION || fa.issuedFor != 0 || resp.GetCode() != "" {
			t.Fatalf("assertion %q: %v %v issuedFor=%d", a, resp.GetResult(), err, fa.issuedFor)
		}
	}

	for _, tc := range []struct {
		res  account.PlayCodeResult
		want webv1.PlayCodeResult
	}{
		{account.PlayCodeBlocked, webv1.PlayCodeResult_PLAY_CODE_RESULT_BLOCKED},
		{account.PlayCodeNoAccount, webv1.PlayCodeResult_PLAY_CODE_RESULT_NO_ACCOUNT},
	} {
		s := New(&fakeAccounts{issueRes: tc.res}).WithPlayCodes(playcode.NewVerifier(playCodeSecret))
		resp, err := s.IssuePlayCode(ctx, &webv1.IssuePlayCodeRequest{Assertion: assertion("7")})
		if err != nil || resp.GetResult() != tc.want || resp.GetCode() != "" {
			t.Fatalf("%v: %v err=%v", tc.res, resp.GetResult(), err)
		}
	}

	s = New(&fakeAccounts{issueErr: errors.New("db down")}).WithPlayCodes(playcode.NewVerifier(playCodeSecret))
	if _, err := s.IssuePlayCode(ctx, &webv1.IssuePlayCodeRequest{Assertion: assertion("7")}); err == nil {
		t.Fatal("infra error was swallowed")
	}
}
