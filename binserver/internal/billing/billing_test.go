package billing

import (
	"testing"
	"time"
)

func TestCheck(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	s := New(true) // free-to-play default
	s.Set(Account{Name: "active", Status: StatusActive})
	s.Set(Account{Name: "blocked", Status: StatusBlocked})
	s.Set(Account{Name: "expired", Status: StatusActive, ExpiresAt: now.Add(-time.Hour)})
	s.Set(Account{Name: "valid", Status: StatusActive, ExpiresAt: now.Add(time.Hour)})

	cases := []struct {
		name        string
		wantAllowed bool
		wantStatus  Status
	}{
		{"active", true, StatusActive},
		{"blocked", false, StatusBlocked},
		{"expired", false, StatusExpired},
		{"valid", true, StatusActive},
		{"unknown", true, StatusUnknown}, // free-to-play allows unknown
	}
	for _, tt := range cases {
		got := s.Check(tt.name, now)
		if got.Allowed != tt.wantAllowed || got.Status != tt.wantStatus {
			t.Errorf("Check(%s) = %+v, want allowed=%v status=%v", tt.name, got, tt.wantAllowed, tt.wantStatus)
		}
	}
}

func TestCheckUnknownDenied(t *testing.T) {
	s := New(false) // require a record
	if got := s.Check("ghost", time.Now()); got.Allowed {
		t.Errorf("unknown account allowed under deny policy")
	}
}
