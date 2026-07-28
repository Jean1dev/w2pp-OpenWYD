-- Pontos Caos / PK karma (GetFunc.cpp KILL_MARK carry slot, issue #210).
-- pk_point: chaos counter, clamped [1,150] on write, 75 = neutral (display = pk_point-75).
-- guilty: PvP "red nick" decay counter, clamped [0,50] on write, forces chaos=0 while >0.
-- cur_kill/tot_kill: PvP kill streak / lifetime kill count (cosmetic, packed into MobName).
ALTER TABLE character ADD COLUMN IF NOT EXISTS pk_point SMALLINT NOT NULL DEFAULT 75;
ALTER TABLE character ADD COLUMN IF NOT EXISTS guilty SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE character ADD COLUMN IF NOT EXISTS cur_kill SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE character ADD COLUMN IF NOT EXISTS tot_kill INTEGER NOT NULL DEFAULT 0;
