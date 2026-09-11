DELETE FROM drop_rule WHERE mob IN (
    'COrc_GraoLorde', 'COrc_Guarda', 'COrc_Sentinela', 'COrc_Capitao',
    'COrc_Chefe', 'COrc_Cavaleiro', 'COrc_Arqueiro', 'COrc_MeioOrc'
);
UPDATE drop_rule_meta SET version = version + 1 WHERE id = TRUE;
