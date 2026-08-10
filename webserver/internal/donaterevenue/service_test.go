package donaterevenue

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

// fakeStore records what the service asked for and returns scripted data, so the
// tests pin authorization, validation and masking without a database.
type fakeStore struct {
	roles map[int64]string
	err   error // when set, every read returns it

	orders []domain.TopupOrderRow
	buyers []domain.TopBuyer
	ledger []domain.DonateLedgerEntry
	accts  []domain.AccountSummary

	// recorded arguments from the last call
	gotWindow  store.RevenueWindow
	gotFilter  store.TopupOrderFilter
	gotBucket  string
	gotActions []string
	gotAccount int64
	gotLimit   int
	gotOffset  int
	gotPrefix  string

	seriesCalls int
	readCalls   int
}

func (f *fakeStore) AccountRole(_ context.Context, id int64) (string, error) {
	r, ok := f.roles[id]
	if !ok {
		return "", store.ErrNotFound
	}
	return r, nil
}

func (f *fakeStore) RevenueTotals(_ context.Context, w store.RevenueWindow, accountID int64) (domain.RevenueTotals, error) {
	f.readCalls++
	f.gotWindow, f.gotAccount = w, accountID
	if f.err != nil {
		return domain.RevenueTotals{}, f.err
	}
	return domain.RevenueTotals{PaidOrders: 3, GrossCents: 4500}, nil
}

func (f *fakeStore) RevenueByMethod(_ context.Context, _ store.RevenueWindow, _ int64) ([]domain.RevenueByMethod, error) {
	f.readCalls++
	if f.err != nil {
		return nil, f.err
	}
	return []domain.RevenueByMethod{{PaymentMethod: 1, PaidOrders: 3, GrossCents: 4500}}, nil
}

func (f *fakeStore) RevenueSeries(_ context.Context, _ store.RevenueWindow, bucket string, _ int64) ([]domain.RevenuePoint, error) {
	f.readCalls++
	f.seriesCalls++
	f.gotBucket = bucket
	if f.err != nil {
		return nil, f.err
	}
	return []domain.RevenuePoint{{GrossCents: 4500}}, nil
}

func (f *fakeStore) DonateLedgerTotals(_ context.Context, _ store.RevenueWindow, _ int64) (domain.DonateLedgerTotals, error) {
	f.readCalls++
	if f.err != nil {
		return domain.DonateLedgerTotals{}, f.err
	}
	return domain.DonateLedgerTotals{ShopPurchases: 2, CreditsSpent: 80}, nil
}

func (f *fakeStore) ListTopupOrders(_ context.Context, w store.RevenueWindow, fl store.TopupOrderFilter, limit, offset int) ([]domain.TopupOrderRow, int, error) {
	f.readCalls++
	f.gotWindow, f.gotFilter, f.gotLimit, f.gotOffset = w, fl, limit, offset
	if f.err != nil {
		return nil, 0, f.err
	}
	return f.orders, len(f.orders), nil
}

func (f *fakeStore) ListTopBuyers(_ context.Context, w store.RevenueWindow, limit, offset int) ([]domain.TopBuyer, int, error) {
	f.readCalls++
	f.gotWindow, f.gotLimit, f.gotOffset = w, limit, offset
	if f.err != nil {
		return nil, 0, f.err
	}
	return f.buyers, len(f.buyers), nil
}

func (f *fakeStore) ListDonateLedger(_ context.Context, w store.RevenueWindow, actions []string, accountID int64, limit, offset int) ([]domain.DonateLedgerEntry, int, error) {
	f.readCalls++
	f.gotWindow, f.gotActions, f.gotAccount, f.gotLimit, f.gotOffset = w, actions, accountID, limit, offset
	if f.err != nil {
		return nil, 0, f.err
	}
	return f.ledger, len(f.ledger), nil
}

func (f *fakeStore) SearchAccountsByNamePrefix(_ context.Context, prefix string, limit int) ([]domain.AccountSummary, error) {
	f.readCalls++
	f.gotPrefix, f.gotLimit = prefix, limit
	if f.err != nil {
		return nil, f.err
	}
	return f.accts, nil
}

func newFake() *fakeStore {
	return &fakeStore{roles: map[int64]string{
		1: "player", 2: "moderator", 3: "admin",
	}}
}

// --- authorization ---

func TestSummaryForbiddenForPlayer(t *testing.T) {
	f := newFake()
	got, _, err := New(f).Summary(context.Background(), 1, 0, 0, BucketUnspecified, 0)
	if err != nil || got != Forbidden {
		t.Fatalf("Summary(player) = (%v, %v), want (Forbidden, nil)", got, err)
	}
	if f.readCalls != 0 {
		t.Errorf("store was read %d times for a forbidden caller, want 0", f.readCalls)
	}
}

func TestForbiddenForMissingAccount(t *testing.T) {
	f := newFake()
	got, _, err := New(f).Summary(context.Background(), 999, 0, 0, BucketUnspecified, 0)
	if err != nil || got != Forbidden {
		t.Fatalf("Summary(unknown account) = (%v, %v), want (Forbidden, nil) — never NotFound, which would leak existence", got, err)
	}
}

func TestForbiddenForZeroModeratorID(t *testing.T) {
	f := newFake()
	for _, id := range []int64{0, -5} {
		got, _, err := New(f).Summary(context.Background(), id, 0, 0, BucketUnspecified, 0)
		if err != nil || got != Forbidden {
			t.Errorf("Summary(moderatorID=%d) = (%v, %v), want (Forbidden, nil)", id, got, err)
		}
	}
	if f.readCalls != 0 {
		t.Errorf("store was read %d times, want 0 (short-circuit before any query)", f.readCalls)
	}
}

func TestAdminRoleIsAllowed(t *testing.T) {
	f := newFake()
	got, _, err := New(f).Summary(context.Background(), 3, 0, 0, BucketUnspecified, 0)
	if err != nil || got != OK {
		t.Fatalf("Summary(admin) = (%v, %v), want (OK, nil)", got, err)
	}
}

func TestAllMethodsAreGated(t *testing.T) {
	svc := New(newFake())
	ctx := context.Background()

	if r, _, _, _, err := svc.Orders(ctx, 1, 0, 0, 0, 0, 0, 0, 0); r != Forbidden || err != nil {
		t.Errorf("Orders(player) = %v, want Forbidden", r)
	}
	if r, _, _, _, err := svc.TopBuyers(ctx, 1, 0, 0, 0, 0); r != Forbidden || err != nil {
		t.Errorf("TopBuyers(player) = %v, want Forbidden", r)
	}
	if r, _, _, _, err := svc.Spend(ctx, 1, 0, 0, LedgerAny, 0, 0, 0); r != Forbidden || err != nil {
		t.Errorf("Spend(player) = %v, want Forbidden", r)
	}
	if r, _, err := svc.SearchAccounts(ctx, 1, "abc", 0); r != Forbidden || err != nil {
		t.Errorf("SearchAccounts(player) = %v, want Forbidden", r)
	}
}

// --- window validation ---

func TestWindowDefaultsToLast30Days(t *testing.T) {
	f := newFake()
	before := time.Now().UTC()
	got, sum, err := New(f).Summary(context.Background(), 2, 0, 0, BucketUnspecified, 0)
	if err != nil || got != OK {
		t.Fatalf("Summary = (%v, %v)", got, err)
	}
	span := f.gotWindow.To.Sub(f.gotWindow.From)
	if span < 29*24*time.Hour || span > 31*24*time.Hour {
		t.Errorf("default window span = %v, want ~30 days", span)
	}
	if f.gotWindow.To.Before(before.Add(-time.Minute)) {
		t.Errorf("default window ends at %v, want ~now", f.gotWindow.To)
	}
	if sum.Window.FromUnix != f.gotWindow.From.Unix() || sum.Window.ToUnix != f.gotWindow.To.Unix() {
		t.Errorf("echoed window %+v does not match what the store received", sum.Window)
	}
}

func TestWindowInvalidWhenFromNotBeforeTo(t *testing.T) {
	f := newFake()
	svc := New(f)
	cases := []struct {
		name     string
		from, to int64
	}{
		{"equal", 1_700_000_000, 1_700_000_000},
		{"reversed", 1_700_000_100, 1_700_000_000},
		{"negative", -1, 1_700_000_000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := svc.Summary(context.Background(), 2, tc.from, tc.to, BucketUnspecified, 0)
			if err != nil || got != Invalid {
				t.Errorf("Summary(%d,%d) = (%v, %v), want (Invalid, nil)", tc.from, tc.to, got, err)
			}
		})
	}
	if f.readCalls != 0 {
		t.Errorf("store was read %d times for invalid windows, want 0", f.readCalls)
	}
}

func TestWindowInvalidWhenWiderThan366Days(t *testing.T) {
	to := time.Now().UTC()
	from := to.AddDate(0, 0, -400)
	got, _, err := New(newFake()).Summary(context.Background(), 2, from.Unix(), to.Unix(), BucketUnspecified, 0)
	if err != nil || got != Invalid {
		t.Fatalf("400-day window = (%v, %v), want (Invalid, nil)", got, err)
	}
}

func TestWindowAcceptsExactlyMaxSpan(t *testing.T) {
	to := time.Now().UTC()
	from := to.Add(-maxWindowDays * 24 * time.Hour)
	got, _, err := New(newFake()).Summary(context.Background(), 2, from.Unix(), to.Unix(), BucketUnspecified, 0)
	if err != nil || got != OK {
		t.Fatalf("366-day window = (%v, %v), want (OK, nil)", got, err)
	}
}

// --- pagination ---

func TestPageLimitClampedAndDefaulted(t *testing.T) {
	f := newFake()
	svc := New(f)
	ctx := context.Background()

	cases := []struct {
		inLimit, inOffset  int
		wantLimit, wantOff int
	}{
		{0, 0, defaultLimit, 0},
		{1000, 0, maxLimit, 0},
		{10, -5, 10, 0},
		{25, 50, 25, 50},
	}
	for _, tc := range cases {
		if _, _, _, _, err := svc.Orders(ctx, 2, 0, 0, 0, 0, 0, tc.inLimit, tc.inOffset); err != nil {
			t.Fatalf("Orders: %v", err)
		}
		if f.gotLimit != tc.wantLimit || f.gotOffset != tc.wantOff {
			t.Errorf("Orders(limit=%d,offset=%d) -> store got (%d,%d), want (%d,%d)",
				tc.inLimit, tc.inOffset, f.gotLimit, f.gotOffset, tc.wantLimit, tc.wantOff)
		}
	}
}

// --- CPF masking ---

func TestCPFIsMaskedInOrderRows(t *testing.T) {
	f := newFake()
	f.orders = []domain.TopupOrderRow{{PayerCPF: "12345678909", PayerName: "Fulano"}}

	_, rows, _, _, err := New(f).Orders(context.Background(), 2, 0, 0, 0, 0, 0, 0, 0)
	if err != nil {
		t.Fatalf("Orders: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].PayerCPF != "***.456.789-**" {
		t.Errorf("masked CPF = %q, want ***.456.789-**", rows[0].PayerCPF)
	}
	if strings.Contains(rows[0].PayerCPF, "123") || strings.Contains(rows[0].PayerCPF, "909") {
		t.Errorf("raw CPF digits leaked into %q", rows[0].PayerCPF)
	}
}

func TestCPFEmptyWhenProfileMissing(t *testing.T) {
	f := newFake()
	f.orders = []domain.TopupOrderRow{{PayerCPF: ""}}
	_, rows, _, _, err := New(f).Orders(context.Background(), 2, 0, 0, 0, 0, 0, 0, 0)
	if err != nil {
		t.Fatalf("Orders: %v", err)
	}
	if rows[0].PayerCPF != "" {
		t.Errorf("masked empty CPF = %q, want empty", rows[0].PayerCPF)
	}
}

// --- series bucketing ---

func TestBucketUnspecifiedSkipsSeries(t *testing.T) {
	f := newFake()
	_, sum, err := New(f).Summary(context.Background(), 2, 0, 0, BucketUnspecified, 0)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if f.seriesCalls != 0 {
		t.Errorf("RevenueSeries called %d times for UNSPECIFIED, want 0", f.seriesCalls)
	}
	if len(sum.Series) != 0 {
		t.Errorf("series = %v, want empty", sum.Series)
	}
}

func TestBucketMapsToStoreKeyword(t *testing.T) {
	cases := map[Bucket]string{
		BucketDay:   store.BucketDay,
		BucketWeek:  store.BucketWeek,
		BucketMonth: store.BucketMonth,
	}
	for b, want := range cases {
		f := newFake()
		if _, _, err := New(f).Summary(context.Background(), 2, 0, 0, b, 0); err != nil {
			t.Fatalf("Summary: %v", err)
		}
		if f.gotBucket != want {
			t.Errorf("bucket %v -> %q, want %q", b, f.gotBucket, want)
		}
	}
}

func TestUnknownBucketDegradesToNoSeries(t *testing.T) {
	f := newFake()
	got, sum, err := New(f).Summary(context.Background(), 2, 0, 0, Bucket(99), 0)
	if err != nil || got != OK {
		t.Fatalf("Summary(unknown bucket) = (%v, %v), want (OK, nil) — a newer portal must not 400 an older server", got, err)
	}
	if f.seriesCalls != 0 || len(sum.Series) != 0 {
		t.Errorf("unknown bucket produced a series, want none")
	}
}

// --- ledger filter ---

func TestLedgerActionMapsToStoreKeywords(t *testing.T) {
	cases := []struct {
		action LedgerAction
		want   []string
	}{
		{LedgerAny, nil},
		{LedgerPurchase, []string{store.LedgerActionPurchase}},
		{LedgerCredit, []string{store.LedgerActionCredit}},
	}
	for _, tc := range cases {
		f := newFake()
		if _, _, _, _, err := New(f).Spend(context.Background(), 2, 0, 0, tc.action, 0, 0, 0); err != nil {
			t.Fatalf("Spend: %v", err)
		}
		if len(f.gotActions) != len(tc.want) {
			t.Fatalf("action %v -> %v, want %v", tc.action, f.gotActions, tc.want)
		}
		for i := range tc.want {
			if f.gotActions[i] != tc.want[i] {
				t.Errorf("action %v -> %v, want %v", tc.action, f.gotActions, tc.want)
			}
		}
	}
}

// --- account search ---

func TestSearchAccountsRejectsShortPrefix(t *testing.T) {
	f := newFake()
	svc := New(f)
	got, _, err := svc.SearchAccounts(context.Background(), 2, "a", 0)
	if err != nil || got != Invalid {
		t.Fatalf("SearchAccounts(1 char) = (%v, %v), want (Invalid, nil)", got, err)
	}
	if f.readCalls != 0 {
		t.Errorf("store was read for a too-short prefix")
	}

	got, _, err = svc.SearchAccounts(context.Background(), 2, "  AB  ", 0)
	if err != nil || got != OK {
		t.Fatalf("SearchAccounts(2 chars) = (%v, %v), want (OK, nil)", got, err)
	}
	if f.gotPrefix != "ab" {
		t.Errorf("prefix reached the store as %q, want lowercased+trimmed %q", f.gotPrefix, "ab")
	}
	if f.gotLimit != defaultSearchLimit {
		t.Errorf("search limit = %d, want %d", f.gotLimit, defaultSearchLimit)
	}
}

func TestSearchLimitClamped(t *testing.T) {
	f := newFake()
	if _, _, err := New(f).SearchAccounts(context.Background(), 2, "abc", 500); err != nil {
		t.Fatalf("SearchAccounts: %v", err)
	}
	if f.gotLimit != maxSearchLimit {
		t.Errorf("search limit = %d, want clamped to %d", f.gotLimit, maxSearchLimit)
	}
}

// --- error propagation ---

func TestStoreErrorBecomesInvalidAndIsWrapped(t *testing.T) {
	boom := errors.New("connection reset")
	svc := New(&fakeStore{roles: map[int64]string{2: "moderator"}, err: boom})
	ctx := context.Background()

	got, _, err := svc.Summary(ctx, 2, 0, 0, BucketUnspecified, 0)
	if got != Invalid {
		t.Errorf("Summary result = %v, want Invalid", got)
	}
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("Summary error = %v, want it to wrap the store error", err)
	}
	if !strings.Contains(err.Error(), "donaterevenue:") {
		t.Errorf("error %q is missing the package prefix", err)
	}

	if _, _, _, _, err := svc.Orders(ctx, 2, 0, 0, 0, 0, 0, 0, 0); err == nil || !errors.Is(err, boom) {
		t.Errorf("Orders error = %v, want the store error wrapped", err)
	}
	if _, _, _, _, err := svc.TopBuyers(ctx, 2, 0, 0, 0, 0); err == nil || !errors.Is(err, boom) {
		t.Errorf("TopBuyers error = %v, want the store error wrapped", err)
	}
	if _, _, _, _, err := svc.Spend(ctx, 2, 0, 0, LedgerAny, 0, 0, 0); err == nil || !errors.Is(err, boom) {
		t.Errorf("Spend error = %v, want the store error wrapped", err)
	}
	if _, _, err := svc.SearchAccounts(ctx, 2, "abc", 0); err == nil || !errors.Is(err, boom) {
		t.Errorf("SearchAccounts error = %v, want the store error wrapped", err)
	}
}

// TestNegativeAccountIDRejected guards the drill-down filter.
func TestNegativeAccountIDRejected(t *testing.T) {
	svc := New(newFake())
	ctx := context.Background()
	if r, _, err := svc.Summary(ctx, 2, 0, 0, BucketUnspecified, -1); r != Invalid || err != nil {
		t.Errorf("Summary(accountID=-1) = %v, want Invalid", r)
	}
	if r, _, _, _, err := svc.Orders(ctx, 2, 0, 0, 0, 0, -1, 0, 0); r != Invalid || err != nil {
		t.Errorf("Orders(accountID=-1) = %v, want Invalid", r)
	}
	if r, _, _, _, err := svc.Spend(ctx, 2, 0, 0, LedgerAny, -1, 0, 0); r != Invalid || err != nil {
		t.Errorf("Spend(accountID=-1) = %v, want Invalid", r)
	}
}
