package grpcsrv

import (
	"context"
	"testing"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/donatetopup"
)

// fakeTopup is a scripted DonateTopup for testing the gRPC mapping in isolation.
type fakeTopup struct {
	saveRes    donatetopup.Result
	found      bool
	name, cpf  string
	createRes  donatetopup.Result
	createID   int64
	confirmOut donatetopup.ConfirmOutcome
	confirmBal int64
	getStatus  int16
	getCredits int32
	getBalance int64

	lastOrder domain.TopupOrder
	lastRef   string
}

func (f *fakeTopup) SavePayerProfile(_ context.Context, _ int64, name, cpf string) (donatetopup.Result, error) {
	f.name, f.cpf = name, cpf
	return f.saveRes, nil
}
func (f *fakeTopup) GetPayerProfile(context.Context, int64) (bool, string, string, error) {
	return f.found, f.name, f.cpf, nil
}
func (f *fakeTopup) CreateTopupOrder(_ context.Context, o domain.TopupOrder) (donatetopup.Result, int64, error) {
	f.lastOrder = o
	return f.createRes, f.createID, nil
}
func (f *fakeTopup) ConfirmTopupOrder(_ context.Context, ref string) (donatetopup.ConfirmOutcome, int64, error) {
	f.lastRef = ref
	return f.confirmOut, f.confirmBal, nil
}
func (f *fakeTopup) GetTopupOrder(_ context.Context, ref string, _ int64) (int16, int32, int64, error) {
	f.lastRef = ref
	return f.getStatus, f.getCredits, f.getBalance, nil
}

// TestSavePayerProfileMapping verifies fields flow through and Result maps to AdminResult.
func TestSavePayerProfileMapping(t *testing.T) {
	f := &fakeTopup{saveRes: donatetopup.OK}
	s := NewDonateTopup(f)
	resp, err := s.SavePayerProfile(context.Background(), &webv1.SavePayerProfileRequest{
		AccountId: 7, Name: "Jean", Cpf: "12345678909",
	})
	if err != nil {
		t.Fatalf("SavePayerProfile: %v", err)
	}
	if resp.GetResult() != webv1.AdminResult_ADMIN_RESULT_OK {
		t.Errorf("result = %v, want OK", resp.GetResult())
	}
	if f.name != "Jean" || f.cpf != "12345678909" {
		t.Errorf("service got (%q, %q)", f.name, f.cpf)
	}
}

// TestCreateTopupOrderMapping verifies the request maps into a domain.TopupOrder.
func TestCreateTopupOrderMapping(t *testing.T) {
	f := &fakeTopup{createRes: donatetopup.OK, createID: 42}
	s := NewDonateTopup(f)
	resp, err := s.CreateTopupOrder(context.Background(), &webv1.CreateTopupOrderRequest{
		AccountId: 7, ExternalReference: "ref-x", Credits: 10, AmountCents: 990,
		PaymentMethod: webv1.PaymentMethod_PAYMENT_METHOD_PIX,
	})
	if err != nil {
		t.Fatalf("CreateTopupOrder: %v", err)
	}
	if resp.GetResult() != webv1.AdminResult_ADMIN_RESULT_OK || resp.GetOrderId() != 42 {
		t.Errorf("resp = (%v, %d)", resp.GetResult(), resp.GetOrderId())
	}
	got := f.lastOrder
	if got.AccountID != 7 || got.ExternalReference != "ref-x" || got.Credits != 10 ||
		got.AmountCents != 990 || got.PaymentMethod != 1 {
		t.Errorf("mapped order = %+v", got)
	}
}

// TestConfirmTopupOrderMapping maps every ConfirmOutcome to its wire enum.
func TestConfirmTopupOrderMapping(t *testing.T) {
	tests := []struct {
		out  donatetopup.ConfirmOutcome
		want webv1.TopupResult
	}{
		{donatetopup.Confirmed, webv1.TopupResult_TOPUP_RESULT_CONFIRMED},
		{donatetopup.AlreadyConfirmed, webv1.TopupResult_TOPUP_RESULT_ALREADY_CONFIRMED},
		{donatetopup.ConfirmNotFound, webv1.TopupResult_TOPUP_RESULT_NOT_FOUND},
	}
	for _, tc := range tests {
		f := &fakeTopup{confirmOut: tc.out, confirmBal: 150}
		s := NewDonateTopup(f)
		resp, err := s.ConfirmTopupOrder(context.Background(), &webv1.ConfirmTopupOrderRequest{ExternalReference: "r"})
		if err != nil {
			t.Fatalf("ConfirmTopupOrder: %v", err)
		}
		if resp.GetResult() != tc.want {
			t.Errorf("outcome %v -> %v, want %v", tc.out, resp.GetResult(), tc.want)
		}
		if resp.GetNewBalance() != 150 {
			t.Errorf("new_balance = %d, want 150", resp.GetNewBalance())
		}
	}
}

// TestGetTopupOrderMapping maps stored status ints to the wire TopupStatus enum.
func TestGetTopupOrderMapping(t *testing.T) {
	tests := []struct {
		status int16
		want   webv1.TopupStatus
	}{
		{0, webv1.TopupStatus_TOPUP_STATUS_UNSPECIFIED},
		{1, webv1.TopupStatus_TOPUP_STATUS_PENDING},
		{2, webv1.TopupStatus_TOPUP_STATUS_PAID},
	}
	for _, tc := range tests {
		f := &fakeTopup{getStatus: tc.status, getCredits: 20, getBalance: 200}
		s := NewDonateTopup(f)
		resp, err := s.GetTopupOrder(context.Background(), &webv1.GetTopupOrderRequest{ExternalReference: "r", AccountId: 7})
		if err != nil {
			t.Fatalf("GetTopupOrder: %v", err)
		}
		if resp.GetStatus() != tc.want || resp.GetCredits() != 20 || resp.GetNewBalance() != 200 {
			t.Errorf("status %d -> (%v, %d, %d)", tc.status, resp.GetStatus(), resp.GetCredits(), resp.GetNewBalance())
		}
	}
}

// TestGetPayerProfileMapping passes through found/name/cpf.
func TestGetPayerProfileMapping(t *testing.T) {
	f := &fakeTopup{found: true, name: "Jean", cpf: "12345678909"}
	s := NewDonateTopup(f)
	resp, err := s.GetPayerProfile(context.Background(), &webv1.GetPayerProfileRequest{AccountId: 7})
	if err != nil {
		t.Fatalf("GetPayerProfile: %v", err)
	}
	if !resp.GetFound() || resp.GetName() != "Jean" || resp.GetCpf() != "12345678909" {
		t.Errorf("resp = %+v", resp)
	}
}
