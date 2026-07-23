-- Restore the legacy spelling only for definitions whose slug still carries
-- the original NPCGener.txt name.

WITH changed AS (
    UPDATE npc_definition
    SET template_name = CASE
        WHEN template_name = 'Cap.Explor' AND left(slug, length('Cap.Explor.-')) = 'Cap.Explor.-' THEN 'Cap.Explor.'
        WHEN template_name = 'Diretor_Treina' AND left(slug, length('Diretor_Treina.-')) = 'Diretor_Treina.-' THEN 'Diretor_Treina.'
        WHEN template_name = 'Chefe_Treina' AND left(slug, length('Chefe_Treina.-')) = 'Chefe_Treina.-' THEN 'Chefe_Treina.'
        WHEN template_name = 'reiners' AND left(slug, length('Reiners-')) = 'Reiners-' THEN 'Reiners'
        ELSE template_name
    END
    WHERE (template_name = 'Cap.Explor' AND left(slug, length('Cap.Explor.-')) = 'Cap.Explor.-')
       OR (template_name = 'Diretor_Treina' AND left(slug, length('Diretor_Treina.-')) = 'Diretor_Treina.-')
       OR (template_name = 'Chefe_Treina' AND left(slug, length('Chefe_Treina.-')) = 'Chefe_Treina.-')
       OR (template_name = 'reiners' AND left(slug, length('Reiners-')) = 'Reiners-')
    RETURNING 1
)
UPDATE npc_config_meta
SET version = version + 1
WHERE id = TRUE AND EXISTS (SELECT 1 FROM changed);
