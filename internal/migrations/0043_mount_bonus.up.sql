-- 0043_mount_bonus — o ataque, a magia, a evasão e a imunidade que cada montaria
-- adulta empresta ao dono.
--
-- Esses números não moram no ItemList: para 2330-2389 ele traz só mesh, nível e
-- preço. O legado os lê de uma tabela fixa (g_pMountBonus, Basedef.cpp:238), e a
-- tabela que o servidor legado distribui foi achatada inteira para os valores do
-- Dragão Vermelho — todas as trinta montarias batendo e resistindo igual. O
-- cliente guardou a sua própria tabela, por montaria, e desenha o tooltip com
-- ela. O padrão compilado deste servidor é a tabela do CLIENTE
-- (internal/mountbonus), justamente para o que o jogador lê ser o que vale.
--
-- Esta tabela é o que o painel muda por cima desse padrão. Mudar aqui muda o
-- servidor; o cliente só mostra o número novo depois que o gerador de arquivos
-- do cliente for rodado e o resultado publicado — sem isso o jogador passa a ler
-- um número e levar outro, que foi exatamente o defeito que esta tabela resolve.
--
-- A AUSÊNCIA de linha significa "usa o padrão compilado", não "bônus zero" — a
-- mesma regra de 0023, 0029, 0030 e 0035. Restaurar é apagar.
--
-- Os quatro números vão juntos, como a absorção: são uma decisão só sobre o que
-- a montaria é, e gravar metade deixaria uma linhagem que ninguém desenhou.
--
-- Sem hot-reload, como todo o overlay de montaria: o tmServer lê no boot.

CREATE TABLE IF NOT EXISTS mount_bonus (
    mount_index SMALLINT NOT NULL CHECK (mount_index BETWEEN 2360 AND 2389),
    -- Coeficientes: no nível N da montaria o dano é (N+20)*attack/100 e a magia
    -- (N+15)*magic/100 (Basedef.cpp:1623-1626). O teto é largo de propósito, só
    -- para pegar erro de digitação: a mais forte das tabelas tem 950/145.
    attack      SMALLINT NOT NULL CHECK (attack  BETWEEN 0 AND 2000),
    magic       SMALLINT NOT NULL CHECK (magic   BETWEEN 0 AND 500),
    -- Em décimos de porcento: 60 é o "6,0%" do tooltip. O teto é o do motor —
    -- a esquiva que vem de equipamento para em 100 (10%) em GetParryRate, então
    -- acima disso seria um número na tela que não muda luta nenhuma.
    evasion     SMALLINT NOT NULL CHECK (evasion BETWEEN 0 AND 100),
    -- Vale para as quatro resistências; o motor as limita a 100 (CMob.cpp:640).
    resist      SMALLINT NOT NULL CHECK (resist  BETWEEN 0 AND 100),
    updated_by  TEXT     NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (mount_index)
);
