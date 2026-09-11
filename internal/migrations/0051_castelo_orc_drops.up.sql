-- 0051_castelo_orc_drops — o saque dos monstros da quest do Castelo Orc.
--
-- Os oito templates COrc_* (Release/TMsrv/run/npc) nascem sem drop próprio: o
-- Carry deles só guarda a chave de portão dos três guardiões (slot 56, sempre).
-- Tudo o que cai vem daqui, na chance exata da Mesa de Drops, e continua
-- editável no painel (/drops) como qualquer outra linha.
--
-- O acabamento que a mesa não diz — o add sorteado do amuleto e a vida da
-- montaria — é do tmServer (handler/castelo_orc.go).
--
-- As chances que o design da quest fixou são as da tropa: Moeda de Prata (1Mi)
-- 5%, Restos de Oriharucon 20% e de Lactolerium 10%, Poeiras 5% e 2%. O resto é
-- proposta para o primeiro teste, feita para ser mexida no painel.
--
-- ON CONFLICT DO NOTHING: uma linha que alguém já gravou pelo painel antes do
-- deploy vale mais que a proposta.

INSERT INTO drop_rule (mob, item, chance) VALUES
    -- Tropa e guardiões: o saque do castelo.
    ('COrc_Cavaleiro', 4026,  500), ('COrc_Arqueiro', 4026,  500), ('COrc_MeioOrc', 4026,  500),
    ('COrc_Cavaleiro', 4018,  100), ('COrc_Arqueiro', 4018,  100), ('COrc_MeioOrc', 4018,  100),
    ('COrc_Cavaleiro', 4019,   50), ('COrc_Arqueiro', 4019,   50), ('COrc_MeioOrc', 4019,   50),
    ('COrc_Cavaleiro', 2392,  100), ('COrc_Arqueiro', 2392,  100), ('COrc_MeioOrc', 2392,  100),
    ('COrc_Cavaleiro', 2394,  100), ('COrc_Arqueiro', 2394,  100), ('COrc_MeioOrc', 2394,  100),
    ('COrc_Cavaleiro', 2393,  100), ('COrc_Arqueiro', 2393,  100), ('COrc_MeioOrc', 2393,  100),
    ('COrc_Cavaleiro', 2395,   50), ('COrc_Arqueiro', 2395,   50), ('COrc_MeioOrc', 2395,   50),
    ('COrc_Cavaleiro',  419, 2000), ('COrc_Arqueiro',  419, 2000), ('COrc_MeioOrc',  419, 2000),
    ('COrc_Cavaleiro',  420, 1000), ('COrc_Arqueiro',  420, 1000), ('COrc_MeioOrc',  420, 1000),
    ('COrc_Cavaleiro',  412,  500), ('COrc_Arqueiro',  412,  500), ('COrc_MeioOrc',  412,  500),
    ('COrc_Cavaleiro',  413,  200), ('COrc_Arqueiro',  413,  200), ('COrc_MeioOrc',  413,  200),
    ('COrc_Sentinela', 4026,  500), ('COrc_Capitao', 4026,  500), ('COrc_Chefe', 4026,  500),
    ('COrc_Sentinela', 4018,  100), ('COrc_Capitao', 4018,  100), ('COrc_Chefe', 4018,  100),
    ('COrc_Sentinela', 4019,   50), ('COrc_Capitao', 4019,   50), ('COrc_Chefe', 4019,   50),
    ('COrc_Sentinela', 2392,  100), ('COrc_Capitao', 2392,  100), ('COrc_Chefe', 2392,  100),
    ('COrc_Sentinela', 2394,  100), ('COrc_Capitao', 2394,  100), ('COrc_Chefe', 2394,  100),
    ('COrc_Sentinela', 2393,  100), ('COrc_Capitao', 2393,  100), ('COrc_Chefe', 2393,  100),
    ('COrc_Sentinela', 2395,   50), ('COrc_Capitao', 2395,   50), ('COrc_Chefe', 2395,   50),
    ('COrc_Sentinela',  419, 2000), ('COrc_Capitao',  419, 2000), ('COrc_Chefe',  419, 2000),
    ('COrc_Sentinela',  420, 1000), ('COrc_Capitao',  420, 1000), ('COrc_Chefe',  420, 1000),
    ('COrc_Sentinela',  412,  500), ('COrc_Capitao',  412,  500), ('COrc_Chefe',  412,  500),
    ('COrc_Sentinela',  413,  200), ('COrc_Capitao',  413,  200), ('COrc_Chefe',  413,  200),
    -- Guarda do Lorde: os seguidores que renascem na última sala.
    ('COrc_Guarda',  419, 2000),
    ('COrc_Guarda',  420, 1000),
    ('COrc_Guarda', 2393,  300),
    ('COrc_Guarda', 2395,  200),
    ('COrc_Guarda', 2366,   50),
    ('COrc_Guarda', 2371,   25),
    ('COrc_Guarda', 2306,  100),
    ('COrc_Guarda', 2311,   50),
    -- Grão-Lorde: os amuletos só caem dele, com as Poeiras e os ovos em chance maior.
    ('COrc_GraoLorde',  551, 1500),
    ('COrc_GraoLorde',  552, 1500),
    ('COrc_GraoLorde',  553, 1500),
    ('COrc_GraoLorde',  554, 1500),
    ('COrc_GraoLorde',  412, 3000),
    ('COrc_GraoLorde',  413, 1500),
    ('COrc_GraoLorde', 2306, 2500),
    ('COrc_GraoLorde', 2311, 1250)
ON CONFLICT (mob, item) DO NOTHING;

-- O tmServer relê a mesa quando a versão muda; sem isto, um servidor já de pé
-- só veria as linhas novas no próximo reinício.
UPDATE drop_rule_meta SET version = version + 1 WHERE id = TRUE;
