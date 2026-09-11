DELETE FROM drop_rule WHERE mob IN (
    'COrc_GraoLorde', 'COrc_Guarda', 'COrc_Sentinela', 'COrc_Capitao',
    'COrc_Chefe', 'COrc_Cavaleiro', 'COrc_Arqueiro', 'COrc_MeioOrc'
);
DELETE FROM drop_rule WHERE item = 465 AND mob IN (
    '*', 'Adamant_Tauron', 'Aeon_Tauron', 'Aranha_Inferno', 'Arqueiro_Tauron',
    'Cav._Lugefer', 'Ladrao_Tauron', 'Lugefer', 'Manticora', 'Taron_Assassino',
    'Treant', 'Verme_', 'Tauron_Agmo', 'Verme_Agmo'
);
UPDATE drop_rule_meta SET version = version + 1 WHERE id = TRUE;
