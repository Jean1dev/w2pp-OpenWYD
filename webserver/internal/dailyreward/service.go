// Package dailyreward holds the web platform's daily-reward logic (issue #35):
// the moderator CRUD over the cold daily_reward_item catalog, and the
// player-facing vitrine + claim-status + claim. Admin operations are gated on
// account.role (mirroring donateshop); the player surface trusts the
// account_id the BFF supplies from its httpOnly session. It never touches live
// game state — a claim only enqueues a delivery_queue grant that the tmServer
// drains inside its single-owner loop.
package dailyreward

import (
	"context"
	"errors"
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

// Store is the persistence surface the service needs (satisfied by *store.Store).
type Store interface {
	AccountRole(ctx context.Context, id int64) (string, error)
	ListDailyRewardItems(ctx context.Context) ([]domain.DailyRewardItem, error)
	ListEnabledDailyRewardItems(ctx context.Context) ([]domain.DailyRewardItem, error)
	UpsertDailyRewardItem(ctx context.Context, d domain.DailyRewardItem, moderatorID int64) (int64, error)
	SetDailyRewardItemEnabled(ctx context.Context, id int64, enabled bool, moderatorID int64) error
	DeleteDailyRewardItem(ctx context.Context, id int64, moderatorID int64) error
	DailyRewardClaimedToday(ctx context.Context, accountID int64) (claimed bool, itemID int64, title string, err error)
	ClaimDailyReward(ctx context.Context, accountID, rewardItemID int64) error
}

// Result is the business outcome of an admin operation, mirroring
// donateshop.Result. Only infra failures are returned as errors.
type Result int

const (
	// OK means the operation succeeded.
	OK Result = iota
	// Forbidden means the caller is not a moderator/admin.
	Forbidden
	// Invalid means the request failed validation.
	Invalid
	// NotFound means the target offer/account does not exist.
	NotFound
)

// ClaimOutcome is the result of a player claim.
type ClaimOutcome int

const (
	// ClaimOK means the claim was recorded and the item queued for delivery.
	ClaimOK ClaimOutcome = iota
	// ClaimAlreadyClaimed means the account already claimed today (UTC).
	ClaimAlreadyClaimed
	// ClaimNotFound means the offer or account does not exist.
	ClaimNotFound
	// ClaimDisabled means the offer exists but is not claimable.
	ClaimDisabled
)

// Service implements the daily-reward operations.
type Service struct {
	store Store
}

// New builds the service over the given store.
func New(s Store) *Service { return &Service{store: s} }

// --- moderator surface ---

// List returns every offer (enabled or not), after authorizing the caller.
func (s *Service) List(ctx context.Context, moderatorID int64) (Result, []domain.DailyRewardItem, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, nil, err
	}
	items, err := s.store.ListDailyRewardItems(ctx)
	if err != nil {
		return Invalid, nil, fmt.Errorf("dailyreward: list: %w", err)
	}
	return OK, items, nil
}

// Upsert creates (d.ID == 0) or updates an offer after validating it. Unlike
// donateshop.Upsert there is no price check — daily rewards are free.
func (s *Service) Upsert(ctx context.Context, moderatorID int64, d domain.DailyRewardItem) (Result, int64, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, 0, err
	}
	if d.ItemIndex <= 0 || d.ExpiresDays < 0 {
		return Invalid, 0, nil
	}
	id, err := s.store.UpsertDailyRewardItem(ctx, d, moderatorID)
	if errors.Is(err, store.ErrNotFound) {
		return NotFound, 0, nil
	}
	if err != nil {
		return Invalid, 0, fmt.Errorf("dailyreward: upsert %d: %w", d.ID, err)
	}
	return OK, id, nil
}

// SetEnabled toggles whether an offer is claimable.
func (s *Service) SetEnabled(ctx context.Context, moderatorID, itemID int64, enabled bool) (Result, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, err
	}
	return classifyWrite(s.store.SetDailyRewardItemEnabled(ctx, itemID, enabled, moderatorID), "set enabled")
}

// Delete removes an offer.
func (s *Service) Delete(ctx context.Context, moderatorID, itemID int64) (Result, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, err
	}
	return classifyWrite(s.store.DeleteDailyRewardItem(ctx, itemID, moderatorID), "delete")
}

// --- player surface ---

// Vitrine returns the enabled offers for the reward front-end.
func (s *Service) Vitrine(ctx context.Context) ([]domain.DailyRewardItem, error) {
	items, err := s.store.ListEnabledDailyRewardItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("dailyreward: vitrine: %w", err)
	}
	return items, nil
}

// ClaimStatus reports whether the account already claimed today (UTC).
func (s *Service) ClaimStatus(ctx context.Context, accountID int64) (claimed bool, itemID int64, title string, err error) {
	claimed, itemID, title, err = s.store.DailyRewardClaimedToday(ctx, accountID)
	if err != nil {
		return false, 0, "", fmt.Errorf("dailyreward: claim status a=%d: %w", accountID, err)
	}
	return claimed, itemID, title, nil
}

// Claim redeems a reward offer for the account for today.
func (s *Service) Claim(ctx context.Context, accountID, rewardItemID int64) (ClaimOutcome, error) {
	if accountID <= 0 || rewardItemID <= 0 {
		return ClaimNotFound, nil
	}
	err := s.store.ClaimDailyReward(ctx, accountID, rewardItemID)
	switch {
	case err == nil:
		return ClaimOK, nil
	case errors.Is(err, store.ErrAlreadyClaimedToday):
		return ClaimAlreadyClaimed, nil
	case errors.Is(err, store.ErrShopItemDisabled):
		return ClaimDisabled, nil
	case errors.Is(err, store.ErrNotFound):
		return ClaimNotFound, nil
	default:
		return ClaimNotFound, fmt.Errorf("dailyreward: claim a=%d item=%d: %w", accountID, rewardItemID, err)
	}
}

// --- helpers ---

// authorize checks the caller has a moderator/admin role. A missing account or
// a plain-player role yields Forbidden (never leaks whether the account exists).
func (s *Service) authorize(ctx context.Context, moderatorID int64) (Result, error) {
	if moderatorID <= 0 {
		return Forbidden, nil
	}
	role, err := s.store.AccountRole(ctx, moderatorID)
	if errors.Is(err, store.ErrNotFound) {
		return Forbidden, nil
	}
	if err != nil {
		return Invalid, fmt.Errorf("dailyreward: role lookup %d: %w", moderatorID, err)
	}
	if role != "moderator" && role != "admin" {
		return Forbidden, nil
	}
	return OK, nil
}

// classifyWrite maps a store write error to a Result: ErrNotFound → NotFound,
// nil → OK, anything else → Invalid (wrapped).
func classifyWrite(err error, op string) (Result, error) {
	switch {
	case err == nil:
		return OK, nil
	case errors.Is(err, store.ErrNotFound):
		return NotFound, nil
	default:
		return Invalid, fmt.Errorf("dailyreward: %s: %w", op, err)
	}
}
