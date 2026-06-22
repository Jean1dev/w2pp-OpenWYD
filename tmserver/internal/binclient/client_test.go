package binclient

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"

	binv1 "github.com/jeanluca/w2pp-openwyd/api/bin/v1"
)

type fakeAPI struct {
	resp *binv1.CheckBillingResponse
	err  error
}

func (f *fakeAPI) CheckBilling(_ context.Context, _ *binv1.CheckBillingRequest, _ ...grpc.CallOption) (*binv1.CheckBillingResponse, error) {
	return f.resp, f.err
}

func TestCheckMapping(t *testing.T) {
	for _, tc := range []struct {
		name    string
		allowed bool
	}{{"allowed", true}, {"denied", false}} {
		t.Run(tc.name, func(t *testing.T) {
			c := &Client{api: &fakeAPI{resp: &binv1.CheckBillingResponse{Allowed: tc.allowed}}}
			got, err := c.Check(context.Background(), "acct")
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			if got != tc.allowed {
				t.Fatalf("got %v, want %v", got, tc.allowed)
			}
		})
	}
}

func TestCheckError(t *testing.T) {
	c := &Client{api: &fakeAPI{err: errors.New("rpc down")}}
	if _, err := c.Check(context.Background(), "acct"); err == nil {
		t.Fatal("expected error to propagate")
	}
}
