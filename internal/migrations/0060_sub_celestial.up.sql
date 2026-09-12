-- 0060_sub_celestial — onde mora a SEGUNDA VIDA do Celestial.
--
-- O Sub Celestial é uma segunda vida do mesmo personagem: o jogador cria o Sub
-- com a Pedra Ideal (já sendo Celestial 120+, com o Sephirot no espaço 11) e daí
-- em diante alterna entre as duas vidas com a Pedra Misteriosa. Cada vida tem o
-- seu nível, a sua XP, os seus pontos e as suas habilidades.
--
-- POR QUE UMA COLUNA JSONB E NÃO UM ESPELHO DE CADA CAMPO
--
-- Espelhar cada campo de progressão em dois (level/level_sub, exp/exp_sub, ...)
-- multiplicaria por dois um caminho que hoje já toca DEZESSEIS arquivos por
-- campo: migração, domain, store (2), db.proto, db.pb.go, mapping do dbServer,
-- dbclient do tmServer, persistence (duas structs), session, world, character e
-- score_derive. E todo código que hoje lê `level` teria de aprender qual das
-- duas colunas vale agora.
--
-- Em vez disso: as colunas que já existem continuam sendo A VIDA ATIVA, com o
-- mesmo significado de sempre, e a vida GUARDADA inteira mora num jsonb. Trocar
-- de vida é trocar o conteúdo dos dois lugares. O resto do servidor não precisa
-- saber que existe segunda vida — ele continua lendo o que sempre leu.
--
-- A EXCEÇÃO, E ELA É REAL: o bônus de pontos do Celestial CS lê o nível da vida
-- INATIVA em toda derivação de score (internal/level/scorebonus.go, ramo
-- classCelestialCS: sub*6/2 mais os degraus dele). Isso roda no login, na subida
-- de nível e na troca de equipamento — ler jsonb a cada vez seria caro sem
-- motivo. Por isso o NÍVEL da vida guardada sai do jsonb e ganha coluna própria,
-- que é a única coisa que o resto do código precisa enxergar.
--
-- sub_celestial_ativo diz QUAL vida está em uso: 0 = a principal, 1 = o Sub.
-- Ele não é redundante com o class_master: um personagem CS que está com a vida
-- principal ativa e um que está com o Sub ativo têm o mesmo class_master 4.
--
-- celestial_reset já era lido pela fórmula de pontos (200 por reset) e ficava
-- sempre em zero por não ter onde morar. Entra junto porque é do mesmo fluxo.

ALTER TABLE character
    ADD COLUMN IF NOT EXISTS sub_celestial_guardada  JSONB,
    ADD COLUMN IF NOT EXISTS sub_celestial_level     SMALLINT  NOT NULL DEFAULT 0 CHECK (sub_celestial_level BETWEEN 0 AND 199),
    ADD COLUMN IF NOT EXISTS sub_celestial_ativo     SMALLINT  NOT NULL DEFAULT 0 CHECK (sub_celestial_ativo IN (0, 1)),
    ADD COLUMN IF NOT EXISTS celestial_reset         SMALLINT  NOT NULL DEFAULT 0 CHECK (celestial_reset BETWEEN 0 AND 99);

-- Um personagem só pode ter vida guardada se tiver um Sub: sem o jsonb não há
-- para onde voltar, e um "ativo = 1" sem vida guardada deixaria o jogador preso
-- na segunda vida sem caminho de volta. A trava fica no banco porque é onde ela
-- não depende de ninguém lembrar.
-- Postgres não tem ADD CONSTRAINT IF NOT EXISTS, e sem o guarda uma segunda
-- passagem desta migração morre no meio. Isso não é hipótese: a 0060 foi
-- aplicada à mão na cópia antes de existir deploy, e o boot seguinte tentaria
-- aplicá-la de novo — as colunas passariam pelo IF NOT EXISTS e a trava
-- derrubaria o servidor na subida.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'character_sub_celestial_coerente'
    ) THEN
        ALTER TABLE character
            ADD CONSTRAINT character_sub_celestial_coerente
            CHECK (sub_celestial_ativo = 0 OR sub_celestial_guardada IS NOT NULL);
    END IF;
END $$;
