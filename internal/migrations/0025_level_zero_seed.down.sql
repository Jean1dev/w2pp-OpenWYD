-- Restores the old level-1 seed for characters still below g_pNextLevel[1].
UPDATE character SET level = 1 WHERE level = 0 AND exp < 500;
