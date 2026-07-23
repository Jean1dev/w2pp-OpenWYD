-- Normalize legacy NPC template names to the concrete Linux file names.
-- NPCGener.txt keeps the old Windows-friendly spelling; the database stores the
-- actual file name so the moderator UI and tmServer overlay agree.

WITH changed AS (
    UPDATE npc_definition
    SET template_name = CASE template_name
        WHEN 'Cap.Explor.' THEN 'Cap.Explor'
        WHEN 'Diretor_Treina.' THEN 'Diretor_Treina'
        WHEN 'Chefe_Treina.' THEN 'Chefe_Treina'
        WHEN 'Reiners' THEN 'reiners'
        ELSE template_name
    END
    WHERE template_name IN ('Cap.Explor.', 'Diretor_Treina.', 'Chefe_Treina.', 'Reiners')
    RETURNING 1
)
UPDATE npc_config_meta
SET version = version + 1
WHERE id = TRUE AND EXISTS (SELECT 1 FROM changed);
