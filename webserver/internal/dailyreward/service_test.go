package dailyreward

import (
	"context"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

// fakeStore is an in-memory Store for exercising authorization, validation and
// the claim outcomes without a database.
type fakeStore struct {
	roles  map[int64]string
	items  map[int64]domain.DailyRewardItem
	claims map[int64]struct {
		itemID int64
		title  string
	} // accountID -> today's claim
	claimErr  error // forces ClaimDailyReward to return this
	upsertErr error
	lastClaim int64
}

func (f *fakeStore) AccountRole(_ context.Context, id int64) (string, error) {
	r, ok := f.roles[id]
	if !ok {
		return "", store.ErrNotFound
	}
	return r, nil
}
func (f *fakeStore) ListDailyRewardItems(context.Context) ([]domain.DailyRewardItem, error) {
	return f.all(false), nil
}
func (f *fakeStore) ListEnabledDailyRewardItems(context.Context) ([]domain.DailyRewardItem, error) {
	return f.all(true), nil
}
func (f *fakeStore) all(enabledOnly bool) []domain.DailyRewardItem {
	out := make([]domain.DailyRewardItem, 0, len(f.items))
	for _, it := range f.items {
		if enabledOnly && !it.Enabled {
			continue
		}
		out = append(out, it)
	}
	return out
}
func (f *fakeStore) UpsertDailyRewardItem(_ context.Context, d domain.DailyRewardItem, _ int64) (int64, error) {
	if f.upsertErr != nil {
		return 0, f.upsertErr
	}
	if d.ID != 0 {
		if _, ok := f.items[d.ID]; !ok {
			return 0, store.ErrNotFound
		}
		return d.ID, nil
	}
	return 99, nil
}
func (f *fakeStore) SetDailyRewardItemEnabled(_ context.Context, id int64, _ bool, _ int64) error {
	if _, ok := f.items[id]; !ok {
		return store.ErrNotFound
	}
	return nil
}
func (f *fakeStore) DeleteDailyRewardItem(_ context.Context, id int64, _ int64) error {
	if _, ok := f.items[id]; !ok {
		return store.ErrNotFound
	}
	return nil
}
func (f *fakeStore) DailyRewardClaimedToday(_ context.Context, accountID int64) (bool, int64, string, error) {
	c, ok := f.claims[accountID]
	if !ok {
		return false, 0, "", nil
	}
	return true, c.itemID, c.title, nil
}
func (f *fakeStore) ClaimDailyReward(_ context.Context, accountID, rewardItemID int64) error {
	if f.claimErr != nil {
		return f.claimErr
	}
	f.lastClaim = rewardItemID
	if f.claims == nil {
		f.claims = map[int64]struct {
			itemID int64
			title  string
		}{}
	}
	f.claims[accountID] = struct {
		itemID int64
		title  string
	}{itemID: rewardItemID, title: f.items[rewardItemID].Title}
	return nil
}

func newFake() *fakeStore {
	return &fakeStore{
		roles: map[int64]string{1: "moderator", 2: "admin", 3: "player"},
		items: map[int64]domain.DailyRewardItem{
			10: {ID: 10, ItemIndex: 100, Title: "Potion", Enabled: true},
		},
	}
}

// TestAdminAuthorization checks every moderator operation refuses non-moderators.
func TestAdminAuthorization(t *testing.T) {
	tests := []struct {
		name       string
		moderator  int64
		wantResult Result
	}{
		{"moderator", 1, OK},
		{"admin", 2, OK},
		{"player", 3, Forbidden},
		{"missing account", 999, Forbidden},
		{"zero id", 0, Forbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New(newFake())
			ctx := context.Background()

			if r, _, err := s.List(ctx, tc.moderator); err != nil || r != tc.wantResult {
				t.Errorf("List = (%v, %v), want %v", r, err, tc.wantResult)
			}
			valid := domain.DailyRewardItem{ItemIndex: 100}
			if r, _, err := s.Upsert(ctx, tc.moderator, valid); err != nil || r != tc.wantResult {
				t.Errorf("Upsert = (%v, %v), want %v", r, err, tc.wantResult)
			}
			if r, err := s.SetEnabled(ctx, tc.moderator, 10, false); err != nil || r != tc.wantResult {
				t.Errorf("SetEnabled = (%v, %v), want %v", r, err, tc.wantResult)
			}
			if r, err := s.Delete(ctx, tc.moderator, 10); err != nil || r != tc.wantResult {
				t.Errorf("Delete = (%v, %v), want %v", r, err, tc.wantResult)
			}
		})
	}
}

// TestUpsertValidation rejects offers with a bad item index or negative expiry.
// Unlike donateshop there is no price check — daily rewards are free.
func TestUpsertValidation(t *testing.T) {
	s := New(newFake())
	ctx := context.Background()
	bad := []domain.DailyRewardItem{
		{ItemIndex: 0},                    // no item
		{ItemIndex: 100, ExpiresDays: -1}, // negative expiry
	}
	for _, d := range bad {
		if r, _, err := s.Upsert(ctx, 1, d); err != nil || r != Invalid {
			t.Errorf("Upsert(%+v) = (%v, %v), want Invalid", d, r, err)
		}
	}
}

// TestClaimOutcomes maps every store claim result to the right ClaimOutcome.
func TestClaimOutcomes(t *testing.T) {
	tests := []struct {
		name     string
		claimErr error
		account  int64
		item     int64
		want     ClaimOutcome
	}{
		{"ok", nil, 7, 10, ClaimOK},
		{"already claimed", store.ErrAlreadyClaimedToday, 7, 10, ClaimAlreadyClaimed},
		{"disabled", store.ErrShopItemDisabled, 7, 10, ClaimDisabled},
		{"not found", store.ErrNotFound, 7, 10, ClaimNotFound},
		{"bad account id", nil, 0, 10, ClaimNotFound},
		{"bad item id", nil, 7, 0, ClaimNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFake()
			f.claimErr = tc.claimErr
			s := New(f)
			got, err := s.Claim(context.Background(), tc.account, tc.item)
			if err != nil {
				t.Fatalf("Claim error: %v", err)
			}
			if got != tc.want {
				t.Errorf("Claim = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestClaimStatus covers the player claim-status read.
func TestClaimStatus(t *testing.T) {
	f := newFake()
	s := New(f)
	ctx := context.Background()

	claimed, _, _, err := s.ClaimStatus(ctx, 7)
	if err != nil {
		t.Fatalf("ClaimStatus: %v", err)
	}
	if claimed {
		t.Errorf("ClaimStatus(unclaimed) claimed = true, want false")
	}

	if _, err := s.Claim(ctx, 7, 10); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	claimed, itemID, title, err := s.ClaimStatus(ctx, 7)
	if err != nil {
		t.Fatalf("ClaimStatus: %v", err)
	}
	if !claimed || itemID != 10 || title != "Potion" {
		t.Errorf("ClaimStatus(claimed) = (%v, %d, %q), want (true, 10, \"Potion\")", claimed, itemID, title)
	}
}

// TestVitrine covers the player read surface: only enabled offers are returned.
func TestVitrine(t *testing.T) {
	f := newFake()
	f.items[11] = domain.DailyRewardItem{ID: 11, ItemIndex: 200, Enabled: false}
	s := New(f)
	ctx := context.Background()

	vit, err := s.Vitrine(ctx)
	if err != nil {
		t.Fatalf("Vitrine: %v", err)
	}
	if len(vit) != 1 || vit[0].ID != 10 {
		t.Errorf("Vitrine returned %d items, want only the enabled offer 10", len(vit))
	}
}
