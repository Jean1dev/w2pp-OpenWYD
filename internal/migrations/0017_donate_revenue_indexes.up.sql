-- 0017_donate_revenue_indexes — read paths for the painel de faturamento
-- (web.v1.DonateRevenueAdminService).
--
-- No new tables and no column changes: every number the panel shows has been
-- persisted since 0008_donate_shop and 0010_donate_topup. What was missing is
-- indexes — donate_topup_order only had (account_id) and donate_shop_audit only
-- had (shop_item_id), so every date-windowed report would sequentially scan.

-- Recognized revenue is ALWAYS "status = PAID, filtered and ordered by
-- confirmed_at" (an order is only money once the gateway settled it). A partial
-- index keeps it to the settled rows — PENDING rows have confirmed_at NULL and
-- are excluded for free — and DESC matches the order table's ORDER BY so the
-- pager needs no sort step.
-- Serves: RevenueTotals (the paid FILTERs), RevenueByMethod, RevenueSeries,
--         ListTopupOrders variant A, ListTopBuyers.
CREATE INDEX IF NOT EXISTS donate_topup_order_confirmed_idx
    ON donate_topup_order (confirmed_at DESC)
    WHERE status = 2;

-- The funnel side is scoped by created_at regardless of status: a PENDING order
-- has no confirmed_at at all, so it is invisible to the index above.
-- Serves: RevenueTotals (the created/pending FILTERs and the second branch of
--         its outer WHERE), ListTopupOrders variant B.
CREATE INDEX IF NOT EXISTS donate_topup_order_created_idx
    ON donate_topup_order (created_at DESC);

-- Every donate_shop_audit read in the panel filters by action FIRST and then by
-- a created_at window, so the composite in that order serves both the ledger
-- list (action = ANY(...)) and the ledger totals. A bare (created_at) index
-- would be strictly weaker and is deliberately NOT created.
-- Serves: ListDonateLedger, DonateLedgerTotals.
CREATE INDEX IF NOT EXISTS donate_shop_audit_action_created_idx
    ON donate_shop_audit (action, created_at DESC);

-- Per-account drill-down into the ledger. This CANNOT be a plain (account_id)
-- index: donate_shop_audit is asymmetric. For action='purchase' the account_id
-- column IS the buyer, but for action='credit_balance' it is the MODERATOR who
-- granted the credit, and the credited account exists only inside
-- after->>'account_id' (internal/store/donate.go:207). Indexing the raw column
-- would silently attribute every courtesy credit to the moderator.
--
-- This expression is byte-identical to ledgerSubjectExpr in
-- internal/store/donate_revenue.go (modulo the `au.` alias, which is not part of
-- an index expression). If you edit one, edit the other, or the planner quietly
-- stops using this index.
CREATE INDEX IF NOT EXISTS donate_shop_audit_subject_idx
    ON donate_shop_audit ((COALESCE(NULLIF(after->>'account_id', '')::bigint, account_id)))
    WHERE action IN ('purchase', 'credit_balance');

-- SearchAccounts does `name LIKE 'prefix%'`. The UNIQUE constraint on
-- account.name only helps that under the C collation; text_pattern_ops makes the
-- prefix search index-backed under whatever collation the database was
-- initdb'd with.
CREATE INDEX IF NOT EXISTS account_name_prefix_idx
    ON account (name text_pattern_ops);
