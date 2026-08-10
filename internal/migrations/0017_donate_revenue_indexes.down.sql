-- Reverses 0017_donate_revenue_indexes. Indexes only — no data is lost.

DROP INDEX IF EXISTS account_name_prefix_idx;
DROP INDEX IF EXISTS donate_shop_audit_subject_idx;
DROP INDEX IF EXISTS donate_shop_audit_action_created_idx;
DROP INDEX IF EXISTS donate_topup_order_created_idx;
DROP INDEX IF EXISTS donate_topup_order_confirmed_idx;
