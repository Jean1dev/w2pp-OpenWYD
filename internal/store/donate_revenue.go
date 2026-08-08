package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// Donate revenue read models for the painel de faturamento
// (web.v1.DonateRevenueAdminService). Everything here is READ-ONLY and spans
// donate_topup_order (real money), donate_shop_audit (wallet movements),
// account (buyer identity) and donate_payer_profile. The write lifecycle lives
// in donate_topup.go and donate.go; this file deliberately adds nothing to it.
//
// Revenue recognition: money is recognized on confirmed_at, never created_at —
// an order is only revenue once the gateway settled it and ConfirmTopupOrder
// flipped it to PAID. created_at is used ONLY for the funnel counters
// (created/pending), which measure volume, not revenue.
//
// The indexes these queries depend on are in
// internal/migrations/0017_donate_revenue_indexes.up.sql.

// RevenueWindow is the half-open [From, To) instant range every revenue read is
// scoped to. The caller always resolves it to a concrete bounded range before
// calling in, so no query here needs a NULL-guarded date predicate and every
// range scan stays sargable on an indexed column.
type RevenueWindow struct {
	From time.Time
	To   time.Time
}

// TopupOrderFilter narrows ListTopupOrders. Zero means "any" on every dimension.
type TopupOrderFilter struct {
	Status        int16 // 0 = any; 1 = PENDING; 2 = PAID
	PaymentMethod int16 // 0 = any
	AccountID     int64 // 0 = any
}

// Series bucket keywords. Passed to date_trunc as a BIND PARAMETER (never string
// concatenation) and always one of these server-owned constants — a caller's
// enum is mapped to them, never forwarded as free text.
const (
	BucketDay   = "day"
	BucketWeek  = "week"
	BucketMonth = "month"
)

// Donate ledger actions that move a wallet. The catalog CRUD actions written by
// UpsertDonateShopItem/SetDonateShopItemEnabled/DeleteDonateShopItem are
// deliberately excluded — they change config, not balances.
const (
	LedgerActionPurchase = "purchase"
	LedgerActionCredit   = "credit_balance"
)

// The status discriminators below are written as SQL literals (1 = PENDING,
// 2 = PAID, matching TopupStatusPending/TopupStatusPaid) rather than bind
// parameters on purpose: donate_topup_order_confirmed_idx (migration 0017) is a
// PARTIAL index on `status = 2`, and Postgres can only prove a query implies a
// partial index predicate when the value is a literal. A `$n` would defeat the
// index under a generic plan.

// revenueZone is the timezone the day/week/month buckets close in. The panel is
// read against Brazilian bank statements, so "hoje" and "este mês" must end at
// midnight in Brasília — NOT at midnight UTC, which would shift every boundary
// three hours and make month-end totals disagree with the statement. This is a
// deliberate divergence from the rest of the server (the daily-reward day rule
// is UTC); do not "fix" it to UTC.
//
// It is always passed to the three-argument date_trunc, which pins the boundary
// regardless of the session TimeZone. The two-argument form would silently
// follow the session and make results environment-dependent.
//
// Brazil has had no DST since 2019, so the generate_series steps below do not
// currently cross a DST discontinuity.
const revenueZone = "America/Sao_Paulo"

// ledgerSubjectExpr is the canonical "whose wallet moved" expression over
// donate_shop_audit. It exists because that table is asymmetric: for
// action='purchase' the account_id column IS the buyer, but for
// action='credit_balance' the column is the MODERATOR who granted the credit and
// the credited account exists only in after->>'account_id' (donate.go:207). A
// naive "GROUP BY account_id" spend report would therefore attribute every
// courtesy credit to whichever moderator granted it.
//
// The expression index donate_shop_audit_subject_idx (migration 0017) matches
// this text. If you edit one, edit the other, or the planner quietly stops using
// the index.
const ledgerSubjectExpr = `COALESCE(NULLIF(au.after->>'account_id', '')::bigint, au.account_id)`

// bucketStep maps a validated bucket keyword to its generate_series step.
func bucketStep(bucket string) (string, error) {
	switch bucket {
	case BucketDay:
		return "1 day", nil
	case BucketWeek:
		return "1 week", nil
	case BucketMonth:
		return "1 month", nil
	default:
		return "", fmt.Errorf("store: revenue series: unknown bucket %q", bucket)
	}
}

// escapeLike neutralizes the LIKE metacharacters in a user-typed prefix so a
// literal '%' cannot turn a prefix search into a full-table wildcard scan. The
// default ESCAPE '\' applies.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// RevenueTotals is the KPI header over donate_topup_order. PAID figures are
// scoped by confirmed_at (revenue recognition); created/pending figures by
// created_at (funnel volume). accountID 0 means all accounts.
func (s *Store) RevenueTotals(ctx context.Context, w RevenueWindow, accountID int64) (domain.RevenueTotals, error) {
	var t domain.RevenueTotals
	// The outer WHERE ORs the two date ranges so the planner can BitmapOr the
	// confirmed_at and created_at indexes; the FILTERs then split the row set.
	//
	// `status = 2` has to appear in the OUTER WHERE, not only inside the FILTERs:
	// FILTER clauses are applied after the scan and contribute no index quals, so
	// without it the planner cannot prove the query implies
	// donate_topup_order_confirmed_idx's partial predicate and the whole query
	// degrades to a seq scan. It costs nothing semantically — confirmed_at is
	// only ever set when an order flips to PAID, so a non-PAID row can never
	// satisfy the first branch anyway.
	err := s.pool.QueryRow(ctx, `
		SELECT
			count(*)                   FILTER (WHERE status = 2 AND confirmed_at >= $1 AND confirmed_at < $2),
			COALESCE(sum(amount_cents) FILTER (WHERE status = 2 AND confirmed_at >= $1 AND confirmed_at < $2), 0),
			COALESCE(sum(credits)      FILTER (WHERE status = 2 AND confirmed_at >= $1 AND confirmed_at < $2), 0),
			count(DISTINCT account_id) FILTER (WHERE status = 2 AND confirmed_at >= $1 AND confirmed_at < $2),
			count(*)                   FILTER (WHERE created_at >= $1 AND created_at < $2),
			count(*)                   FILTER (WHERE status = 1 AND created_at >= $1 AND created_at < $2),
			COALESCE(sum(amount_cents) FILTER (WHERE status = 1 AND created_at >= $1 AND created_at < $2), 0)
		FROM donate_topup_order
		WHERE ((status = 2 AND confirmed_at >= $1 AND confirmed_at < $2)
		    OR (created_at >= $1 AND created_at < $2))
		  AND ($3 = 0 OR account_id = $3)`,
		w.From, w.To, accountID,
	).Scan(&t.PaidOrders, &t.GrossCents, &t.CreditsSold, &t.DistinctBuyers,
		&t.CreatedOrders, &t.PendingOrders, &t.PendingCents)
	if err != nil {
		return domain.RevenueTotals{}, fmt.Errorf("store: revenue totals: %w", err)
	}
	return t, nil
}

// RevenueByMethod splits recognized revenue by payment gateway.
func (s *Store) RevenueByMethod(ctx context.Context, w RevenueWindow, accountID int64) ([]domain.RevenueByMethod, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT payment_method, count(*), COALESCE(sum(amount_cents), 0)
		FROM donate_topup_order
		WHERE status = 2 AND confirmed_at >= $1 AND confirmed_at < $2
		  AND ($3 = 0 OR account_id = $3)
		GROUP BY payment_method
		ORDER BY payment_method`,
		w.From, w.To, accountID)
	if err != nil {
		return nil, fmt.Errorf("store: revenue by method: %w", err)
	}
	defer rows.Close()

	out := make([]domain.RevenueByMethod, 0, 4)
	for rows.Next() {
		var m domain.RevenueByMethod
		if err := rows.Scan(&m.PaymentMethod, &m.PaidOrders, &m.GrossCents); err != nil {
			return nil, fmt.Errorf("store: scan revenue by method: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: revenue by method: %w", err)
	}
	return out, nil
}

// RevenueSeries buckets recognized revenue by day/week/month in revenueZone.
// bucket must be one of BucketDay/BucketWeek/BucketMonth. Buckets with no orders
// are returned with zeroed counters (generate_series LEFT JOIN aggregate), so
// callers never gap-fill. Note date_trunc('week') starts on MONDAY.
func (s *Store) RevenueSeries(ctx context.Context, w RevenueWindow, bucket string, accountID int64) ([]domain.RevenuePoint, error) {
	step, err := bucketStep(bucket)
	if err != nil {
		return nil, err
	}
	// The first bucket is aligned to its own boundary, so it may open slightly
	// before w.From — that is a genuine partial bucket, not an off-by-one.
	rows, err := s.pool.Query(ctx, `
		WITH buckets AS (
			SELECT gs AS bucket_start
			FROM generate_series(
				date_trunc($3, $1::timestamptz, $6),
				$2::timestamptz - interval '1 microsecond',
				$4::interval) AS gs
		), agg AS (
			SELECT date_trunc($3, o.confirmed_at, $6) AS bucket_start,
			       count(*)                         AS paid_orders,
			       COALESCE(sum(o.amount_cents), 0) AS gross_cents,
			       COALESCE(sum(o.credits), 0)      AS credits_sold,
			       count(DISTINCT o.account_id)     AS distinct_buyers
			FROM donate_topup_order o
			WHERE o.status = 2 AND o.confirmed_at >= $1 AND o.confirmed_at < $2
			  AND ($5 = 0 OR o.account_id = $5)
			GROUP BY 1
		)
		SELECT b.bucket_start,
		       COALESCE(a.paid_orders, 0),
		       COALESCE(a.gross_cents, 0),
		       COALESCE(a.credits_sold, 0),
		       COALESCE(a.distinct_buyers, 0)
		FROM buckets b
		LEFT JOIN agg a ON a.bucket_start = b.bucket_start
		ORDER BY b.bucket_start`,
		w.From, w.To, bucket, step, accountID, revenueZone)
	if err != nil {
		return nil, fmt.Errorf("store: revenue series: %w", err)
	}
	defer rows.Close()

	out := make([]domain.RevenuePoint, 0, 32)
	for rows.Next() {
		var p domain.RevenuePoint
		if err := rows.Scan(&p.BucketStart, &p.PaidOrders, &p.GrossCents, &p.CreditsSold, &p.DistinctBuyers); err != nil {
			return nil, fmt.Errorf("store: scan revenue series: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: revenue series: %w", err)
	}
	return out, nil
}

// topupOrderRowCols is the column list shared by both ListTopupOrders variants,
// matched positionally by the scan below. count(*) OVER() rides along as the
// last column so the pager needs no second round trip.
const topupOrderRowCols = `
	o.id, o.external_reference, o.account_id, a.name, a.email,
	COALESCE(p.name, ''), COALESCE(p.cpf, ''),
	o.credits, o.amount_cents, o.payment_method, COALESCE(o.provider, ''),
	o.status, o.created_at, o.confirmed_at, count(*) OVER()`

// topupOrderFrom joins the buyer identity in one query. Both joins are on a
// primary key, which is why this is a JOIN and not up to `limit` extra lookups
// per rendered page. donate_payer_profile is LEFT — a buyer who never filled the
// PIX payer form still has orders.
const topupOrderFrom = `
	FROM donate_topup_order o
	JOIN account a ON a.id = o.account_id
	LEFT JOIN donate_payer_profile p ON p.account_id = o.account_id`

// topupOrderPaidWhere scopes the window to confirmed_at — the revenue basis. The
// literal `status = 2` is what lets the partial index apply; see the status note
// at the top of this file. Shared verbatim between the page and count queries so
// the two can never drift.
const topupOrderPaidWhere = `
	WHERE o.status = 2 AND o.confirmed_at >= $1 AND o.confirmed_at < $2
	  AND ($3 = 0 OR o.payment_method = $3)
	  AND ($4 = 0 OR o.account_id = $4)`

// topupOrderAnyWhere is the funnel basis: created_at, any status. A PENDING order
// has no confirmed_at at all, so it is invisible to the predicate above.
const topupOrderAnyWhere = `
	WHERE o.created_at >= $1 AND o.created_at < $2
	  AND ($3 = 0 OR o.status = $3)
	  AND ($4 = 0 OR o.payment_method = $4)
	  AND ($5 = 0 OR o.account_id = $5)`

// ListTopupOrders returns one page of the order table plus the total row count.
//
// The status filter also picks which date column the window applies to: PAID
// filters and orders by confirmed_at (the revenue basis, matching the partial
// index), anything else by created_at. The two variants are separate constant
// predicates rather than one query with a conditional date column so each gets a
// clean sargable predicate.
func (s *Store) ListTopupOrders(ctx context.Context, w RevenueWindow, f TopupOrderFilter, limit, offset int) ([]domain.TopupOrderRow, int, error) {
	var (
		where string
		order string
		args  []any
	)
	if f.Status == TopupStatusPaid {
		where = topupOrderPaidWhere
		order = `
			ORDER BY o.confirmed_at DESC, o.id DESC
			LIMIT $5 OFFSET $6`
		args = []any{w.From, w.To, f.PaymentMethod, f.AccountID, limit, offset}
	} else {
		where = topupOrderAnyWhere
		order = `
			ORDER BY o.created_at DESC, o.id DESC
			LIMIT $6 OFFSET $7`
		args = []any{w.From, w.To, f.Status, f.PaymentMethod, f.AccountID, limit, offset}
	}
	// The count reuses the same predicate minus the two pagination binds, and
	// drops the identity joins — it counts orders, which the joins cannot change.
	countQ := `SELECT count(*) FROM donate_topup_order o` + where
	cArgs := args[:len(args)-2]

	rows, err := s.pool.Query(ctx, `SELECT `+topupOrderRowCols+topupOrderFrom+where+order, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("store: list topup orders: %w", err)
	}
	defer rows.Close()

	out := make([]domain.TopupOrderRow, 0, limit)
	total := 0
	for rows.Next() {
		var r domain.TopupOrderRow
		var confirmed *time.Time
		if err := rows.Scan(&r.ID, &r.ExternalReference, &r.AccountID, &r.AccountName, &r.AccountEmail,
			&r.PayerName, &r.PayerCPF,
			&r.Credits, &r.AmountCents, &r.PaymentMethod, &r.Provider,
			&r.Status, &r.CreatedAt, &confirmed, &total); err != nil {
			return nil, 0, fmt.Errorf("store: scan topup order: %w", err)
		}
		r.ConfirmedAt = confirmed
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("store: list topup orders: %w", err)
	}

	total, err = fallbackTotal(ctx, len(out), offset, total, func(ctx context.Context) (int, error) {
		var n int
		if err := s.pool.QueryRow(ctx, countQ, cArgs...).Scan(&n); err != nil {
			return 0, fmt.Errorf("store: count topup orders: %w", err)
		}
		return n, nil
	})
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// listTopBuyersBody is shared between the page query and its count closure so
// the two can never drift. The outer WHERE is status-only with NO date bound —
// that is what makes the lifetime_* aggregates a true LTV; the window is applied
// inside the FILTERs and the HAVING.
const listTopBuyersBody = `
	FROM donate_topup_order o
	JOIN account a ON a.id = o.account_id
	WHERE o.status = 2
	GROUP BY o.account_id, a.name, a.email, a.donate_balance
	HAVING count(*) FILTER (WHERE o.confirmed_at >= $1 AND o.confirmed_at < $2) > 0`

// ListTopBuyers ranks accounts by revenue inside the window and carries the
// all-time aggregate alongside. Only accounts that paid inside the window are
// returned (the HAVING); total is the number of such accounts.
func (s *Store) ListTopBuyers(ctx context.Context, w RevenueWindow, limit, offset int) ([]domain.TopBuyer, int, error) {
	// count(*) OVER() runs AFTER grouping, so it counts GROUPS — exactly the
	// distinct-buyer total the pager needs, not the underlying order count.
	rows, err := s.pool.Query(ctx, `
		SELECT o.account_id, a.name, a.email,
		       count(*)                     FILTER (WHERE o.confirmed_at >= $1 AND o.confirmed_at < $2) AS window_orders,
		       COALESCE(sum(o.amount_cents) FILTER (WHERE o.confirmed_at >= $1 AND o.confirmed_at < $2), 0) AS window_cents,
		       count(*)                         AS lifetime_orders,
		       COALESCE(sum(o.amount_cents), 0) AS lifetime_cents,
		       COALESCE(sum(o.credits), 0)      AS lifetime_credits,
		       min(o.confirmed_at)              AS first_paid_at,
		       max(o.confirmed_at)              AS last_paid_at,
		       a.donate_balance,
		       count(*) OVER()                  AS total`+
		listTopBuyersBody+`
		ORDER BY window_cents DESC, o.account_id
		LIMIT $3 OFFSET $4`,
		w.From, w.To, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("store: list top buyers: %w", err)
	}
	defer rows.Close()

	out := make([]domain.TopBuyer, 0, limit)
	total := 0
	for rows.Next() {
		var b domain.TopBuyer
		var first, last *time.Time
		if err := rows.Scan(&b.AccountID, &b.AccountName, &b.AccountEmail,
			&b.WindowPaidOrders, &b.WindowGrossCents,
			&b.LifetimePaidOrders, &b.LifetimeGrossCents, &b.LifetimeCredits,
			&first, &last, &b.DonateBalance, &total); err != nil {
			return nil, 0, fmt.Errorf("store: scan top buyer: %w", err)
		}
		if first != nil {
			b.FirstPaidAt = *first
		}
		if last != nil {
			b.LastPaidAt = *last
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("store: list top buyers: %w", err)
	}

	total, err = fallbackTotal(ctx, len(out), offset, total, func(ctx context.Context) (int, error) {
		// Must count GROUPS, mirroring count(*) OVER() above — a plain count(*)
		// over the table would report orders instead of buyers.
		var n int
		if err := s.pool.QueryRow(ctx,
			`SELECT count(*) FROM (SELECT 1`+listTopBuyersBody+`) t`, w.From, w.To).Scan(&n); err != nil {
			return 0, fmt.Errorf("store: count top buyers: %w", err)
		}
		return n, nil
	})
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// ListDonateLedger reads donate_shop_audit as a signed wallet ledger. actions
// must be a subset of {LedgerActionPurchase, LedgerActionCredit}; empty means
// both. accountID 0 means all accounts and is matched against the SUBJECT (see
// ledgerSubjectExpr), never the raw audit column.
func (s *Store) ListDonateLedger(ctx context.Context, w RevenueWindow, actions []string, accountID int64, limit, offset int) ([]domain.DonateLedgerEntry, int, error) {
	if len(actions) == 0 {
		actions = []string{LedgerActionPurchase, LedgerActionCredit}
	}
	// The account and donate_shop_item joins are LEFT on purpose:
	// donate_shop_audit.shop_item_id is deliberately not an FK so history
	// survives offer deletion (0008_donate_shop), and an audit row can outlive
	// the account. Inner joins would silently erase that history.
	const body = `
		FROM donate_shop_audit au
		WHERE au.action = ANY($3::text[])
		  AND au.created_at >= $1 AND au.created_at < $2
		  AND ($4 = 0 OR ` + ledgerSubjectExpr + ` = $4)`

	rows, err := s.pool.Query(ctx, `
		WITH led AS (
			SELECT au.id, au.action, au.created_at, au.shop_item_id, au.after,
			       au.account_id AS actor_account_id,
			       `+ledgerSubjectExpr+` AS subject_account_id`+body+`
		)
		SELECT l.id, l.action, l.created_at,
		       l.subject_account_id, COALESCE(subj.name, ''),
		       l.actor_account_id,   COALESCE(actor.name, ''),
		       CASE l.action
		           WHEN '`+LedgerActionPurchase+`' THEN -COALESCE((l.after->>'price')::bigint, 0)
		           WHEN '`+LedgerActionCredit+`'   THEN  COALESCE((l.after->>'amount')::bigint, 0)
		           ELSE 0
		       END,
		       COALESCE((l.after->>'balance')::bigint, 0),
		       COALESCE(l.shop_item_id, 0),
		       COALESCE(si.title, ''),
		       COALESCE(l.after->>'reason', ''),
		       count(*) OVER()
		FROM led l
		LEFT JOIN account          subj  ON subj.id  = l.subject_account_id
		LEFT JOIN account          actor ON actor.id = l.actor_account_id
		LEFT JOIN donate_shop_item si    ON si.id    = l.shop_item_id
		ORDER BY l.created_at DESC, l.id DESC
		LIMIT $5 OFFSET $6`,
		w.From, w.To, actions, accountID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("store: list donate ledger: %w", err)
	}
	defer rows.Close()

	out := make([]domain.DonateLedgerEntry, 0, limit)
	total := 0
	for rows.Next() {
		var e domain.DonateLedgerEntry
		if err := rows.Scan(&e.ID, &e.Action, &e.CreatedAt,
			&e.SubjectAccountID, &e.SubjectAccountName,
			&e.ActorAccountID, &e.ActorAccountName,
			&e.CreditsDelta, &e.BalanceAfter,
			&e.ShopItemID, &e.ShopItemTitle, &e.Reason, &total); err != nil {
			return nil, 0, fmt.Errorf("store: scan donate ledger: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("store: list donate ledger: %w", err)
	}

	total, err = fallbackTotal(ctx, len(out), offset, total, func(ctx context.Context) (int, error) {
		var n int
		if err := s.pool.QueryRow(ctx, `SELECT count(*)`+body,
			w.From, w.To, actions, accountID).Scan(&n); err != nil {
			return 0, fmt.Errorf("store: count donate ledger: %w", err)
		}
		return n, nil
	})
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// DonateLedgerTotals aggregates the wallet movements in the window, in credits.
// accountID 0 means all accounts; otherwise it matches the SUBJECT.
func (s *Store) DonateLedgerTotals(ctx context.Context, w RevenueWindow, accountID int64) (domain.DonateLedgerTotals, error) {
	var t domain.DonateLedgerTotals
	err := s.pool.QueryRow(ctx, `
		SELECT
			count(*)                                    FILTER (WHERE au.action = '`+LedgerActionPurchase+`'),
			COALESCE(sum((au.after->>'price')::bigint)  FILTER (WHERE au.action = '`+LedgerActionPurchase+`'), 0),
			count(*)                                    FILTER (WHERE au.action = '`+LedgerActionCredit+`'),
			COALESCE(sum((au.after->>'amount')::bigint) FILTER (WHERE au.action = '`+LedgerActionCredit+`'), 0)
		FROM donate_shop_audit au
		WHERE au.action IN ('`+LedgerActionPurchase+`', '`+LedgerActionCredit+`')
		  AND au.created_at >= $1 AND au.created_at < $2
		  AND ($3 = 0 OR `+ledgerSubjectExpr+` = $3)`,
		w.From, w.To, accountID,
	).Scan(&t.ShopPurchases, &t.CreditsSpent, &t.ManualCredits, &t.CreditsGranted)
	if err != nil {
		return domain.DonateLedgerTotals{}, fmt.Errorf("store: donate ledger totals: %w", err)
	}
	return t, nil
}

// SearchAccountsByNamePrefix resolves a canonical (lowercase) login prefix so the
// panel can turn a typed name into an account_id filter. The caller lowercases
// and length-checks the prefix; this escapes the LIKE metacharacters.
func (s *Store) SearchAccountsByNamePrefix(ctx context.Context, prefix string, limit int) ([]domain.AccountSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, email, donate_balance, role, is_blocked
		FROM account
		WHERE name LIKE $1
		ORDER BY name
		LIMIT $2`, escapeLike(prefix)+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("store: search accounts: %w", err)
	}
	defer rows.Close()

	out := make([]domain.AccountSummary, 0, limit)
	for rows.Next() {
		var a domain.AccountSummary
		if err := rows.Scan(&a.ID, &a.Name, &a.Email, &a.DonateBalance, &a.Role, &a.IsBlocked); err != nil {
			return nil, fmt.Errorf("store: scan account summary: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: search accounts: %w", err)
	}
	return out, nil
}
