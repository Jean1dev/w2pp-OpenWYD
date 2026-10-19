-- Sub-Celestial (issue #410). STRUCT_MOBEXTRA.SaveCelestial[2] (Basedef.h:682-707)
-- holds the parked sheet of the Celestial tier not being played: the Pedra Ideal
-- writes [1] when a Celestial becomes CELESTIALCS (_MSG_UseItem.cpp:3040-3073),
-- the Urnammu changes its class (_MSG_Quest.cpp:1248-1310) and the Pedra
-- Misteriosa swaps sheets (_MSG_UseItem.cpp:3188-3260). The blob layout is owned
-- by the tmServer (world.MarshalCelestialSheets); '' = no parked sheets.
-- QuestInfo.Celestial.CelestialLevel/SubCelestialLevel (Basedef.h:661-662) record
-- the parked tier's level, which BASE_GetBonusScorePoint adds at half weight.
ALTER TABLE character ADD COLUMN IF NOT EXISTS save_celestial BYTEA NOT NULL DEFAULT ''::bytea;
ALTER TABLE character ADD COLUMN IF NOT EXISTS celestial_level SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE character ADD COLUMN IF NOT EXISTS sub_celestial_level SMALLINT NOT NULL DEFAULT 0;
