-- New characters used to be created at level 1 while the legacy BaseMob
-- templates start at 0. Level and Exp follow the same g_pNextLevel curve
-- (level L needs Exp >= g_pNextLevel[L]), so every character that leveled up is
-- already consistent; only the ones still at the seed level with less than
-- g_pNextLevel[1] = 500 Exp are one level ahead. Arch twins were seeded the same
-- way and ride the same curve.
UPDATE character SET level = 0 WHERE level = 1 AND exp < 500;
