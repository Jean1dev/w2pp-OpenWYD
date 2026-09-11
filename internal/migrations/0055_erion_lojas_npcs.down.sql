UPDATE npc_definition SET enabled = TRUE
 WHERE origin = 'content' AND generator_index IN (6084, 6085, 6086, 6087, 6088);

UPDATE npc_config_meta SET version = version + 1 WHERE id = TRUE;
