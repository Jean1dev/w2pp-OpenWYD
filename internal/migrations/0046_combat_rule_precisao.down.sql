ALTER TABLE combat_rule
    DROP COLUMN IF EXISTS max_miss_streak,
    DROP COLUMN IF EXISTS spell_int_accuracy_pct;
