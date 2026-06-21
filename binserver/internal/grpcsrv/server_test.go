package grpcsrv

import (
	"context"
	"testing"
	"time"

	binv1 "github.com/jeanluca/w2pp-openwyd/api/bin/v1"
	"github.com/jeanluca/w2pp-openwyd/binserver/internal/billing"
)

func TestCheckBilling(t *testing.T) {
	now := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)
	svc := billing.New(false) // unknown accounts denied
	svc.Set(billing.Account{Name: "active", Status: billing.StatusActive})
	svc.Set(billing.Account{Name: "blocked", Status: billing.StatusBlocked})
	svc.Set(billing.Account{Name: "expired", Status: billing.StatusActive, ExpiresAt: now.Add(-time.Hour)})

	s := New(svc, func() time.Time { return now })

	cases := []struct {
		account     string
		wantAllowed bool
		wantStatus  binv1.Status
	}{
		{"active", true, binv1.Status_STATUS_ACTIVE},
		{"blocked", false, binv1.Status_STATUS_BLOCKED},
		{"expired", false, binv1.Status_STATUS_EXPIRED},
		{"ghost", false, binv1.Status_STATUS_UNSPECIFIED},
	}
	for _, tc := range cases {
		t.Run(tc.account, func(t *testing.T) {
			resp, err := s.CheckBilling(context.Background(), &binv1.CheckBillingRequest{AccountName: tc.account})
			if err != nil {
				t.Fatalf("CheckBilling: %v", err)
			}
			if resp.GetAllowed() != tc.wantAllowed || resp.GetStatus() != tc.wantStatus {
				t.Errorf("got allowed=%v status=%v, want allowed=%v status=%v",
					resp.GetAllowed(), resp.GetStatus(), tc.wantAllowed, tc.wantStatus)
			}
		})
	}
}
