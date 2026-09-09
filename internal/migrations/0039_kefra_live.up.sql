-- 0039_kefra_live — o terceiro interruptor de experiência vai para o painel.
--
-- KefraLive é o mais forte dos três e o único que não tinha onde ser mexido.
-- Ele não some nem multiplica: DESLIGADO, ele DIVIDE A EXPERIÊNCIA POR DOIS
-- (internal/level/expreward.go, passo 11 — `if !in.Events.KefraLive { exp /= 2 }`),
-- e desligado é o estado de hoje. Somando com o evento de novato desligado, que
-- tira mais 15%, o servidor entrega hoje 42,5% do que a fórmula calcula.
--
-- Até aqui ele só existia como opção de linha de comando (-kefra-live), então
-- mexer nele custava um redeploy — enquanto os outros dois, que moram nesta
-- mesma linha, mudam pelo painel em segundos. Não havia motivo para a diferença:
-- os três entram no mesmo lugar da conta, pelo mesmo caminho.
--
-- O padrão é FALSE porque é o valor que o legado usa (KefraLive=0) e o mesmo que
-- a opção de linha de comando já usava. Ninguém acorda com o servidor diferente
-- por causa desta migração.
--
-- ATENÇÃO ao que isto muda de precedência: a partir daqui o BANCO manda, e a
-- opção -kefra-live vira só o valor que vale antes da primeira leitura, igual ao
-- que já acontecia com -double-exp e -newbie-event. Um servidor que estivesse
-- subindo com -kefra-live=true passa a ser corrigido para o que o painel diz.
-- O tmServer avisa no log quando as duas pontas discordam, em vez de escolher
-- calado.
--
-- CORREÇÃO de uma frase que as migrações 0035 a 0038 repetem: "lido no boot,
-- como a Mesa de XP" deixou de valer. A Mesa passou a recarregar ao vivo. O que
-- continua só-no-boot é a sobreposição de template de monstro, a recompensa de
-- quest, a escada do bônus de drop e as curvas de montaria.

-- E NÃO se mexe em world_event_meta.version aqui, embora dê vontade: aquele
-- número conta quantas vezes um moderador editou, e é o que a tela usa para
-- dizer "nunca foi salvo". Empurrar por causa de mudança de esquema mentiria
-- sobre isso. Também não é preciso: o valor gravado é igual ao que o servidor já
-- estava usando, e quem roda esta migração é o dbServer subindo num deploy, que
-- reinicia o tmServer logo em seguida.

ALTER TABLE world_event_config
    ADD COLUMN kefra_live_enabled BOOLEAN NOT NULL DEFAULT FALSE;
