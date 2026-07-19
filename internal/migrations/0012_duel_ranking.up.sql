-- 0012_duel_ranking — persisted win/loss counters for the 1v1 duel system
-- (issue #118, _MSG_ReqRanking).
--
-- A separate table, not new `character` columns: wins/losses are incremented
-- out-of-band by dbServer (RecordDuelResult, partial UPDATE) independent of
-- tmServer's periodic full-row SaveCharacter, which would otherwise clobber a
-- column it doesn't know about (same reasoning as CreditDonateBalance,
-- 0008_donate_shop / 0010_donate_topup). Keyed by character_id, resolved from
-- the (unique) character name at write time — no new id needs to be threaded
-- through the game loop's Session/Entity.

CREATE TABLE character_pvp_stats (
    character_id BIGINT PRIMARY KEY REFERENCES character(id) ON DELETE CASCADE,
    wins         INTEGER NOT NULL DEFAULT 0,
    losses       INTEGER NOT NULL DEFAULT 0,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
