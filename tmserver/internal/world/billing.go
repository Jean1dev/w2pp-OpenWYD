package world

import "context"

// Billing is the port the character-login gate uses to ask the binServer whether
// an account may enter the world (api/bin/v1). This is the NEW billing boundary
// designed in Fase 6 (migration-plan.md §4) — the legacy _AUTH_GAME link is not
// reproduced. The real implementation is a gRPC client adapter; the world
// depends only on this interface. Check is called OFF the loop via World.Go.
type Billing interface {
	Check(ctx context.Context, accountName string) (allowed bool, err error)
}

// AllowAllBilling is the default gate when no binServer is wired (free-to-play
// bring-up and tests): every account is allowed.
type AllowAllBilling struct{}

// Check always allows.
func (AllowAllBilling) Check(context.Context, string) (bool, error) { return true, nil }
