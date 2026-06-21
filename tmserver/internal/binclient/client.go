// Package binclient adapts the binServer gRPC BillingService (api/bin/v1) to the
// world.Billing port. The character-login gate calls it (off the loop via
// World.Go) to decide whether an account may enter the world (migration-plan.md
// §4, Fase 6).
package binclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	binv1 "github.com/jeanluca/w2pp-openwyd/api/bin/v1"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Client is a world.Billing backed by the binServer.
type Client struct {
	api binv1.BillingServiceClient
}

// New wraps a gRPC connection as a Billing gate.
func New(conn grpc.ClientConnInterface) *Client {
	return &Client{api: binv1.NewBillingServiceClient(conn)}
}

var _ world.Billing = (*Client)(nil)

// Check asks the binServer whether the account may enter the world.
func (c *Client) Check(ctx context.Context, accountName string) (bool, error) {
	resp, err := c.api.CheckBilling(ctx, &binv1.CheckBillingRequest{AccountName: accountName})
	if err != nil {
		return false, fmt.Errorf("binclient: check billing: %w", err)
	}
	return resp.GetAllowed(), nil
}
