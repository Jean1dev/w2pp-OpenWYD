-- 0007_ranking_index — supports the public Top EXP ranking query.
-- The CASE mirrors legacy CRanking.cpp class-master normalization:
-- MORTAL(2)->1, ARCH(1)->2, other tiers keep their stored value.
CREATE INDEX IF NOT EXISTS character_exp_ranking_idx
    ON character (
        (CASE class_master WHEN 2 THEN 1 WHEN 1 THEN 2 ELSE class_master END) DESC,
        exp DESC,
        level DESC,
        name ASC
    )
    WHERE level < 1000;
