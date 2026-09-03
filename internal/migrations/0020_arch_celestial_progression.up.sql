ALTER TABLE character
    ADD COLUMN mortal_level SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN celestial_arch_level SMALLINT NOT NULL DEFAULT 0;

-- Existing Arch characters predate this explicit QuestInfo field. Their current
-- level is the safest conservative backfill available: it never grants more
-- Mortal progression than the account has demonstrated in the rewrite.
UPDATE character
   SET mortal_level = LEAST(GREATEST(level, 0), 399)
 WHERE class_master = 1 AND mortal_level = 0;
