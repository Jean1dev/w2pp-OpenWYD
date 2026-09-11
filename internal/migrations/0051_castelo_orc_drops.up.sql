-- 0051_castelo_orc_drops — o saque da quest do Castelo Orc e a chave que a abre.
--
-- Os oito templates COrc_* (Release/TMsrv/run/npc) nascem sem drop próprio: o
-- Carry deles só guarda uma chave de portão em cada guardião (slot 56, sempre).
-- Tudo o que cai vem daqui, na chance exata da Mesa de Drops, e continua
-- editável no painel (/drops) como qualquer outra linha. O acabamento que a
-- mesa não diz — o add sorteado do amuleto — é do tmServer
-- (handler/castelo_orc.go).
--
-- As chances saem da meta por entrada que o design fixou, contando 60 de tropa,
-- 3 guardiões, o boss e ~44 seguidores (4 no começo, mais 4 a cada 30 s em uns
-- 5 min na última sala):
--
--   Moeda de Prata (1Mi)  10       Âmago de Lobo           15
--   Repletion C + D       10       Âmago de Dragão Menor   12
--   Restos Ori + Lac      ~32      Âmago de Dente de Sabre 10
--   Poeiras Ori + Lac     7        Âmago de Cav. s/ Sela   7 (N 4, B 3)
--   Ovos de Cav. s/ Sela  ~1       Amuleto com add         0,6
--
-- Moedas, Repletion e Âmago de Lobo caem só da tropa e dos guardiões, que são
-- sempre 63: a meta não depende de quanto o grupo fica na última sala. Os
-- seguidores, que renascem, carregam os âmagos de montaria, os ovos e metade
-- dos restos.
--
-- ON CONFLICT DO NOTHING: uma linha que alguém já gravou pelo painel antes do
-- deploy vale mais que a proposta.

INSERT INTO drop_rule (mob, item, chance) VALUES
    -- Tropa e guardiões: o saque do castelo.
    ('COrc_Cavaleiro', 4026, 1600), ('COrc_Arqueiro', 4026, 1600), ('COrc_MeioOrc', 4026, 1600),
    ('COrc_Cavaleiro', 4018,  950), ('COrc_Arqueiro', 4018,  950), ('COrc_MeioOrc', 4018,  950),
    ('COrc_Cavaleiro', 4019,  650), ('COrc_Arqueiro', 4019,  650), ('COrc_MeioOrc', 4019,  650),
    ('COrc_Cavaleiro', 2392, 2400), ('COrc_Arqueiro', 2392, 2400), ('COrc_MeioOrc', 2392, 2400),
    ('COrc_Cavaleiro', 2394,  100), ('COrc_Arqueiro', 2394,  100), ('COrc_MeioOrc', 2394,  100),
    ('COrc_Cavaleiro', 2393, 1000), ('COrc_Arqueiro', 2393, 1000), ('COrc_MeioOrc', 2393, 1000),
    ('COrc_Cavaleiro', 2395,  800), ('COrc_Arqueiro', 2395,  800), ('COrc_MeioOrc', 2395,  800),
    ('COrc_Cavaleiro',  419, 2000), ('COrc_Arqueiro',  419, 2000), ('COrc_MeioOrc',  419, 2000),
    ('COrc_Cavaleiro',  420, 1000), ('COrc_Arqueiro',  420, 1000), ('COrc_MeioOrc',  420, 1000),
    ('COrc_Cavaleiro',  412,  750), ('COrc_Arqueiro',  412,  750), ('COrc_MeioOrc',  412,  750),
    ('COrc_Cavaleiro',  413,  280), ('COrc_Arqueiro',  413,  280), ('COrc_MeioOrc',  413,  280),
    ('COrc_Sentinela', 4026, 1600), ('COrc_Capitao', 4026, 1600), ('COrc_Chefe', 4026, 1600),
    ('COrc_Sentinela', 4018,  950), ('COrc_Capitao', 4018,  950), ('COrc_Chefe', 4018,  950),
    ('COrc_Sentinela', 4019,  650), ('COrc_Capitao', 4019,  650), ('COrc_Chefe', 4019,  650),
    ('COrc_Sentinela', 2392, 2400), ('COrc_Capitao', 2392, 2400), ('COrc_Chefe', 2392, 2400),
    ('COrc_Sentinela', 2394,  100), ('COrc_Capitao', 2394,  100), ('COrc_Chefe', 2394,  100),
    ('COrc_Sentinela', 2393, 1000), ('COrc_Capitao', 2393, 1000), ('COrc_Chefe', 2393, 1000),
    ('COrc_Sentinela', 2395,  800), ('COrc_Capitao', 2395,  800), ('COrc_Chefe', 2395,  800),
    ('COrc_Sentinela',  419, 2000), ('COrc_Capitao',  419, 2000), ('COrc_Chefe',  419, 2000),
    ('COrc_Sentinela',  420, 1000), ('COrc_Capitao',  420, 1000), ('COrc_Chefe',  420, 1000),
    ('COrc_Sentinela',  412,  750), ('COrc_Capitao',  412,  750), ('COrc_Chefe',  412,  750),
    ('COrc_Sentinela',  413,  280), ('COrc_Capitao',  413,  280), ('COrc_Chefe',  413,  280),
    -- Guarda do Lorde: os seguidores que renascem na última sala.
    ('COrc_Guarda',  419, 2000),
    ('COrc_Guarda',  420, 1000),
    ('COrc_Guarda', 2393, 1300),
    ('COrc_Guarda', 2395, 1100),
    ('COrc_Guarda', 2396,  900),
    ('COrc_Guarda', 2401,  700),
    ('COrc_Guarda', 2306,  100),
    ('COrc_Guarda', 2311,   50),
    -- Grão-Lorde: os amuletos só caem dele, com as Poeiras e os ovos em chance maior.
    ('COrc_GraoLorde',  551, 1500),
    ('COrc_GraoLorde',  552, 1500),
    ('COrc_GraoLorde',  553, 1500),
    ('COrc_GraoLorde',  554, 1500),
    ('COrc_GraoLorde',  412, 3000),
    ('COrc_GraoLorde',  413, 2500),
    ('COrc_GraoLorde', 2306, 2500),
    ('COrc_GraoLorde', 2311, 1250),
    -- A chave: a Chave Portão Orc Sul (465), a primeira das quatro do castelo no
    -- legado, é o que o Xamã Orc pede. Ela sai de todo monstro ('*' a 0% — hoje o
    -- Guarda_Orc_ do castelo aberto a dá sempre, a cada 6 min) e volta onde o
    -- design quis, na meta que ele fixou:
    --   Deserto                         1 chave a cada 1.000 abates
    -- As arenas da Quest 256 das Hidras e dos Elfos dão a chave na ENTRADA, e não
    -- no abate: 1 a cada 4 entradas pagas nas Hidras, 1 a cada 3 nos Elfos
    -- (handler/castelo_orc.go, casteloOrcKeyOnEntry). As arenas não têm relógio e
    -- renascem sozinhas, então a chave no abate premiaria quem acampa lá dentro.
    -- Os monstros delas ficam no '*' a 0%. No Deserto entram os
    -- templates que só nascem lá; o Tauron comum tem 1.648 dos seus 1.826 fora
    -- do deserto e fica de fora.
    ('*', 465, 0),
    ('Adamant_Tauron',  465,  10),
    ('Aeon_Tauron',     465,  10),
    ('Aranha_Inferno',  465,  10),
    ('Arqueiro_Tauron', 465,  10),
    ('Cav._Lugefer',    465,  10),
    ('Ladrao_Tauron',   465,  10),
    ('Lugefer',         465,  10),
    ('Manticora',       465,  10),
    ('Taron_Assassino', 465,  10),
    ('Treant',          465,  10),
    ('Verme_',          465,  10),
    ('Tauron_Agmo',     465,  10),
    ('Verme_Agmo',      465,  10)
ON CONFLICT (mob, item) DO NOTHING;

-- O tmServer relê a mesa quando a versão muda; sem isto, um servidor já de pé
-- só veria as linhas novas no próximo reinício.
UPDATE drop_rule_meta SET version = version + 1 WHERE id = TRUE;
