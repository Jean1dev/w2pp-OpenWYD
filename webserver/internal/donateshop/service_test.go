package donateshop

import (
	"context"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

// fakeStore is an in-memory Store for exercising authorization, validation and
// the purchase/credit outcomes without a database.
type fakeStore struct {
	roles     map[int64]string
	items     map[int64]domain.DonateShopItem
	balances  map[int64]int32
	buyErr    error // forces BuyDonateItem to return this
	creditErr error // forces CreditDonateBalance to return this
	upsertErr error
	lastBuy   int64
}

func (f *fakeStore) AccountRole(_ context.Context, id int64) (string, error) {
	r, ok := f.roles[id]
	if !ok {
		return "", store.ErrNotFound
	}
	return r, nil
}
func (f *fakeStore) ListDonateShopItems(context.Context) ([]domain.DonateShopItem, error) {
	return f.all(false), nil
}
func (f *fakeStore) ListEnabledDonateShopItems(context.Context) ([]domain.DonateShopItem, error) {
	return f.all(true), nil
}
func (f *fakeStore) all(enabledOnly bool) []domain.DonateShopItem {
	out := make([]domain.DonateShopItem, 0, len(f.items))
	for _, it := range f.items {
		if enabledOnly && !it.Enabled {
			continue
		}
		out = append(out, it)
	}
	return out
}
func (f *fakeStore) UpsertDonateShopItem(_ context.Context, d domain.DonateShopItem, _ int64) (int64, error) {
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
func (f *fakeStore) SetDonateShopItemEnabled(_ context.Context, id int64, _ bool, _ int64) error {
	if _, ok := f.items[id]; !ok {
		return store.ErrNotFound
	}
	return nil
}
func (f *fakeStore) DeleteDonateShopItem(_ context.Context, id int64, _ int64) error {
	if _, ok := f.items[id]; !ok {
		return store.ErrNotFound
	}
	return nil
}
func (f *fakeStore) CreditDonateBalance(_ context.Context, accountID int64, amount int32, _ int64, _ string) (int32, error) {
	if f.creditErr != nil {
		return 0, f.creditErr
	}
	bal, ok := f.balances[accountID]
	if !ok {
		return 0, store.ErrNotFound
	}
	f.balances[accountID] = bal + amount
	return f.balances[accountID], nil
}
func (f *fakeStore) DonateBalance(_ context.Context, accountID int64) (int32, error) {
	bal, ok := f.balances[accountID]
	if !ok {
		return 0, store.ErrNotFound
	}
	return bal, nil
}
func (f *fakeStore) BuyDonateItem(_ context.Context, accountID, shopItemID int64) (int32, error) {
	if f.buyErr != nil {
		return 0, f.buyErr
	}
	f.lastBuy = shopItemID
	return f.balances[accountID], nil
}

func newFake() *fakeStore {
	return &fakeStore{
		roles:    map[int64]string{1: "moderator", 2: "admin", 3: "player"},
		items:    map[int64]domain.DonateShopItem{10: {ID: 10, ItemIndex: 100, Price: 50, Enabled: true}},
		balances: map[int64]int32{7: 200},
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
			valid := domain.DonateShopItem{ItemIndex: 100, Price: 50}
			if r, _, err := s.Upsert(ctx, tc.moderator, valid); err != nil || r != tc.wantResult {
				t.Errorf("Upsert = (%v, %v), want %v", r, err, tc.wantResult)
			}
			if r, err := s.SetEnabled(ctx, tc.moderator, 10, false); err != nil || r != tc.wantResult {
				t.Errorf("SetEnabled = (%v, %v), want %v", r, err, tc.wantResult)
			}
			if r, err := s.Delete(ctx, tc.moderator, 10); err != nil || r != tc.wantResult {
				t.Errorf("Delete = (%v, %v), want %v", r, err, tc.wantResult)
			}
			if r, _, err := s.CreditBalance(ctx, tc.moderator, 7, 100, "test"); err != nil || r != tc.wantResult {
				t.Errorf("CreditBalance = (%v, %v), want %v", r, err, tc.wantResult)
			}
		})
	}
}

// TestUpsertValidation rejects offers with a bad item index or non-positive price.
func TestUpsertValidation(t *testing.T) {
	s := New(newFake())
	ctx := context.Background()
	bad := []domain.DonateShopItem{
		{ItemIndex: 0, Price: 50},   // no item
		{ItemIndex: 100, Price: 0},  // free
		{ItemIndex: 100, Price: -5}, // negative
		{ItemIndex: 100, Price: 50, ExpiresDays: -1},
	}
	for _, d := range bad {
		if r, _, err := s.Upsert(ctx, 1, d); err != nil || r != Invalid {
			t.Errorf("Upsert(%+v) = (%v, %v), want Invalid", d, r, err)
		}
	}
}

// TestCreditValidation rejects a non-positive amount or bad account id.
func TestCreditValidation(t *testing.T) {
	s := New(newFake())
	ctx := context.Background()
	if r, _, _ := s.CreditBalance(ctx, 1, 7, 0, ""); r != Invalid {
		t.Errorf("credit amount 0 = %v, want Invalid", r)
	}
	if r, _, _ := s.CreditBalance(ctx, 1, 0, 100, ""); r != Invalid {
		t.Errorf("credit account 0 = %v, want Invalid", r)
	}
	// A missing account surfaces as NotFound.
	if r, _, _ := s.CreditBalance(ctx, 1, 555, 100, ""); r != NotFound {
		t.Errorf("credit missing account = %v, want NotFound", r)
	}
	// A valid credit succeeds and returns the new balance.
	if r, bal, err := s.CreditBalance(ctx, 1, 7, 100, "donation"); err != nil || r != OK || bal != 300 {
		t.Errorf("credit = (%v, %d, %v), want (OK, 300, nil)", r, bal, err)
	}
}

// TestBuyOutcomes maps every store buy result to the right BuyOutcome.
func TestBuyOutcomes(t *testing.T) {
	tests := []struct {
		name    string
		buyErr  error
		account int64
		item    int64
		want    BuyOutcome
	}{
		{"ok", nil, 7, 10, BuyOK},
		{"insufficient", store.ErrInsufficientDonate, 7, 10, BuyInsufficient},
		{"disabled", store.ErrShopItemDisabled, 7, 10, BuyDisabled},
		{"not found", store.ErrNotFound, 7, 10, BuyNotFound},
		{"bad account id", nil, 0, 10, BuyNotFound},
		{"bad item id", nil, 7, 0, BuyNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFake()
			f.buyErr = tc.buyErr
			s := New(f)
			got, _, err := s.Buy(context.Background(), tc.account, tc.item)
			if err != nil {
				t.Fatalf("Buy error: %v", err)
			}
			if got != tc.want {
				t.Errorf("Buy = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestVitrineAndBalance covers the player read surface.
func TestVitrineAndBalance(t *testing.T) {
	f := newFake()
	f.items[11] = domain.DonateShopItem{ID: 11, ItemIndex: 200, Price: 10, Enabled: false}
	s := New(f)
	ctx := context.Background()

	vit, err := s.Vitrine(ctx)
	if err != nil {
		t.Fatalf("Vitrine: %v", err)
	}
	if len(vit) != 1 || vit[0].ID != 10 {
		t.Errorf("Vitrine returned %d items, want only the enabled offer 10", len(vit))
	}

	if bal, _ := s.Balance(ctx, 7); bal != 200 {
		t.Errorf("Balance = %d, want 200", bal)
	}
	// A missing account reports 0, not an error.
	if bal, err := s.Balance(ctx, 555); err != nil || bal != 0 {
		t.Errorf("Balance(missing) = (%d, %v), want (0, nil)", bal, err)
	}
}
