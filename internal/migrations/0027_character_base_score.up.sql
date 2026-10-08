-- Persist the equipment-free BaseScore attributes and HP/MP maxima. The columns
-- str/int/dex/con/max_hp/max_mp hold the CurrentScore (base + equipment), and the
-- tmServer used to rebuild the base at login by subtracting the equipment bonus
-- computed with the rules of that moment. Any change to those rules between a
-- save and the next login (a refine formula, an item catalog row, a timed item)
-- shifted the base for good: a +9 Pedra Amunra saved at +100 and loaded at +200
-- (refine scaling, 2026-08-17) left a character 100 short on every attribute.
-- NULL means "not saved yet": the login derives the base once, as before, and
-- the next save fills these columns.
ALTER TABLE character
    ADD COLUMN base_str    SMALLINT,
    ADD COLUMN base_int    SMALLINT,
    ADD COLUMN base_dex    SMALLINT,
    ADD COLUMN base_con    SMALLINT,
    ADD COLUMN base_max_hp INTEGER,
    ADD COLUMN base_max_mp INTEGER;
