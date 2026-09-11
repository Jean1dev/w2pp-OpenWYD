-- Devolve as cinco montarias B ao padrão compilado (sem linha = padrão). Não há
-- como saber o que havia antes da 0050, então é o padrão, não o estado anterior.
DELETE FROM mount_growth_rate WHERE mount_index BETWEEN 2371 AND 2375;
DELETE FROM mount_absorb      WHERE mount_index BETWEEN 2371 AND 2375;
DELETE FROM mount_bonus       WHERE mount_index BETWEEN 2371 AND 2375;
