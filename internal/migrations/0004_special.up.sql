-- special: BaseScore.Special[4] — the ALLOCATED per-tree mastery points spent
-- via ApplyBonus BonusType=1. Distinct from the live CurrentScore.Special
-- (= allocated + equipment/affects), which is recomputed and never persisted.
ALTER TABLE character ADD COLUMN IF NOT EXISTS special SMALLINT[] NOT NULL DEFAULT '{}';
