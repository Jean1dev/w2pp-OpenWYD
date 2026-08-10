//go:build integration

// Integration tests for the donate revenue read models (painel de faturamento).
// They require a real database and are excluded from the default build. Run with:
//
//	W2PP_TEST_DSN=postgres://postgres:dev@localhost:5432/postgres go test -tags=integration ./internal/store/
//
// Orders are seeded with raw INSERTs rather than CreateTopupOrder because the
// write path can only produce now() for created_at and has no way to set
// confirmed_at — and the whole point of these tests is which date column each
// figure is scoped to.
package store

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// revenueFixture is a freshly-migrated store with the donate tables emptied.
func revenueFixture(t *testing.T) (context.Context, *pgxpool.Pool, *Store) {
	t.Helper()
	ctx := context.Background()
	pool := testPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM donate_topup_order`)
	_, _ = pool.Exec(ctx, `DELETE FROM donate_shop_audit`)
	_, _ = pool.Exec(ctx, `DELETE FROM donate_shop_item`)
	_, _ = pool.Exec(ctx, `DELETE FROM delivery_queue`)
	return ctx, pool, New(pool)
}

func seedAccount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name, email, role string, balance int32) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO account (name, pass_hash, email, role, donate_balance)
		 VALUES ($1,'x',$2,$3,$4) RETURNING id`, name, email, role, balance).Scan(&id); err != nil {
		t.Fatalf("seed account %q: %v", name, err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM account WHERE id = $1`, id) })
	return id
}

// seedOrder inserts one order with explicit timestamps. confirmedAt nil = PENDING.
func seedOrder(t *testing.T, ctx context.Context, pool *pgxpool.Pool, ref string, accID int64,
	credits int32, cents int64, method int16, createdAt time.Time, confirmedAt *time.Time,
) {
	t.Helper()
	status := int16(TopupStatusPending)
	if confirmedAt != nil {
		status = TopupStatusPaid
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO donate_topup_order
			(external_reference, account_id, credits, amount_cents, payment_method, status, created_at, confirmed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		ref, accID, credits, cents, method, status, createdAt, confirmedAt); err != nil {
		t.Fatalf("seed order %q: %v", ref, err)
	}
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return ts
}

func window(from, to time.Time) RevenueWindow { return RevenueWindow{From: from, To: to} }

// --- revenue recognition ---

// TestRevenueTotalsRecognizesOnConfirmedAt pins the core accounting rule: an
// order created in January but settled in February is FEBRUARY revenue. January
// still sees it as funnel volume (created_orders) but not as money.
func TestRevenueTotalsRecognizesOnConfirmedAt(t *testing.T) {
	ctx, pool, s := revenueFixture(t)
	acc := seedAccount(t, ctx, pool, "rev_recog", "a@b.c", "player", 0)

	created := mustTime(t, "2026-01-20T12:00:00Z")
	confirmed := mustTime(t, "2026-02-03T12:00:00Z")
	seedOrder(t, ctx, pool, "rev-recog-1", acc, 50, 2500, 1, created, &confirmed)

	jan := window(mustTime(t, "2026-01-01T00:00:00Z"), mustTime(t, "2026-02-01T00:00:00Z"))
	feb := window(mustTime(t, "2026-02-01T00:00:00Z"), mustTime(t, "2026-03-01T00:00:00Z"))

	janT, err := s.RevenueTotals(ctx, jan, 0)
	if err != nil {
		t.Fatalf("jan totals: %v", err)
	}
	if janT.GrossCents != 0 || janT.PaidOrders != 0 {
		t.Errorf("january recognized revenue = (%d cents, %d orders), want zero", janT.GrossCents, janT.PaidOrders)
	}
	if janT.CreatedOrders != 1 {
		t.Errorf("january created_orders = %d, want 1 (funnel volume still counts)", janT.CreatedOrders)
	}

	febT, err := s.RevenueTotals(ctx, feb, 0)
	if err != nil {
		t.Fatalf("feb totals: %v", err)
	}
	if febT.GrossCents != 2500 || febT.PaidOrders != 1 || febT.CreditsSold != 50 || febT.DistinctBuyers != 1 {
		t.Errorf("february totals = %+v, want gross=2500 paid=1 credits=50 buyers=1", febT)
	}
	if febT.CreatedOrders != 0 {
		t.Errorf("february created_orders = %d, want 0 (it was created in January)", febT.CreatedOrders)
	}
}

// TestRevenueTotalsExcludesPendingFromGross: a PENDING order is funnel volume and
// pending_cents, never revenue.
func TestRevenueTotalsExcludesPendingFromGross(t *testing.T) {
	ctx, pool, s := revenueFixture(t)
	acc := seedAccount(t, ctx, pool, "rev_pending", "a@b.c", "player", 0)

	created := mustTime(t, "2026-03-10T12:00:00Z")
	seedOrder(t, ctx, pool, "rev-pend-1", acc, 20, 1000, 1, created, nil)

	w := window(mustTime(t, "2026-03-01T00:00:00Z"), mustTime(t, "2026-04-01T00:00:00Z"))
	got, err := s.RevenueTotals(ctx, w, 0)
	if err != nil {
		t.Fatalf("totals: %v", err)
	}
	if got.GrossCents != 0 || got.PaidOrders != 0 {
		t.Errorf("gross=%d paid=%d, want zero for a PENDING order", got.GrossCents, got.PaidOrders)
	}
	if got.PendingOrders != 1 || got.PendingCents != 1000 || got.CreatedOrders != 1 {
		t.Errorf("pending=%d pendingCents=%d created=%d, want 1/1000/1", got.PendingOrders, got.PendingCents, got.CreatedOrders)
	}
}

// TestRevenueByMethodSplitsGateways checks the per-gateway breakdown.
func TestRevenueByMethodSplitsGateways(t *testing.T) {
	ctx, pool, s := revenueFixture(t)
	acc := seedAccount(t, ctx, pool, "rev_method", "a@b.c", "player", 0)

	at := mustTime(t, "2026-04-10T12:00:00Z")
	seedOrder(t, ctx, pool, "m-pix-1", acc, 10, 1000, 1, at, &at)
	seedOrder(t, ctx, pool, "m-pix-2", acc, 10, 1500, 1, at, &at)
	seedOrder(t, ctx, pool, "m-card-1", acc, 10, 700, 2, at, &at)

	w := window(mustTime(t, "2026-04-01T00:00:00Z"), mustTime(t, "2026-05-01T00:00:00Z"))
	got, err := s.RevenueByMethod(ctx, w, 0)
	if err != nil {
		t.Fatalf("by method: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d method rows, want 2: %+v", len(got), got)
	}
	if got[0].PaymentMethod != 1 || got[0].PaidOrders != 2 || got[0].GrossCents != 2500 {
		t.Errorf("pix row = %+v, want method=1 orders=2 cents=2500", got[0])
	}
	if got[1].PaymentMethod != 2 || got[1].PaidOrders != 1 || got[1].GrossCents != 700 {
		t.Errorf("card row = %+v, want method=2 orders=1 cents=700", got[1])
	}
}

// --- series bucketing ---

// TestRevenueSeriesMonthBoundaryBRT is the timezone test. Both orders settle on
// 2026-02-01 in UTC, but in America/Sao_Paulo (UTC-3) the first is still
// 2026-01-31T22:00 and belongs to JANUARY. It is run with the session forced to
// UTC to prove the three-argument date_trunc ignores the session TimeZone.
func TestRevenueSeriesMonthBoundaryBRT(t *testing.T) {
	ctx, pool, s := revenueFixture(t)
	if _, err := pool.Exec(ctx, `SET TIME ZONE 'UTC'`); err != nil {
		t.Fatalf("set session tz: %v", err)
	}
	acc := seedAccount(t, ctx, pool, "rev_tz", "a@b.c", "player", 0)

	janBRT := mustTime(t, "2026-02-01T01:00:00Z") // = 2026-01-31 22:00 BRT
	febBRT := mustTime(t, "2026-02-01T05:00:00Z") // = 2026-02-01 02:00 BRT
	seedOrder(t, ctx, pool, "tz-jan", acc, 10, 1000, 1, janBRT, &janBRT)
	seedOrder(t, ctx, pool, "tz-feb", acc, 10, 2000, 1, febBRT, &febBRT)

	w := window(mustTime(t, "2026-01-01T03:00:00Z"), mustTime(t, "2026-03-01T03:00:00Z"))
	pts, err := s.RevenueSeries(ctx, w, BucketMonth, 0)
	if err != nil {
		t.Fatalf("series: %v", err)
	}
	if len(pts) != 2 {
		t.Fatalf("got %d month buckets, want 2: %+v", len(pts), pts)
	}
	if pts[0].GrossCents != 1000 {
		t.Errorf("january bucket gross = %d, want 1000 (22:00 BRT on Jan 31)", pts[0].GrossCents)
	}
	if pts[1].GrossCents != 2000 {
		t.Errorf("february bucket gross = %d, want 2000", pts[1].GrossCents)
	}
}

// TestRevenueSeriesWeekStartsMonday documents Postgres' ISO week boundary.
func TestRevenueSeriesWeekStartsMonday(t *testing.T) {
	ctx, pool, s := revenueFixture(t)
	acc := seedAccount(t, ctx, pool, "rev_week", "a@b.c", "player", 0)

	// 2026-03-08 is a Sunday, 2026-03-09 a Monday (both midday BRT).
	sun := mustTime(t, "2026-03-08T15:00:00Z")
	mon := mustTime(t, "2026-03-09T15:00:00Z")
	seedOrder(t, ctx, pool, "wk-sun", acc, 10, 100, 1, sun, &sun)
	seedOrder(t, ctx, pool, "wk-mon", acc, 10, 200, 1, mon, &mon)

	w := window(mustTime(t, "2026-03-02T03:00:00Z"), mustTime(t, "2026-03-16T03:00:00Z"))
	pts, err := s.RevenueSeries(ctx, w, BucketWeek, 0)
	if err != nil {
		t.Fatalf("series: %v", err)
	}
	if len(pts) != 2 {
		t.Fatalf("got %d week buckets, want 2: %+v", len(pts), pts)
	}
	if pts[0].GrossCents != 100 || pts[1].GrossCents != 200 {
		t.Errorf("week buckets = (%d, %d), want (100, 200) — Sunday closes the previous week",
			pts[0].GrossCents, pts[1].GrossCents)
	}
}

// TestRevenueSeriesFillsEmptyBuckets: a 5-day window with orders only on days 1
// and 5 still returns 5 rows, so the chart axis is continuous.
func TestRevenueSeriesFillsEmptyBuckets(t *testing.T) {
	ctx, pool, s := revenueFixture(t)
	acc := seedAccount(t, ctx, pool, "rev_gaps", "a@b.c", "player", 0)

	d1 := mustTime(t, "2026-05-01T15:00:00Z")
	d5 := mustTime(t, "2026-05-05T15:00:00Z")
	seedOrder(t, ctx, pool, "gap-1", acc, 10, 100, 1, d1, &d1)
	seedOrder(t, ctx, pool, "gap-5", acc, 10, 500, 1, d5, &d5)

	w := window(mustTime(t, "2026-05-01T03:00:00Z"), mustTime(t, "2026-05-06T03:00:00Z"))
	pts, err := s.RevenueSeries(ctx, w, BucketDay, 0)
	if err != nil {
		t.Fatalf("series: %v", err)
	}
	if len(pts) != 5 {
		t.Fatalf("got %d day buckets, want 5 (gap-filled): %+v", len(pts), pts)
	}
	want := []int64{100, 0, 0, 0, 500}
	for i, w := range want {
		if pts[i].GrossCents != w {
			t.Errorf("bucket %d gross = %d, want %d", i, pts[i].GrossCents, w)
		}
	}
}

// TestRevenueSeriesEmptyWindowReturnsZeroedBuckets: no orders at all still yields
// a continuous axis rather than an empty slice.
func TestRevenueSeriesEmptyWindowReturnsZeroedBuckets(t *testing.T) {
	ctx, _, s := revenueFixture(t)
	w := window(mustTime(t, "2026-06-01T03:00:00Z"), mustTime(t, "2026-06-04T03:00:00Z"))
	pts, err := s.RevenueSeries(ctx, w, BucketDay, 0)
	if err != nil {
		t.Fatalf("series: %v", err)
	}
	if len(pts) != 3 {
		t.Fatalf("got %d buckets, want 3", len(pts))
	}
	for i, p := range pts {
		if p.PaidOrders != 0 || p.GrossCents != 0 || p.DistinctBuyers != 0 {
			t.Errorf("bucket %d = %+v, want all zero", i, p)
		}
	}
}

// TestRevenueSeriesRejectsUnknownBucket guards the defense-in-depth check.
func TestRevenueSeriesRejectsUnknownBucket(t *testing.T) {
	ctx, _, s := revenueFixture(t)
	w := window(mustTime(t, "2026-06-01T00:00:00Z"), mustTime(t, "2026-06-02T00:00:00Z"))
	if _, err := s.RevenueSeries(ctx, w, "century", 0); err == nil {
		t.Fatal("RevenueSeries with an unknown bucket = nil error, want an error")
	}
}

// --- order table ---

// TestListTopupOrdersPaidUsesConfirmedAtBasis: the PAID filter selects the
// confirmed_at variant, so an order created before the window but settled inside
// it IS returned.
func TestListTopupOrdersPaidUsesConfirmedAtBasis(t *testing.T) {
	ctx, pool, s := revenueFixture(t)
	acc := seedAccount(t, ctx, pool, "ord_basis", "a@b.c", "player", 0)

	created := mustTime(t, "2026-01-20T12:00:00Z")
	confirmed := mustTime(t, "2026-02-03T12:00:00Z")
	seedOrder(t, ctx, pool, "basis-1", acc, 50, 2500, 1, created, &confirmed)

	feb := window(mustTime(t, "2026-02-01T00:00:00Z"), mustTime(t, "2026-03-01T00:00:00Z"))
	rows, total, err := s.ListTopupOrders(ctx, feb, TopupOrderFilter{Status: TopupStatusPaid}, 50, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 || total != 1 {
		t.Fatalf("got %d rows total=%d, want 1/1", len(rows), total)
	}
	if rows[0].ConfirmedAt == nil || !rows[0].ConfirmedAt.Equal(confirmed) {
		t.Errorf("confirmed_at = %v, want %v", rows[0].ConfirmedAt, confirmed)
	}

	// The created_at basis (no status filter) must NOT see it in February.
	rows, _, err = s.ListTopupOrders(ctx, feb, TopupOrderFilter{}, 50, 0)
	if err != nil {
		t.Fatalf("list any-status: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("created_at basis returned %d rows in February, want 0", len(rows))
	}
}

// TestListTopupOrdersJoinsBuyerAndPayerProfile: identity comes from the JOIN, and
// a buyer with no payer profile still appears (LEFT JOIN) with empty payer fields.
func TestListTopupOrdersJoinsBuyerAndPayerProfile(t *testing.T) {
	ctx, pool, s := revenueFixture(t)
	withProfile := seedAccount(t, ctx, pool, "ord_withprof", "wp@x.c", "player", 0)
	noProfile := seedAccount(t, ctx, pool, "ord_noprof", "np@x.c", "player", 0)

	if err := s.SavePayerProfile(ctx, withProfile, "Fulano de Tal", "12345678909"); err != nil {
		t.Fatalf("save payer profile: %v", err)
	}

	at := mustTime(t, "2026-07-10T12:00:00Z")
	seedOrder(t, ctx, pool, "join-wp", withProfile, 10, 1000, 1, at, &at)
	seedOrder(t, ctx, pool, "join-np", noProfile, 10, 2000, 1, at, &at)

	w := window(mustTime(t, "2026-07-01T00:00:00Z"), mustTime(t, "2026-08-01T00:00:00Z"))
	rows, _, err := s.ListTopupOrders(ctx, w, TopupOrderFilter{Status: TopupStatusPaid}, 50, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	byRef := map[string]domain.TopupOrderRow{}
	for _, r := range rows {
		byRef[r.ExternalReference] = r
	}
	wp := byRef["join-wp"]
	if wp.AccountName != "ord_withprof" || wp.AccountEmail != "wp@x.c" {
		t.Errorf("buyer identity = (%q,%q), want (ord_withprof, wp@x.c)", wp.AccountName, wp.AccountEmail)
	}
	if wp.PayerName != "Fulano de Tal" || wp.PayerCPF != "12345678909" {
		t.Errorf("payer = (%q,%q), want the profile values raw", wp.PayerName, wp.PayerCPF)
	}
	np := byRef["join-np"]
	if np.AccountName != "ord_noprof" {
		t.Errorf("no-profile buyer name = %q, want ord_noprof", np.AccountName)
	}
	if np.PayerName != "" || np.PayerCPF != "" {
		t.Errorf("no-profile payer = (%q,%q), want empty (LEFT JOIN)", np.PayerName, np.PayerCPF)
	}
	if np.Provider != "" {
		t.Errorf("provider = %q, want empty — nothing writes that column today", np.Provider)
	}
}

// TestListTopupOrdersPagination covers the pager, including the fallbackTotal
// path where a page past the end must still report the true total.
func TestListTopupOrdersPagination(t *testing.T) {
	ctx, pool, s := revenueFixture(t)
	acc := seedAccount(t, ctx, pool, "ord_page", "a@b.c", "player", 0)

	base := mustTime(t, "2026-08-01T12:00:00Z")
	for i := 0; i < 5; i++ {
		at := base.Add(time.Duration(i) * time.Hour)
		seedOrder(t, ctx, pool, "page-"+string(rune('a'+i)), acc, 10, int64(100*(i+1)), 1, at, &at)
	}

	w := window(mustTime(t, "2026-08-01T00:00:00Z"), mustTime(t, "2026-09-01T00:00:00Z"))
	f := TopupOrderFilter{Status: TopupStatusPaid}

	rows, total, err := s.ListTopupOrders(ctx, w, f, 2, 2)
	if err != nil {
		t.Fatalf("page: %v", err)
	}
	if len(rows) != 2 || total != 5 {
		t.Fatalf("page(limit=2,offset=2) = %d rows total=%d, want 2/5", len(rows), total)
	}
	// Ordered confirmed_at DESC: offset 2 is the 3rd newest = 300 cents.
	if rows[0].AmountCents != 300 || rows[1].AmountCents != 200 {
		t.Errorf("page contents = (%d,%d), want (300,200)", rows[0].AmountCents, rows[1].AmountCents)
	}

	rows, total, err = s.ListTopupOrders(ctx, w, f, 2, 10)
	if err != nil {
		t.Fatalf("past-end page: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("past-end page returned %d rows, want 0", len(rows))
	}
	if total != 5 {
		t.Errorf("past-end total = %d, want 5 (fallbackTotal path)", total)
	}
}

// TestListTopupOrdersFiltersStatusAndMethod covers the non-PAID variant's filters.
func TestListTopupOrdersFiltersStatusAndMethod(t *testing.T) {
	ctx, pool, s := revenueFixture(t)
	acc := seedAccount(t, ctx, pool, "ord_filter", "a@b.c", "player", 0)

	at := mustTime(t, "2026-09-10T12:00:00Z")
	seedOrder(t, ctx, pool, "f-pend-pix", acc, 10, 100, 1, at, nil)
	seedOrder(t, ctx, pool, "f-pend-card", acc, 10, 200, 2, at, nil)
	seedOrder(t, ctx, pool, "f-paid-pix", acc, 10, 300, 1, at, &at)

	w := window(mustTime(t, "2026-09-01T00:00:00Z"), mustTime(t, "2026-10-01T00:00:00Z"))

	rows, total, err := s.ListTopupOrders(ctx, w, TopupOrderFilter{Status: TopupStatusPending}, 50, 0)
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(rows) != 2 || total != 2 {
		t.Errorf("pending filter = %d rows total=%d, want 2/2", len(rows), total)
	}

	rows, _, err = s.ListTopupOrders(ctx, w, TopupOrderFilter{PaymentMethod: 2}, 50, 0)
	if err != nil {
		t.Fatalf("card: %v", err)
	}
	if len(rows) != 1 || rows[0].ExternalReference != "f-pend-card" {
		t.Errorf("card filter = %+v, want just f-pend-card", rows)
	}

	rows, _, err = s.ListTopupOrders(ctx, w, TopupOrderFilter{}, 50, 0)
	if err != nil {
		t.Fatalf("any: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("no filter = %d rows, want all 3", len(rows))
	}
}

// --- top buyers ---

// TestListTopBuyersLifetimeVsWindow: the ranking is window-scoped but the
// lifetime aggregate is not, and a buyer with no in-window order is absent.
func TestListTopBuyersLifetimeVsWindow(t *testing.T) {
	ctx, pool, s := revenueFixture(t)
	a := seedAccount(t, ctx, pool, "buyer_a", "a@x.c", "player", 7)
	b := seedAccount(t, ctx, pool, "buyer_b", "b@x.c", "player", 0)

	old := mustTime(t, "2025-01-15T12:00:00Z")
	inWin := mustTime(t, "2026-10-15T12:00:00Z")

	// A: one big old order + one small in-window order.
	seedOrder(t, ctx, pool, "tb-a-old", a, 100, 90000, 1, old, &old)
	seedOrder(t, ctx, pool, "tb-a-new", a, 10, 500, 1, inWin, &inWin)
	// B: only an old order -> must not appear.
	seedOrder(t, ctx, pool, "tb-b-old", b, 10, 50000, 1, old, &old)

	w := window(mustTime(t, "2026-10-01T00:00:00Z"), mustTime(t, "2026-11-01T00:00:00Z"))
	rows, total, err := s.ListTopBuyers(ctx, w, 50, 0)
	if err != nil {
		t.Fatalf("top buyers: %v", err)
	}
	if len(rows) != 1 || total != 1 {
		t.Fatalf("got %d rows total=%d, want 1/1 (B has no in-window order)", len(rows), total)
	}
	got := rows[0]
	if got.AccountID != a || got.AccountName != "buyer_a" {
		t.Errorf("ranked account = (%d,%q), want buyer_a", got.AccountID, got.AccountName)
	}
	if got.WindowGrossCents != 500 || got.WindowPaidOrders != 1 {
		t.Errorf("window = (%d cents, %d orders), want (500, 1)", got.WindowGrossCents, got.WindowPaidOrders)
	}
	if got.LifetimeGrossCents != 90500 || got.LifetimePaidOrders != 2 || got.LifetimeCredits != 110 {
		t.Errorf("lifetime = (%d cents, %d orders, %d credits), want (90500, 2, 110)",
			got.LifetimeGrossCents, got.LifetimePaidOrders, got.LifetimeCredits)
	}
	if !got.FirstPaidAt.Equal(old) || !got.LastPaidAt.Equal(inWin) {
		t.Errorf("first/last paid = (%v,%v), want (%v,%v)", got.FirstPaidAt, got.LastPaidAt, old, inWin)
	}
	if got.DonateBalance != 7 {
		t.Errorf("donate_balance = %d, want 7", got.DonateBalance)
	}

	// Past-the-end page must still report the group count, not the order count.
	rows, total, err = s.ListTopBuyers(ctx, w, 50, 10)
	if err != nil {
		t.Fatalf("past-end: %v", err)
	}
	if len(rows) != 0 || total != 1 {
		t.Errorf("past-end = %d rows total=%d, want 0/1 (groups, not orders)", len(rows), total)
	}
}

// --- donate ledger ---

// TestListDonateLedgerCreditBalanceSubjectIsCreditedAccount is THE critical test.
// donate_shop_audit.account_id holds the MODERATOR for a manual credit; the
// credited account lives only in the JSON. A naive report would bill the
// moderator for every courtesy credit.
func TestListDonateLedgerCreditBalanceSubjectIsCreditedAccount(t *testing.T) {
	ctx, pool, s := revenueFixture(t)
	player := seedAccount(t, ctx, pool, "led_player", "p@x.c", "player", 0)
	mod := seedAccount(t, ctx, pool, "led_mod", "m@x.c", "moderator", 0)

	if _, err := s.CreditDonateBalance(ctx, player, 100, mod, "compensacao evento"); err != nil {
		t.Fatalf("credit: %v", err)
	}

	w := window(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	rows, total, err := s.ListDonateLedger(ctx, w, nil, 0, 50, 0)
	if err != nil {
		t.Fatalf("ledger: %v", err)
	}
	if len(rows) != 1 || total != 1 {
		t.Fatalf("got %d rows total=%d, want 1/1", len(rows), total)
	}
	e := rows[0]
	if e.SubjectAccountID != player {
		t.Errorf("subject = %d, want the CREDITED player %d (not the moderator)", e.SubjectAccountID, player)
	}
	if e.SubjectAccountName != "led_player" {
		t.Errorf("subject name = %q, want led_player", e.SubjectAccountName)
	}
	if e.ActorAccountID != mod || e.ActorAccountName != "led_mod" {
		t.Errorf("actor = (%d,%q), want the moderator (%d, led_mod)", e.ActorAccountID, e.ActorAccountName, mod)
	}
	if e.CreditsDelta != 100 {
		t.Errorf("credits_delta = %d, want +100 (a credit is positive)", e.CreditsDelta)
	}
	if e.BalanceAfter != 100 || e.Reason != "compensacao evento" {
		t.Errorf("balance_after=%d reason=%q, want 100/compensacao evento", e.BalanceAfter, e.Reason)
	}
	if e.ShopItemID != 0 {
		t.Errorf("shop_item_id = %d, want 0 for a manual credit", e.ShopItemID)
	}

	// The subject filter must find it by the credited account, not the moderator.
	rows, _, err = s.ListDonateLedger(ctx, w, nil, player, 50, 0)
	if err != nil {
		t.Fatalf("subject filter: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("filtering by the credited account returned %d rows, want 1", len(rows))
	}
	rows, _, err = s.ListDonateLedger(ctx, w, nil, mod, 50, 0)
	if err != nil {
		t.Fatalf("moderator filter: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("filtering by the MODERATOR returned %d rows, want 0 — the moderator is the actor, not the subject", len(rows))
	}
}

// TestListDonateLedgerPurchaseIsNegativeAndSubjectIsBuyer: on a purchase the
// subject and actor are the same account and the delta is a debit.
func TestListDonateLedgerPurchaseIsNegativeAndSubjectIsBuyer(t *testing.T) {
	ctx, pool, s := revenueFixture(t)
	buyer := seedAccount(t, ctx, pool, "led_buyer", "b@x.c", "player", 500)
	mod := seedAccount(t, ctx, pool, "led_mod2", "m@x.c", "moderator", 0)

	offerID, err := s.UpsertDonateShopItem(ctx, domain.DonateShopItem{
		ItemIndex: 3540, Price: 120, Title: "Asa Celestial", Enabled: true,
	}, mod)
	if err != nil {
		t.Fatalf("upsert offer: %v", err)
	}
	if _, err := s.BuyDonateItem(ctx, buyer, offerID); err != nil {
		t.Fatalf("buy: %v", err)
	}

	w := window(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	rows, _, err := s.ListDonateLedger(ctx, w, []string{LedgerActionPurchase}, 0, 50, 0)
	if err != nil {
		t.Fatalf("ledger: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d purchase rows, want 1", len(rows))
	}
	e := rows[0]
	if e.SubjectAccountID != buyer || e.ActorAccountID != buyer {
		t.Errorf("subject=%d actor=%d, want both = buyer %d", e.SubjectAccountID, e.ActorAccountID, buyer)
	}
	if e.CreditsDelta != -120 {
		t.Errorf("credits_delta = %d, want -120 (a purchase is a debit)", e.CreditsDelta)
	}
	if e.BalanceAfter != 380 {
		t.Errorf("balance_after = %d, want 380", e.BalanceAfter)
	}
	if e.ShopItemID != offerID || e.ShopItemTitle != "Asa Celestial" {
		t.Errorf("offer = (%d,%q), want (%d, Asa Celestial)", e.ShopItemID, e.ShopItemTitle, offerID)
	}
}

// TestListDonateLedgerSurvivesDeletedOffer: shop_item_id is deliberately not an
// FK, so history outlives the offer — with an empty title, not a missing row.
func TestListDonateLedgerSurvivesDeletedOffer(t *testing.T) {
	ctx, pool, s := revenueFixture(t)
	buyer := seedAccount(t, ctx, pool, "led_del_buyer", "b@x.c", "player", 500)
	mod := seedAccount(t, ctx, pool, "led_del_mod", "m@x.c", "moderator", 0)

	offerID, err := s.UpsertDonateShopItem(ctx, domain.DonateShopItem{
		ItemIndex: 10, Price: 50, Title: "Efemero", Enabled: true,
	}, mod)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := s.BuyDonateItem(ctx, buyer, offerID); err != nil {
		t.Fatalf("buy: %v", err)
	}
	if err := s.DeleteDonateShopItem(ctx, offerID, mod); err != nil {
		t.Fatalf("delete offer: %v", err)
	}

	w := window(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	rows, _, err := s.ListDonateLedger(ctx, w, []string{LedgerActionPurchase}, 0, 50, 0)
	if err != nil {
		t.Fatalf("ledger: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows after deleting the offer, want the purchase to survive", len(rows))
	}
	if rows[0].ShopItemTitle != "" {
		t.Errorf("title = %q, want empty after the offer was deleted", rows[0].ShopItemTitle)
	}
	if rows[0].ShopItemID != offerID {
		t.Errorf("shop_item_id = %d, want %d preserved as the UI fallback", rows[0].ShopItemID, offerID)
	}
}

// TestListDonateLedgerExcludesCatalogActions: CRUD audit rows move no wallet and
// must never reach the ledger.
func TestListDonateLedgerExcludesCatalogActions(t *testing.T) {
	ctx, pool, s := revenueFixture(t)
	mod := seedAccount(t, ctx, pool, "led_cat_mod", "m@x.c", "moderator", 0)

	id, err := s.UpsertDonateShopItem(ctx, domain.DonateShopItem{
		ItemIndex: 11, Price: 10, Title: "Cfg", Enabled: true,
	}, mod)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.SetDonateShopItemEnabled(ctx, id, false, mod); err != nil {
		t.Fatalf("toggle: %v", err)
	}

	w := window(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	rows, total, err := s.ListDonateLedger(ctx, w, nil, 0, 50, 0)
	if err != nil {
		t.Fatalf("ledger: %v", err)
	}
	if len(rows) != 0 || total != 0 {
		t.Errorf("got %d rows total=%d, want 0 — 'create'/'set_enabled' are config, not wallet movements", len(rows), total)
	}
}

// TestDonateLedgerTotals aggregates both movement kinds.
func TestDonateLedgerTotals(t *testing.T) {
	ctx, pool, s := revenueFixture(t)
	buyer := seedAccount(t, ctx, pool, "tot_buyer", "b@x.c", "player", 1000)
	mod := seedAccount(t, ctx, pool, "tot_mod", "m@x.c", "moderator", 0)

	offerID, err := s.UpsertDonateShopItem(ctx, domain.DonateShopItem{
		ItemIndex: 12, Price: 70, Title: "X", Enabled: true,
	}, mod)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := s.BuyDonateItem(ctx, buyer, offerID); err != nil {
		t.Fatalf("buy 1: %v", err)
	}
	if _, err := s.BuyDonateItem(ctx, buyer, offerID); err != nil {
		t.Fatalf("buy 2: %v", err)
	}
	if _, err := s.CreditDonateBalance(ctx, buyer, 250, mod, "bonus"); err != nil {
		t.Fatalf("credit: %v", err)
	}

	w := window(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	got, err := s.DonateLedgerTotals(ctx, w, 0)
	if err != nil {
		t.Fatalf("totals: %v", err)
	}
	if got.ShopPurchases != 2 || got.CreditsSpent != 140 {
		t.Errorf("purchases = (%d, %d credits), want (2, 140)", got.ShopPurchases, got.CreditsSpent)
	}
	if got.ManualCredits != 1 || got.CreditsGranted != 250 {
		t.Errorf("manual credits = (%d, %d credits), want (1, 250)", got.ManualCredits, got.CreditsGranted)
	}
}

// --- account search ---

// TestSearchAccountsByNamePrefix covers the prefix match and the LIKE escaping.
func TestSearchAccountsByNamePrefix(t *testing.T) {
	ctx, pool, s := revenueFixture(t)
	seedAccount(t, ctx, pool, "zarco_one", "1@x.c", "player", 5)
	seedAccount(t, ctx, pool, "zarco_two", "2@x.c", "admin", 0)
	seedAccount(t, ctx, pool, "outro", "3@x.c", "player", 0)

	got, err := s.SearchAccountsByNamePrefix(ctx, "zarco", 20)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d accounts, want 2: %+v", len(got), got)
	}
	if got[0].Name != "zarco_one" || got[0].DonateBalance != 5 {
		t.Errorf("first = %+v, want zarco_one with balance 5", got[0])
	}
	if got[1].Role != "admin" {
		t.Errorf("second role = %q, want admin", got[1].Role)
	}

	// A literal '%' must be escaped, not treated as a wildcard.
	got, err = s.SearchAccountsByNamePrefix(ctx, "%", 20)
	if err != nil {
		t.Fatalf("wildcard search: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("prefix %%%% matched %d accounts, want 0 — the metacharacter must be escaped", len(got))
	}
}
