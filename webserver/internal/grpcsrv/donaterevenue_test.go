package grpcsrv

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/donaterevenue"
)

// fakeRevenue is a scripted DonateRevenue for testing the gRPC mapping alone.
type fakeRevenue struct {
	res     donaterevenue.Result
	err     error
	summary donaterevenue.Summary
	orders  []domain.TopupOrderRow
	buyers  []domain.TopBuyer
	entries []domain.DonateLedgerEntry
	accts   []domain.AccountSummary
	total   int
	window  donaterevenue.Window

	// recorded request arguments
	gotModerator int64
	gotFrom      int64
	gotTo        int64
	gotBucket    donaterevenue.Bucket
	gotAction    donaterevenue.LedgerAction
	gotStatus    int16
	gotMethod    int16
	gotAccount   int64
	gotLimit     int
	gotOffset    int
	gotPrefix    string
}

func (f *fakeRevenue) Summary(_ context.Context, mod, from, to int64, b donaterevenue.Bucket, acct int64) (donaterevenue.Result, donaterevenue.Summary, error) {
	f.gotModerator, f.gotFrom, f.gotTo, f.gotBucket, f.gotAccount = mod, from, to, b, acct
	return f.res, f.summary, f.err
}

func (f *fakeRevenue) Orders(_ context.Context, mod, from, to int64, st, method int16, acct int64, limit, offset int) (donaterevenue.Result, []domain.TopupOrderRow, int, donaterevenue.Window, error) {
	f.gotModerator, f.gotFrom, f.gotTo = mod, from, to
	f.gotStatus, f.gotMethod, f.gotAccount, f.gotLimit, f.gotOffset = st, method, acct, limit, offset
	return f.res, f.orders, f.total, f.window, f.err
}

func (f *fakeRevenue) TopBuyers(_ context.Context, mod, from, to int64, limit, offset int) (donaterevenue.Result, []domain.TopBuyer, int, donaterevenue.Window, error) {
	f.gotModerator, f.gotFrom, f.gotTo, f.gotLimit, f.gotOffset = mod, from, to, limit, offset
	return f.res, f.buyers, f.total, f.window, f.err
}

func (f *fakeRevenue) Spend(_ context.Context, mod, from, to int64, a donaterevenue.LedgerAction, acct int64, limit, offset int) (donaterevenue.Result, []domain.DonateLedgerEntry, int, donaterevenue.Window, error) {
	f.gotModerator, f.gotFrom, f.gotTo = mod, from, to
	f.gotAction, f.gotAccount, f.gotLimit, f.gotOffset = a, acct, limit, offset
	return f.res, f.entries, f.total, f.window, f.err
}

func (f *fakeRevenue) SearchAccounts(_ context.Context, mod int64, prefix string, limit int) (donaterevenue.Result, []domain.AccountSummary, error) {
	f.gotModerator, f.gotPrefix, f.gotLimit = mod, prefix, limit
	return f.res, f.accts, f.err
}

func TestListTopupOrdersMapping(t *testing.T) {
	created := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	confirmed := time.Date(2026, 2, 2, 11, 0, 0, 0, time.UTC)
	f := &fakeRevenue{
		res:    donaterevenue.OK,
		total:  7,
		window: donaterevenue.Window{FromUnix: 100, ToUnix: 200},
		orders: []domain.TopupOrderRow{
			{
				ID: 12, ExternalReference: "uuid-1", AccountID: 34,
				AccountName: "zarco", AccountEmail: "z@x.c",
				PayerName: "Fulano", PayerCPF: "***.456.789-**",
				Credits: 50, AmountCents: 2500, PaymentMethod: 1,
				Provider: "", Status: 2,
				CreatedAt: created, ConfirmedAt: &confirmed,
			},
			{ID: 13, ExternalReference: "uuid-2", PaymentMethod: 2, Status: 1, CreatedAt: created},
		},
	}
	srv := NewDonateRevenue(f)

	got, err := srv.ListTopupOrders(context.Background(), &webv1.ListTopupOrdersRequest{
		ModeratorId:   9,
		Window:        &webv1.RevenueWindow{FromUnix: 100, ToUnix: 200},
		Status:        webv1.TopupStatus_TOPUP_STATUS_PAID,
		PaymentMethod: webv1.PaymentMethod_PAYMENT_METHOD_PIX,
		Limit:         25,
		Offset:        5,
	})
	if err != nil {
		t.Fatalf("ListTopupOrders: %v", err)
	}
	if got.GetResult() != webv1.AdminResult_ADMIN_RESULT_OK {
		t.Errorf("result = %v, want OK", got.GetResult())
	}
	if got.GetTotalCount() != 7 || got.GetFromUnix() != 100 || got.GetToUnix() != 200 {
		t.Errorf("total/window = (%d,%d,%d), want (7,100,200)", got.GetTotalCount(), got.GetFromUnix(), got.GetToUnix())
	}
	if f.gotModerator != 9 || f.gotStatus != 2 || f.gotMethod != 1 || f.gotLimit != 25 || f.gotOffset != 5 {
		t.Errorf("service got mod=%d status=%d method=%d limit=%d offset=%d",
			f.gotModerator, f.gotStatus, f.gotMethod, f.gotLimit, f.gotOffset)
	}

	rows := got.GetOrders()
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	r := rows[0]
	if r.GetId() != 12 || r.GetExternalReference() != "uuid-1" || r.GetAccountId() != 34 {
		t.Errorf("identity = (%d,%q,%d)", r.GetId(), r.GetExternalReference(), r.GetAccountId())
	}
	if r.GetAccountName() != "zarco" || r.GetAccountEmail() != "z@x.c" || r.GetPayerName() != "Fulano" {
		t.Errorf("buyer fields wrong: %+v", r)
	}
	if r.GetPayerCpfMasked() != "***.456.789-**" {
		t.Errorf("cpf = %q, want the masked value passed through", r.GetPayerCpfMasked())
	}
	if r.GetCredits() != 50 || r.GetAmountCents() != 2500 {
		t.Errorf("amounts = (%d,%d), want (50,2500)", r.GetCredits(), r.GetAmountCents())
	}
	if r.GetPaymentMethod() != webv1.PaymentMethod_PAYMENT_METHOD_PIX {
		t.Errorf("method = %v, want PIX", r.GetPaymentMethod())
	}
	if r.GetStatus() != webv1.TopupStatus_TOPUP_STATUS_PAID {
		t.Errorf("status = %v, want PAID", r.GetStatus())
	}
	if r.GetCreatedAtUnix() != created.Unix() || r.GetConfirmedAtUnix() != confirmed.Unix() {
		t.Errorf("times = (%d,%d), want (%d,%d)", r.GetCreatedAtUnix(), r.GetConfirmedAtUnix(), created.Unix(), confirmed.Unix())
	}

	// A PENDING row has no confirmed_at at all.
	if rows[1].GetConfirmedAtUnix() != 0 {
		t.Errorf("pending confirmed_at_unix = %d, want 0", rows[1].GetConfirmedAtUnix())
	}
	if rows[1].GetPaymentMethod() != webv1.PaymentMethod_PAYMENT_METHOD_CREDIT_CARD {
		t.Errorf("method = %v, want CREDIT_CARD", rows[1].GetPaymentMethod())
	}
}

// TestZeroTimeMapsToZeroUnix guards against time.Time{}.Unix() leaking the
// negative epoch (-62135596800) onto the wire.
func TestZeroTimeMapsToZeroUnix(t *testing.T) {
	if got := unixOrZero(time.Time{}); got != 0 {
		t.Errorf("unixOrZero(zero) = %d, want 0", got)
	}
	if got := unixPtrOrZero(nil); got != 0 {
		t.Errorf("unixPtrOrZero(nil) = %d, want 0", got)
	}
	var zero time.Time
	if got := unixPtrOrZero(&zero); got != 0 {
		t.Errorf("unixPtrOrZero(&zero) = %d, want 0", got)
	}
	now := time.Unix(1_700_000_000, 0)
	if got := unixOrZero(now); got != 1_700_000_000 {
		t.Errorf("unixOrZero(now) = %d, want 1700000000", got)
	}
}

// TestTopBuyersZeroTimestamps: a buyer whose confirmed_at aggregates came back
// NULL must not emit a negative epoch.
func TestTopBuyersZeroTimestamps(t *testing.T) {
	f := &fakeRevenue{
		res:    donaterevenue.OK,
		buyers: []domain.TopBuyer{{AccountID: 1, AccountName: "a", LifetimeGrossCents: 10}},
	}
	got, err := NewDonateRevenue(f).ListTopBuyers(context.Background(), &webv1.ListTopBuyersRequest{ModeratorId: 2})
	if err != nil {
		t.Fatalf("ListTopBuyers: %v", err)
	}
	b := got.GetBuyers()[0]
	if b.GetFirstPaidAtUnix() != 0 || b.GetLastPaidAtUnix() != 0 {
		t.Errorf("zero timestamps = (%d,%d), want (0,0)", b.GetFirstPaidAtUnix(), b.GetLastPaidAtUnix())
	}
}

func TestForbiddenMapsToAdminResultForbidden(t *testing.T) {
	f := &fakeRevenue{res: donaterevenue.Forbidden}
	srv := NewDonateRevenue(f)
	ctx := context.Background()

	o, err := srv.ListTopupOrders(ctx, &webv1.ListTopupOrdersRequest{ModeratorId: 1})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	if o.GetResult() != webv1.AdminResult_ADMIN_RESULT_FORBIDDEN || len(o.GetOrders()) != 0 {
		t.Errorf("orders = (%v, %d rows), want (FORBIDDEN, 0)", o.GetResult(), len(o.GetOrders()))
	}

	s, err := srv.GetRevenueSummary(ctx, &webv1.GetRevenueSummaryRequest{ModeratorId: 1})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	if s.GetResult() != webv1.AdminResult_ADMIN_RESULT_FORBIDDEN {
		t.Errorf("summary result = %v, want FORBIDDEN", s.GetResult())
	}
	if s.GetTotals() != nil {
		t.Errorf("forbidden summary carried totals %+v, want none", s.GetTotals())
	}
}

func TestGetRevenueSummaryMapsTotalsAndSeries(t *testing.T) {
	b1 := time.Date(2026, 3, 1, 3, 0, 0, 0, time.UTC)
	b2 := time.Date(2026, 3, 2, 3, 0, 0, 0, time.UTC)
	f := &fakeRevenue{
		res:    donaterevenue.OK,
		window: donaterevenue.Window{FromUnix: 11, ToUnix: 22},
		summary: donaterevenue.Summary{
			Window: donaterevenue.Window{FromUnix: 11, ToUnix: 22},
			Totals: domain.RevenueTotals{
				PaidOrders: 4, GrossCents: 8000, CreditsSold: 200, DistinctBuyers: 3,
				CreatedOrders: 6, PendingOrders: 2, PendingCents: 1500,
			},
			Ledger: domain.DonateLedgerTotals{
				ShopPurchases: 5, CreditsSpent: 350, ManualCredits: 1, CreditsGranted: 90,
			},
			ByMethod: []domain.RevenueByMethod{
				{PaymentMethod: 1, PaidOrders: 3, GrossCents: 6000},
				{PaymentMethod: 2, PaidOrders: 1, GrossCents: 2000},
			},
			Series: []domain.RevenuePoint{
				{BucketStart: b1, PaidOrders: 1, GrossCents: 3000},
				{BucketStart: b2, PaidOrders: 3, GrossCents: 5000},
			},
		},
	}
	got, err := NewDonateRevenue(f).GetRevenueSummary(context.Background(), &webv1.GetRevenueSummaryRequest{
		ModeratorId: 2,
		Window:      &webv1.RevenueWindow{FromUnix: 11, ToUnix: 22},
		Bucket:      webv1.RevenueBucket_REVENUE_BUCKET_DAY,
	})
	if err != nil {
		t.Fatalf("GetRevenueSummary: %v", err)
	}
	if f.gotBucket != donaterevenue.BucketDay {
		t.Errorf("bucket reached the service as %v, want BucketDay", f.gotBucket)
	}
	if got.GetFromUnix() != 11 || got.GetToUnix() != 22 {
		t.Errorf("echoed window = (%d,%d), want (11,22)", got.GetFromUnix(), got.GetToUnix())
	}

	tot := got.GetTotals()
	if tot.GetPaidOrders() != 4 || tot.GetGrossCents() != 8000 || tot.GetDistinctBuyers() != 3 {
		t.Errorf("money totals wrong: %+v", tot)
	}
	if tot.GetCreatedOrders() != 6 || tot.GetPendingOrders() != 2 || tot.GetPendingCents() != 1500 {
		t.Errorf("funnel totals wrong: %+v", tot)
	}
	// The ledger counters are merged into the same flat message.
	if tot.GetShopPurchases() != 5 || tot.GetCreditsSpent() != 350 ||
		tot.GetManualCredits() != 1 || tot.GetCreditsGranted() != 90 {
		t.Errorf("ledger totals were not merged into RevenueTotals: %+v", tot)
	}

	bm := got.GetByMethod()
	if len(bm) != 2 || bm[0].GetPaymentMethod() != webv1.PaymentMethod_PAYMENT_METHOD_PIX ||
		bm[1].GetPaymentMethod() != webv1.PaymentMethod_PAYMENT_METHOD_CREDIT_CARD {
		t.Errorf("by_method mapping wrong: %+v", bm)
	}

	series := got.GetSeries()
	if len(series) != 2 {
		t.Fatalf("got %d series points, want 2", len(series))
	}
	if series[0].GetBucketStartUnix() != b1.Unix() || series[1].GetBucketStartUnix() != b2.Unix() {
		t.Errorf("bucket order/values wrong: %d then %d", series[0].GetBucketStartUnix(), series[1].GetBucketStartUnix())
	}
	if series[0].GetGrossCents() != 3000 || series[1].GetGrossCents() != 5000 {
		t.Errorf("series values wrong: %+v", series)
	}
}

func TestListDonateSpendMapping(t *testing.T) {
	at := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	f := &fakeRevenue{
		res:   donaterevenue.OK,
		total: 2,
		entries: []domain.DonateLedgerEntry{
			{
				ID: 1, Action: "credit_balance", CreatedAt: at,
				SubjectAccountID: 10, SubjectAccountName: "player",
				ActorAccountID: 20, ActorAccountName: "mod",
				CreditsDelta: 100, BalanceAfter: 300, Reason: "cortesia",
			},
			{
				ID: 2, Action: "purchase", CreatedAt: at,
				SubjectAccountID: 10, SubjectAccountName: "player",
				ActorAccountID: 10, ActorAccountName: "player",
				CreditsDelta: -70, BalanceAfter: 230, ShopItemID: 5, ShopItemTitle: "Asa",
			},
		},
	}
	got, err := NewDonateRevenue(f).ListDonateSpend(context.Background(), &webv1.ListDonateSpendRequest{
		ModeratorId: 2,
		Action:      webv1.DonateLedgerAction_DONATE_LEDGER_ACTION_CREDIT,
		AccountId:   10,
	})
	if err != nil {
		t.Fatalf("ListDonateSpend: %v", err)
	}
	if f.gotAction != donaterevenue.LedgerCredit || f.gotAccount != 10 {
		t.Errorf("service got action=%v account=%d, want (LedgerCredit, 10)", f.gotAction, f.gotAccount)
	}

	e := got.GetEntries()
	if len(e) != 2 {
		t.Fatalf("got %d entries, want 2", len(e))
	}
	if e[0].GetAction() != webv1.DonateLedgerAction_DONATE_LEDGER_ACTION_CREDIT {
		t.Errorf("action[0] = %v, want CREDIT", e[0].GetAction())
	}
	// The asymmetry the whole ledger design exists for.
	if e[0].GetSubjectAccountId() != 10 || e[0].GetActorAccountId() != 20 {
		t.Errorf("credit subject/actor = (%d,%d), want (10,20)", e[0].GetSubjectAccountId(), e[0].GetActorAccountId())
	}
	if e[0].GetCreditsDelta() != 100 || e[0].GetReason() != "cortesia" {
		t.Errorf("credit row wrong: %+v", e[0])
	}
	if e[1].GetAction() != webv1.DonateLedgerAction_DONATE_LEDGER_ACTION_PURCHASE {
		t.Errorf("action[1] = %v, want PURCHASE", e[1].GetAction())
	}
	if e[1].GetCreditsDelta() != -70 {
		t.Errorf("purchase delta = %d, want -70 (signed debit)", e[1].GetCreditsDelta())
	}
	if e[1].GetShopItemId() != 5 || e[1].GetShopItemTitle() != "Asa" {
		t.Errorf("purchase offer = (%d,%q)", e[1].GetShopItemId(), e[1].GetShopItemTitle())
	}
}

func TestSearchAccountsMapping(t *testing.T) {
	f := &fakeRevenue{
		res: donaterevenue.OK,
		accts: []domain.AccountSummary{
			{ID: 3, Name: "zarco", Email: "z@x.c", DonateBalance: 42, Role: "admin", IsBlocked: true},
		},
	}
	got, err := NewDonateRevenue(f).SearchAccounts(context.Background(), &webv1.SearchAccountsRequest{
		ModeratorId: 2, NamePrefix: "zar", Limit: 10,
	})
	if err != nil {
		t.Fatalf("SearchAccounts: %v", err)
	}
	if f.gotPrefix != "zar" || f.gotLimit != 10 {
		t.Errorf("service got prefix=%q limit=%d", f.gotPrefix, f.gotLimit)
	}
	a := got.GetAccounts()[0]
	if a.GetId() != 3 || a.GetName() != "zarco" || a.GetEmail() != "z@x.c" ||
		a.GetDonateBalance() != 42 || a.GetRole() != "admin" || !a.GetIsBlocked() {
		t.Errorf("account mapping wrong: %+v", a)
	}
}

// TestUnspecifiedEnumsForwardAsZero: UNSPECIFIED must reach the service as 0,
// which every filter reads as "any".
func TestUnspecifiedEnumsForwardAsZero(t *testing.T) {
	f := &fakeRevenue{res: donaterevenue.OK}
	_, err := NewDonateRevenue(f).ListTopupOrders(context.Background(), &webv1.ListTopupOrdersRequest{
		ModeratorId:   2,
		Status:        webv1.TopupStatus_TOPUP_STATUS_UNSPECIFIED,
		PaymentMethod: webv1.PaymentMethod_PAYMENT_METHOD_UNSPECIFIED,
	})
	if err != nil {
		t.Fatalf("ListTopupOrders: %v", err)
	}
	if f.gotStatus != 0 || f.gotMethod != 0 {
		t.Errorf("unspecified enums reached the service as (%d,%d), want (0,0)", f.gotStatus, f.gotMethod)
	}

	f2 := &fakeRevenue{res: donaterevenue.OK}
	if _, err := NewDonateRevenue(f2).ListDonateSpend(context.Background(), &webv1.ListDonateSpendRequest{
		ModeratorId: 2, Action: webv1.DonateLedgerAction_DONATE_LEDGER_ACTION_UNSPECIFIED,
	}); err != nil {
		t.Fatalf("ListDonateSpend: %v", err)
	}
	if f2.gotAction != donaterevenue.LedgerAny {
		t.Errorf("unspecified action = %v, want LedgerAny", f2.gotAction)
	}
}

// TestNilWindowUsesZeroes: a request with no window block must forward (0,0),
// which the service turns into the last-30-days default.
func TestNilWindowUsesZeroes(t *testing.T) {
	f := &fakeRevenue{res: donaterevenue.OK}
	if _, err := NewDonateRevenue(f).GetRevenueSummary(context.Background(),
		&webv1.GetRevenueSummaryRequest{ModeratorId: 2}); err != nil {
		t.Fatalf("GetRevenueSummary: %v", err)
	}
	if f.gotFrom != 0 || f.gotTo != 0 {
		t.Errorf("nil window forwarded as (%d,%d), want (0,0)", f.gotFrom, f.gotTo)
	}
}

func TestInfraErrorBecomesInternalStatus(t *testing.T) {
	f := &fakeRevenue{err: errors.New("db down")}
	srv := NewDonateRevenue(f)
	ctx := context.Background()

	cases := map[string]func() error{
		"summary": func() error {
			_, err := srv.GetRevenueSummary(ctx, &webv1.GetRevenueSummaryRequest{ModeratorId: 2})
			return err
		},
		"orders": func() error {
			_, err := srv.ListTopupOrders(ctx, &webv1.ListTopupOrdersRequest{ModeratorId: 2})
			return err
		},
		"buyers": func() error {
			_, err := srv.ListTopBuyers(ctx, &webv1.ListTopBuyersRequest{ModeratorId: 2})
			return err
		},
		"spend": func() error {
			_, err := srv.ListDonateSpend(ctx, &webv1.ListDonateSpendRequest{ModeratorId: 2})
			return err
		},
		"search": func() error {
			_, err := srv.SearchAccounts(ctx, &webv1.SearchAccountsRequest{ModeratorId: 2, NamePrefix: "ab"})
			return err
		},
	}
	for name, call := range cases {
		err := call()
		if err == nil {
			t.Errorf("%s: got nil error, want codes.Internal", name)
			continue
		}
		if status.Code(err) != codes.Internal {
			t.Errorf("%s: code = %v, want Internal", name, status.Code(err))
		}
	}
}
