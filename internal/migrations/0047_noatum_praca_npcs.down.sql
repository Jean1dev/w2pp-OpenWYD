UPDATE npc_definition SET enabled = TRUE
 WHERE (origin = 'content' AND generator_index IN (3903, 4511, 4849, 4852, 5995, 5996, 5997, 5998, 5999, 6058, 6077))
    OR slug IN ('Cap_Rowena-6062', 'Merc_Fantasma-4511', 'Pescador-5999', 'Torcedor-5995');

UPDATE npc_config_meta SET version = version + 1 WHERE id = TRUE;
