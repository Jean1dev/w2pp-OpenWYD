-- 0010_donate_topup — payment-method-agnostic donate top-ups (web-api).
--
-- The Next.js portal buys donate credits through a payment gateway (PIX first,
-- credit card later). The portal owns no database: the durable order state and
-- the idempotency of the credit live HERE. Two tables:
--
--   donate_payer_profile  — the payer's name + CPF, reused across top-ups.
--   donate_topup_order    — one top-up order. The payment method is a COLUMN,
--                           never part of a table/RPC name, so credit card reuses
--                           the same table. UNIQUE(external_reference) is the sole
--                           idempotency anchor: ConfirmTopupOrder credits exactly
--                           once regardless of how many times the gateway replays
--                           its webhook. Crediting account.donate_balance is a
--                           partial UPDATE that no tmServer save clobbers (same
--                           technique as CreditDonateBalance, see 0008_donate_shop).

CREATE TABLE donate_payer_profile (
    account_id BIGINT PRIMARY KEY REFERENCES account(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    cpf        TEXT NOT NULL,                 -- 11 digits, numbers only
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE donate_topup_order (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    external_reference TEXT   NOT NULL UNIQUE,           -- portal UUID; idempotency/correlation key
    account_id         BIGINT NOT NULL REFERENCES account(id) ON DELETE CASCADE,
    credits            INTEGER NOT NULL CHECK (credits > 0),      -- donate credits added when paid
    amount_cents       BIGINT  NOT NULL CHECK (amount_cents > 0), -- amount charged, in cents
    payment_method     SMALLINT NOT NULL,                -- 1=PIX, 2=CREDIT_CARD, ...
    provider           TEXT,                             -- nullable; gateway name for future reconciliation
    status             SMALLINT NOT NULL,                -- 1=PENDING, 2=PAID
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    confirmed_at       TIMESTAMPTZ
);

CREATE INDEX donate_topup_order_account_idx ON donate_topup_order(account_id);
